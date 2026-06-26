package db

import (
	"fmt"
	"testing"

	"github.com/earworm/vibesboard/internal/model"
	"pgregory.net/rapid"
)

// TestProperty1_VibeCreationRoundTrip verifies that for any valid youtube_url and thought,
// creating and retrieving SHALL produce matching fields with non-zero id and created_at.
//
// **Validates: Requirements 3.1, 3.6**
func TestProperty1_VibeCreationRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store, err := New(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()

		// Generate a valid 11-character video ID
		videoID := rapid.StringMatching(`[A-Za-z0-9_-]{11}`).Draw(t, "videoID")

		// Pick one of the three accepted URL formats
		format := rapid.IntRange(0, 2).Draw(t, "urlFormat")
		var url string
		switch format {
		case 0:
			url = "https://www.youtube.com/watch?v=" + videoID
		case 1:
			url = "https://youtu.be/" + videoID
		case 2:
			url = "https://www.youtube.com/shorts/" + videoID
		}

		// Generate a valid thought (1-150 printable chars, no leading/trailing whitespace)
		thought := rapid.StringMatching(`[a-zA-Z0-9 ]{1,150}`).Draw(t, "thought")

		vibe := &model.Vibe{YouTubeURL: url, Thought: thought}
		if err := store.Insert(vibe); err != nil {
			t.Fatal(err)
		}

		// Verify non-zero ID and CreatedAt after insert
		if vibe.ID == 0 {
			t.Fatal("expected non-zero ID after insert")
		}
		if vibe.CreatedAt.IsZero() {
			t.Fatal("expected non-zero CreatedAt after insert")
		}

		// Retrieve and verify round-trip
		vibes, err := store.ListRecent(1)
		if err != nil {
			t.Fatal(err)
		}
		if len(vibes) != 1 {
			t.Fatalf("expected 1 vibe from ListRecent, got %d", len(vibes))
		}

		retrieved := vibes[0]
		if retrieved.YouTubeURL != url {
			t.Fatalf("youtube_url mismatch: got %q, want %q", retrieved.YouTubeURL, url)
		}
		if retrieved.Thought != thought {
			t.Fatalf("thought mismatch: got %q, want %q", retrieved.Thought, thought)
		}
		if retrieved.ID != vibe.ID {
			t.Fatalf("ID mismatch: got %d, want %d", retrieved.ID, vibe.ID)
		}
		if retrieved.CreatedAt.IsZero() {
			t.Fatal("expected non-zero CreatedAt on retrieved vibe")
		}
	})
}

// TestProperty7_ListVibesOrderingAndLimit verifies that for any N vibes inserted,
// listing SHALL return min(N, 50) vibes ordered by created_at DESC.
//
// **Validates: Requirements 4.1**
func TestProperty7_ListVibesOrderingAndLimit(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store, err := New(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()

		n := rapid.IntRange(0, 70).Draw(t, "numVibes")

		for i := 0; i < n; i++ {
			videoID := rapid.StringMatching(`[a-zA-Z0-9]{11}`).Draw(t, fmt.Sprintf("id%d", i))
			vibe := &model.Vibe{
				YouTubeURL: "https://youtu.be/" + videoID,
				Thought:    fmt.Sprintf("thought %d", i),
			}
			if err := store.Insert(vibe); err != nil {
				t.Fatal(err)
			}
		}

		vibes, err := store.ListRecent(50)
		if err != nil {
			t.Fatal(err)
		}

		expectedLen := n
		if expectedLen > 50 {
			expectedLen = 50
		}
		if len(vibes) != expectedLen {
			t.Fatalf("expected %d vibes, got %d", expectedLen, len(vibes))
		}

		// Verify descending order: since all inserts happen in the same second
		// (CURRENT_TIMESTAMP resolution), the secondary sort is by id DESC.
		for i := 0; i < len(vibes)-1; i++ {
			if vibes[i].CreatedAt.Before(vibes[i+1].CreatedAt) {
				t.Fatalf("not in descending created_at order at index %d: %v < %v",
					i, vibes[i].CreatedAt, vibes[i+1].CreatedAt)
			}
			// When created_at is the same, id should be descending
			if vibes[i].CreatedAt.Equal(vibes[i+1].CreatedAt) && vibes[i].ID <= vibes[i+1].ID {
				t.Fatalf("not in descending id order at index %d: %d <= %d",
					i, vibes[i].ID, vibes[i+1].ID)
			}
		}
	})
}

// TestProperty_VoteIncrementCorrectness verifies that for any existing vibe and any valid
// direction ("up" or "down"), calling Vote increments the correct counter by exactly 1
// and leaves the other counter unchanged.
//
// **Validates: Requirements 1.3, 2.1**
func TestProperty_VoteIncrementCorrectness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store, err := New(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()

		// Insert a random vibe
		videoID := rapid.StringMatching(`[a-zA-Z0-9]{11}`).Draw(t, "videoID")
		thought := rapid.StringMatching(`[a-zA-Z0-9 ]{1,100}`).Draw(t, "thought")
		vibe := &model.Vibe{
			YouTubeURL: "https://youtu.be/" + videoID,
			Thought:    thought,
		}
		if err := store.Insert(vibe); err != nil {
			t.Fatal(err)
		}

		// Apply some random initial votes to make it non-trivial
		numInitialVotes := rapid.IntRange(0, 10).Draw(t, "numInitialVotes")
		for i := 0; i < numInitialVotes; i++ {
			dir := rapid.SampledFrom([]string{"up", "down"}).Draw(t, fmt.Sprintf("initDir%d", i))
			store.Vote(vibe.ID, dir)
		}

		// Get current state before the test vote
		vibes, err := store.ListRecent(1)
		if err != nil {
			t.Fatal(err)
		}
		if len(vibes) != 1 {
			t.Fatalf("expected 1 vibe, got %d", len(vibes))
		}
		beforeLikes := vibes[0].Likes
		beforeDislikes := vibes[0].Dislikes

		// Draw a random valid direction and vote
		direction := rapid.SampledFrom([]string{"up", "down"}).Draw(t, "direction")
		updated, err := store.Vote(vibe.ID, direction)
		if err != nil {
			t.Fatalf("Vote(%d, %q) failed: %v", vibe.ID, direction, err)
		}

		// Verify correct counter incremented by 1 and other unchanged
		if direction == "up" {
			if updated.Likes != beforeLikes+1 {
				t.Fatalf("expected likes=%d after upvote, got %d", beforeLikes+1, updated.Likes)
			}
			if updated.Dislikes != beforeDislikes {
				t.Fatalf("expected dislikes=%d unchanged after upvote, got %d", beforeDislikes, updated.Dislikes)
			}
		} else {
			if updated.Dislikes != beforeDislikes+1 {
				t.Fatalf("expected dislikes=%d after downvote, got %d", beforeDislikes+1, updated.Dislikes)
			}
			if updated.Likes != beforeLikes {
				t.Fatalf("expected likes=%d unchanged after downvote, got %d", beforeLikes, updated.Likes)
			}
		}
	})
}

// TestProperty_NetScoreComputation verifies that for any vibe with random sequences
// of up/down votes, net_score always equals likes minus dislikes.
//
// **Validates: Requirements 1.2**
func TestProperty_NetScoreComputation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store, err := New(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()

		// Insert a vibe
		videoID := rapid.StringMatching(`[a-zA-Z0-9]{11}`).Draw(t, "videoID")
		vibe := &model.Vibe{
			YouTubeURL: "https://youtu.be/" + videoID,
			Thought:    "net score test",
		}
		if err := store.Insert(vibe); err != nil {
			t.Fatal(err)
		}

		// Apply a random sequence of votes
		numVotes := rapid.IntRange(1, 20).Draw(t, "numVotes")
		for i := 0; i < numVotes; i++ {
			dir := rapid.SampledFrom([]string{"up", "down"}).Draw(t, fmt.Sprintf("dir%d", i))
			store.Vote(vibe.ID, dir)
		}

		// Retrieve the vibe and verify net_score = likes - dislikes
		vibes, err := store.ListRecent(1)
		if err != nil {
			t.Fatal(err)
		}
		if len(vibes) != 1 {
			t.Fatalf("expected 1 vibe, got %d", len(vibes))
		}

		got := vibes[0]
		expectedNetScore := got.Likes - got.Dislikes
		if got.NetScore != expectedNetScore {
			t.Fatalf("net_score mismatch: got %d, expected likes(%d) - dislikes(%d) = %d",
				got.NetScore, got.Likes, got.Dislikes, expectedNetScore)
		}
	})
}

// TestProperty8_DeleteRemovesVibeFromListing verifies that for any set of vibes,
// deleting one SHALL remove it from list and decrease count by 1.
//
// **Validates: Requirements 5.1**
func TestProperty8_DeleteRemovesVibeFromListing(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store, err := New(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()

		n := rapid.IntRange(1, 20).Draw(t, "numVibes")
		var ids []int64

		for i := 0; i < n; i++ {
			videoID := rapid.StringMatching(`[a-zA-Z0-9]{11}`).Draw(t, fmt.Sprintf("id%d", i))
			vibe := &model.Vibe{
				YouTubeURL: "https://youtu.be/" + videoID,
				Thought:    fmt.Sprintf("thought %d", i),
			}
			if err := store.Insert(vibe); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, vibe.ID)
		}

		// Delete a random vibe
		idx := rapid.IntRange(0, n-1).Draw(t, "deleteIdx")
		deleteID := ids[idx]

		deleted, err := store.Delete(deleteID)
		if err != nil {
			t.Fatal(err)
		}
		if !deleted {
			t.Fatal("expected delete to succeed for existing vibe")
		}

		// Verify list count decreased by 1
		vibes, err := store.ListRecent(50)
		if err != nil {
			t.Fatal(err)
		}
		if len(vibes) != n-1 {
			t.Fatalf("expected %d vibes after delete, got %d", n-1, len(vibes))
		}

		// Verify deleted vibe is not in the list
		for _, v := range vibes {
			if v.ID == deleteID {
				t.Fatalf("deleted vibe (ID=%d) still appears in list", deleteID)
			}
		}
	})
}
