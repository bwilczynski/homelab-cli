package storage

import (
	"context"
	"net/http"

	storageapi "github.com/bwilczynski/hlctl/internal/api/storage"
)

// StorageClient is the interface used by storage commands.
type StorageClient interface {
	ListStorageVolumesWithResponse(ctx context.Context, params *storageapi.ListStorageVolumesParams, reqEditors ...storageapi.RequestEditorFn) (*storageapi.ListStorageVolumesResponse, error)
	GetStorageVolumeWithResponse(ctx context.Context, volumeId string, reqEditors ...storageapi.RequestEditorFn) (*storageapi.GetStorageVolumeResponse, error)
	ListBackupsWithResponse(ctx context.Context, params *storageapi.ListBackupsParams, reqEditors ...storageapi.RequestEditorFn) (*storageapi.ListBackupsResponse, error)
	GetBackupWithResponse(ctx context.Context, backupId string, reqEditors ...storageapi.RequestEditorFn) (*storageapi.GetBackupResponse, error)
}

// NewStorageClient constructs a StorageClient backed by the real API.
func NewStorageClient(httpClient *http.Client, apiURL string) (StorageClient, error) {
	return storageapi.NewClientWithResponses(apiURL, storageapi.WithHTTPClient(httpClient))
}
