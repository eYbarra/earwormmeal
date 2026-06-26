package identity

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Feature: anonymous-identities, Property 1: Output validity (format and word list membership)

// TestProperty1_OutputValidity verifies that for any random IP string,
// Generate(ip) returns "Adjective Noun" where both words are members
// of the respective word lists.
// **Validates: Requirements 1.6, 5.4**
func TestProperty1_OutputValidity(t *testing.T) {
	// Build lookup sets for fast membership checks.
	adjSet := make(map[string]bool, len(adjectives))
	for _, a := range adjectives {
		adjSet[a] = true
	}
	nounSet := make(map[string]bool, len(nouns))
	for _, n := range nouns {
		nounSet[n] = true
	}

	gen := New([]byte("property-test-salt-output-valid"), 168)

	rapid.Check(t, func(t *rapid.T) {
		// Generate a random valid IPv4 address.
		ip := fmt.Sprintf("%d.%d.%d.%d",
			rapid.IntRange(0, 255).Draw(t, "octet1"),
			rapid.IntRange(0, 255).Draw(t, "octet2"),
			rapid.IntRange(0, 255).Draw(t, "octet3"),
			rapid.IntRange(0, 255).Draw(t, "octet4"),
		)

		result := gen.Generate(ip)

		// Assert format: exactly two words separated by a single space.
		parts := strings.SplitN(result, " ", 3)
		if len(parts) != 2 {
			t.Fatalf("Generate(%q) = %q: expected exactly 2 space-separated words, got %d parts", ip, result, len(parts))
		}

		// Assert first word is in the adjectives list.
		if !adjSet[parts[0]] {
			t.Fatalf("Generate(%q) = %q: first word %q is not in adjectives list", ip, result, parts[0])
		}

		// Assert second word is in the nouns list.
		if !nounSet[parts[1]] {
			t.Fatalf("Generate(%q) = %q: second word %q is not in nouns list", ip, result, parts[1])
		}
	})
}
