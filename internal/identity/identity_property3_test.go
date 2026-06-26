package identity

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

// Feature: anonymous-identities, Property 3: Collision resistance
//
// For any pair of distinct random IP strings, Generate(ip1) != Generate(ip2)
// with high probability. Over 200 iterations, the collision rate must be < 1%.
//
// **Validates: Requirements 1.5**
func TestProperty3_CollisionResistance(t *testing.T) {
	gen := New([]byte("property-test-salt-collisions!!"), 168)
	const iterations = 200
	collisions := 0

	for i := range iterations {
		ip1 := generateRandomIP(i * 2)
		ip2 := generateRandomIP(i*2 + 1)

		// Ensure the IPs are distinct.
		for ip1 == ip2 {
			ip2 = generateRandomIP(i*2 + 1 + iterations)
		}

		name1 := gen.Generate(ip1)
		name2 := gen.Generate(ip2)

		if name1 == name2 {
			collisions++
		}
	}

	collisionRate := float64(collisions) / float64(iterations)
	if collisionRate >= 0.01 {
		t.Fatalf("Collision rate too high: %d/%d = %.4f (must be < 1%%)", collisions, iterations, collisionRate)
	}

	t.Logf("Collision rate: %d/%d = %.4f", collisions, iterations, collisionRate)
}

// Feature: anonymous-identities, Property 3: Collision resistance
//
// TestProperty3_CollisionResistance_Rapid uses pgregory.net/rapid to generate
// random pairs of distinct IP addresses and verifies they produce different names.
//
// **Validates: Requirements 1.5**
func TestProperty3_CollisionResistance_Rapid(t *testing.T) {
	gen := New([]byte("property-test-salt-rapid-collis"), 168)

	rapid.Check(t, func(t *rapid.T) {
		// Generate two distinct random IPv4 addresses.
		ip1 := fmt.Sprintf("%d.%d.%d.%d",
			rapid.IntRange(0, 255).Draw(t, "ip1_a"),
			rapid.IntRange(0, 255).Draw(t, "ip1_b"),
			rapid.IntRange(0, 255).Draw(t, "ip1_c"),
			rapid.IntRange(0, 255).Draw(t, "ip1_d"),
		)
		ip2 := fmt.Sprintf("%d.%d.%d.%d",
			rapid.IntRange(0, 255).Draw(t, "ip2_a"),
			rapid.IntRange(0, 255).Draw(t, "ip2_b"),
			rapid.IntRange(0, 255).Draw(t, "ip2_c"),
			rapid.IntRange(0, 255).Draw(t, "ip2_d"),
		)

		// Skip if the two generated IPs happen to be identical.
		if ip1 == ip2 {
			return
		}

		name1 := gen.Generate(ip1)
		name2 := gen.Generate(ip2)

		if name1 == name2 {
			t.Fatalf("Collision detected: Generate(%q) == Generate(%q) == %q", ip1, ip2, name1)
		}
	})
}

// generateRandomIP produces a deterministic pseudo-random IP from a seed.
func generateRandomIP(seed int) string {
	a := (seed*31 + 7) % 256
	b := (seed*37 + 13) % 256
	c := (seed*41 + 19) % 256
	d := (seed*43 + 23) % 256
	return fmt.Sprintf("%d.%d.%d.%d", a, b, c, d)
}
