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
	"github.com/earworm/vibesboard/internal/model"
	"github.com/earworm/vibesboard/internal/oembed"
)

func setupTestHandler(t *testing.T) *VibeHandler {
	t.Helper()
	tmpFile := t.TempDir() + "/test.db"
	store, err := db.New(tmpFile)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	oembedClient := oembed.NewClient(0) // zero timeout — won't be used in most tests
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	identityGen := identity.New([]byte("test-salt-for-handler"), 168)

	return NewVibeHandler(store, oembedClient, nil, logger, identityGen)
}

func TestYouTubeURLRegex(t *testing.T) {
	valid := []string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=abc-123_XYZ",
	}
	invalid := []string{
		"",
		"http://www.youtube.com/watch?v=dQw4w9WgXcQ", // http not https
		"https://youtube.com/watch?v=dQw4w9WgXcQ",    // missing www
		"https://www.youtube.com/watch?v=",           // empty id
		"https://www.youtube.com/embed/dQw4w9WgXcQ",  // embed not accepted
		"https://vimeo.com/123456",                   // wrong site
		"not a url",
	}

	for _, u := range valid {
		if !youtubeURLRegex.MatchString(u) {
			t.Errorf("expected valid but got invalid: %s", u)
		}
	}
	for _, u := range invalid {
		if youtubeURLRegex.MatchString(u) {
			t.Errorf("expected invalid but got valid: %s", u)
		}
	}
}

func TestCreate_InvalidJSON(t *testing.T) {
	h := setupTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreate_InvalidURL(t *testing.T) {
	h := setupTestHandler(t)
	body := `{"youtube_url": "https://vimeo.com/123", "thought": "nice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreate_EmptyThought(t *testing.T) {
	h := setupTestHandler(t)
	body := `{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": "   "}`
	req := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreate_ThoughtTooLong(t *testing.T) {
	h := setupTestHandler(t)
	longThought := make([]byte, 151)
	for i := range longThought {
		longThought[i] = 'a'
	}
	body, _ := json.Marshal(createRequest{
		YouTubeURL: "https://youtu.be/dQw4w9WgXcQ",
		Thought:    string(longThought),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreate_Success(t *testing.T) {
	h := setupTestHandler(t)
	body := `{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": "love this song"}`
	req := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var vibe model.Vibe
	if err := json.Unmarshal(w.Body.Bytes(), &vibe); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if vibe.ID == 0 {
		t.Error("expected non-zero vibe ID")
	}
	if vibe.Thought != "love this song" {
		t.Errorf("expected thought 'love this song', got %q", vibe.Thought)
	}
}

func TestList_Empty(t *testing.T) {
	h := setupTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/vibes", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Should be [] not null
	body := bytes.TrimSpace(w.Body.Bytes())
	if string(body) != "[]" {
		t.Errorf("expected empty array [], got %s", body)
	}
}

func TestList_WithVibes(t *testing.T) {
	h := setupTestHandler(t)

	// Insert a vibe first
	body := `{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": "hello"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	// List
	req := httptest.NewRequest(http.MethodGet, "/api/vibes", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var vibes []model.Vibe
	if err := json.Unmarshal(w.Body.Bytes(), &vibes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(vibes) != 1 {
		t.Errorf("expected 1 vibe, got %d", len(vibes))
	}
}

func TestDelete_NotFound(t *testing.T) {
	h := setupTestHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/vibes/999", nil)
	w := httptest.NewRecorder()

	h.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDelete_InvalidID(t *testing.T) {
	h := setupTestHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/vibes/abc", nil)
	w := httptest.NewRecorder()

	h.Delete(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDelete_Success(t *testing.T) {
	h := setupTestHandler(t)

	// Insert a vibe first
	body := `{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": "delete me"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	var created model.Vibe
	json.Unmarshal(createW.Body.Bytes(), &created)

	// Delete it
	req := httptest.NewRequest(http.MethodDelete, "/api/vibes/1", nil)
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreate_OEmbedFailure_Still201(t *testing.T) {
	// Create a mock oEmbed server that always returns 500 to simulate failure.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	// Set up handler with a custom oEmbed client pointing to the mock server.
	tmpFile := t.TempDir() + "/test.db"
	store, err := db.New(tmpFile)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Create an oEmbed client with a very short timeout and an unreachable URL.
	// We use 1ns timeout to guarantee the fetch fails.
	oembedClient := oembed.NewClient(1 * time.Nanosecond)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	identityGen := identity.New([]byte("test-salt-for-handler"), 168)
	h := NewVibeHandler(store, oembedClient, nil, logger, identityGen)

	body := `{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": "oembed should fail gracefully"}`
	req := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 even with oEmbed failure, got %d: %s", w.Code, w.Body.String())
	}

	var vibe model.Vibe
	if err := json.Unmarshal(w.Body.Bytes(), &vibe); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if vibe.ID == 0 {
		t.Error("expected non-zero vibe ID")
	}
	// When oEmbed fails, metadata fields should be empty
	if vibe.VideoTitle != "" {
		t.Errorf("expected empty video_title on oEmbed failure, got %q", vibe.VideoTitle)
	}
	if vibe.ThumbnailURL != "" {
		t.Errorf("expected empty thumbnail_url on oEmbed failure, got %q", vibe.ThumbnailURL)
	}
	if vibe.Thought != "oembed should fail gracefully" {
		t.Errorf("expected thought to be stored correctly, got %q", vibe.Thought)
	}
}

// --- Task 6.2: Unit tests for Vote handler ---

func TestVote_ValidUpvote_Returns200(t *testing.T) {
	h := setupTestHandler(t)

	// Insert a vibe first
	body := `{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": "vote test"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	// Vote up using Go 1.22+ pattern
	voteBody := `{"direction": "up"}`
	req := httptest.NewRequest(http.MethodPost, "/api/vibes/1/vote", bytes.NewBufferString(voteBody))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.Vote(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var vibe model.Vibe
	if err := json.Unmarshal(w.Body.Bytes(), &vibe); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if vibe.Likes != 1 {
		t.Errorf("expected likes=1 after upvote, got %d", vibe.Likes)
	}
	if vibe.Dislikes != 0 {
		t.Errorf("expected dislikes=0 after upvote, got %d", vibe.Dislikes)
	}
}

func TestVote_ValidDownvote_Returns200(t *testing.T) {
	h := setupTestHandler(t)

	// Insert a vibe first
	body := `{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": "vote test"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	// Vote down
	voteBody := `{"direction": "down"}`
	req := httptest.NewRequest(http.MethodPost, "/api/vibes/1/vote", bytes.NewBufferString(voteBody))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.Vote(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var vibe model.Vibe
	if err := json.Unmarshal(w.Body.Bytes(), &vibe); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if vibe.Dislikes != 1 {
		t.Errorf("expected dislikes=1 after downvote, got %d", vibe.Dislikes)
	}
	if vibe.Likes != 0 {
		t.Errorf("expected likes=0 after downvote, got %d", vibe.Likes)
	}
}

func TestVote_InvalidDirection_Returns400(t *testing.T) {
	h := setupTestHandler(t)

	voteBody := `{"direction": "sideways"}`
	req := httptest.NewRequest(http.MethodPost, "/api/vibes/1/vote", bytes.NewBufferString(voteBody))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.Vote(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVote_NonNumericID_Returns400(t *testing.T) {
	h := setupTestHandler(t)

	voteBody := `{"direction": "up"}`
	req := httptest.NewRequest(http.MethodPost, "/api/vibes/abc/vote", bytes.NewBufferString(voteBody))
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()
	h.Vote(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVote_NonExistentVibe_Returns404(t *testing.T) {
	h := setupTestHandler(t)

	voteBody := `{"direction": "up"}`
	req := httptest.NewRequest(http.MethodPost, "/api/vibes/9999/vote", bytes.NewBufferString(voteBody))
	req.SetPathValue("id", "9999")
	w := httptest.NewRecorder()
	h.Vote(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestList_Ordering_NewestFirst(t *testing.T) {
	h := setupTestHandler(t)

	// Insert multiple vibes — with second-precision timestamps they may share
	// the same created_at, but ordering by (created_at DESC, id DESC) ensures
	// the most recently inserted vibe appears first.
	thoughts := []string{"first post", "second post", "third post"}
	for _, thought := range thoughts {
		body := fmt.Sprintf(`{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": %q}`, thought)
		req := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("failed to create vibe %q: %d %s", thought, w.Code, w.Body.String())
		}
	}

	// List vibes
	req := httptest.NewRequest(http.MethodGet, "/api/vibes", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var vibes []model.Vibe
	if err := json.Unmarshal(w.Body.Bytes(), &vibes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(vibes) != 3 {
		t.Fatalf("expected 3 vibes, got %d", len(vibes))
	}

	// Vibes should be ordered newest first (third, second, first)
	expectedOrder := []string{"third post", "second post", "first post"}
	for i, expected := range expectedOrder {
		if vibes[i].Thought != expected {
			t.Errorf("vibe[%d]: expected thought %q, got %q", i, expected, vibes[i].Thought)
		}
	}

	// Verify IDs are in descending order (newest has highest ID)
	for i := 0; i < len(vibes)-1; i++ {
		if vibes[i].ID <= vibes[i+1].ID {
			t.Errorf("expected descending ID order, but vibe[%d].ID=%d <= vibe[%d].ID=%d",
				i, vibes[i].ID, i+1, vibes[i+1].ID)
		}
	}
}

// --- Task 5.7: Handler integration tests for anonymous identities ---

func TestCreateVibeIncludesAuthor(t *testing.T) {
	h := setupTestHandler(t)

	body := `{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": "author test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
	// httptest.NewRequest sets RemoteAddr to "192.0.2.1:1234" by default
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var vibe model.Vibe
	if err := json.Unmarshal(w.Body.Bytes(), &vibe); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// The default RemoteAddr from httptest is "192.0.2.1:1234", so IP is "192.0.2.1"
	identityGen := identity.New([]byte("test-salt-for-handler"), 168)
	expectedAuthor := identityGen.Generate("192.0.2.1")
	if vibe.Author != expectedAuthor {
		t.Errorf("expected author %q, got %q", expectedAuthor, vibe.Author)
	}
}

func TestListVibesIncludesAuthor(t *testing.T) {
	h := setupTestHandler(t)

	// Insert a vibe via handler
	body := `{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": "list author test"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createW.Code, createW.Body.String())
	}

	// List vibes
	req := httptest.NewRequest(http.MethodGet, "/api/vibes", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var vibes []model.Vibe
	if err := json.Unmarshal(w.Body.Bytes(), &vibes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(vibes) == 0 {
		t.Fatal("expected at least 1 vibe in list")
	}

	// Author should be persisted and returned in list response
	for i, v := range vibes {
		if v.Author == "" {
			t.Errorf("vibe[%d]: expected non-empty author in list response", i)
		}
	}
}

func TestXForwardedForUsed(t *testing.T) {
	h := setupTestHandler(t)

	body := `{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": "xff test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
	req.Header.Set("X-Forwarded-For", "10.0.0.42, 172.16.0.1")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var vibe model.Vibe
	if err := json.Unmarshal(w.Body.Bytes(), &vibe); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// ExtractIP takes the first IP from X-Forwarded-For
	identityGen := identity.New([]byte("test-salt-for-handler"), 168)
	expectedAuthor := identityGen.Generate("10.0.0.42")
	if vibe.Author != expectedAuthor {
		t.Errorf("expected author %q (from XFF IP 10.0.0.42), got %q", expectedAuthor, vibe.Author)
	}
}

func TestRemoteAddrFallback(t *testing.T) {
	h := setupTestHandler(t)

	body := `{"youtube_url": "https://youtu.be/dQw4w9WgXcQ", "thought": "remoteaddr test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/vibes", bytes.NewBufferString(body))
	// No X-Forwarded-For header — explicitly set RemoteAddr with port
	req.RemoteAddr = "192.168.1.100:54321"
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var vibe model.Vibe
	if err := json.Unmarshal(w.Body.Bytes(), &vibe); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// ExtractIP strips port from RemoteAddr: "192.168.1.100:54321" → "192.168.1.100"
	identityGen := identity.New([]byte("test-salt-for-handler"), 168)
	expectedAuthor := identityGen.Generate("192.168.1.100")
	if vibe.Author != expectedAuthor {
		t.Errorf("expected author %q (from RemoteAddr 192.168.1.100), got %q", expectedAuthor, vibe.Author)
	}
}
