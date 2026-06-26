package handler

import (
	"testing"
)

func TestValidateYouTubeURL_Valid(t *testing.T) {
	valid := []string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=abc-123_XYZ",
		"https://youtu.be/abc-123_XYZ",
		"https://www.youtube.com/shorts/abc-123_XYZ",
	}

	for _, u := range valid {
		if err := ValidateYouTubeURL(u); err != nil {
			t.Errorf("expected valid but got error for %q: %v", u, err)
		}
	}
}

func TestValidateYouTubeURL_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"http://www.youtube.com/watch?v=dQw4w9WgXcQ",   // http not https
		"https://youtube.com/watch?v=dQw4w9WgXcQ",      // missing www
		"https://www.youtube.com/watch?v=",             // empty id
		"https://www.youtube.com/watch?v=short",        // too short
		"https://www.youtube.com/watch?v=waytoolongid", // too long
		"https://www.youtube.com/embed/dQw4w9WgXcQ",    // embed not accepted
		"https://vimeo.com/123456",                     // wrong site
		"not a url",
	}

	for _, u := range invalid {
		err := ValidateYouTubeURL(u)
		if err == nil {
			t.Errorf("expected error for %q but got nil", u)
		}
	}
}

func TestValidateYouTubeURL_ErrorMessage(t *testing.T) {
	err := ValidateYouTubeURL("https://vimeo.com/123")
	if err == nil {
		t.Fatal("expected error")
	}
	expected := "invalid youtube_url: accepted formats are https://www.youtube.com/watch?v=ID, https://youtu.be/ID, https://www.youtube.com/shorts/ID"
	if err.Error() != expected {
		t.Errorf("unexpected error message:\ngot:  %q\nwant: %q", err.Error(), expected)
	}
}

func TestParseVideoID_AllFormats(t *testing.T) {
	tests := []struct {
		url        string
		expectedID string
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://www.youtube.com/shorts/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://www.youtube.com/watch?v=abc-123_XYZ", "abc-123_XYZ"},
		{"https://youtu.be/abc-123_XYZ", "abc-123_XYZ"},
		{"https://www.youtube.com/shorts/abc-123_XYZ", "abc-123_XYZ"},
	}

	for _, tc := range tests {
		id, err := ParseVideoID(tc.url)
		if err != nil {
			t.Errorf("ParseVideoID(%q) returned error: %v", tc.url, err)
			continue
		}
		if id != tc.expectedID {
			t.Errorf("ParseVideoID(%q) = %q, want %q", tc.url, id, tc.expectedID)
		}
	}
}

func TestParseVideoID_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"https://vimeo.com/123456",
		"not a url",
		"https://www.youtube.com/watch?v=short",
	}

	for _, u := range invalid {
		_, err := ParseVideoID(u)
		if err == nil {
			t.Errorf("ParseVideoID(%q) expected error but got nil", u)
		}
	}
}

func TestReconstructURL(t *testing.T) {
	id := "dQw4w9WgXcQ"
	got := ReconstructURL(id)
	expected := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	if got != expected {
		t.Errorf("ReconstructURL(%q) = %q, want %q", id, got, expected)
	}
}

func TestParseVideoID_RoundTrip(t *testing.T) {
	// For all three formats, parse then reconstruct should produce a valid URL
	// that also yields the same video ID when parsed again.
	urls := []string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ",
	}

	for _, url := range urls {
		id, err := ParseVideoID(url)
		if err != nil {
			t.Errorf("ParseVideoID(%q) error: %v", url, err)
			continue
		}

		reconstructed := ReconstructURL(id)
		if err := ValidateYouTubeURL(reconstructed); err != nil {
			t.Errorf("ReconstructURL(%q) produced invalid URL %q: %v", id, reconstructed, err)
			continue
		}

		id2, err := ParseVideoID(reconstructed)
		if err != nil {
			t.Errorf("ParseVideoID(%q) error on reconstructed URL: %v", reconstructed, err)
			continue
		}
		if id != id2 {
			t.Errorf("round-trip failed: original ID %q, after round-trip %q", id, id2)
		}
	}
}
