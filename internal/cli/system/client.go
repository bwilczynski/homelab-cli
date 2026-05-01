// internal/cli/system/client.go
package system

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/system"
)

// SystemClient is the interface used by system commands.
type SystemClient interface {
	GetSystemHealth(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSystemInfo(ctx context.Context, params *gen.ListSystemInfoParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSystemUtilization(ctx context.Context, params *gen.ListSystemUtilizationParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSystemUpdates(ctx context.Context, params *gen.ListSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetSystemUpdate(ctx context.Context, updateId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	CheckSystemUpdates(ctx context.Context, params *gen.CheckSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

// NewSystemClient constructs a SystemClient backed by the real API.
func NewSystemClient(httpClient *http.Client, apiURL string) (SystemClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
