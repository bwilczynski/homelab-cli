package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/auth"
)

func TestLogin_success(t *testing.T) {
	pollCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/device/code":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "dev-code-123",
				"user_code":                 "ABCD-1234",
				"verification_uri":          "http://idp.example.com/activate",
				"verification_uri_complete": "http://idp.example.com/activate?user_code=ABCD-1234",
				"expires_in":                300,
				"interval":                  1,
			})
		case "/token":
			pollCount++
			if pollCount < 2 {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]any{"error": "authorization_pending"})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "access-tok",
				"refresh_token": "refresh-tok",
				"token_type":    "Bearer",
				"expires_in":    3600,
			})
		}
	}))
	defer srv.Close()

	endpoints := &auth.OIDCEndpoints{
		DeviceAuthorizationEndpoint: srv.URL + "/device/code",
		TokenEndpoint:               srv.URL + "/token",
	}

	var buf bytes.Buffer
	creds, err := auth.Login(context.Background(), endpoints, "homelab-cli", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessToken != "access-tok" {
		t.Errorf("unexpected access token: %s", creds.AccessToken)
	}
	if creds.RefreshToken != "refresh-tok" {
		t.Errorf("unexpected refresh token: %s", creds.RefreshToken)
	}
	if creds.ClientID != "homelab-cli" {
		t.Errorf("unexpected client_id: %s", creds.ClientID)
	}
	if creds.TokenEndpoint != srv.URL+"/token" {
		t.Errorf("unexpected token_endpoint: %s", creds.TokenEndpoint)
	}
	out := buf.String()
	if !strings.Contains(out, "ABCD-1234") {
		t.Errorf("expected user_code in output, got: %s", out)
	}
}

func TestLogin_accessDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/device/code":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "dev-code-123",
				"user_code":                 "ABCD-1234",
				"verification_uri":          "http://idp.example.com/activate",
				"verification_uri_complete": "http://idp.example.com/activate?user_code=ABCD-1234",
				"expires_in":                300,
				"interval":                  1,
			})
		case "/token":
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"error": "access_denied"})
		}
	}))
	defer srv.Close()

	endpoints := &auth.OIDCEndpoints{
		DeviceAuthorizationEndpoint: srv.URL + "/device/code",
		TokenEndpoint:               srv.URL + "/token",
	}

	_, err := auth.Login(context.Background(), endpoints, "homelab-cli", &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "authorization denied") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLogin_expiredToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/device/code":
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":               "dev-code-123",
				"user_code":                 "ABCD-1234",
				"verification_uri":          "http://idp.example.com/activate",
				"verification_uri_complete": "http://idp.example.com/activate?user_code=ABCD-1234",
				"expires_in":                300,
				"interval":                  1,
			})
		case "/token":
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"error": "expired_token"})
		}
	}))
	defer srv.Close()

	endpoints := &auth.OIDCEndpoints{
		DeviceAuthorizationEndpoint: srv.URL + "/device/code",
		TokenEndpoint:               srv.URL + "/token",
	}

	_, err := auth.Login(context.Background(), endpoints, "homelab-cli", &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "device authorization expired") {
		t.Errorf("unexpected error: %v", err)
	}
}
