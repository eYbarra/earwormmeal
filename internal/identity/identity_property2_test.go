package identity

import (
	"testing"

	"pgregory.net/rapid"
)

// Feature: anonymous-identities, Property 2: Determinism (idempotence)
// **Validates: Requirements 3.4, 4.6**
//
// For any random IP string, calling Generate(ip) twice on the same Generator
// within the same epoch produces the same result.
func TestProperty2_Determinism(t *testing.T) {
	gen := New([]byte("property-test-salt-determinism!"), 168)

	rapid.Check(t, func(t *rapid.T) {
		// Generate a random IPv4-like string
		ip := rapid.StringMatching(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`).Draw(t, "ip")

		result1 := gen.Generate(ip)
		result2 := gen.Generate(ip)

		if result1 != result2 {
			t.Fatalf("Generate(%q) is not deterministic: got %q and %q", ip, result1, result2)
		}
	})
}
