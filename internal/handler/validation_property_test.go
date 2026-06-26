package handler

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/earworm/vibesboard/internal/db"
	"github.com/earworm/vibesboard/internal/identity"
	"github.com/earworm/vibesboard/internal/oembed"
	"pgregory.net/rapid"
)

// setupVoteTestHandler creates a VibeHandler backed by an in-memory SQLite store for property tests.
func setupVoteTestHandler(t *rapid.T) *VibeHandler {
	store, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	oembedClient := oembed.NewClient(0)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	identityGen := identity.New([]byte("test-salt-for-handler"), 168)
	return NewVibeHandler(store, oembedClient, nil, logger, identityGen)
}

// **Validates: Requirements 3.3, 15.1, 15.2**
// Property 5: Invalid thought rejection — For any string with trimmed length < 1 or > 150,
// validation SHALL reject it.
func TestProperty5_InvalidThoughtRejection(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate either empty/whitespace-only strings or strings > 150 chars after trim
		choice := rapid.IntRange(0, 1).Draw(t, "choice")
		var input string
		if choice == 0 {
			// Generate whitespace-only string (trimmed length = 0)
			ws := rapid.StringMatching(`[\s]{0,20}`).Draw(t, "whitespace")
			input = ws
		} else {
			// Generate string > 150 chars after trimming
			core := rapid.StringMatching(`.{151,200}`).Draw(t, "longCore")
			padding := rapid.StringMatching(`\s{0,10}`).Draw(t, "padding")
			input = padding + core + padding
		}

		// If by chance it's valid, skip
		trimmed := strings.TrimSpace(input)
		if len(trimmed) >= 1 && len(trimmed) <= 150 {
			return
		}

		_, err := ValidateThought(input)
		if err == nil {
			t.Fatalf("expected rejection for input with trimmed length %d", len(trimmed))
		}
	})
}

// **Validates: Requirements 15.3**
// Property 6: Thought whitespace trimming — For any valid thought with arbitrary whitespace
// padding, stored thought SHALL equal strings.TrimSpace(input).
func TestProperty6_ThoughtWhitespaceTrimming(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a valid thought core (1-150 printable non-whitespace chars).
		// Use a character class that excludes all Unicode whitespace characters
		// that strings.TrimSpace would strip (not just regex \s).
		core := rapid.StringMatching(`[a-zA-Z0-9!@#$%^&*()\-_=+]{1,150}`).Draw(t, "core")

		// Add arbitrary whitespace padding (spaces and tabs)
		leading := rapid.StringMatching(`[ \t]{0,20}`).Draw(t, "leading")
		trailing := rapid.StringMatching(`[ \t]{0,20}`).Draw(t, "trailing")
		input := leading + core + trailing

		result, err := ValidateThought(input)
		if err != nil {
			t.Fatalf("ValidateThought(%q) failed: %v", input, err)
		}

		expected := strings.TrimSpace(input)
		if result != expected {
			t.Fatalf("expected %q, got %q", expected, result)
		}
	})
}

// **Validates: Requirements 2.2, 2.4**
// Property 3: Invalid vote input rejection — For any string that is not "up" or "down"
// used as direction, the Vote handler SHALL reject the request with HTTP 400.
func TestProperty_InvalidVoteInputRejection(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		h := setupVoteTestHandler(t)

		// Generate a random string that is NOT "up" or "down"
		direction := rapid.StringMatching(`[a-zA-Z0-9!@#$%^& ]{0,50}`).Draw(t, "direction")
		if direction == "up" || direction == "down" {
			// Skip valid directions
			return
		}

		body := `{"direction": "` + direction + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/vibes/1/vote", bytes.NewBufferString(body))
		req.SetPathValue("id", "1")
		w := httptest.NewRecorder()
		h.Vote(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for direction=%q, got %d", direction, w.Code)
		}
	})
}
