package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/earworm/vibesboard/internal/db"
	"github.com/earworm/vibesboard/internal/identity"
	"github.com/earworm/vibesboard/internal/oembed"
	"pgregory.net/rapid"
)

// Feature: anonymous-identities, Property 5: Author field correctness on create
// **Validates: Requirements 3.2**
//
// For any valid creation request from IP `ip`, the response "author" field
// equals identity.Generate(ip).
func TestProperty5_AuthorFieldCorrectness(t *testing.T) {
	// Set up test dependencies once (shared across iterations).
	tmpDir := t.TempDir()
	store, err := db.New(tmpDir + "/prop5.db")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Use a mock oEmbed server that returns minimal valid metadata.
	mockOEmbed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"title":         "Test Video",
			"thumbnail_url": "https://i.ytimg.com/vi/test/hqdefault.jpg",
		})
	}))
	t.Cleanup(mockOEmbed.Close)

	oembedClient := oembed.NewClientWithBaseURL(5*time.Second, mockOEmbed.URL)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	identityGen := identity.New([]byte("test-salt-for-handler"), 168)
	h := NewVibeHandler(store, oembedClient, nil, logger, identityGen)

	rapid.Check(t, func(t *rapid.T) {
		// Generate a random valid IPv4 address.
		ip := fmt.Sprintf("%d.%d.%d.%d",
			rapid.IntRange(1, 254).Draw(t, "octet1"),
			rapid.IntRange(0, 255).Draw(t, "octet2"),
			rapid.IntRange(0, 255).Draw(t, "octet3"),
			rapid.IntRange(1, 254).Draw(t, "octet4"),
		)

		// Build a valid create request with the generated IP as RemoteAddr.
		body := `{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": "property test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
		req.RemoteAddr = ip + ":12345"

		w := httptest.NewRecorder()
		h.Create(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
		}

		// Decode the response to extract the author field.
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response JSON: %v", err)
		}

		author, ok := resp["author"]
		if !ok {
			t.Fatal("response is missing 'author' field")
		}

		authorStr, ok := author.(string)
		if !ok {
			t.Fatalf("'author' field is not a string: %v", author)
		}

		// The expected author is the identity generated from the IP.
		expected := identityGen.Generate(ip)
		if authorStr != expected {
			t.Fatalf("for IP %q: expected author %q, got %q", ip, expected, authorStr)
		}
	})
}
