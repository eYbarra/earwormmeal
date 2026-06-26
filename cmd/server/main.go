package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/earworm/vibesboard/internal/config"
	"github.com/earworm/vibesboard/internal/db"
	"github.com/earworm/vibesboard/internal/handler"
	"github.com/earworm/vibesboard/internal/hub"
	"github.com/earworm/vibesboard/internal/identity"
	"github.com/earworm/vibesboard/internal/oembed"
	"github.com/earworm/vibesboard/internal/ratelimit"
)

func main() {
	// Structured JSON logger writing to stdout.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg := config.Load()

	// Open SQLite database.
	store, err := db.New(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}

	// Create oEmbed client with 5-second timeout.
	oembedClient := oembed.NewClient(5 * time.Second)

	// Create WebSocket hub and start its event loop.
	h := hub.New()
	go h.Run()

	// Create identity generator.
	identityGen := identity.New(cfg.IdentitySalt, cfg.IdentityRotationHours)

	// Create handlers.
	vibeHandler := handler.NewVibeHandler(store, oembedClient, h, logger, identityGen)
	wsHandler := handler.NewWSHandler(h, logger)

	// Register routes.
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/vibes", vibeHandler.Create)
	mux.HandleFunc("GET /api/vibes", vibeHandler.List)
	mux.HandleFunc("POST /api/vibes/{id}/vote", vibeHandler.Vote)
	// DELETE endpoint disabled in production — no auth, too dangerous to expose.
	// mux.HandleFunc("/api/vibes/", vibeHandler.Delete)
	mux.Handle("GET /ws", wsHandler)
	mux.Handle("/", http.FileServer(http.Dir("web")))

	// Wrap with logging middleware.
	rateLimiter := ratelimit.New(map[string]ratelimit.Config{
		"create": {Rate: 5.0 / 60.0, Capacity: 5},
		"vote":   {Rate: 30.0 / 60.0, Capacity: 30},
		"list":   {Rate: 60.0 / 60.0, Capacity: 60},
	})
	logged := loggingMiddleware(logger, rateLimiter.Handler(mux))

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: logged,
	}

	// Start server in a goroutine.
	go func() {
		logger.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server listen error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown: wait for SIGINT or SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("shutdown signal received", "signal", sig.String())

	// Phase 1: drain HTTP requests with a 10-second timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("http server shutdown error", "error", err)
	}

	// Phase 2: stop rate limiter cleanup goroutine.
	rateLimiter.Stop()

	// Phase 3: close all WebSocket connections.
	h.Shutdown()

	// Phase 4: close database connection.
	if err := store.Close(); err != nil {
		logger.Error("database close error", "error", err)
	}

	logger.Info("server stopped")
}

// responseWriter wraps http.ResponseWriter to capture the status code.
// It also implements http.Hijacker so WebSocket upgrades work through the middleware.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker, required for WebSocket upgrades.
// The gorilla/websocket Upgrader calls Hijack() to take over the raw connection.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
}

// loggingMiddleware logs method, path, status code, and duration for every request.
func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
