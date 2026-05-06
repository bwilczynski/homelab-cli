package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type HomelabInfo struct {
	Enabled bool   `json:"enabled"`
	Issuer  string `json:"issuer,omitempty"`
}

type OIDCEndpoints struct {
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
}

func DiscoverHomelab(apiURL string) (*HomelabInfo, error) {
	resp, err := http.Get(apiURL + "/.well-known/homelab")
	if err != nil {
		return nil, fmt.Errorf("reaching API: %w", err)
	}
	defer resp.Body.Close()

	var info HomelabInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("parsing homelab discovery: %w", err)
	}
	return &info, nil
}

func DiscoverOIDC(issuer string) (*OIDCEndpoints, error) {
	resp, err := http.Get(issuer + "/.well-known/openid-configuration")
	if err != nil {
		return nil, fmt.Errorf("reaching OIDC provider: %w", err)
	}
	defer resp.Body.Close()

	var endpoints OIDCEndpoints
	if err := json.NewDecoder(resp.Body).Decode(&endpoints); err != nil {
		return nil, fmt.Errorf("parsing OIDC configuration: %w", err)
	}
	if endpoints.DeviceAuthorizationEndpoint == "" {
		return nil, fmt.Errorf("OIDC provider does not support device authorization flow")
	}
	return &endpoints, nil
}
