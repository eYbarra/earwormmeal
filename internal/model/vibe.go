package model

import "time"

// Vibe represents a single post on the Earworm board — a YouTube link paired with a thought.
type Vibe struct {
	ID           int64     `json:"id"`
	YouTubeURL   string    `json:"youtube_url"`
	VideoTitle   string    `json:"video_title"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Thought      string    `json:"thought"`
	Likes        int64     `json:"likes"`
	Dislikes     int64     `json:"dislikes"`
	NetScore     int64     `json:"net_score"`
	CreatedAt    time.Time `json:"created_at"`
	Author       string    `json:"author"`
}

// WSMessage is the envelope for all WebSocket messages broadcast to clients.
type WSMessage struct {
	Type    string      `json:"type"` // "new_vibe" | "connected_count" | "vote_update"
	Payload interface{} `json:"payload"`
}
