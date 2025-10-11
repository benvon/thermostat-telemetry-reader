package ecobee

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	// #nosec G101
	ecobeeTokenURL = "https://api.ecobee.com/token"
	ecobeeAPIURL   = "https://api.ecobee.com/1"
)

// AuthManager implements authentication for the Ecobee API
type AuthManager struct {
	clientID     string
	refreshToken string
	accessToken  string
	tokenExpiry  time.Time
	httpClient   *http.Client
}

// NewAuthManager creates a new Ecobee authentication manager
func NewAuthManager(clientID, refreshToken string) *AuthManager {
	return &AuthManager{
		clientID:     clientID,
		refreshToken: refreshToken,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// tokenResponse represents the response from the token endpoint
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

// RefreshToken refreshes the authentication token
func (a *AuthManager) RefreshToken(ctx context.Context) error {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", a.refreshToken)
	data.Set("client_id", a.clientID)

	req, err := http.NewRequestWithContext(ctx, "POST", ecobeeTokenURL, nil)
	if err != nil {
		return fmt.Errorf("creating refresh token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.URL.RawQuery = data.Encode()

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("refreshing token: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token refresh failed with status %d", resp.StatusCode)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("decoding token response: %w", err)
	}

	a.accessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		a.refreshToken = tokenResp.RefreshToken
	}
	a.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return nil
}

// GetAccessToken returns the current access token, refreshing if needed
func (a *AuthManager) GetAccessToken(ctx context.Context) (string, error) {
	if !a.IsTokenValid(ctx) {
		if err := a.RefreshToken(ctx); err != nil {
			return "", fmt.Errorf("refreshing token: %w", err)
		}
	}
	return a.accessToken, nil
}

// IsTokenValid checks if the current token is valid
func (a *AuthManager) IsTokenValid(ctx context.Context) bool {
	return a.accessToken != "" && time.Now().Before(a.tokenExpiry.Add(-5*time.Minute))
}

// makeAuthenticatedRequest makes an authenticated request to the Ecobee API
func (a *AuthManager) makeAuthenticatedRequest(ctx context.Context, endpoint string, params map[string]string) (*http.Response, error) {
	token, err := a.GetAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", ecobeeAPIURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Add query parameters
	q := req.URL.Query()
	q.Set("json", `{"selection":{"selectionType":"registered","selectionMatch":"","includeRuntime":true,"includeSettings":true,"includeEvents":true,"includeProgram":true,"includeEquipmentStatus":true}}`)
	for key, value := range params {
		q.Set(key, value)
	}
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		// Try to refresh token and retry once
		if err := a.RefreshToken(ctx); err != nil {
			return nil, fmt.Errorf("refreshing token after 401: %w", err)
		}

		token, err := a.GetAccessToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting refreshed token: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+token)
		resp, err = a.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("retrying request after token refresh: %w", err)
		}
	}

	return resp, nil
}
