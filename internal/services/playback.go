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
// If plexUserUUID is provided, the factory should switch to that Plex Home user's context
type PlexClientFactory func(serverID string, userID int64, plexUserUUID string) PlexClient

// PlexClient defines the Plex operations needed for playback
type PlexClient interface {
	GetNextUnwatchedEpisode(ctx context.Context, showRatingKey string) (*plex.MediaMetadata, error)
	GetChildren(ctx context.Context, ratingKey string) ([]plex.MediaMetadata, error)
	GetMetadata(ctx context.Context, ratingKey string) (*plex.MediaMetadata, error)
	GetAuthToken() string
}

const (
	// PlayerInfuse sends Infuse deep links instead of plex:// links.
	// The Plex tvOS app (2026.18+) opens on plex:// links but ignores the item in them.
	PlayerInfuse = "infuse"

	// PlayerPlexInfuseFallback sends the plex:// link first and sends the Infuse link only when
	// Plex isn't playing within plexCheckTimeout. When Plex fixes deep links, taps open in Plex again.
	PlayerPlexInfuseFallback = "plex-infuse-fallback"

	plexAppID = "com.plexapp.plex"
)

// PlaybackService handles media playback orchestration
type PlaybackService struct {
	db                DBClient
	haRest            HARestClient
	plexClientFactory PlexClientFactory
	wakeTime          time.Duration
	plexCheckTimeout  time.Duration

	// Player is the Apple TV app that plays cards: "" or "plex" for Plex,
	// PlayerInfuse for Infuse, or PlayerPlexInfuseFallback
	Player string
}

// DBClient defines the database operations needed for playback
type DBClient interface {
	GetCardMappingByTagID(ctx context.Context, tagID string) (*models.CardMapping, error)
	CreatePlaybackLog(ctx context.Context, log *models.PlaybackLog) (int64, error)
	GetLastWatchedEpisode(ctx context.Context, plexAccountID, showRatingKey string) (*models.WatchProgress, error)
	DeleteWatchProgressForShow(ctx context.Context, plexAccountID, showRatingKey string) error
}

// HARestClient defines the HA REST operations needed for playback
type HARestClient interface {
	GetEntityState(ctx context.Context, entityID string) (string, error)
	GetEntityApp(ctx context.Context, entityID string) (state, appID string, err error)
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
	PlexUserUUID  string // Optional: Plex Home user UUID for watch history
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
		wakeTime:          constants.AppleTVWakeTime,
		plexCheckTimeout:  constants.PlexDeepLinkTimeout,
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
		PlexUserUUID:  mapping.PlexUserUUID,
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

	// Get Plex clients for playback
	// We need two tokens:
	// 1. Admin token - for API calls (managed users get 401 on direct server API)
	// 2. Managed user token - for deep link (to set playback context)
	var adminClient PlexClient
	var authToken string // Token to use in deep link

	if s.plexClientFactory != nil {
		// Always get admin client for API calls (empty plexUserUUID = admin)
		adminClient = s.plexClientFactory(req.PlexServerID, req.UserID, "")

		// If a Plex Home user is specified, get their token for the deep link
		if req.PlexUserUUID != "" {
			managedClient := s.plexClientFactory(req.PlexServerID, req.UserID, req.PlexUserUUID)
			if managedClient != nil {
				authToken = managedClient.GetAuthToken()
				logger.Info("Using managed user token for deep link",
					"plex_user_uuid", req.PlexUserUUID)
			}
		}

		// Fall back to admin token if no managed user token
		if authToken == "" && adminClient != nil {
			authToken = adminClient.GetAuthToken()
		}
	}

	// For TV shows, resolve to the next unwatched episode
	// For managed users, check local watch progress first (from Plex webhooks)
	// Then fall back to admin's watch history from Plex API
	mediaID := req.MediaID
	if req.MediaType == "show" && adminClient != nil {
		var episode *plex.MediaMetadata
		var foundFromLocal bool

		// For managed users, check local watch progress first
		if req.PlexUserUUID != "" {
			lastWatched, err := s.db.GetLastWatchedEpisode(ctx, req.PlexUserUUID, req.MediaID)
			if err != nil {
				logger.Warn("Failed to query local watch progress",
					"plex_user_uuid", req.PlexUserUUID,
					"show", req.MediaTitle,
					"error", err)
			} else if lastWatched != nil {
				logger.Info("Found local watch progress for managed user",
					"plex_user_uuid", req.PlexUserUUID,
					"last_watched", fmt.Sprintf("S%02dE%02d", lastWatched.SeasonNumber, lastWatched.EpisodeNumber))

				// Find the next episode after the last watched one
				nextEp, restarted, err := s.findNextEpisode(ctx, adminClient, req.MediaID, lastWatched.SeasonNumber, lastWatched.EpisodeNumber)
				if err != nil {
					logger.Warn("Failed to find next episode from local progress",
						"error", err)
				} else if nextEp != nil {
					episode = nextEp
					foundFromLocal = true

					if restarted {
						// Series complete - wipe local watch history and restart fresh
						logger.Info("Series complete for managed user, wiping local watch history and restarting",
							"plex_user_uuid", req.PlexUserUUID,
							"show", req.MediaTitle,
							"episode", fmt.Sprintf("S%02dE%02d - %s", episode.ParentIndex, episode.Index, episode.Title))

						if delErr := s.db.DeleteWatchProgressForShow(ctx, req.PlexUserUUID, req.MediaID); delErr != nil {
							logger.Warn("Failed to delete watch progress", "error", delErr)
						}
					} else {
						logger.Info("Resolved show to next episode (from local progress)",
							"show", req.MediaTitle,
							"episode", fmt.Sprintf("S%02dE%02d - %s", episode.ParentIndex, episode.Index, episode.Title))
					}
				}
			}
		}

		// Fall back to admin's watch history if no local progress
		if episode == nil {
			var err error
			episode, err = adminClient.GetNextUnwatchedEpisode(ctx, req.MediaID)
			if err != nil {
				logger.Warn("Failed to get next episode, playing show directly",
					"show", req.MediaTitle,
					"error", err)
			} else if episode != nil && !foundFromLocal {
				logger.Info("Resolved show to next episode (from admin watch history)",
					"show", req.MediaTitle,
					"episode", fmt.Sprintf("S%02dE%02d - %s", episode.ParentIndex, episode.Index, episode.Title))
			}
		}

		if episode != nil {
			mediaID = episode.RatingKey
		}
	}

	// Build Plex deep link based on media type
	// Include X-Plex-Token to ensure playback uses the correct user context
	var plexURL string
	if req.MediaType == "playlist" {
		// Playlist deep link with shuffle parameter
		if authToken != "" {
			plexURL = fmt.Sprintf(
				"plex://play/?metadataKey=/playlists/%s&server=%s&shuffle=1&X-Plex-Token=%s",
				mediaID,
				req.PlexServerID,
				authToken,
			)
		} else {
			plexURL = fmt.Sprintf(
				"plex://play/?metadataKey=/playlists/%s&server=%s&shuffle=1",
				mediaID,
				req.PlexServerID,
			)
		}
		logger.Info("Built playlist deep link with shuffle", "url", logger.RedactTokens(plexURL))
	} else {
		// Regular library content (movie, show, episode, etc.)
		// For shows, mediaID has been resolved to the next episode's ratingKey
		if authToken != "" {
			plexURL = fmt.Sprintf(
				"plex://play/?metadataKey=/library/metadata/%s&server=%s&X-Plex-Token=%s",
				mediaID,
				req.PlexServerID,
				authToken,
			)
		} else {
			plexURL = fmt.Sprintf(
				"plex://play/?metadataKey=/library/metadata/%s&server=%s",
				mediaID,
				req.PlexServerID,
			)
		}
		logger.Info("Built library content deep link", "url", logger.RedactTokens(plexURL), "has_token", authToken != "")
	}

	contentURL := plexURL
	var fallbackURL string // sent only if Plex doesn't play contentURL
	switch s.Player {
	case PlayerInfuse:
		contentURL = s.infuseOrPlex(ctx, adminClient, req.MediaType, mediaID, plexURL)
	case PlayerPlexInfuseFallback:
		if link := s.infuseOrPlex(ctx, adminClient, req.MediaType, mediaID, plexURL); link != plexURL {
			fallbackURL = link
		}
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

	logger.Info("Apple TV state check",
		"apple_tv", req.AppleTVEntity,
		"state", state)

	// Wake device if needed
	// Note: "idle" can mean the TV is asleep but HA still reports it as idle,
	// so we wake on idle too. Waking an already-awake device is harmless.
	if state == "off" || state == "standby" || state == "idle" {
		logger.Info("Waking Apple TV",
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

	// Play media.
	// A paused Plex app rejects the Open URL command when the Apple TV has gone to
	// sleep or is still mid-launch from a previous deep link - HA surfaces that as a
	// 500. Waking the device and sending it again is exactly what a second card tap
	// does by hand, so do it here instead of making someone tap twice.
	if err := s.haRest.PlayMedia(ctx, req.AppleTVEntity, "url", contentURL); err != nil {
		logger.Warn("Play failed, waking Apple TV and retrying once",
			"apple_tv", req.AppleTVEntity,
			"state", state,
			"error", err)

		if wakeErr := s.wakeAppleTV(ctx, req.AppleTVEntity); wakeErr != nil {
			result.Error = fmt.Errorf("failed to wake Apple TV after play failure: %w", wakeErr)
			return result, result.Error
		}
		result.WokeDevice = true

		if retryErr := s.haRest.PlayMedia(ctx, req.AppleTVEntity, "url", contentURL); retryErr != nil {
			result.Error = fmt.Errorf("failed to play media after wake retry: %w", retryErr)
			return result, result.Error
		}
	}

	if fallbackURL != "" {
		if s.waitForPlex(ctx, req.AppleTVEntity) {
			logger.Info("Plex played the deep link, Infuse fallback not needed", "apple_tv", req.AppleTVEntity)
		} else {
			logger.Warn("Plex didn't play the deep link, sending Infuse link",
				"apple_tv", req.AppleTVEntity, "waited", s.plexCheckTimeout)
			if err := s.haRest.PlayMedia(ctx, req.AppleTVEntity, "url", fallbackURL); err != nil {
				result.Error = fmt.Errorf("failed to play Infuse fallback: %w", err)
				return result, result.Error
			}
		}
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

// waitForPlex reports whether the Apple TV plays in the Plex app before plexCheckTimeout ends.
// It checks app_id as well as state, because an Infuse video that was stopped a moment ago
// can still read "playing".
func (s *PlaybackService) waitForPlex(ctx context.Context, entityID string) bool {
	deadline := time.Now().Add(s.plexCheckTimeout)
	for {
		state, appID, err := s.haRest.GetEntityApp(ctx, entityID)
		if err == nil && state == "playing" && appID == plexAppID {
			return true
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(min(time.Second, remaining)):
		}
	}
}

// infuseOrPlex returns an Infuse link for a movie or episode, or plexURL when Infuse can't play the item.
// Playlists stay on plex:// because Infuse has no playlist link, so they resume working if Plex fixes deep links.
func (s *PlaybackService) infuseOrPlex(ctx context.Context, client PlexClient, mediaType, ratingKey, plexURL string) string {
	if mediaType == "playlist" {
		logger.Warn("Infuse has no playlist link, sending plex:// instead", "rating_key", ratingKey)
		return plexURL
	}
	if client == nil {
		logger.Warn("No Plex client for Infuse link lookup, sending plex:// instead", "rating_key", ratingKey)
		return plexURL
	}

	link, err := infuseLink(ctx, client, ratingKey)
	if err != nil {
		logger.Warn("Could not build Infuse link, sending plex:// instead", "rating_key", ratingKey, "error", err)
		return plexURL
	}
	logger.Info("Built Infuse deep link", "url", link)
	return link
}

// infuseLink builds an Infuse deep link from the TMDB IDs in Plex.
// Infuse matches its library by TMDB, and ?play starts playback without a button press.
func infuseLink(ctx context.Context, client PlexClient, ratingKey string) (string, error) {
	item, err := client.GetMetadata(ctx, ratingKey)
	if err != nil {
		return "", fmt.Errorf("fetch metadata: %w", err)
	}

	switch item.Type {
	case "movie":
		id := item.TMDBID()
		if id == "" {
			return "", fmt.Errorf("movie %s has no TMDB ID in Plex", ratingKey)
		}
		return fmt.Sprintf("infuse://movie/%s?play", id), nil
	case "episode":
		show, err := client.GetMetadata(ctx, item.GrandparentRatingKey)
		if err != nil {
			return "", fmt.Errorf("fetch show metadata: %w", err)
		}
		id := show.TMDBID()
		if id == "" {
			return "", fmt.Errorf("show %s has no TMDB ID in Plex", item.GrandparentRatingKey)
		}
		return fmt.Sprintf("infuse://series/%s-%d-%d?play", id, item.ParentIndex, item.Index), nil
	default:
		return "", fmt.Errorf("no Infuse link for %q items", item.Type)
	}
}

// wakeAppleTV wakes an Apple TV and waits for it to be ready
func (s *PlaybackService) wakeAppleTV(ctx context.Context, entityID string) error {
	if err := s.haRest.TurnOn(ctx, entityID); err != nil {
		return fmt.Errorf("turn_on failed: %w", err)
	}

	// Wait for device to wake up
	logger.Debug("Waiting for Apple TV to wake up",
		"apple_tv", entityID,
		"wait_time", s.wakeTime)

	select {
	case <-time.After(s.wakeTime):
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

// findNextEpisode finds the episode that comes after a given season/episode number.
// Returns (episode, restarted, error). If all episodes have been watched,
// returns S1E1 with restarted=true so the caller can clear watch history.
func (s *PlaybackService) findNextEpisode(ctx context.Context, client PlexClient, showRatingKey string, lastSeason, lastEpisode int) (*plex.MediaMetadata, bool, error) {
	// Get all seasons
	seasons, err := client.GetChildren(ctx, showRatingKey)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get seasons: %w", err)
	}

	// Separate regular seasons from specials, sort by index
	var regularSeasons []plex.MediaMetadata
	var firstEpisode *plex.MediaMetadata
	for _, s := range seasons {
		if s.Type == "season" && s.Index > 0 {
			regularSeasons = append(regularSeasons, s)
		}
	}

	// Find the next episode
	for _, season := range regularSeasons {
		episodes, err := client.GetChildren(ctx, season.RatingKey)
		if err != nil {
			logger.Warn("Failed to get episodes for season", "season", season.Index, "error", err)
			continue
		}

		for _, ep := range episodes {
			if ep.Type != "episode" {
				continue
			}

			// Track first episode for restart fallback
			if firstEpisode == nil {
				firstEpisode = &ep
			}

			// Skip episodes up to and including the last watched
			if season.Index < lastSeason || (season.Index == lastSeason && ep.Index <= lastEpisode) {
				continue
			}

			// Found the next episode
			return &ep, false, nil
		}
	}

	// No more episodes - series complete, return first episode with restart flag
	if firstEpisode != nil {
		return firstEpisode, true, nil
	}
	return nil, false, nil
}
