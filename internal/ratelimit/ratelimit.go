package ratelimit

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Config holds rate limit settings for a single endpoint category.
type Config struct {
	Rate     float64 // Tokens added per second (e.g., 5/60 = 0.0833)
	Capacity int     // Maximum token count (burst size)
}

// Limiter is the HTTP middleware that enforces per-IP rate limits.
type Limiter struct {
	configs     map[string]Config // keyed by category: "create", "vote", "list"
	buckets     sync.Map          // key: "ip:category" → *bucket
	cleanupStop chan struct{}
}

// bucket represents a single token bucket for one IP+category pair.
type bucket struct {
	tokens   float64
	capacity int
	rate     float64   // tokens per second
	lastTime time.Time // last time tokens were calculated
	lastSeen time.Time // last request time (for cleanup)
	mu       sync.Mutex
}

// New creates a Limiter with the given category configs and starts
// the background cleanup goroutine.
func New(configs map[string]Config) *Limiter {
	l := &Limiter{
		configs:     configs,
		cleanupStop: make(chan struct{}),
	}
	go l.cleanup()
	return l
}

// Stop signals the cleanup goroutine to exit.
func (l *Limiter) Stop() {
	close(l.cleanupStop)
}

// cleanup runs every 5 minutes and removes buckets that haven't been
// seen in more than 10 minutes.
func (l *Limiter) cleanup() {
	defer func() { recover() }()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-l.cleanupStop:
			return
		case <-ticker.C:
			now := time.Now()
			l.buckets.Range(func(key, value any) bool {
				b := value.(*bucket)
				b.mu.Lock()
				stale := now.Sub(b.lastSeen) > 10*time.Minute
				b.mu.Unlock()
				if stale {
					l.buckets.Delete(key)
				}
				return true
			})
		}
	}
}

// consume attempts to take one token from the bucket.
// It refills tokens based on elapsed time since the last request,
// caps at capacity, then tries to consume 1 token.
// Returns whether the request is allowed and the remaining token count.
func (b *bucket) consume() (bool, float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > float64(b.capacity) {
		b.tokens = float64(b.capacity)
	}
	b.lastTime = now
	b.lastSeen = now

	if b.tokens < 1 {
		return false, b.tokens
	}
	b.tokens--
	return true, b.tokens
}

// getOrCreateBucket returns the bucket for the given key and category,
// creating one at full capacity if it doesn't already exist.
func (l *Limiter) getOrCreateBucket(key string, category string) *bucket {
	cfg := l.configs[category]

	val, _ := l.buckets.LoadOrStore(key, &bucket{
		tokens:   float64(cfg.Capacity),
		capacity: cfg.Capacity,
		rate:     cfg.Rate,
		lastTime: time.Now(),
		lastSeen: time.Now(),
	})

	return val.(*bucket)
}

// Handler wraps an http.Handler with rate limiting middleware.
// Requests to exempt endpoints pass through unchanged.
// Rate-limited requests receive standard rate limit headers.
// Rejected requests receive a 429 response with Retry-After and a JSON body.
func (l *Limiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		category := Classify(r.Method, r.URL.Path)
		if category == "" {
			next.ServeHTTP(w, r)
			return
		}

		ip := ExtractIP(r)
		key := ip + ":" + category
		b := l.getOrCreateBucket(key, category)

		allowed, remaining := b.consume()
		cfg := l.configs[category]

		// Always add rate limit headers.
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(cfg.Capacity))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(int(remaining)))
		resetSeconds := float64(cfg.Capacity-int(remaining)) / cfg.Rate
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Duration(resetSeconds*float64(time.Second))).Unix(), 10))

		if !allowed {
			retryAfter := int(math.Ceil(1.0 / cfg.Rate))
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}

		next.ServeHTTP(w, r)
	})
}
