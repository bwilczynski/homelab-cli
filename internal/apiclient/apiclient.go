package apiclient

import (
	"net/http"

	"github.com/bwilczynski/hlctl/internal/auth"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/config"
)

// NewHTTPClient returns an authenticated *http.Client and the resolved API
// base URL. Precedence: --api-url flag → HOMELAB_API_URL env → config file.
// Call once per RunE invocation to construct domain-specific API clients.
func NewHTTPClient() (*http.Client, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, "", err
	}

	apiURL := flags.GetAPIURL()
	if apiURL == "" {
		apiURL, err = cfg.ResolveAPIURL()
		if err != nil {
			return nil, "", err
		}
	}

	httpClient := &http.Client{
		Transport: auth.NewAuthenticatedTransport(nil),
	}
	return httpClient, apiURL, nil
}
