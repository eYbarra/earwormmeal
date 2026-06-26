package handler

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/earworm/vibesboard/internal/db"
	"github.com/earworm/vibesboard/internal/identity"
	"github.com/earworm/vibesboard/internal/oembed"
	"pgregory.net/rapid"
)

// Feature: anonymous-identities, Property 4: Privacy (no IP leakage in API response)

// TestProperty4_NoIPLeakage verifies that for any valid creation request with a given
// client IP, the JSON response body does not contain the raw IP address string or its
// numeric FNV-1a hash value.
// **Validates: Requirements 2.3**
func TestProperty4_NoIPLeakage(t *testing.T) {
	// Set up handler with a mock oEmbed server that returns valid metadata.
	mockOEmbed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"title":"Test Video","thumbnail_url":"https://i.ytimg.com/vi/test/hqdefault.jpg"}`)
	}))
	defer mockOEmbed.Close()

	tmpFile := t.TempDir() + "/test.db"
	store, err := db.New(tmpFile)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	oembedClient := oembed.NewClientWithBaseURL(5*time.Second, mockOEmbed.URL)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	identityGen := identity.New([]byte("test-salt-for-handler"), 168)
	h := NewVibeHandler(store, oembedClient, nil, logger, identityGen)

	rapid.Check(t, func(t *rapid.T) {
		// Generate a random IPv4 address.
		ip := fmt.Sprintf("%d.%d.%d.%d",
			rapid.IntRange(1, 255).Draw(t, "octet1"),
			rapid.IntRange(0, 255).Draw(t, "octet2"),
			rapid.IntRange(0, 255).Draw(t, "octet3"),
			rapid.IntRange(1, 255).Draw(t, "octet4"),
		)

		// Create a valid vibe creation request.
		body := `{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": "testing privacy"}`
		req := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
		req.RemoteAddr = ip + ":12345"
		w := httptest.NewRecorder()

		h.Create(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
		}

		responseBody := w.Body.String()

		// Assert the raw IP string does not appear in the response body.
		if strings.Contains(responseBody, ip) {
			t.Fatalf("response body contains raw IP %q: %s", ip, responseBody)
		}

		// Compute FNV-1a hash of the IP and assert it does not appear in the response.
		hasher := fnv.New64a()
		hasher.Write([]byte(ip))
		hashValue := hasher.Sum64()
		hashStr := strconv.FormatUint(hashValue, 10)

		if strings.Contains(responseBody, hashStr) {
			t.Fatalf("response body contains FNV-1a hash %q of IP %q: %s", hashStr, ip, responseBody)
		}
	})
}
