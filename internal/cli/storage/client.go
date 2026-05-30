package storage

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/storage"
)

// StorageClient is the interface used by storage commands.
type StorageClient interface {
	ListStorageVolumesWithResponse(ctx context.Context, params *gen.ListStorageVolumesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListStorageVolumesResponse, error)
	GetStorageVolumeWithResponse(ctx context.Context, volumeId string, reqEditors ...gen.RequestEditorFn) (*gen.GetStorageVolumeResponse, error)
	ListBackupsWithResponse(ctx context.Context, params *gen.ListBackupsParams, reqEditors ...gen.RequestEditorFn) (*gen.ListBackupsResponse, error)
	GetBackupWithResponse(ctx context.Context, backupId string, reqEditors ...gen.RequestEditorFn) (*gen.GetBackupResponse, error)
}

// NewStorageClient constructs a StorageClient backed by the real API.
func NewStorageClient(httpClient *http.Client, apiURL string) (StorageClient, error) {
	return gen.NewClientWithResponses(apiURL, gen.WithHTTPClient(httpClient))
}
