// Package models defines data models for the TapeDeck application.
package models

import (
	"errors"
	"time"
)

// WatchProgress represents a completed episode watch event from Plex webhooks.
// Used to track which episodes each Plex Home user has watched, enabling
// accurate "next episode" resolution for managed users whose tokens can't
// directly query the Plex server API.
type WatchProgress struct {
	ID               int64
	PlexAccountID    string // Plex user ID (e.g., "269359635" for Aldous)
	PlexAccountName  string // Plex username for display
	PlexServerID     string // Server UUID
	ShowRatingKey    string // Parent TV show's ratingKey
	EpisodeRatingKey string // Episode's ratingKey
	SeasonNumber     int
	EpisodeNumber    int
	EpisodeTitle     string
	CompletedAt      time.Time // When the scrobble event was received
	CreatedAt        time.Time
}

// NewWatchProgress creates a new WatchProgress with the given parameters
func NewWatchProgress(
	plexAccountID, plexAccountName, plexServerID string,
	showRatingKey, episodeRatingKey string,
	seasonNumber, episodeNumber int,
	episodeTitle string,
) *WatchProgress {
	now := time.Now()
	return &WatchProgress{
		PlexAccountID:    plexAccountID,
		PlexAccountName:  plexAccountName,
		PlexServerID:     plexServerID,
		ShowRatingKey:    showRatingKey,
		EpisodeRatingKey: episodeRatingKey,
		SeasonNumber:     seasonNumber,
		EpisodeNumber:    episodeNumber,
		EpisodeTitle:     episodeTitle,
		CompletedAt:      now,
		CreatedAt:        now,
	}
}

// Validate checks that all required fields are present
func (wp *WatchProgress) Validate() error {
	if wp.PlexAccountID == "" {
		return errors.New("plex account ID is required")
	}
	if wp.PlexAccountName == "" {
		return errors.New("plex account name is required")
	}
	if wp.PlexServerID == "" {
		return errors.New("plex server ID is required")
	}
	if wp.ShowRatingKey == "" {
		return errors.New("show rating key is required")
	}
	if wp.EpisodeRatingKey == "" {
		return errors.New("episode rating key is required")
	}
	if wp.SeasonNumber < 0 {
		return errors.New("season number must be non-negative")
	}
	if wp.EpisodeNumber <= 0 {
		return errors.New("episode number must be positive")
	}
	return nil
}
