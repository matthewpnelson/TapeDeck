package plex

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/Chuntttttt/tapedeck/internal/constants"
	"github.com/Chuntttttt/tapedeck/internal/logger"
)

// APIError represents an error response from the Plex API
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("plex API error (status %d): %s", e.StatusCode, e.Message)
}

// IsUnauthorized checks if an error is a 401 Unauthorized from Plex
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusUnauthorized
	}
	return false
}

// Client handles Plex Media Server API operations
type Client struct {
	serverURL  string
	serverID   string
	authToken  string
	httpClient *http.Client
}

// Library represents a Plex library section
type Library struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

// MediaContainer wraps library sections
type MediaContainer struct {
	Size      int       `json:"size"`
	Directory []Library `json:"Directory"`
}

// LibrariesResponse represents the response from /library/sections
type LibrariesResponse struct {
	MediaContainer MediaContainer `json:"MediaContainer"`
}

// MediaItem represents a media item (movie, show, episode, etc.)
type MediaItem struct {
	RatingKey  string `json:"ratingKey"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Year       int    `json:"year,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Thumb      string `json:"thumb,omitempty"`
	ServerName string `json:"-"` // Not from JSON, populated by handler
	ServerID   string `json:"-"` // Not from JSON, populated by handler
}

// Playlist represents a Plex playlist
type Playlist struct {
	RatingKey      string `json:"ratingKey"`
	Title          string `json:"title"`
	Type           string `json:"type"`         // "audio" or "video"
	PlaylistType   string `json:"playlistType"` // "audio", "video", "photo"
	Smart          bool   `json:"smart"`        // true for smart playlists
	Thumb          string `json:"thumb,omitempty"`
	LeafCount      int    `json:"leafCount"` // Number of items in playlist
	Duration       int    `json:"duration,omitempty"`
	CompositeThumb string `json:"composite,omitempty"`
	Summary        string `json:"summary,omitempty"`
}

// MediaItemContainer wraps media items
type MediaItemContainer struct {
	Size     int         `json:"size"`
	Metadata []MediaItem `json:"Metadata"`
}

// LibraryContentsResponse represents the response from /library/sections/{id}/all
type LibraryContentsResponse struct {
	MediaContainer MediaItemContainer `json:"MediaContainer"`
}

// SearchResponse represents the response from /search
type SearchResponse struct {
	MediaContainer MediaItemContainer `json:"MediaContainer"`
}

// PlaylistContainer wraps playlists
type PlaylistContainer struct {
	Size     int        `json:"size"`
	Metadata []Playlist `json:"Metadata"`
}

// PlaylistsResponse represents the response from /playlists/all
type PlaylistsResponse struct {
	MediaContainer PlaylistContainer `json:"MediaContainer"`
}

// NewClient creates a new Plex API client
func NewClient(serverURL, serverID, authToken string, devMode bool) *Client {
	client := &http.Client{Timeout: constants.PlexClientTimeout}

	// In dev mode, skip TLS verification (useful for macOS certificate issues)
	if devMode {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}

	return &Client{
		serverURL:  serverURL,
		serverID:   serverID,
		authToken:  authToken,
		httpClient: client,
	}
}

// GetAuthToken returns the auth token used by this client
func (c *Client) GetAuthToken() string {
	return c.authToken
}

// GetLibraries retrieves all library sections from the Plex server
func (c *Client) GetLibraries(ctx context.Context) ([]Library, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.serverURL+"/library/sections", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Plex-Token", c.authToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    "failed to get libraries",
		}
	}

	var libResp LibrariesResponse
	if err := json.NewDecoder(resp.Body).Decode(&libResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return libResp.MediaContainer.Directory, nil
}

// GetLibraryContents retrieves all items from a specific library
func (c *Client) GetLibraryContents(ctx context.Context, libraryKey string) ([]MediaItem, error) {
	url := fmt.Sprintf("%s/library/sections/%s/all", c.serverURL, libraryKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Plex-Token", c.authToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    "failed to get library contents",
		}
	}

	var contentsResp LibraryContentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&contentsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return contentsResp.MediaContainer.Metadata, nil
}

// Search performs a search across all libraries
// TODO: Shared servers may return 401 Unauthorized when accessed via direct .plex.direct URLs
// even with a valid auth token. This appears to be a Plex permission limitation for shared content.
// Consider investigating alternative access methods or documenting this as a known limitation.
func (c *Client) Search(ctx context.Context, query string) ([]MediaItem, error) {
	searchURL := fmt.Sprintf("%s/search", c.serverURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameter
	q := req.URL.Query()
	q.Add("query", query)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("X-Plex-Token", c.authToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    "failed to search",
		}
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return searchResp.MediaContainer.Metadata, nil
}

// GetPlaylists retrieves all playlists from the Plex server
func (c *Client) GetPlaylists(ctx context.Context) ([]Playlist, error) {
	playlistURL := fmt.Sprintf("%s/playlists/all", c.serverURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, playlistURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Plex-Token", c.authToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    "failed to get playlists",
		}
	}

	var playlistResp PlaylistsResponse
	if err := json.NewDecoder(resp.Body).Decode(&playlistResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return playlistResp.MediaContainer.Metadata, nil
}

// MediaMetadata represents detailed metadata for a media item
type MediaMetadata struct {
	RatingKey string `json:"ratingKey"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Thumb     string `json:"thumb"`
	Year      int    `json:"year"`
	Summary   string `json:"summary"`

	// Episode/season fields
	Index            int    `json:"index"`            // Episode number (for episodes)
	ParentIndex      int    `json:"parentIndex"`      // Season number (for episodes)
	ParentRatingKey  string `json:"parentRatingKey"`  // Season ratingKey (for episodes)
	ParentTitle      string `json:"parentTitle"`      // Season title (for episodes)
	GrandparentTitle string `json:"grandparentTitle"` // Show title (for episodes)

	// Watch status fields
	ViewOffset int `json:"viewOffset"` // Resume position in milliseconds
	ViewCount  int `json:"viewCount"`  // Number of times watched
}

// DVR represents a Plex Live TV DVR device
type DVR struct {
	Key         string `json:"key"`
	LineupTitle string `json:"lineupTitle"` // Tunarr uses lineupTitle instead of title
	Title       string `json:"title"`       // Fallback for other DVR types
}

// GetTitle returns the DVR title, preferring LineupTitle if available
func (d *DVR) GetTitle() string {
	if d.LineupTitle != "" {
		return d.LineupTitle
	}
	return d.Title
}

// ProgramMedia represents the Media array element in a Live TV program response
// This contains channel information for the currently airing program
type ProgramMedia struct {
	ID                int    `json:"id"`
	ChannelCallSign   string `json:"channelCallSign"`
	ChannelIdentifier string `json:"channelIdentifier"`
	ChannelThumb      string `json:"channelThumb"`
	ChannelTitle      string `json:"channelTitle"`
	ChannelVcn        string `json:"channelVcn"`
	ChannelID         int    `json:"channelID"`
	Protocol          string `json:"protocol"`
}

// Channel represents a Live TV channel
// Can be populated from either channel-specific endpoints or from program Media arrays
type Channel struct {
	RatingKey     string         `json:"ratingKey"`
	Title         string         `json:"title"`
	Type          string         `json:"type"` // Should be "channel"
	Thumb         string         `json:"thumb,omitempty"`
	ChannelNumber string         `json:"channelIdentifier,omitempty"`
	CallSign      string         `json:"callSign,omitempty"`
	Media         []ProgramMedia `json:"Media,omitempty"` // Present in program responses, contains channel metadata
}

// DVRContainer wraps DVR devices
type DVRContainer struct {
	Size int   `json:"size"`
	Dvr  []DVR `json:"Dvr"` // Note: Plex uses "Dvr" not "Directory" for Live TV DVRs
}

// ChannelContainer wraps Live TV channels
type ChannelContainer struct {
	Size     int       `json:"size"`
	Metadata []Channel `json:"Metadata"`
}

// DVRResponse represents the response from /livetv/dvrs
type DVRResponse struct {
	MediaContainer DVRContainer `json:"MediaContainer"`
}

// ChannelsResponse represents the response from /livetv/dvrs/{id}/channels
type ChannelsResponse struct {
	MediaContainer ChannelContainer `json:"MediaContainer"`
}

// GetMetadata fetches detailed metadata for a specific media item
func (c *Client) GetMetadata(ctx context.Context, ratingKey string) (*MediaMetadata, error) {
	logger.Info("GetMetadata called", "rating_key", ratingKey, "length", len(ratingKey))
	var reqURL string

	// Check if this is a Live TV rating key (starts with tv.plex.xmltv://)
	// These need to use the provider-specific endpoint instead of /library/metadata
	if isLiveTVRatingKey(ratingKey) {
		logger.Info("Identified as Live TV rating key")
		providerID := c.getLiveTVProviderID(ctx)
		if providerID == "" {
			return nil, fmt.Errorf("no Live TV provider found for Live TV rating key %s", ratingKey)
		}
		// URL-encode the rating key for the API request
		encodedKey := url.PathEscape(ratingKey)
		reqURL = fmt.Sprintf("%s/%s/metadata/%s", c.serverURL, providerID, encodedKey)
		logger.Info("Using Live TV metadata endpoint", "url", reqURL, "rating_key", ratingKey, "encoded_key", encodedKey)
	} else {
		logger.Info("Using standard library metadata endpoint", "rating_key", ratingKey)
		reqURL = fmt.Sprintf("%s/library/metadata/%s", c.serverURL, ratingKey)
	}
	logger.Info("Constructed request URL", "url", reqURL)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Plex-Token", c.authToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to make metadata request", "error", err)
		return nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", "error", err)
		}
	}()

	logger.Info("Received metadata response", "status_code", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		logger.Error("Plex API returned non-OK status for metadata", "status", resp.StatusCode, "url", reqURL)
		return nil, fmt.Errorf("plex returned status %d", resp.StatusCode)
	}

	var result struct {
		MediaContainer struct {
			Metadata []MediaMetadata `json:"Metadata"`
		} `json:"MediaContainer"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.MediaContainer.Metadata) == 0 {
		return nil, fmt.Errorf("no metadata found for ratingKey %s", ratingKey)
	}

	return &result.MediaContainer.Metadata[0], nil
}

// GetChildren fetches child items (seasons for shows, episodes for seasons)
func (c *Client) GetChildren(ctx context.Context, ratingKey string) ([]MediaMetadata, error) {
	reqURL := fmt.Sprintf("%s/library/metadata/%s/children", c.serverURL, ratingKey)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Plex-Token", c.authToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch children: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex returned status %d", resp.StatusCode)
	}

	var result struct {
		MediaContainer struct {
			Metadata []MediaMetadata `json:"Metadata"`
		} `json:"MediaContainer"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.MediaContainer.Metadata, nil
}

// GetNextUnwatchedEpisode finds the next episode to play for a TV show.
// It returns either:
// - A partially watched episode (has viewOffset > 0) to resume
// - The first unwatched episode (viewCount == 0)
// - The first episode (S1E1) if all episodes are watched
func (c *Client) GetNextUnwatchedEpisode(ctx context.Context, showRatingKey string) (*MediaMetadata, error) {
	// Log which token is being used (masked for security)
	tokenPreview := ""
	if len(c.authToken) > 8 {
		tokenPreview = c.authToken[:4] + "..." + c.authToken[len(c.authToken)-4:]
	}
	logger.Info("Finding next unwatched episode", "show_rating_key", showRatingKey, "token_preview", tokenPreview)

	// Get all seasons for the show
	seasons, err := c.GetChildren(ctx, showRatingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get seasons: %w", err)
	}

	if len(seasons) == 0 {
		return nil, fmt.Errorf("no seasons found for show %s", showRatingKey)
	}

	// Sort seasons by index (they should already be sorted, but ensure it)
	// Skip "specials" season (index 0) - process regular seasons first
	var regularSeasons []MediaMetadata
	var specialsSeason *MediaMetadata
	for i := range seasons {
		if seasons[i].Type != "season" {
			continue
		}
		if seasons[i].Index == 0 {
			specialsSeason = &seasons[i]
		} else {
			regularSeasons = append(regularSeasons, seasons[i])
		}
	}

	// Track the first episode for fallback (when all are watched)
	var firstEpisode *MediaMetadata

	// Check each season in order
	for _, season := range regularSeasons {
		episodes, err := c.GetChildren(ctx, season.RatingKey)
		if err != nil {
			logger.Warn("Failed to get episodes for season", "season", season.Index, "error", err)
			continue
		}

		for i := range episodes {
			ep := &episodes[i]
			if ep.Type != "episode" {
				continue
			}

			// Log episode watch status for debugging
			logger.Debug("Checking episode watch status",
				"season", ep.ParentIndex,
				"episode", ep.Index,
				"title", ep.Title,
				"view_count", ep.ViewCount,
				"view_offset", ep.ViewOffset)

			// Track first episode for fallback
			if firstEpisode == nil {
				firstEpisode = ep
			}

			// If this episode is partially watched, resume it
			if ep.ViewOffset > 0 {
				logger.Info("Found partially watched episode",
					"season", ep.ParentIndex,
					"episode", ep.Index,
					"title", ep.Title,
					"view_offset_ms", ep.ViewOffset)
				return ep, nil
			}

			// If this episode is unwatched, play it
			if ep.ViewCount == 0 {
				logger.Info("Found unwatched episode",
					"season", ep.ParentIndex,
					"episode", ep.Index,
					"title", ep.Title)
				return ep, nil
			}
		}
	}

	// All regular episodes watched - check specials if present
	if specialsSeason != nil {
		episodes, err := c.GetChildren(ctx, specialsSeason.RatingKey)
		if err == nil {
			for i := range episodes {
				ep := &episodes[i]
				if ep.Type != "episode" {
					continue
				}
				if ep.ViewOffset > 0 || ep.ViewCount == 0 {
					logger.Info("Found unwatched special",
						"episode", ep.Index,
						"title", ep.Title)
					return ep, nil
				}
			}
		}
	}

	// All episodes watched - return first episode to restart series
	if firstEpisode != nil {
		logger.Info("All episodes watched, restarting series",
			"season", firstEpisode.ParentIndex,
			"episode", firstEpisode.Index,
			"title", firstEpisode.Title)
		return firstEpisode, nil
	}

	return nil, fmt.Errorf("no episodes found for show %s", showRatingKey)
}

// GetDVRs retrieves all Live TV DVR devices from the Plex server
func (c *Client) GetDVRs(ctx context.Context) ([]DVR, error) {
	logger.Info("Fetching DVRs from Plex", "url", c.serverURL+"/livetv/dvrs")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.serverURL+"/livetv/dvrs", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Plex-Token", c.authToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to make DVR request", "error", err)
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		logger.Error("DVR API returned non-OK status", "status", resp.StatusCode)
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    "failed to get DVRs",
		}
	}

	// Read the entire response body for logging and parsing
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body", "error", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Log the raw response for debugging
	logger.Info("Raw DVR API response", "body", string(bodyBytes))

	var dvrResp DVRResponse
	if err := json.Unmarshal(bodyBytes, &dvrResp); err != nil {
		logger.Error("Failed to decode DVR response", "error", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	logger.Info("Successfully fetched DVRs", "count", len(dvrResp.MediaContainer.Dvr), "size", dvrResp.MediaContainer.Size)
	return dvrResp.MediaContainer.Dvr, nil
}

// GetLiveTVChannels retrieves all Live TV channels from a specific DVR
func (c *Client) GetLiveTVChannels(ctx context.Context, dvrKey string) ([]Channel, error) {
	url := fmt.Sprintf("%s/livetv/dvrs/%s/channels", c.serverURL, dvrKey)
	logger.Info("Fetching channels from DVR endpoint", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Plex-Token", c.authToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to make channels request", "error", err)
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		logger.Error("Channels API returned non-OK status", "status", resp.StatusCode, "url", url)

		// Try alternative endpoint for Tunarr/XMLTV-based DVRs
		logger.Info("Trying alternative channel listing endpoint")
		return c.getChannelsFromLibrary(ctx, dvrKey)
	}

	// Read response body for logging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read channels response body", "error", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	logger.Info("Raw channels API response", "body", string(bodyBytes))

	var channelsResp ChannelsResponse
	if err := json.Unmarshal(bodyBytes, &channelsResp); err != nil {
		logger.Error("Failed to decode channels response", "error", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return channelsResp.MediaContainer.Metadata, nil
}

// getChannelsFromLibrary tries to get channels from alternative endpoints (fallback for Tunarr)
func (c *Client) getChannelsFromLibrary(ctx context.Context, dvrKey string) ([]Channel, error) {
	// Try 1: Parse /media/providers to find Live TV provider and use its endpoints
	logger.Info("Checking /media/providers for Live TV provider")
	providerID := c.getLiveTVProviderID(ctx)
	if providerID != "" {
		logger.Info("Found Live TV provider", "provider_id", providerID)

		// Try provider-specific endpoints
		endpoints := []string{
			fmt.Sprintf("/%s/watchnow/all", providerID), // "All Channels" endpoint - try this first
			fmt.Sprintf("/%s/watchnow", providerID),
			fmt.Sprintf("/%s/grid", providerID),
			fmt.Sprintf("/%s/sections/2/all", providerID),        // Shows section
			fmt.Sprintf("/%s/sections/2/all?type=4", providerID), // Browse shows with type filter
			fmt.Sprintf("/%s/hubs/discover", providerID),
		}

		for _, endpoint := range endpoints {
			logger.Info("Trying provider endpoint", "endpoint", endpoint)
			if programs := c.tryEndpoint(ctx, endpoint); len(programs) > 0 {
				// Extract channels from the currently playing programs
				// This works around the Plex/Tunarr limitation where only currently
				// airing programs have accessible metadata endpoints
				channels := extractChannelsFromPrograms(programs)
				if len(channels) > 0 {
					return channels, nil
				}
			}
		}
	}

	// Try 2: /livetv/lineups endpoint
	logger.Info("Trying /livetv/lineups endpoint")
	if channels := c.tryEndpoint(ctx, "/livetv/lineups"); len(channels) > 0 {
		return channels, nil
	}

	// Try 3: Check library sections for livetv type
	libraries, err := c.GetLibraries(ctx)
	if err == nil {
		for _, lib := range libraries {
			logger.Info("Checking library for channels", "library_title", lib.Title, "library_type", lib.Type, "library_key", lib.Key)

			if lib.Type == "livetv" || lib.Type == "channel" {
				logger.Info("Found potential Live TV library", "key", lib.Key, "title", lib.Title)

				url := fmt.Sprintf("/library/sections/%s/all", lib.Key)
				if channels := c.tryEndpoint(ctx, url); len(channels) > 0 {
					return channels, nil
				}
			}
		}
	}

	logger.Warn("No Plex API endpoint found for channels - this may be a Tunarr/XMLTV limitation")
	logger.Info("NOTE: Tunarr channels may only be accessible through Plex's web UI or native apps, not via API")
	return []Channel{}, nil
}

// getLiveTVProviderID extracts the Live TV provider ID from /media/providers
func (c *Client) getLiveTVProviderID(ctx context.Context) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.serverURL+"/media/providers", nil)
	if err != nil {
		return ""
	}

	req.Header.Set("X-Plex-Token", c.authToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	bodyBytes, _ := io.ReadAll(resp.Body)

	var result struct {
		MediaContainer struct {
			MediaProvider []struct {
				ID         int    `json:"id"`
				Identifier string `json:"identifier"`
				Title      string `json:"title"`
				Types      string `json:"types"`
				Protocols  string `json:"protocols"`
			} `json:"MediaProvider"`
		} `json:"MediaContainer"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return ""
	}

	// Look for Live TV provider (protocols contains "livetv")
	for _, provider := range result.MediaContainer.MediaProvider {
		if provider.Protocols == "livetv" || provider.Title == "Live TV & DVR" {
			logger.Info("Found Live TV media provider", "identifier", provider.Identifier, "title", provider.Title)
			return provider.Identifier
		}
	}

	return ""
}

// tryEndpoint attempts to fetch channels from a given endpoint path
func (c *Client) tryEndpoint(ctx context.Context, path string) []Channel {
	url := c.serverURL + path
	logger.Info("Trying endpoint", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logger.Warn("Failed to create request", "error", err)
		return nil
	}

	req.Header.Set("X-Plex-Token", c.authToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Warn("Failed to make request", "endpoint", path, "error", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Info("Endpoint returned non-OK status", "endpoint", path, "status", resp.StatusCode)
		return nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warn("Failed to read response body", "error", err)
		return nil
	}

	logger.Info("Endpoint response", "endpoint", path, "body", string(bodyBytes))

	// Try to parse as ChannelsResponse
	var channelsResp ChannelsResponse
	if err := json.Unmarshal(bodyBytes, &channelsResp); err == nil && len(channelsResp.MediaContainer.Metadata) > 0 {
		logger.Info("Successfully parsed channels from endpoint", "endpoint", path, "count", len(channelsResp.MediaContainer.Metadata))
		return channelsResp.MediaContainer.Metadata
	}

	return nil
}

// GetAllLiveTVChannels retrieves all Live TV channels from all DVRs on the server
func (c *Client) GetAllLiveTVChannels(ctx context.Context) ([]MediaItem, error) {
	logger.Info("Fetching all Live TV channels")

	// Get all DVRs
	dvrs, err := c.GetDVRs(ctx)
	if err != nil {
		logger.Error("Failed to get DVRs", "error", err)
		return nil, err
	}

	if len(dvrs) == 0 {
		logger.Warn("No DVRs found on Plex server - Live TV may not be configured")
		return []MediaItem{}, nil
	}

	logger.Info("Found DVRs, fetching channels", "dvr_count", len(dvrs))

	var allChannels []MediaItem

	// Fetch channels from each DVR
	for _, dvr := range dvrs {
		dvrTitle := dvr.GetTitle()
		logger.Info("Fetching channels from DVR", "dvr_title", dvrTitle, "dvr_key", dvr.Key)

		channels, err := c.GetLiveTVChannels(ctx, dvr.Key)
		if err != nil {
			logger.Warn("Failed to get channels from DVR", "dvr", dvrTitle, "error", err)
			continue
		}

		logger.Info("Got channels from DVR", "dvr_title", dvrTitle, "channel_count", len(channels))

		// Convert Channel to MediaItem format for consistency
		for _, ch := range channels {
			allChannels = append(allChannels, MediaItem{
				RatingKey: ch.RatingKey,
				Title:     formatChannelTitle(ch),
				Type:      "channel",
				Thumb:     ch.Thumb,
			})
		}
	}

	logger.Info("Total Live TV channels retrieved", "total_count", len(allChannels))
	return allChannels, nil
}

// formatChannelTitle formats channel title with channel number and call sign
func formatChannelTitle(ch Channel) string {
	if ch.ChannelNumber != "" && ch.CallSign != "" {
		return fmt.Sprintf("%s - %s (%s)", ch.ChannelNumber, ch.Title, ch.CallSign)
	} else if ch.ChannelNumber != "" {
		return fmt.Sprintf("%s - %s", ch.ChannelNumber, ch.Title)
	} else if ch.CallSign != "" {
		return fmt.Sprintf("%s (%s)", ch.Title, ch.CallSign)
	}
	return ch.Title
}

// isLiveTVRatingKey checks if a rating key is for a Live TV item
// Live TV rating keys start with "tv.plex.xmltv://" (may be URL-encoded as "tv%2Eplex%2Exmltv")
func isLiveTVRatingKey(ratingKey string) bool {
	// Check decoded version (most common after router processes URL)
	if len(ratingKey) > 16 && ratingKey[:16] == "tv.plex.xmltv://" {
		return true
	}
	// Check URL-encoded version (in case it comes pre-encoded)
	if len(ratingKey) > 15 && ratingKey[:15] == "tv%2Eplex%2Exmltv" {
		return true
	}
	return false
}

// extractChannelsFromPrograms extracts unique channels from currently playing programs
// Since Plex/Tunarr only expose metadata for currently airing programs, we extract
// channel information from the Media array of each program to build a stable channel list.
func extractChannelsFromPrograms(programs []Channel) []Channel {
	// Map of channelID -> Channel to deduplicate
	channelMap := make(map[int]*Channel)

	for _, program := range programs {
		// Each program should have a Media array with channel information
		if len(program.Media) == 0 {
			logger.Warn("Program has no Media array, skipping", "program_title", program.Title, "rating_key", program.RatingKey)
			continue
		}

		// Use the first Media element (there should only be one for Live TV)
		media := program.Media[0]
		channelID := media.ChannelID

		// Skip if we've already seen this channel
		if _, exists := channelMap[channelID]; exists {
			continue
		}

		// URL-decode the rating key since it comes double-encoded from the API
		// First decode: tv%2Eplex%2Exmltv... → tv.plex.xmltv://episode/Llama%20Llama/1/6
		// Second decode: tv.plex.xmltv://episode/Llama%20Llama/1/6 → tv.plex.xmltv://episode/Llama Llama/1/6
		var decodedRatingKey string
		decodedOnce, err := url.QueryUnescape(program.RatingKey)
		if err != nil {
			logger.Warn("Failed first URL decode, using as-is", "rating_key", program.RatingKey, "error", err)
			decodedRatingKey = program.RatingKey
		} else {
			// Try second decode to handle double encoding
			decodedTwice, err := url.QueryUnescape(decodedOnce)
			if err != nil {
				// If second decode fails, use first decode result
				logger.Info("Single URL decode successful", "decoded", decodedOnce)
				decodedRatingKey = decodedOnce
			} else {
				// Successfully double-decoded
				logger.Info("Double URL decode successful", "decoded", decodedTwice)
				decodedRatingKey = decodedTwice
			}
		}

		// Create a Channel entry for this channel with the current program's rating key
		channelMap[channelID] = &Channel{
			RatingKey:     decodedRatingKey, // Link to the currently airing program (decoded)
			Title:         media.ChannelTitle,
			Type:          "channel",
			Thumb:         media.ChannelThumb,
			ChannelNumber: media.ChannelIdentifier,
			CallSign:      media.ChannelCallSign,
		}

		logger.Info("Extracted channel from program",
			"channel_id", channelID,
			"channel_title", media.ChannelTitle,
			"channel_number", media.ChannelIdentifier,
			"current_program", program.Title,
			"program_rating_key_encoded", program.RatingKey,
			"program_rating_key_decoded", decodedRatingKey)
	}

	// Convert map to slice
	var channels []Channel
	for _, ch := range channelMap {
		channels = append(channels, *ch)
	}

	logger.Info("Extracted unique channels from programs", "channel_count", len(channels), "program_count", len(programs))
	return channels
}
