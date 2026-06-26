package handler

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// videoIDPattern matches exactly 11 characters: alphanumeric, hyphens, underscores.
var videoIDPattern = `[A-Za-z0-9_-]{11}`

// youtubeURLPatterns matches the three accepted YouTube URL formats with exact 11-char video IDs.
var youtubeURLPatterns = regexp.MustCompile(
	`^https://(www\.youtube\.com/watch\?v=` + videoIDPattern +
		`|youtu\.be/` + videoIDPattern +
		`|www\.youtube\.com/shorts/` + videoIDPattern + `)$`,
)

// validFormatsMessage is the descriptive error returned when a YouTube URL is invalid.
const validFormatsMessage = "invalid youtube_url: accepted formats are https://www.youtube.com/watch?v=ID, https://youtu.be/ID, https://www.youtube.com/shorts/ID"

// ValidateYouTubeURL checks whether the given URL matches one of the accepted YouTube URL
// formats. It returns nil on success or a descriptive error listing valid formats on rejection.
func ValidateYouTubeURL(url string) error {
	if !youtubeURLPatterns.MatchString(url) {
		return errors.New(validFormatsMessage)
	}
	return nil
}

// ParseVideoID extracts the 11-character video ID from a valid YouTube URL.
// It supports all three accepted formats:
//   - https://www.youtube.com/watch?v={id}
//   - https://youtu.be/{id}
//   - https://www.youtube.com/shorts/{id}
//
// Returns an error if the URL does not match any accepted format.
func ParseVideoID(url string) (string, error) {
	if err := ValidateYouTubeURL(url); err != nil {
		return "", err
	}

	// Extract video ID based on which pattern matched.
	switch {
	case strings.HasPrefix(url, "https://www.youtube.com/watch?v="):
		return url[len("https://www.youtube.com/watch?v="):], nil
	case strings.HasPrefix(url, "https://youtu.be/"):
		return url[len("https://youtu.be/"):], nil
	case strings.HasPrefix(url, "https://www.youtube.com/shorts/"):
		return url[len("https://www.youtube.com/shorts/"):], nil
	default:
		return "", fmt.Errorf(validFormatsMessage)
	}
}

// ReconstructURL builds a canonical YouTube URL from a video ID.
// The canonical form is https://www.youtube.com/watch?v={id}.
func ReconstructURL(videoID string) string {
	return "https://www.youtube.com/watch?v=" + videoID
}

// ValidateThought trims whitespace from the input and validates thought length.
// It returns the trimmed thought on success, or an error with a descriptive message.
func ValidateThought(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if len(trimmed) < 1 {
		return "", errors.New("thought is required")
	}
	if len(trimmed) > 150 {
		return "", errors.New("thought must be 150 characters or fewer")
	}
	return trimmed, nil
}
