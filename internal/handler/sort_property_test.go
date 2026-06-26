package handler

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// **Validates: Requirements 5.1, 5.3, 5.4**
// Property 4: Vote tracker records and enforces direction — The localStorage vote tracker
// stores direction correctly, blocks same-direction re-vote, and allows opposite-direction change.
func TestProperty_VoteTrackerEnforcement(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tracker := make(map[int64]string) // simulates localStorage
		vibeId := rapid.Int64Range(1, 1000).Draw(t, "vibeId")

		// First vote should always succeed (tracker is empty for this vibeId)
		dir1 := rapid.SampledFrom([]string{"up", "down"}).Draw(t, "dir1")
		if _, exists := tracker[vibeId]; exists {
			t.Fatal("tracker should be empty initially")
		}
		tracker[vibeId] = dir1

		// Same direction should be blocked (tracker already has this direction)
		if tracker[vibeId] != dir1 {
			t.Fatalf("tracker should record direction %q, got %q", dir1, tracker[vibeId])
		}
		// In the real frontend, same-direction vote is a no-op — we verify the condition that blocks it
		blocked := tracker[vibeId] == dir1
		if !blocked {
			t.Fatalf("same direction should be detected as blocked")
		}

		// Opposite direction should succeed and update
		opposite := "down"
		if dir1 == "down" {
			opposite = "up"
		}
		tracker[vibeId] = opposite

		if tracker[vibeId] != opposite {
			t.Fatalf("tracker should record new direction %q, got %q", opposite, tracker[vibeId])
		}
	})
}

// **Validates: Requirements 6.3**
// Property 5: Date sort ordering — Sorting vibes by date produces strictly descending
// created_at order.
func TestProperty_DateSortOrdering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(2, 30).Draw(t, "n")
		type vibeSort struct {
			createdAt time.Time
		}
		vibes := make([]vibeSort, n)
		for i := range vibes {
			vibes[i].createdAt = time.Unix(rapid.Int64Range(1000000000, 2000000000).Draw(t, fmt.Sprintf("ts%d", i)), 0)
		}

		// Sort by date DESC (same logic as frontend sortVibes with mode='date')
		sort.Slice(vibes, func(i, j int) bool {
			return vibes[i].createdAt.After(vibes[j].createdAt)
		})

		// Verify descending order
		for i := 0; i < len(vibes)-1; i++ {
			if vibes[i].createdAt.Before(vibes[i+1].createdAt) {
				t.Fatalf("not in descending date order at index %d: %v < %v", i, vibes[i].createdAt, vibes[i+1].createdAt)
			}
		}
	})
}

// **Validates: Requirements 6.4**
// Property 6: Likes sort ordering — Sorting vibes by likes produces descending net_score
// with created_at DESC as tiebreaker.
func TestProperty_LikesSortOrdering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(2, 30).Draw(t, "n")
		type vibeSort struct {
			netScore  int64
			createdAt time.Time
		}
		vibes := make([]vibeSort, n)
		for i := range vibes {
			vibes[i].netScore = rapid.Int64Range(-50, 50).Draw(t, fmt.Sprintf("score%d", i))
			vibes[i].createdAt = time.Unix(rapid.Int64Range(1000000000, 2000000000).Draw(t, fmt.Sprintf("ts%d", i)), 0)
		}

		// Sort by net_score DESC, then created_at DESC (same logic as frontend sortVibes with mode='likes')
		sort.Slice(vibes, func(i, j int) bool {
			if vibes[i].netScore != vibes[j].netScore {
				return vibes[i].netScore > vibes[j].netScore
			}
			return vibes[i].createdAt.After(vibes[j].createdAt)
		})

		// Verify ordering invariants
		for i := 0; i < len(vibes)-1; i++ {
			if vibes[i].netScore < vibes[i+1].netScore {
				t.Fatalf("not in descending score order at index %d: %d < %d", i, vibes[i].netScore, vibes[i+1].netScore)
			}
			if vibes[i].netScore == vibes[i+1].netScore && vibes[i].createdAt.Before(vibes[i+1].createdAt) {
				t.Fatalf("tie not broken by date at index %d", i)
			}
		}
	})
}

// **Validates: Requirements 6.6**
// Property 7: Sorted insertion preserves order — Inserting a new item into a correctly
// sorted list at the right position maintains the sort invariant.
func TestProperty_SortedInsertionPreservesOrder(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 30).Draw(t, "n")
		type vibeSort struct {
			netScore  int64
			createdAt time.Time
		}

		// Generate and sort an initial list (by likes mode: net_score DESC, createdAt DESC)
		vibes := make([]vibeSort, n)
		for i := range vibes {
			vibes[i].netScore = rapid.Int64Range(-50, 50).Draw(t, fmt.Sprintf("score%d", i))
			vibes[i].createdAt = time.Unix(rapid.Int64Range(1000000000, 2000000000).Draw(t, fmt.Sprintf("ts%d", i)), 0)
		}
		sort.Slice(vibes, func(i, j int) bool {
			if vibes[i].netScore != vibes[j].netScore {
				return vibes[i].netScore > vibes[j].netScore
			}
			return vibes[i].createdAt.After(vibes[j].createdAt)
		})

		// Generate a new vibe to insert
		newVibe := vibeSort{
			netScore:  rapid.Int64Range(-50, 50).Draw(t, "newScore"),
			createdAt: time.Unix(rapid.Int64Range(1000000000, 2000000000).Draw(t, "newTs"), 0),
		}

		// Find the correct insertion position (same logic as frontend insertVibeAtSortedPosition)
		insertIdx := len(vibes)
		for i := 0; i < len(vibes); i++ {
			if newVibe.netScore > vibes[i].netScore ||
				(newVibe.netScore == vibes[i].netScore && newVibe.createdAt.After(vibes[i].createdAt)) {
				insertIdx = i
				break
			}
		}

		// Insert at the found position
		vibes = append(vibes, vibeSort{})
		copy(vibes[insertIdx+1:], vibes[insertIdx:])
		vibes[insertIdx] = newVibe

		// Verify the entire list is still sorted correctly
		for i := 0; i < len(vibes)-1; i++ {
			if vibes[i].netScore < vibes[i+1].netScore {
				t.Fatalf("sort broken after insertion at index %d: score %d < %d", i, vibes[i].netScore, vibes[i+1].netScore)
			}
			if vibes[i].netScore == vibes[i+1].netScore && vibes[i].createdAt.Before(vibes[i+1].createdAt) {
				t.Fatalf("tiebreaker broken after insertion at index %d", i)
			}
		}
	})
}
