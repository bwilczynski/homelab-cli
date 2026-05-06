package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type deviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type tokenErrorResponse struct {
	Error string `json:"error"`
}

// Login runs the OAuth2 device authorization flow and returns credentials on success.
// User-facing instructions are written to w.
func Login(ctx context.Context, endpoints *OIDCEndpoints, clientID string, w io.Writer) (*Credentials, error) {
	dcResp, err := requestDeviceCode(ctx, endpoints.DeviceAuthorizationEndpoint, clientID)
	if err != nil {
		return nil, err
	}

	uri := dcResp.VerificationURIComplete
	if uri == "" {
		uri = dcResp.VerificationURI
	}
	fmt.Fprintf(w, "Open:  %s\nCode:  %s\nWaiting for authorization...\n", uri, dcResp.UserCode)

	interval := dcResp.Interval
	if interval == 0 {
		interval = 5
	}
	deadline := time.Now().Add(time.Duration(dcResp.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(interval) * time.Second):
		}

		tok, retry, err := pollToken(ctx, endpoints.TokenEndpoint, clientID, dcResp.DeviceCode, &interval)
		if err != nil {
			return nil, err
		}
		if retry {
			continue
		}

		return &Credentials{
			AccessToken:   tok.AccessToken,
			RefreshToken:  tok.RefreshToken,
			TokenType:     tok.TokenType,
			ExpiresAt:     time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second),
			ClientID:      clientID,
			TokenEndpoint: endpoints.TokenEndpoint,
		}, nil
	}

	return nil, fmt.Errorf("device authorization expired")
}

func requestDeviceCode(ctx context.Context, endpoint, clientID string) (*deviceCodeResponse, error) {
	data := url.Values{
		"client_id": {clientID},
		"scope":     {"openid profile email offline_access"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device authorization request failed (status %d)", resp.StatusCode)
	}

	var dcResp deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcResp); err != nil {
		return nil, fmt.Errorf("parsing device code response: %w", err)
	}
	return &dcResp, nil
}

// pollToken polls the token endpoint once. Returns (token, shouldRetry, error).
func pollToken(ctx context.Context, endpoint, clientID, deviceCode string, interval *int) (*tokenResponse, bool, error) {
	data := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":   {clientID},
		"device_code": {deviceCode},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("polling token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}

	if resp.StatusCode == http.StatusOK {
		var tok tokenResponse
		if err := json.Unmarshal(body, &tok); err != nil {
			return nil, false, fmt.Errorf("parsing token response: %w", err)
		}
		return &tok, false, nil
	}

	var errResp tokenErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return nil, false, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	switch errResp.Error {
	case "authorization_pending":
		return nil, true, nil
	case "slow_down":
		*interval += 5
		return nil, true, nil
	case "expired_token":
		return nil, false, fmt.Errorf("device authorization expired")
	case "access_denied":
		return nil, false, fmt.Errorf("authorization denied")
	default:
		return nil, false, fmt.Errorf("token error: %s", errResp.Error)
	}
}
