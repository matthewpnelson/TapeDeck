// Package plex provides Plex Media Server API client functionality.
package plex

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Chuntttttt/tapedeck/internal/constants"
	"github.com/Chuntttttt/tapedeck/internal/logger"
)

// AuthClientInterface defines the methods required for Plex authentication
type AuthClientInterface interface {
	RequestPIN(ctx context.Context) (*PINResponse, error)
	CheckPIN(ctx context.Context, pinID int) (*PINCheckResponse, error)
}

// AuthClient handles Plex authentication operations
type AuthClient struct {
	baseURL     string
	clientID    string
	productName string
	version     string
	platform    string
	device      string
	httpClient  *http.Client
}

// PINResponse represents the response from requesting a new PIN
type PINResponse struct {
	ID        int       `json:"id"`
	Code      string    `json:"code"`
	ExpiresIn int       `json:"expiresIn"`
	CreatedAt time.Time `json:"createdAt"`
}

// PINCheckResponse represents the response when checking a PIN's status
type PINCheckResponse struct {
	ID        int    `json:"id"`
	Code      string `json:"code"`
	AuthToken string `json:"authToken"`
}

// NewAuthClient creates a new Plex authentication client
func NewAuthClient(baseURL, clientID, productName string, devMode bool) *AuthClient {
	client := &http.Client{Timeout: constants.PlexAuthTimeout}

	// In dev mode, skip TLS verification (useful for macOS certificate issues)
	if devMode {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}

	return &AuthClient{
		baseURL:     baseURL,
		clientID:    clientID,
		productName: productName,
		version:     "1.0.0",
		platform:    "Web",
		device:      "TapeDeck",
		httpClient:  client,
	}
}

// RequestPIN requests a new authentication PIN from Plex
func (c *AuthClient) RequestPIN(ctx context.Context) (*PINResponse, error) {
	if c.clientID == "" {
		return nil, fmt.Errorf("client ID is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/pins", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("X-Plex-Client-Identifier", c.clientID)
	req.Header.Set("X-Plex-Product", c.productName)
	req.Header.Set("X-Plex-Version", c.version)
	req.Header.Set("X-Plex-Platform", c.platform)
	req.Header.Set("X-Plex-Device", c.device)
	req.Header.Set("X-Plex-Device-Name", c.device)
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

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var pinResp PINResponse
	if err := json.NewDecoder(resp.Body).Decode(&pinResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &pinResp, nil
}

// CheckPIN checks the status of a PIN to see if it has been authorized
func (c *AuthClient) CheckPIN(ctx context.Context, pinID int) (*PINCheckResponse, error) {
	url := fmt.Sprintf("%s/api/v2/pins/%d", c.baseURL, pinID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Plex-Client-Identifier", c.clientID)
	req.Header.Set("X-Plex-Product", c.productName)
	req.Header.Set("X-Plex-Version", c.version)
	req.Header.Set("X-Plex-Platform", c.platform)
	req.Header.Set("X-Plex-Device", c.device)
	req.Header.Set("X-Plex-Device-Name", c.device)
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
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var checkResp PINCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&checkResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &checkResp, nil
}

// GetAuthURL constructs the Plex authentication URL for user authorization
//
// NOTE: This method is currently unused due to Plex OAuth being broken for 3rd party apps
// in v4.152.0 (Sept 2025). We use PIN polling instead until Plex fixes their OAuth.
// See: https://forums.plex.tv/t/plex-oauth-authenticate-with-plex-broken-after-plex-web-update-v4-152-0/931098
//
// TODO: Re-enable forwardUrl redirect flow when Plex fixes OAuth
func (c *AuthClient) GetAuthURL(pinCode, forwardURL string) string {
	params := url.Values{}
	params.Add("clientID", c.clientID)
	params.Add("code", pinCode)

	// Only add forwardUrl for redirect flow
	if forwardURL != "" {
		params.Add("forwardUrl", forwardURL)
	}

	return fmt.Sprintf("https://app.plex.tv/auth#!?%s", params.Encode())
}

// HomeUser represents a Plex Home user (regular or managed/restricted)
type HomeUser struct {
	ID         string `xml:"id,attr"`
	UUID       string `xml:"uuid,attr"`
	Title      string `xml:"title,attr"`
	Username   string `xml:"username,attr"`
	Email      string `xml:"email,attr"`
	Thumb      string `xml:"thumb,attr"`
	Restricted bool   `xml:"restricted,attr"` // true = managed user (child account)
	Admin      bool   `xml:"admin,attr"`
	Guest      bool   `xml:"guest,attr"`
	Protected  bool   `xml:"protected,attr"` // true = user has a PIN
}

// homeUsersResponse represents the XML response from GET /api/v2/home/users
type homeUsersResponse struct {
	XMLName xml.Name   `xml:"home"`
	Users   []HomeUser `xml:"users>user"`
}

// switchUserResponse represents the XML response from POST /api/home/users/{id}/switch
type switchUserResponse struct {
	XMLName             xml.Name `xml:"user"`
	AuthenticationToken string   `xml:"authenticationToken,attr"`
}

// GetHomeUsers fetches all users in the Plex Home
// This returns both regular users and managed/restricted users (child accounts)
func (c *AuthClient) GetHomeUsers(ctx context.Context, authToken string) ([]HomeUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/home/users", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Plex-Token", authToken)
	req.Header.Set("X-Plex-Client-Identifier", c.clientID)
	req.Header.Set("X-Plex-Product", c.productName)
	req.Header.Set("Accept", "application/xml")

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
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var homeResp homeUsersResponse
	if err := xml.NewDecoder(resp.Body).Decode(&homeResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	logger.Info("Fetched Plex Home users", "count", len(homeResp.Users))
	return homeResp.Users, nil
}

// SwitchHomeUser switches to a specific Plex Home user and returns their auth token
// This allows making API calls in the context of that user (e.g., to see their watch history)
// The userID parameter is the numeric user ID (not UUID)
func (c *AuthClient) SwitchHomeUser(ctx context.Context, adminToken, userID string) (string, error) {
	switchURL := fmt.Sprintf("%s/api/home/users/%s/switch", c.baseURL, userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, switchURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Plex-Token", adminToken)
	req.Header.Set("X-Plex-Client-Identifier", c.clientID)
	req.Header.Set("X-Plex-Product", c.productName)
	req.Header.Set("Accept", "application/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var switchResp switchUserResponse
	if err := xml.NewDecoder(resp.Body).Decode(&switchResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if switchResp.AuthenticationToken == "" {
		return "", fmt.Errorf("no authentication token in response")
	}

	logger.Info("Switched to Plex Home user", "user_id", userID)
	return switchResp.AuthenticationToken, nil
}
