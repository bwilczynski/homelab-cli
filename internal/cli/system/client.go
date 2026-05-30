// internal/cli/system/client.go
package system

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/system"
)

// SystemClient is the interface used by system commands.
type SystemClient interface {
	GetSystemHealthWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.GetSystemHealthResponse, error)
	ListSystemInfoWithResponse(ctx context.Context, params *gen.ListSystemInfoParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemInfoResponse, error)
	ListSystemUtilizationWithResponse(ctx context.Context, params *gen.ListSystemUtilizationParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemUtilizationResponse, error)
	ListSystemUpdatesWithResponse(ctx context.Context, params *gen.ListSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemUpdatesResponse, error)
	GetSystemUpdateWithResponse(ctx context.Context, updateId string, reqEditors ...gen.RequestEditorFn) (*gen.GetSystemUpdateResponse, error)
	CheckSystemUpdatesWithResponse(ctx context.Context, params *gen.CheckSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*gen.CheckSystemUpdatesResponse, error)
}

// NewSystemClient constructs a SystemClient backed by the real API.
func NewSystemClient(httpClient *http.Client, apiURL string) (SystemClient, error) {
	return gen.NewClientWithResponses(apiURL, gen.WithHTTPClient(httpClient))
}
