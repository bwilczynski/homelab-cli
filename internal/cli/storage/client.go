// internal/cli/storage/client.go
package storage

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/storage"
)

// StorageClient is the interface used by storage commands.
type StorageClient interface {
	ListStorageVolumes(ctx context.Context, params *gen.ListStorageVolumesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetStorageVolume(ctx context.Context, volumeId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

// NewStorageClient constructs a StorageClient backed by the real API.
func NewStorageClient(httpClient *http.Client, apiURL string) (StorageClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
