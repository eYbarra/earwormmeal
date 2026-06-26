package db

import (
	"testing"
	"time"

	"github.com/earworm/vibesboard/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestNew_CreatesSchema(t *testing.T) {
	store := newTestStore(t)

	// Verify table exists by inserting a row.
	v := &model.Vibe{YouTubeURL: "https://youtube.com/watch?v=abc", Thought: "test"}
	if err := store.Insert(v); err != nil {
		t.Fatalf("insert after schema creation failed: %v", err)
	}
}

func TestNew_IdempotentSchema(t *testing.T) {
	store := newTestStore(t)

	// Calling New again on the same DB should not error (CREATE TABLE IF NOT EXISTS).
	// We simulate by running the create statement directly.
	if _, err := store.db.Exec(createTableSQL); err != nil {
		t.Fatalf("idempotent schema creation failed: %v", err)
	}
}

func TestInsert_PopulatesIDAndCreatedAt(t *testing.T) {
	store := newTestStore(t)

	v := &model.Vibe{
		YouTubeURL:   "https://youtube.com/watch?v=xyz",
		VideoTitle:   "Cool Song",
		ThumbnailURL: "https://img.youtube.com/vi/xyz/0.jpg",
		Thought:      "This slaps",
	}

	before := time.Now().UTC().Add(-1 * time.Second)
	if err := store.Insert(v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	after := time.Now().UTC().Add(1 * time.Second)

	if v.ID <= 0 {
		t.Errorf("expected positive ID, got %d", v.ID)
	}
	if v.CreatedAt.Before(before) || v.CreatedAt.After(after) {
		t.Errorf("expected CreatedAt between %v and %v, got %v", before, after, v.CreatedAt)
	}
}

func TestInsert_AutoIncrementsID(t *testing.T) {
	store := newTestStore(t)

	v1 := &model.Vibe{YouTubeURL: "https://youtube.com/watch?v=a", Thought: "first"}
	v2 := &model.Vibe{YouTubeURL: "https://youtube.com/watch?v=b", Thought: "second"}

	if err := store.Insert(v1); err != nil {
		t.Fatalf("insert v1 failed: %v", err)
	}
	if err := store.Insert(v2); err != nil {
		t.Fatalf("insert v2 failed: %v", err)
	}

	if v2.ID <= v1.ID {
		t.Errorf("expected v2.ID > v1.ID, got v1=%d v2=%d", v1.ID, v2.ID)
	}
}

func TestListRecent_OrderByCreatedAtDesc(t *testing.T) {
	store := newTestStore(t)

	// Insert vibes with explicit different timestamps to test ordering.
	vibeData := []struct {
		thought   string
		timestamp string
	}{
		{"oldest", "2024-01-01 10:00:00"},
		{"middle", "2024-01-02 10:00:00"},
		{"newest", "2024-01-03 10:00:00"},
	}

	for _, d := range vibeData {
		_, err := store.db.Exec(
			"INSERT INTO vibes (youtube_url, thought, created_at) VALUES (?, ?, ?)",
			"https://youtube.com/watch?v=x", d.thought, d.timestamp,
		)
		if err != nil {
			t.Fatalf("insert %q failed: %v", d.thought, err)
		}
	}

	vibes, err := store.ListRecent(10)
	if err != nil {
		t.Fatalf("ListRecent failed: %v", err)
	}

	if len(vibes) != 3 {
		t.Fatalf("expected 3 vibes, got %d", len(vibes))
	}

	// Most recent should be first (DESC order).
	if vibes[0].Thought != "newest" {
		t.Errorf("expected first vibe to be 'newest', got %q", vibes[0].Thought)
	}
	if vibes[1].Thought != "middle" {
		t.Errorf("expected second vibe to be 'middle', got %q", vibes[1].Thought)
	}
	if vibes[2].Thought != "oldest" {
		t.Errorf("expected third vibe to be 'oldest', got %q", vibes[2].Thought)
	}

	// Verify timestamps are in descending order.
	for i := 0; i < len(vibes)-1; i++ {
		if vibes[i].CreatedAt.Before(vibes[i+1].CreatedAt) {
			t.Errorf("expected descending order, vibes[%d].CreatedAt=%v before vibes[%d].CreatedAt=%v",
				i, vibes[i].CreatedAt, i+1, vibes[i+1].CreatedAt)
		}
	}
}

func TestListRecent_RespectsLimit(t *testing.T) {
	store := newTestStore(t)

	for i := 0; i < 5; i++ {
		v := &model.Vibe{YouTubeURL: "https://youtube.com/watch?v=x", Thought: "vibe"}
		if err := store.Insert(v); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	vibes, err := store.ListRecent(3)
	if err != nil {
		t.Fatalf("ListRecent failed: %v", err)
	}

	if len(vibes) != 3 {
		t.Errorf("expected 3 vibes with limit=3, got %d", len(vibes))
	}
}

func TestListRecent_EmptyTable(t *testing.T) {
	store := newTestStore(t)

	vibes, err := store.ListRecent(10)
	if err != nil {
		t.Fatalf("ListRecent failed: %v", err)
	}

	if vibes == nil {
		// nil is acceptable for empty result
	}
	if len(vibes) != 0 {
		t.Errorf("expected 0 vibes from empty table, got %d", len(vibes))
	}
}

func TestDelete_ExistingRow(t *testing.T) {
	store := newTestStore(t)

	v := &model.Vibe{YouTubeURL: "https://youtube.com/watch?v=del", Thought: "to delete"}
	if err := store.Insert(v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	deleted, err := store.Delete(v.ID)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !deleted {
		t.Error("expected Delete to return true for existing row")
	}

	// Verify it's gone.
	vibes, err := store.ListRecent(10)
	if err != nil {
		t.Fatalf("ListRecent after delete failed: %v", err)
	}
	if len(vibes) != 0 {
		t.Errorf("expected 0 vibes after delete, got %d", len(vibes))
	}
}

func TestDelete_NonExistingRow(t *testing.T) {
	store := newTestStore(t)

	deleted, err := store.Delete(9999)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if deleted {
		t.Error("expected Delete to return false for non-existing row")
	}
}

// --- Task 6.1: Unit tests for Store Vote method ---

func TestVote_UpvoteIncrementsLikes(t *testing.T) {
	store := newTestStore(t)

	v := &model.Vibe{YouTubeURL: "https://youtube.com/watch?v=abc12345678", Thought: "great vibe"}
	if err := store.Insert(v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	updated, err := store.Vote(v.ID, "up")
	if err != nil {
		t.Fatalf("vote up failed: %v", err)
	}

	if updated.Likes != 1 {
		t.Errorf("expected likes=1 after upvote, got %d", updated.Likes)
	}
	if updated.Dislikes != 0 {
		t.Errorf("expected dislikes=0 after upvote, got %d", updated.Dislikes)
	}
}

func TestVote_DownvoteIncrementsDislikes(t *testing.T) {
	store := newTestStore(t)

	v := &model.Vibe{YouTubeURL: "https://youtube.com/watch?v=abc12345678", Thought: "meh vibe"}
	if err := store.Insert(v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	updated, err := store.Vote(v.ID, "down")
	if err != nil {
		t.Fatalf("vote down failed: %v", err)
	}

	if updated.Dislikes != 1 {
		t.Errorf("expected dislikes=1 after downvote, got %d", updated.Dislikes)
	}
	if updated.Likes != 0 {
		t.Errorf("expected likes=0 after downvote, got %d", updated.Likes)
	}
}

func TestVote_NonExistentID_ReturnsErrNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Vote(9999, "up")
	if err == nil {
		t.Fatal("expected error for non-existent vibe, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestListRecent_ReturnsCorrectVoteFields(t *testing.T) {
	store := newTestStore(t)

	v := &model.Vibe{YouTubeURL: "https://youtube.com/watch?v=abc12345678", Thought: "vote fields test"}
	if err := store.Insert(v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	// Upvote twice, downvote once
	store.Vote(v.ID, "up")
	store.Vote(v.ID, "up")
	store.Vote(v.ID, "down")

	vibes, err := store.ListRecent(10)
	if err != nil {
		t.Fatalf("ListRecent failed: %v", err)
	}
	if len(vibes) != 1 {
		t.Fatalf("expected 1 vibe, got %d", len(vibes))
	}

	got := vibes[0]
	if got.Likes != 2 {
		t.Errorf("expected likes=2, got %d", got.Likes)
	}
	if got.Dislikes != 1 {
		t.Errorf("expected dislikes=1, got %d", got.Dislikes)
	}
	if got.NetScore != 1 {
		t.Errorf("expected net_score=1, got %d", got.NetScore)
	}
}
