// Package services provides business logic for TapeDeck operations.
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/Chuntttttt/tapedeck/internal/constants"
	"github.com/Chuntttttt/tapedeck/internal/logger"
	"github.com/Chuntttttt/tapedeck/internal/models"
	"github.com/Chuntttttt/tapedeck/internal/plex"
)

// PlexClientFactory creates a Plex client for a specific server and user
type PlexClientFactory func(serverID string, userID int64) PlexClient

// PlexClient defines the Plex operations needed for playback
type PlexClient interface {
	GetNextUnwatchedEpisode(ctx context.Context, showRatingKey string) (*plex.MediaMetadata, error)
}

// PlaybackService handles media playback orchestration
type PlaybackService struct {
	db                DBClient
	haRest            HARestClient
	plexClientFactory PlexClientFactory
}

// DBClient defines the database operations needed for playback
type DBClient interface {
	GetCardMappingByTagID(ctx context.Context, tagID string) (*models.CardMapping, error)
	CreatePlaybackLog(ctx context.Context, log *models.PlaybackLog) (int64, error)
}

// HARestClient defines the HA REST operations needed for playback
type HARestClient interface {
	GetEntityState(ctx context.Context, entityID string) (string, error)
	TurnOn(ctx context.Context, entityID string) error
	PlayMedia(ctx context.Context, entityID, contentType, contentID string) error
	MediaStop(ctx context.Context, entityID string) error
}

// PlaybackRequest contains all data needed to play media
type PlaybackRequest struct {
	TagID         string
	UserID        int64
	MediaType     string // "movie", "show", "episode", "playlist", etc.
	MediaID       string
	MediaTitle    string
	PlexServerID  string
	AppleTVEntity string
}

// PlaybackResult contains the outcome of a playback request
type PlaybackResult struct {
	Success      bool
	MediaTitle   string
	MediaID      string
	AppleTVState string
	WokeDevice   bool
	Error        error
}

// NewPlaybackService creates a new playback service
func NewPlaybackService(database DBClient, haRest HARestClient, plexFactory PlexClientFactory) *PlaybackService {
	return &PlaybackService{
		db:                database,
		haRest:            haRest,
		plexClientFactory: plexFactory,
	}
}

// PlayByTagID looks up a card mapping and plays the associated media
func (s *PlaybackService) PlayByTagID(ctx context.Context, tagID string) (*PlaybackResult, error) {
	// Look up mapping
	mapping, err := s.db.GetCardMappingByTagID(ctx, tagID)
	if err != nil {
		return nil, fmt.Errorf("no mapping found for tag %s: %w", tagID, err)
	}

	logger.Info("Found mapping for tag",
		"tag_id", tagID,
		"media_type", mapping.MediaType,
		"media_title", mapping.MediaTitle,
		"apple_tv", mapping.AppleTVEntity)

	// Play the media
	return s.Play(ctx, &PlaybackRequest{
		TagID:         tagID,
		UserID:        mapping.UserID,
		MediaType:     mapping.MediaType,
		MediaID:       mapping.MediaID,
		MediaTitle:    mapping.MediaTitle,
		PlexServerID:  mapping.PlexServerID,
		AppleTVEntity: mapping.AppleTVEntity,
	})
}

// Play executes the playback flow
func (s *PlaybackService) Play(ctx context.Context, req *PlaybackRequest) (*PlaybackResult, error) {
	result := &PlaybackResult{
		MediaTitle: req.MediaTitle,
		MediaID:    req.MediaID,
	}

	// Check if HA REST client is available
	if s.haRest == nil {
		result.Error = fmt.Errorf("HA REST client not initialized")
		return result, result.Error
	}

	// For TV shows, resolve to the next unwatched episode
	mediaID := req.MediaID
	if req.MediaType == "show" && s.plexClientFactory != nil {
		plexClient := s.plexClientFactory(req.PlexServerID, req.UserID)
		if plexClient != nil {
			episode, err := plexClient.GetNextUnwatchedEpisode(ctx, req.MediaID)
			if err != nil {
				logger.Warn("Failed to get next episode, playing show directly",
					"show", req.MediaTitle,
					"error", err)
			} else if episode != nil {
				logger.Info("Resolved show to next episode",
					"show", req.MediaTitle,
					"episode", fmt.Sprintf("S%02dE%02d - %s", episode.ParentIndex, episode.Index, episode.Title))
				mediaID = episode.RatingKey
			}
		}
	}

	// Build Plex deep link based on media type
	var plexURL string
	if req.MediaType == "playlist" {
		// Playlist deep link with shuffle parameter
		plexURL = fmt.Sprintf(
			"plex://play/?metadataKey=/playlists/%s&server=%s&shuffle=1",
			mediaID,
			req.PlexServerID,
		)
		logger.Info("Built playlist deep link with shuffle", "url", plexURL)
	} else {
		// Regular library content (movie, show, episode, etc.)
		// For shows, mediaID has been resolved to the next episode's ratingKey
		plexURL = fmt.Sprintf(
			"plex://play/?metadataKey=/library/metadata/%s&server=%s",
			mediaID,
			req.PlexServerID,
		)
		logger.Info("Built library content deep link", "url", plexURL)
	}

	// Check Apple TV state
	state, err := s.haRest.GetEntityState(ctx, req.AppleTVEntity)
	if err != nil {
		logger.Warn("Failed to get Apple TV state, attempting playback anyway",
			"apple_tv", req.AppleTVEntity,
			"error", err)
		state = "unknown"
	}
	result.AppleTVState = state

	// Wake device if needed
	if state == "off" || state == "standby" {
		logger.Info("Apple TV is asleep, waking up",
			"apple_tv", req.AppleTVEntity,
			"state", state)

		if err := s.wakeAppleTV(ctx, req.AppleTVEntity); err != nil {
			result.Error = fmt.Errorf("failed to wake Apple TV: %w", err)
			return result, result.Error
		}
		result.WokeDevice = true
	} else if state == "playing" || state == "paused" {
		// Stop current playback to avoid Plex logo stuck issue
		// When sending deep links while Plex is active, the app can get stuck
		// showing the logo screen while audio plays in the background
		logger.Info("Stopping current playback before starting new media",
			"apple_tv", req.AppleTVEntity,
			"current_state", state)

		if err := s.haRest.MediaStop(ctx, req.AppleTVEntity); err != nil {
			logger.Warn("Failed to stop playback, continuing anyway",
				"apple_tv", req.AppleTVEntity,
				"error", err)
		} else {
			// Brief delay to let Plex process the stop command
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Play media
	if err := s.haRest.PlayMedia(ctx, req.AppleTVEntity, "url", plexURL); err != nil {
		result.Error = fmt.Errorf("failed to play media: %w", err)
		return result, result.Error
	}

	logger.Info("Playback started",
		"media_title", req.MediaTitle,
		"apple_tv", req.AppleTVEntity)

	// Log playback
	if err := s.logPlayback(ctx, req.UserID, req.TagID, req.MediaID, req.MediaTitle); err != nil {
		logger.Warn("Failed to log playback", "error", err)
		// Don't fail the request if logging fails
	}

	result.Success = true
	return result, nil
}

// wakeAppleTV wakes an Apple TV and waits for it to be ready
func (s *PlaybackService) wakeAppleTV(ctx context.Context, entityID string) error {
	if err := s.haRest.TurnOn(ctx, entityID); err != nil {
		return fmt.Errorf("turn_on failed: %w", err)
	}

	// Wait for device to wake up
	logger.Debug("Waiting for Apple TV to wake up",
		"apple_tv", entityID,
		"wait_time", constants.AppleTVWakeTime)

	select {
	case <-time.After(constants.AppleTVWakeTime):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// logPlayback creates a playback log entry
func (s *PlaybackService) logPlayback(ctx context.Context, userID int64, tagID, mediaID, mediaTitle string) error {
	playbackLog := models.NewPlaybackLog(userID, tagID, mediaID, mediaTitle)
	_, err := s.db.CreatePlaybackLog(ctx, playbackLog)
	return err
}
