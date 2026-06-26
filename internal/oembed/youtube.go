package oembed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Metadata holds the video title and thumbnail URL fetched from YouTube's oEmbed API.
type Metadata struct {
	Title        string
	ThumbnailURL string
}

// Client wraps an HTTP client for calling YouTube's oEmbed endpoint.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

const defaultBaseURL = "https://www.youtube.com/oembed"

// NewClient creates an OEmbed client with the specified request timeout.
func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: defaultBaseURL,
	}
}

// NewClientWithBaseURL creates an OEmbed client with a custom base URL (used for testing).
func NewClientWithBaseURL(timeout time.Duration, baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
	}
}

// oembedResponse represents the relevant fields from YouTube's oEmbed JSON response.
type oembedResponse struct {
	Title        string `json:"title"`
	ThumbnailURL string `json:"thumbnail_url"`
}

// Fetch retrieves video metadata from YouTube's oEmbed endpoint for the given URL.
// It returns an error on non-200 responses, network failures, or JSON decode issues.
func (c *Client) Fetch(youtubeURL string) (Metadata, error) {
	endpoint := fmt.Sprintf("%s?url=%s&format=json",
		c.baseURL, url.QueryEscape(youtubeURL))

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return Metadata{}, fmt.Errorf("oembed: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Metadata{}, fmt.Errorf("oembed: unexpected status %d", resp.StatusCode)
	}

	var result oembedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Metadata{}, fmt.Errorf("oembed: failed to decode response: %w", err)
	}

	return Metadata{
		Title:        result.Title,
		ThumbnailURL: result.ThumbnailURL,
	}, nil
}
