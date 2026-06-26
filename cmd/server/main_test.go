package main_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/earworm/vibesboard/internal/db"
	"github.com/earworm/vibesboard/internal/handler"
	"github.com/earworm/vibesboard/internal/hub"
	"github.com/earworm/vibesboard/internal/identity"
	"github.com/earworm/vibesboard/internal/model"
	"github.com/earworm/vibesboard/internal/oembed"
	"github.com/gorilla/websocket"
)

// setupTestServer creates the full server stack with an in-memory SQLite database
// and returns the test server plus a cleanup function.
func setupTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory store: %v", err)
	}

	// Use a very short timeout — oEmbed calls will fail, which is fine for integration tests.
	oembedClient := oembed.NewClient(1 * time.Nanosecond)

	h := hub.New()
	go h.Run()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	identityGen := identity.New([]byte("test-salt-for-handler"), 168)
	vibeHandler := handler.NewVibeHandler(store, oembedClient, h, logger, identityGen)
	wsHandler := handler.NewWSHandler(h, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/vibes", vibeHandler.Create)
	mux.HandleFunc("GET /api/vibes", vibeHandler.List)
	mux.HandleFunc("/api/vibes/", vibeHandler.Delete)
	mux.Handle("GET /ws", wsHandler)

	srv := httptest.NewServer(mux)
	cleanup := func() {
		srv.Close()
		h.Shutdown()
		store.Close()
	}
	return srv, cleanup
}

// TestPostGetRoundtrip verifies that POSTing a vibe and then GETting the list returns it.
func TestPostGetRoundtrip(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// POST a valid vibe.
	body := `{"youtube_url":"https://www.youtube.com/watch?v=dQw4w9WgXcQ","thought":"Never gonna give you up"}`
	resp, err := http.Post(srv.URL+"/api/vibes", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/vibes failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}

	var created model.Vibe
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode POST response: %v", err)
	}

	// Verify the created vibe has all fields populated.
	if created.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if created.YouTubeURL != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
		t.Errorf("unexpected youtube_url: %s", created.YouTubeURL)
	}
	if created.Thought != "Never gonna give you up" {
		t.Errorf("unexpected thought: %s", created.Thought)
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}

	// GET the vibes list.
	resp2, err := http.Get(srv.URL + "/api/vibes")
	if err != nil {
		t.Fatalf("GET /api/vibes failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	var vibes []model.Vibe
	if err := json.NewDecoder(resp2.Body).Decode(&vibes); err != nil {
		t.Fatalf("failed to decode GET response: %v", err)
	}

	if len(vibes) != 1 {
		t.Fatalf("expected 1 vibe, got %d", len(vibes))
	}

	got := vibes[0]
	if got.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, got.ID)
	}
	if got.YouTubeURL != created.YouTubeURL {
		t.Errorf("expected youtube_url %q, got %q", created.YouTubeURL, got.YouTubeURL)
	}
	if got.Thought != created.Thought {
		t.Errorf("expected thought %q, got %q", created.Thought, got.Thought)
	}
}

// TestWebSocketLifecycle tests connecting via WebSocket, receiving broadcasts, and disconnecting.
func TestWebSocketLifecycle(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Connect to /ws via WebSocket.
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close()

	// We should receive a connected_count message on connect.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read connected_count message: %v", err)
	}

	var countMsg map[string]interface{}
	if err := json.Unmarshal(msg, &countMsg); err != nil {
		t.Fatalf("failed to unmarshal connected_count: %v", err)
	}
	if countMsg["type"] != "connected_count" {
		t.Errorf("expected type connected_count, got %v", countMsg["type"])
	}
	if countMsg["payload"].(float64) != 1 {
		t.Errorf("expected connected_count payload 1, got %v", countMsg["payload"])
	}

	// POST a new vibe through the REST API.
	body := `{"youtube_url":"https://youtu.be/dQw4w9WgXcQ","thought":"Testing WS broadcast"}`
	resp, err := http.Post(srv.URL+"/api/vibes", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/vibes failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	// Read the new_vibe broadcast from WebSocket.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read new_vibe message: %v", err)
	}

	var wsMsg model.WSMessage
	if err := json.Unmarshal(msg, &wsMsg); err != nil {
		t.Fatalf("failed to unmarshal WS message: %v", err)
	}
	if wsMsg.Type != "new_vibe" {
		t.Errorf("expected type new_vibe, got %s", wsMsg.Type)
	}

	// Verify payload contains the vibe data.
	payload, ok := wsMsg.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected payload to be a map, got %T", wsMsg.Payload)
	}
	if payload["thought"] != "Testing WS broadcast" {
		t.Errorf("expected thought 'Testing WS broadcast', got %v", payload["thought"])
	}

	// Close the WebSocket connection gracefully.
	err = conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	if err != nil {
		t.Errorf("failed to send close message: %v", err)
	}
}

// TestGracefulShutdown verifies that the server stops accepting new connections after shutdown
// while completing in-flight requests.
func TestGracefulShutdown(t *testing.T) {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory store: %v", err)
	}

	oembedClient := oembed.NewClient(1 * time.Nanosecond)
	h := hub.New()
	go h.Run()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	identityGen := identity.New([]byte("test-salt-for-handler"), 168)
	vibeHandler := handler.NewVibeHandler(store, oembedClient, h, logger, identityGen)

	// Add a slow handler to test in-flight request completion.
	slowDone := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/vibes", vibeHandler.Create)
	mux.HandleFunc("GET /api/vibes", vibeHandler.List)
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow request — wait until context is canceled or timeout.
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("done"))
		close(slowDone)
	})

	// Use a real http.Server so we can call Shutdown().
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	srvAddr := "http://" + ln.Addr().String()

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)

	// Verify server is working with a quick request.
	resp, err := http.Get(srvAddr + "/api/vibes")
	if err != nil {
		t.Fatalf("GET /api/vibes failed before shutdown: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 before shutdown, got %d", resp.StatusCode)
	}

	// Start a slow request in a goroutine.
	go func() {
		http.Get(srvAddr + "/slow")
	}()

	// Give the slow request time to start being handled.
	time.Sleep(100 * time.Millisecond)

	// Initiate graceful shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("server shutdown failed: %v", err)
	}

	// After shutdown returns, the slow request should have completed.
	select {
	case <-slowDone:
		// Good: in-flight request completed during graceful shutdown.
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for slow request to complete")
	}

	// Verify the server no longer accepts new connections.
	client := &http.Client{Timeout: 2 * time.Second}
	_, err = client.Get(srvAddr + "/api/vibes")
	if err == nil {
		t.Error("expected error after shutdown, but request succeeded")
	}

	// Cleanup.
	h.Shutdown()
	store.Close()
}
