package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Chuntttttt/tapedeck/internal/db"
	"github.com/Chuntttttt/tapedeck/internal/logger"
	"github.com/Chuntttttt/tapedeck/internal/middleware"
	"github.com/Chuntttttt/tapedeck/internal/models"
)

// PlexWebhookHandler handles incoming webhook requests from Plex
type PlexWebhookHandler struct {
	db *db.DB
}

// PlexWebhookPayload represents the JSON payload from Plex webhooks
type PlexWebhookPayload struct {
	Event   string `json:"event"`
	User    bool   `json:"user"`
	Owner   bool   `json:"owner"`
	Account struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
		Thumb string `json:"thumb"`
	} `json:"Account"`
	Server struct {
		Title string `json:"title"`
		UUID  string `json:"uuid"`
	} `json:"Server"`
	Player struct {
		Title         string `json:"title"`
		UUID          string `json:"uuid"`
		PublicAddress string `json:"publicAddress"`
		Local         bool   `json:"local"`
	} `json:"Player"`
	Metadata struct {
		LibrarySectionType   string          `json:"librarySectionType"`
		RatingKey            string          `json:"ratingKey"`
		Key                  string          `json:"key"`
		ParentRatingKey      string          `json:"parentRatingKey"`
		GrandparentRatingKey string          `json:"grandparentRatingKey"`
		GUID                 json.RawMessage `json:"guid"` // Can be string or array depending on Plex version
		LibrarySectionID     int             `json:"librarySectionID"`
		Type                 string          `json:"type"` // "episode", "movie", "track"
		Title                string          `json:"title"`
		GrandparentTitle     string          `json:"grandparentTitle"` // Show name for episodes
		ParentTitle          string          `json:"parentTitle"`      // Season name for episodes
		ParentIndex          int             `json:"parentIndex"`      // Season number
		Index                int             `json:"index"`            // Episode number
		Summary              string          `json:"summary"`
		Thumb                string          `json:"thumb"`
		Duration             int             `json:"duration"` // Duration in milliseconds
		ViewOffset           int             `json:"viewOffset"`
	} `json:"Metadata"`
}

// NewPlexWebhookHandler creates a new Plex webhook handler
func NewPlexWebhookHandler(database *db.DB) *PlexWebhookHandler {
	return &PlexWebhookHandler{
		db: database,
	}
}

// HandleWebhook handles POST /api/webhook/plex
// Plex sends webhooks as multipart form data with a "payload" field containing JSON
func (h *PlexWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqLogger := middleware.GetLogger(ctx)

	// Parse multipart form (Plex sends payload as form field, optionally with thumbnail)
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB max
		reqLogger.Warn("Failed to parse multipart form", "error", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Get the payload field
	payloadStr := r.FormValue("payload")
	if payloadStr == "" {
		reqLogger.Warn("Plex webhook missing payload field")
		http.Error(w, "Missing payload field", http.StatusBadRequest)
		return
	}

	// Parse the JSON payload
	var payload PlexWebhookPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		reqLogger.Warn("Failed to parse Plex webhook payload", "error", err)
		http.Error(w, "Invalid payload JSON", http.StatusBadRequest)
		return
	}

	// Log the event
	logger.Info("Received Plex webhook",
		"event", payload.Event,
		"account_id", payload.Account.ID,
		"account_name", payload.Account.Title,
		"media_type", payload.Metadata.Type,
		"title", payload.Metadata.Title,
		"server", payload.Server.Title)

	// Only process media.scrobble events (content was 90% watched)
	if payload.Event != "media.scrobble" {
		logger.Debug("Ignoring non-scrobble event", "event", payload.Event)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only process TV episodes (not movies or music)
	if payload.Metadata.Type != "episode" {
		logger.Debug("Ignoring non-episode scrobble", "type", payload.Metadata.Type)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Extract episode info
	showRatingKey := payload.Metadata.GrandparentRatingKey
	if showRatingKey == "" {
		reqLogger.Warn("Episode missing grandparentRatingKey (show ID)")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Create watch progress record
	wp := models.NewWatchProgress(
		strconv.Itoa(payload.Account.ID),
		payload.Account.Title,
		payload.Server.UUID,
		showRatingKey,
		payload.Metadata.RatingKey,
		payload.Metadata.ParentIndex,
		payload.Metadata.Index,
		payload.Metadata.Title,
	)

	// Save to database
	if err := h.db.CreateWatchProgress(ctx, wp); err != nil {
		reqLogger.Error("Failed to save watch progress", "error", err)
		http.Error(w, "Failed to save progress", http.StatusInternalServerError)
		return
	}

	logger.Info("Recorded watch progress",
		"account", payload.Account.Title,
		"show", payload.Metadata.GrandparentTitle,
		"season", payload.Metadata.ParentIndex,
		"episode", payload.Metadata.Index,
		"title", payload.Metadata.Title)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"event":   payload.Event,
		"account": payload.Account.Title,
		"media":   fmt.Sprintf("%s S%02dE%02d", payload.Metadata.GrandparentTitle, payload.Metadata.ParentIndex, payload.Metadata.Index),
	}); err != nil {
		reqLogger.Warn("Failed to encode response", "error", err)
	}
}
