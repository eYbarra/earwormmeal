package identity

import (
	"strings"
	"testing"
)

// testSalt is a fixed salt used across unit tests for deterministic results.
var testSalt = []byte("test-salt-for-identity-unit-tests")

// TestGenerate_Determinism verifies that the same IP always produces
// the same deterministic display name within the same epoch.
// Validates: Requirements 3.4, 4.6
func TestGenerate_Determinism(t *testing.T) {
	gen := New(testSalt, 168)
	const ip = "192.168.1.1"

	got1 := gen.Generate(ip)
	got2 := gen.Generate(ip)

	if got1 != got2 {
		t.Errorf("Generate(%q) not deterministic: got %q then %q", ip, got1, got2)
	}

	// Verify it has the expected format.
	parts := strings.SplitN(got1, " ", 3)
	if len(parts) != 2 {
		t.Errorf("Generate(%q) = %q: expected \"Adjective Noun\" format", ip, got1)
	}
}

// TestWordListSizes asserts that both word lists have at least 500 entries,
// ensuring sufficient combinatorial space (500×500 = 250,000+ names).
// Validates: Requirements 1.1, 1.2, 1.3
func TestWordListSizes(t *testing.T) {
	if got := len(adjectives); got < 500 {
		t.Errorf("len(adjectives) = %d, want >= 500", got)
	}
	if got := len(nouns); got < 500 {
		t.Errorf("len(nouns) = %d, want >= 500", got)
	}
}

// TestWordListUniqueness verifies no case-insensitive duplicates exist.
// Validates: Requirements 1.4, 1.5
func TestWordListUniqueness(t *testing.T) {
	seen := make(map[string]bool, len(adjectives))
	for _, a := range adjectives {
		lower := strings.ToLower(a)
		if seen[lower] {
			t.Errorf("duplicate adjective (case-insensitive): %q", a)
		}
		seen[lower] = true
	}

	seen = make(map[string]bool, len(nouns))
	for _, n := range nouns {
		lower := strings.ToLower(n)
		if seen[lower] {
			t.Errorf("duplicate noun (case-insensitive): %q", n)
		}
		seen[lower] = true
	}
}

// TestWordListAlphabetic verifies all entries are alphabetic and title-cased.
// Validates: Requirements 1.6, 1.7
func TestWordListAlphabetic(t *testing.T) {
	for i, a := range adjectives {
		if !isValidTitleCase(a) {
			t.Errorf("adjectives[%d] = %q: not valid title-case alphabetic", i, a)
		}
	}
	for i, n := range nouns {
		if !isValidTitleCase(n) {
			t.Errorf("nouns[%d] = %q: not valid title-case alphabetic", i, n)
		}
	}
}

// isValidTitleCase checks that s starts with an uppercase letter followed by lowercase letters.
func isValidTitleCase(s string) bool {
	if len(s) == 0 {
		return false
	}
	if s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for _, c := range s[1:] {
		if c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}

// TestGenerate_EmptyString confirms that Generate handles empty input
// without panicking and still produces a valid two-word name.
// Validates: Requirements 5.4
func TestGenerate_EmptyString(t *testing.T) {
	gen := New(testSalt, 168)

	got := gen.Generate("")
	if got == "" {
		t.Error("Generate(\"\") returned empty string, want a valid display name")
	}

	// Verify it contains exactly one space (Adjective + space + Noun).
	spaceCount := 0
	for _, c := range got {
		if c == ' ' {
			spaceCount++
		}
	}
	if spaceCount != 1 {
		t.Errorf("Generate(\"\") = %q, expected format \"Adjective Noun\" (1 space), got %d spaces", got, spaceCount)
	}
}

// TestNew_PanicsOnEmptySalt verifies that New panics when given a zero-length salt.
// Validates: Requirements 3.5
func TestNew_PanicsOnEmptySalt(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("New(nil, 168) did not panic, expected panic on empty salt")
		}
	}()
	New(nil, 168)
}

// TestGenerate_DifferentIPs verifies that different IPs produce different names.
// Validates: Requirements 3.2
func TestGenerate_DifferentIPs(t *testing.T) {
	gen := New(testSalt, 168)

	name1 := gen.Generate("192.168.1.1")
	name2 := gen.Generate("10.0.0.1")

	if name1 == name2 {
		t.Errorf("Different IPs produced same name: %q", name1)
	}
}

// TestGenerate_DifferentSalts verifies that different salts produce different names for same IP.
// Validates: Requirements 2.3, 2.4
func TestGenerate_DifferentSalts(t *testing.T) {
	gen1 := New([]byte("salt-one-abcdefghijklmnop"), 168)
	gen2 := New([]byte("salt-two-abcdefghijklmnop"), 168)

	ip := "192.168.1.1"
	name1 := gen1.Generate(ip)
	name2 := gen2.Generate(ip)

	if name1 == name2 {
		t.Errorf("Different salts produced same name for %q: %q", ip, name1)
	}
}
