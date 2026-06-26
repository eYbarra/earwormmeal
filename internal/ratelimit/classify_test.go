package ratelimit

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{name: "POST /api/vibes is create", method: "POST", path: "/api/vibes", want: "create"},
		{name: "POST /api/vibes/123/vote is vote", method: "POST", path: "/api/vibes/123/vote", want: "vote"},
		{name: "POST /api/vibes/abc/vote is vote", method: "POST", path: "/api/vibes/abc/vote", want: "vote"},
		{name: "GET /api/vibes is list", method: "GET", path: "/api/vibes", want: "list"},
		{name: "GET /ws is exempt", method: "GET", path: "/ws", want: ""},
		{name: "GET / is exempt", method: "GET", path: "/", want: ""},
		{name: "DELETE /api/vibes/1 is exempt", method: "DELETE", path: "/api/vibes/1", want: ""},
		{name: "random path is exempt", method: "GET", path: "/some/random/path", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.method, tt.path)
			if got != tt.want {
				t.Errorf("Classify(%q, %q) = %q, want %q", tt.method, tt.path, got, tt.want)
			}
		})
	}
}
