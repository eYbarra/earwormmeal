package ratelimit

import (
	"net"
	"net/http"
	"strings"
)

// ExtractIP returns the client IP from the request.
// It checks X-Forwarded-For first (taking the first comma-separated value),
// then falls back to RemoteAddr. Port numbers are stripped from the result.
func ExtractIP(r *http.Request) string {
	// Check X-Forwarded-For header first.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the comma-separated list.
		ip := xff
		if idx := strings.Index(xff, ","); idx != -1 {
			ip = xff[:idx]
		}
		ip = strings.TrimSpace(ip)
		return stripPort(ip)
	}

	// Fall back to RemoteAddr.
	return stripPort(r.RemoteAddr)
}

// stripPort removes the port portion from an address string.
// Handles "ip:port" (IPv4) and "[ip]:port" (IPv6) formats.
func stripPort(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No port present or unparseable — return as-is.
		return addr
	}
	return host
}
