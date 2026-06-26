package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/earworm/vibesboard/internal/db"
	"github.com/earworm/vibesboard/internal/identity"
	"github.com/earworm/vibesboard/internal/model"
	"github.com/earworm/vibesboard/internal/oembed"
	"github.com/earworm/vibesboard/internal/ratelimit"
)

// broadcaster is a local interface for WebSocket broadcasting.
// The hub package will satisfy this once implemented.
type broadcaster interface {
	Broadcast(msg []byte)
}

// youtubeURLRegex is kept for backward compatibility in tests.
// Prefer ValidateYouTubeURL() from validation.go for new code.
var youtubeURLRegex = youtubeURLPatterns

// VibeHandler handles REST operations for vibes.
type VibeHandler struct {
	store       *db.Store
	oembed      *oembed.Client
	hub         broadcaster
	logger      *slog.Logger
	identityGen *identity.Generator
}

// NewVibeHandler creates a new VibeHandler with the given dependencies.
func NewVibeHandler(store *db.Store, oembed *oembed.Client, hub broadcaster, logger *slog.Logger, identityGen *identity.Generator) *VibeHandler {
	return &VibeHandler{
		store:       store,
		oembed:      oembed,
		hub:         hub,
		logger:      logger,
		identityGen: identityGen,
	}
}

// createRequest represents the JSON body for creating a vibe.
type createRequest struct {
	YouTubeURL string `json:"youtube_url"`
	Thought    string `json:"thought"`
}

// Create handles POST /api/vibes — validates input, fetches oEmbed metadata, inserts vibe, and broadcasts.
func (h *VibeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Validate youtube_url
	if err := ValidateYouTubeURL(req.YouTubeURL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate thought
	thought, err := ValidateThought(req.Thought)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Compute author display name from client IP
	ip := ratelimit.ExtractIP(r)
	authorName := h.identityGen.Generate(ip)

	// Fetch oEmbed metadata (non-fatal on failure)
	var videoTitle, thumbnailURL string
	meta, err := h.oembed.Fetch(req.YouTubeURL)
	if err != nil {
		h.logger.Warn("oEmbed fetch failed, proceeding without metadata", "url", req.YouTubeURL, "error", err)
	} else {
		videoTitle = meta.Title
		thumbnailURL = meta.ThumbnailURL
	}

	// Insert into database
	vibe := &model.Vibe{
		YouTubeURL:   req.YouTubeURL,
		VideoTitle:   videoTitle,
		ThumbnailURL: thumbnailURL,
		Thought:      thought,
		Author:       authorName,
	}
	if err := h.store.Insert(vibe); err != nil {
		h.logger.Error("failed to insert vibe", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save vibe")
		return
	}

	// Broadcast via hub if available
	if h.hub != nil {
		msg := model.WSMessage{
			Type:    "new_vibe",
			Payload: vibe,
		}
		data, err := json.Marshal(msg)
		if err == nil {
			h.hub.Broadcast(data)
		}
	}

	// Return 201 with created vibe
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(vibe)
}

// List handles GET /api/vibes — returns the 50 most recent vibes.
func (h *VibeHandler) List(w http.ResponseWriter, r *http.Request) {
	vibes, err := h.store.ListRecent(50)
	if err != nil {
		h.logger.Error("failed to list vibes", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list vibes")
		return
	}

	// Return [] not null when there are no vibes
	if vibes == nil {
		vibes = make([]model.Vibe, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(vibes)
}

// Delete handles DELETE /api/vibes/{id} — deletes a vibe by ID.
func (h *VibeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/vibes/")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "missing vibe ID")
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid vibe ID")
		return
	}

	found, err := h.store.Delete(id)
	if err != nil {
		h.logger.Error("failed to delete vibe", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete vibe")
		return
	}

	if !found {
		writeError(w, http.StatusNotFound, "vibe not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// voteRequest represents the JSON body for voting on a vibe.
type voteRequest struct {
	Direction string `json:"direction"`
}

// Vote handles POST /api/vibes/{id}/vote — records an upvote or downvote.
func (h *VibeHandler) Vote(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid vibe ID")
		return
	}

	var req voteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Direction != "up" && req.Direction != "down" {
		writeError(w, http.StatusBadRequest, `direction must be "up" or "down"`)
		return
	}

	vibe, err := h.store.Vote(id, req.Direction)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "vibe not found")
			return
		}
		h.logger.Error("failed to record vote", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to record vote")
		return
	}

	// Broadcast vote_update via WebSocket hub.
	if h.hub != nil {
		msg := model.WSMessage{
			Type: "vote_update",
			Payload: map[string]interface{}{
				"id":        vibe.ID,
				"likes":     vibe.Likes,
				"dislikes":  vibe.Dislikes,
				"net_score": vibe.NetScore,
			},
		}
		data, err := json.Marshal(msg)
		if err == nil {
			h.hub.Broadcast(data)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(vibe)
}

// writeError writes a JSON error response with the given status code and message.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
