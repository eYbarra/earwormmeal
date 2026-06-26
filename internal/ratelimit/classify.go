package ratelimit

import "strings"

// Classify returns the rate limit category for the given request method and path.
// Returns an empty string for exempt endpoints (WebSocket, static files, etc.).
func Classify(method, path string) string {
	switch {
	case method == "POST" && path == "/api/vibes":
		return "create"
	case method == "POST" && strings.HasPrefix(path, "/api/vibes/") && strings.HasSuffix(path, "/vote"):
		return "vote"
	case method == "GET" && path == "/api/vibes":
		return "list"
	default:
		return ""
	}
}
