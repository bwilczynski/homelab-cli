package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bwilczynski/hlctl/internal/auth"
)

func TestDiscoverHomelab_enabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/homelab" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"enabled": true, "issuer": "http://idp.example.com/dex"})
	}))
	defer srv.Close()

	info, err := auth.DiscoverHomelab(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Enabled {
		t.Error("expected enabled=true")
	}
	if info.Issuer != "http://idp.example.com/dex" {
		t.Errorf("unexpected issuer: %s", info.Issuer)
	}
}

func TestDiscoverHomelab_disabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"enabled": false})
	}))
	defer srv.Close()

	info, err := auth.DiscoverHomelab(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Enabled {
		t.Error("expected enabled=false")
	}
}

func TestDiscoverOIDC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"device_authorization_endpoint": "http://idp.example.com/dex/device/code",
			"token_endpoint":                "http://idp.example.com/dex/token",
		})
	}))
	defer srv.Close()

	endpoints, err := auth.DiscoverOIDC(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if endpoints.DeviceAuthorizationEndpoint != "http://idp.example.com/dex/device/code" {
		t.Errorf("unexpected device endpoint: %s", endpoints.DeviceAuthorizationEndpoint)
	}
	if endpoints.TokenEndpoint != "http://idp.example.com/dex/token" {
		t.Errorf("unexpected token endpoint: %s", endpoints.TokenEndpoint)
	}
}

func TestDiscoverOIDC_missingDeviceEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"token_endpoint": "http://idp.example.com/dex/token"})
	}))
	defer srv.Close()

	_, err := auth.DiscoverOIDC(srv.URL)
	if err == nil {
		t.Fatal("expected error for missing device_authorization_endpoint")
	}
}
