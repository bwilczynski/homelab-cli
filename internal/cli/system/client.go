// internal/cli/system/client.go
package system

import (
	"context"
	"net/http"

	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
)

// SystemClient is the interface used by system commands.
type SystemClient interface {
	GetSystemHealthWithResponse(ctx context.Context, reqEditors ...systemapi.RequestEditorFn) (*systemapi.GetSystemHealthResponse, error)
	ListSystemInfoWithResponse(ctx context.Context, params *systemapi.ListSystemInfoParams, reqEditors ...systemapi.RequestEditorFn) (*systemapi.ListSystemInfoResponse, error)
	ListSystemUtilizationWithResponse(ctx context.Context, params *systemapi.ListSystemUtilizationParams, reqEditors ...systemapi.RequestEditorFn) (*systemapi.ListSystemUtilizationResponse, error)
	ListSystemUpdatesWithResponse(ctx context.Context, params *systemapi.ListSystemUpdatesParams, reqEditors ...systemapi.RequestEditorFn) (*systemapi.ListSystemUpdatesResponse, error)
	GetSystemUpdateWithResponse(ctx context.Context, updateId string, reqEditors ...systemapi.RequestEditorFn) (*systemapi.GetSystemUpdateResponse, error)
	CheckSystemUpdatesWithResponse(ctx context.Context, params *systemapi.CheckSystemUpdatesParams, reqEditors ...systemapi.RequestEditorFn) (*systemapi.CheckSystemUpdatesResponse, error)
}

// NewSystemClient constructs a SystemClient backed by the real API.
func NewSystemClient(httpClient *http.Client, apiURL string) (SystemClient, error) {
	return systemapi.NewClientWithResponses(apiURL, systemapi.WithHTTPClient(httpClient))
}
