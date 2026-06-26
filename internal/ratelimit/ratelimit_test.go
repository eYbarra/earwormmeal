package ratelimit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func echoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func newTestLimiter(capacity int) *Limiter {
	return New(map[string]Config{
		"create": {Rate: 5.0 / 60.0, Capacity: capacity},
		"vote":   {Rate: 30.0 / 60.0, Capacity: capacity},
		"list":   {Rate: 60.0 / 60.0, Capacity: capacity},
	})
}

func TestTokenConsumption(t *testing.T) {
	limiter := newTestLimiter(3)
	defer limiter.Stop()

	handler := limiter.Handler(echoHandler())
	server := httptest.NewServer(handler)
	defer server.Close()

	for i := 0; i < 3; i++ {
		resp, err := http.Post(server.URL+"/api/vibes", "application/json", nil)
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("request %d: got status %d, want %d", i+1, resp.StatusCode, http.StatusOK)
		}
	}

	// 4th request should be rejected
	resp, err := http.Post(server.URL+"/api/vibes", "application/json", nil)
	if err != nil {
		t.Fatalf("4th request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("4th request: got status %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
}

func TestResponse429Format(t *testing.T) {
	limiter := newTestLimiter(1)
	defer limiter.Stop()

	handler := limiter.Handler(echoHandler())
	server := httptest.NewServer(handler)
	defer server.Close()

	// Exhaust the single token.
	resp, err := http.Post(server.URL+"/api/vibes", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// This request should be rate limited.
	resp, err = http.Post(server.URL+"/api/vibes", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("got status %d, want 429", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Error("JSON body missing 'error' key")
	}
}

func TestRateLimitHeadersOnAllowed(t *testing.T) {
	limiter := newTestLimiter(5)
	defer limiter.Stop()

	handler := limiter.Handler(echoHandler())
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Post(server.URL+"/api/vibes", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	requiredHeaders := []string{"X-Ratelimit-Limit", "X-Ratelimit-Remaining", "X-Ratelimit-Reset"}
	for _, h := range requiredHeaders {
		if resp.Header.Get(h) == "" {
			t.Errorf("missing header %s on allowed response", h)
		}
	}
}

func TestRetryAfterHeaderOn429(t *testing.T) {
	limiter := newTestLimiter(1)
	defer limiter.Stop()

	handler := limiter.Handler(echoHandler())
	server := httptest.NewServer(handler)
	defer server.Close()

	// Exhaust token.
	resp, _ := http.Post(server.URL+"/api/vibes", "application/json", nil)
	resp.Body.Close()

	// Trigger 429.
	resp, err := http.Post(server.URL+"/api/vibes", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.Header.Get("Retry-After") == "" {
		t.Error("missing Retry-After header on 429 response")
	}
}

func TestWebSocketPassthrough(t *testing.T) {
	limiter := newTestLimiter(1)
	defer limiter.Stop()

	handler := limiter.Handler(echoHandler())

	req := httptest.NewRequest("GET", "/ws", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("WebSocket request got status %d, want %d", w.Code, http.StatusOK)
	}

	// WebSocket requests should NOT have rate limit headers.
	rateLimitHeaders := []string{"X-Ratelimit-Limit", "X-Ratelimit-Remaining", "X-Ratelimit-Reset"}
	for _, h := range rateLimitHeaders {
		if w.Header().Get(h) != "" {
			t.Errorf("WebSocket request should not have header %s, got %q", h, w.Header().Get(h))
		}
	}
}

func TestStopDoesNotPanic(t *testing.T) {
	limiter := newTestLimiter(5)

	// Should not panic.
	limiter.Stop()
}
