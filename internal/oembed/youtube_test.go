package oembed

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetch_Success(t *testing.T) {
	expected := oembedResponse{
		Title:        "Never Gonna Give You Up",
		ThumbnailURL: "https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query params are passed
		if r.URL.Query().Get("format") != "json" {
			t.Errorf("expected format=json, got %s", r.URL.Query().Get("format"))
		}
		if r.URL.Query().Get("url") == "" {
			t.Error("expected url query param to be set")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	client := NewClientWithBaseURL(5*time.Second, srv.URL)
	meta, err := client.Fetch("https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.Title != expected.Title {
		t.Errorf("expected title %q, got %q", expected.Title, meta.Title)
	}
	if meta.ThumbnailURL != expected.ThumbnailURL {
		t.Errorf("expected thumbnail %q, got %q", expected.ThumbnailURL, meta.ThumbnailURL)
	}
}

func TestFetch_Non200Response(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"404 not found", http.StatusNotFound},
		{"500 internal error", http.StatusInternalServerError},
		{"403 forbidden", http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer srv.Close()

			client := NewClientWithBaseURL(5*time.Second, srv.URL)
			_, err := client.Fetch("https://www.youtube.com/watch?v=dQw4w9WgXcQ")
			if err == nil {
				t.Fatal("expected error for non-200 response, got nil")
			}
		})
	}
}

func TestFetch_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay longer than client timeout
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Client with a very short timeout
	client := NewClientWithBaseURL(50*time.Millisecond, srv.URL)
	_, err := client.Fetch("https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestFetch_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("this is not json{{{"))
	}))
	defer srv.Close()

	client := NewClientWithBaseURL(5*time.Second, srv.URL)
	_, err := client.Fetch("https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
}
