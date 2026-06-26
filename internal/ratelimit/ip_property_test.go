package ratelimit

import (
	"fmt"
	"net/http"
	"testing"

	"pgregory.net/rapid"
)

// TestProperty9_IPExtractionCorrectness verifies that ExtractIP correctly
// extracts the client IP from X-Forwarded-For or RemoteAddr, stripping ports.
// **Validates: Requirements 8.1, 8.2, 8.3**
func TestProperty9_IPExtractionCorrectness(t *testing.T) {
	t.Run("RemoteAddr_with_port", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate a random IPv4 address.
			ip := fmt.Sprintf("%d.%d.%d.%d",
				rapid.IntRange(1, 254).Draw(t, "ip1"),
				rapid.IntRange(0, 255).Draw(t, "ip2"),
				rapid.IntRange(0, 255).Draw(t, "ip3"),
				rapid.IntRange(1, 254).Draw(t, "ip4"),
			)
			port := rapid.IntRange(1024, 65535).Draw(t, "port")

			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = fmt.Sprintf("%s:%d", ip, port)

			got := ExtractIP(req)
			if got != ip {
				t.Fatalf("ExtractIP with RemoteAddr=%q: expected %q, got %q", req.RemoteAddr, ip, got)
			}
		})
	})

	t.Run("RemoteAddr_without_port", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// An IP without port should be returned as-is.
			ip := fmt.Sprintf("%d.%d.%d.%d",
				rapid.IntRange(1, 254).Draw(t, "ip1"),
				rapid.IntRange(0, 255).Draw(t, "ip2"),
				rapid.IntRange(0, 255).Draw(t, "ip3"),
				rapid.IntRange(1, 254).Draw(t, "ip4"),
			)

			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = ip

			got := ExtractIP(req)
			if got != ip {
				t.Fatalf("ExtractIP with RemoteAddr=%q (no port): expected %q, got %q", req.RemoteAddr, ip, got)
			}
		})
	})

	t.Run("XFF_single_IP", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			ip := fmt.Sprintf("%d.%d.%d.%d",
				rapid.IntRange(1, 254).Draw(t, "ip1"),
				rapid.IntRange(0, 255).Draw(t, "ip2"),
				rapid.IntRange(0, 255).Draw(t, "ip3"),
				rapid.IntRange(1, 254).Draw(t, "ip4"),
			)

			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = "127.0.0.1:9999"
			req.Header.Set("X-Forwarded-For", ip)

			got := ExtractIP(req)
			if got != ip {
				t.Fatalf("ExtractIP with XFF=%q: expected %q, got %q", ip, ip, got)
			}
		})
	})

	t.Run("XFF_multiple_IPs", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Generate 2-5 IPs. The first one should be returned.
			numIPs := rapid.IntRange(2, 5).Draw(t, "numIPs")
			ips := make([]string, numIPs)
			for i := 0; i < numIPs; i++ {
				ips[i] = fmt.Sprintf("%d.%d.%d.%d",
					rapid.IntRange(1, 254).Draw(t, fmt.Sprintf("ip%d_1", i)),
					rapid.IntRange(0, 255).Draw(t, fmt.Sprintf("ip%d_2", i)),
					rapid.IntRange(0, 255).Draw(t, fmt.Sprintf("ip%d_3", i)),
					rapid.IntRange(1, 254).Draw(t, fmt.Sprintf("ip%d_4", i)),
				)
			}

			// Build comma-separated XFF.
			xff := ips[0]
			for i := 1; i < numIPs; i++ {
				xff += ", " + ips[i]
			}

			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = "127.0.0.1:9999"
			req.Header.Set("X-Forwarded-For", xff)

			got := ExtractIP(req)
			if got != ips[0] {
				t.Fatalf("ExtractIP with XFF=%q: expected first IP %q, got %q", xff, ips[0], got)
			}
		})
	})
}
