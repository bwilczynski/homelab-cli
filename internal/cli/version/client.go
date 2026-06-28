package version

import (
	"context"
	"net/http"

	metaapi "github.com/bwilczynski/hlctl/internal/api/meta"
)

// MetaClient is the interface used by the version command.
type MetaClient interface {
	GetMetaVersionWithResponse(ctx context.Context, reqEditors ...metaapi.RequestEditorFn) (*metaapi.GetMetaVersionResponse, error)
}

// NewMetaClient constructs a MetaClient backed by the real API.
func NewMetaClient(httpClient *http.Client, apiURL string) (MetaClient, error) {
	return metaapi.NewClientWithResponses(apiURL, metaapi.WithHTTPClient(httpClient))
}
