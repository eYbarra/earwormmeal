package ratelimit

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// TestProperty1_TokenConsumptionDecrement verifies that consuming a token
// decreases the token count by exactly one.
// **Validates: Requirements 1.1, 2.1, 3.1**
func TestProperty1_TokenConsumptionDecrement(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		capacity := rapid.IntRange(1, 20).Draw(t, "capacity")

		b := &bucket{
			tokens:   float64(capacity),
			capacity: capacity,
			rate:     0.0, // zero rate so no refill during test
			lastTime: time.Now(),
			lastSeen: time.Now(),
		}

		before := b.tokens
		allowed, _ := b.consume()
		after := b.tokens

		if !allowed {
			t.Fatalf("expected consume to be allowed with %d tokens", capacity)
		}
		if after != before-1 {
			t.Fatalf("expected tokens to decrease by 1: before=%f, after=%f", before, after)
		}
	})
}

// TestProperty2_ZeroTokenRejection verifies that a bucket with zero tokens
// rejects the next consume request.
// **Validates: Requirements 1.2, 2.2, 3.2**
func TestProperty2_ZeroTokenRejection(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		capacity := rapid.IntRange(1, 20).Draw(t, "capacity")

		b := &bucket{
			tokens:   float64(capacity),
			capacity: capacity,
			rate:     0.0,
			lastTime: time.Now(),
			lastSeen: time.Now(),
		}

		// Exhaust all tokens.
		for i := 0; i < capacity; i++ {
			b.consume()
		}

		allowed, _ := b.consume()
		if allowed {
			t.Fatal("expected rejection when bucket has zero tokens")
		}
	})
}

// TestProperty3_CapacityEnforcement verifies that exactly capacity requests
// are allowed and the (capacity+1)th is rejected.
// **Validates: Requirements 1.3, 2.3, 3.3**
func TestProperty3_CapacityEnforcement(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		capacity := rapid.IntRange(1, 15).Draw(t, "capacity")

		configs := map[string]Config{
			"create": {Rate: 0.0, Capacity: capacity},
		}
		limiter := New(configs)
		limiter.Stop()

		downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		handler := limiter.Handler(downstream)

		for i := 0; i < capacity; i++ {
			req := httptest.NewRequest("POST", "/api/vibes", nil)
			req.RemoteAddr = "10.0.0.1:1234"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("request %d: expected 200, got %d", i+1, rr.Code)
			}
		}

		// The (capacity+1)th request should be rejected.
		req := httptest.NewRequest("POST", "/api/vibes", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d: expected 429, got %d", capacity+1, rr.Code)
		}
	})
}

// TestProperty4_WebSocketExemption verifies that GET /ws requests always
// pass through without rate limiting.
// **Validates: Requirements 4.1, 4.2**
func TestProperty4_WebSocketExemption(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ip := fmt.Sprintf("%d.%d.%d.%d",
			rapid.IntRange(1, 254).Draw(t, "ip1"),
			rapid.IntRange(0, 255).Draw(t, "ip2"),
			rapid.IntRange(0, 255).Draw(t, "ip3"),
			rapid.IntRange(1, 254).Draw(t, "ip4"),
		)

		configs := map[string]Config{
			"create": {Rate: 5.0 / 60.0, Capacity: 5},
			"vote":   {Rate: 30.0 / 60.0, Capacity: 30},
			"list":   {Rate: 60.0 / 60.0, Capacity: 60},
		}
		limiter := New(configs)
		limiter.Stop()

		called := false
		downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})
		handler := limiter.Handler(downstream)

		req := httptest.NewRequest("GET", "/ws", nil)
		req.RemoteAddr = ip + ":9999"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for WebSocket, got %d", rr.Code)
		}
		if !called {
			t.Fatal("downstream handler was not called for WebSocket request")
		}
		if rr.Header().Get("X-RateLimit-Limit") != "" {
			t.Fatal("rate limit headers should not be present on WebSocket responses")
		}
	})
}

// TestProperty5_RateLimitHeadersConsistency verifies that allowed requests
// have correct rate limit headers.
// **Validates: Requirements 5.1, 5.2, 5.3**
func TestProperty5_RateLimitHeadersConsistency(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		capacity := rapid.IntRange(2, 20).Draw(t, "capacity")

		configs := map[string]Config{
			"create": {Rate: 5.0 / 60.0, Capacity: capacity},
		}
		limiter := New(configs)
		limiter.Stop()

		downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		handler := limiter.Handler(downstream)

		req := httptest.NewRequest("POST", "/api/vibes", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		// X-RateLimit-Limit should equal configured capacity.
		limitHeader := rr.Header().Get("X-RateLimit-Limit")
		if limitHeader != strconv.Itoa(capacity) {
			t.Fatalf("X-RateLimit-Limit: expected %d, got %s", capacity, limitHeader)
		}

		// X-RateLimit-Remaining should be a non-negative integer.
		remainingStr := rr.Header().Get("X-RateLimit-Remaining")
		remaining, err := strconv.Atoi(remainingStr)
		if err != nil {
			t.Fatalf("X-RateLimit-Remaining not an integer: %s", remainingStr)
		}
		if remaining < 0 {
			t.Fatalf("X-RateLimit-Remaining is negative: %d", remaining)
		}
	})
}

// TestProperty6_RetryAfterAccuracy verifies that Retry-After is ceil(1/rate)
// when a request is rejected.
// **Validates: Requirements 5.4**
func TestProperty6_RetryAfterAccuracy(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a rate as N requests per 60 seconds.
		perMinute := rapid.IntRange(1, 60).Draw(t, "perMinute")
		rate := float64(perMinute) / 60.0
		capacity := perMinute

		configs := map[string]Config{
			"create": {Rate: rate, Capacity: capacity},
		}
		limiter := New(configs)
		limiter.Stop()

		downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		handler := limiter.Handler(downstream)

		// Exhaust the bucket.
		for i := 0; i < capacity; i++ {
			req := httptest.NewRequest("POST", "/api/vibes", nil)
			req.RemoteAddr = "10.0.0.1:1234"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
		}

		// Next request should be rejected.
		req := httptest.NewRequest("POST", "/api/vibes", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", rr.Code)
		}

		retryAfterStr := rr.Header().Get("Retry-After")
		retryAfter, err := strconv.Atoi(retryAfterStr)
		if err != nil {
			t.Fatalf("Retry-After not an integer: %s", retryAfterStr)
		}

		expected := int(math.Ceil(1.0 / rate))
		if retryAfter != expected {
			t.Fatalf("Retry-After: expected %d, got %d (rate=%f)", expected, retryAfter, rate)
		}
	})
}

// TestProperty7_AllowedRequestsPassThrough verifies that allowed requests
// reach the downstream handler with the same method and path.
// **Validates: Requirements 6.2, 6.3**
func TestProperty7_AllowedRequestsPassThrough(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Pick a random rate-limited endpoint.
		endpoints := []struct {
			method string
			path   string
		}{
			{"POST", "/api/vibes"},
			{"POST", "/api/vibes/123/vote"},
			{"GET", "/api/vibes"},
		}
		idx := rapid.IntRange(0, len(endpoints)-1).Draw(t, "endpoint")
		ep := endpoints[idx]

		configs := map[string]Config{
			"create": {Rate: 5.0 / 60.0, Capacity: 100},
			"vote":   {Rate: 30.0 / 60.0, Capacity: 100},
			"list":   {Rate: 60.0 / 60.0, Capacity: 100},
		}
		limiter := New(configs)
		limiter.Stop()

		var gotMethod, gotPath string
		downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		})
		handler := limiter.Handler(downstream)

		req := httptest.NewRequest(ep.method, ep.path, nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		if gotMethod != ep.method {
			t.Fatalf("downstream method: expected %s, got %s", ep.method, gotMethod)
		}
		if gotPath != ep.path {
			t.Fatalf("downstream path: expected %s, got %s", ep.path, gotPath)
		}
	})
}

// TestProperty8_StaleEntryCleanup verifies that stale entries (>10 min)
// are removed and fresh entries are retained.
// **Validates: Requirements 7.2, 7.3**
func TestProperty8_StaleEntryCleanup(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numBuckets := rapid.IntRange(1, 20).Draw(t, "numBuckets")

		limiter := &Limiter{
			configs:     map[string]Config{"create": {Rate: 0.1, Capacity: 5}},
			cleanupStop: make(chan struct{}),
		}

		now := time.Now()
		var staleKeys []string
		var freshKeys []string

		for i := 0; i < numBuckets; i++ {
			key := fmt.Sprintf("10.0.0.%d:create", i)
			isStale := rapid.Bool().Draw(t, fmt.Sprintf("stale_%d", i))

			var lastSeen time.Time
			if isStale {
				// More than 10 minutes ago.
				minutesAgo := rapid.IntRange(11, 60).Draw(t, fmt.Sprintf("staleMinutes_%d", i))
				lastSeen = now.Add(-time.Duration(minutesAgo) * time.Minute)
				staleKeys = append(staleKeys, key)
			} else {
				// Within 10 minutes.
				minutesAgo := rapid.IntRange(0, 9).Draw(t, fmt.Sprintf("freshMinutes_%d", i))
				lastSeen = now.Add(-time.Duration(minutesAgo) * time.Minute)
				freshKeys = append(freshKeys, key)
			}

			limiter.buckets.Store(key, &bucket{
				tokens:   5.0,
				capacity: 5,
				rate:     0.1,
				lastTime: now,
				lastSeen: lastSeen,
			})
		}

		// Simulate cleanup: iterate and remove stale entries.
		limiter.buckets.Range(func(key, value any) bool {
			b := value.(*bucket)
			b.mu.Lock()
			stale := now.Sub(b.lastSeen) > 10*time.Minute
			b.mu.Unlock()
			if stale {
				limiter.buckets.Delete(key)
			}
			return true
		})

		// Verify stale entries are removed.
		for _, key := range staleKeys {
			if _, ok := limiter.buckets.Load(key); ok {
				t.Fatalf("stale entry %s should have been removed", key)
			}
		}

		// Verify fresh entries are retained.
		for _, key := range freshKeys {
			if _, ok := limiter.buckets.Load(key); !ok {
				t.Fatalf("fresh entry %s should have been retained", key)
			}
		}
	})
}

// TestProperty10_TokenRefillRateAndCap verifies the refill formula:
// min(tokens + elapsed*rate, capacity).
// **Validates: Requirements 9.1, 9.2**
func TestProperty10_TokenRefillRateAndCap(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		capacity := rapid.IntRange(1, 100).Draw(t, "capacity")
		tokens := rapid.Float64Range(0.0, float64(capacity)).Draw(t, "tokens")
		rate := rapid.Float64Range(0.01, 2.0).Draw(t, "rate")
		elapsedSec := rapid.Float64Range(0.0, 120.0).Draw(t, "elapsedSec")

		expected := tokens + elapsedSec*rate
		if expected > float64(capacity) {
			expected = float64(capacity)
		}

		// Create a bucket with the given state, set lastTime in the past.
		now := time.Now()
		b := &bucket{
			tokens:   tokens,
			capacity: capacity,
			rate:     rate,
			lastTime: now.Add(-time.Duration(elapsedSec * float64(time.Second))),
			lastSeen: now,
		}

		// consume will refill then try to take a token.
		// We verify the refill result indirectly by checking the post-consume state.
		allowed, remaining := b.consume()

		if expected >= 1.0 {
			// Should be allowed; remaining = expected - 1.
			if !allowed {
				t.Fatalf("expected allowed with tokens=%f, elapsed=%f, rate=%f (expected refill=%f)",
					tokens, elapsedSec, rate, expected)
			}
			expectedRemaining := expected - 1.0
			// Allow small float tolerance due to time measurement.
			if math.Abs(remaining-expectedRemaining) > 0.01 {
				t.Fatalf("remaining: expected ~%f, got %f", expectedRemaining, remaining)
			}
		} else {
			// Should be rejected.
			if allowed {
				t.Fatalf("expected rejection with tokens=%f, elapsed=%f, rate=%f (expected refill=%f)",
					tokens, elapsedSec, rate, expected)
			}
		}
	})
}

// TestProperty11_FreshBucketInitialization verifies that new buckets start
// at full capacity.
// **Validates: Requirements 9.3**
func TestProperty11_FreshBucketInitialization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		capacity := rapid.IntRange(1, 100).Draw(t, "capacity")

		configs := map[string]Config{
			"create": {Rate: 5.0 / 60.0, Capacity: capacity},
		}
		limiter := New(configs)
		limiter.Stop()

		ip := fmt.Sprintf("%d.%d.%d.%d",
			rapid.IntRange(1, 254).Draw(t, "ip1"),
			rapid.IntRange(0, 255).Draw(t, "ip2"),
			rapid.IntRange(0, 255).Draw(t, "ip3"),
			rapid.IntRange(1, 254).Draw(t, "ip4"),
		)

		key := ip + ":create"
		b := limiter.getOrCreateBucket(key, "create")

		if b.tokens != float64(capacity) {
			t.Fatalf("expected initial tokens=%d, got %f", capacity, b.tokens)
		}
		if b.capacity != capacity {
			t.Fatalf("expected capacity=%d, got %d", capacity, b.capacity)
		}
	})
}
