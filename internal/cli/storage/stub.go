package storage

import (
	"context"

	storageapi "github.com/bwilczynski/hlctl/internal/api/storage"
)

// StubClient is a StorageClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListStorageVolumesWithResponseFunc func(ctx context.Context, params *storageapi.ListStorageVolumesParams, reqEditors ...storageapi.RequestEditorFn) (*storageapi.ListStorageVolumesResponse, error)
	GetStorageVolumeWithResponseFunc   func(ctx context.Context, volumeId string, reqEditors ...storageapi.RequestEditorFn) (*storageapi.GetStorageVolumeResponse, error)
	ListBackupsWithResponseFunc        func(ctx context.Context, params *storageapi.ListBackupsParams, reqEditors ...storageapi.RequestEditorFn) (*storageapi.ListBackupsResponse, error)
	GetBackupWithResponseFunc          func(ctx context.Context, backupId string, reqEditors ...storageapi.RequestEditorFn) (*storageapi.GetBackupResponse, error)
}

func (s *StubClient) ListStorageVolumesWithResponse(ctx context.Context, params *storageapi.ListStorageVolumesParams, reqEditors ...storageapi.RequestEditorFn) (*storageapi.ListStorageVolumesResponse, error) {
	return s.ListStorageVolumesWithResponseFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetStorageVolumeWithResponse(ctx context.Context, volumeId string, reqEditors ...storageapi.RequestEditorFn) (*storageapi.GetStorageVolumeResponse, error) {
	return s.GetStorageVolumeWithResponseFunc(ctx, volumeId, reqEditors...)
}

func (s *StubClient) ListBackupsWithResponse(ctx context.Context, params *storageapi.ListBackupsParams, reqEditors ...storageapi.RequestEditorFn) (*storageapi.ListBackupsResponse, error) {
	return s.ListBackupsWithResponseFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetBackupWithResponse(ctx context.Context, backupId string, reqEditors ...storageapi.RequestEditorFn) (*storageapi.GetBackupResponse, error) {
	return s.GetBackupWithResponseFunc(ctx, backupId, reqEditors...)
}
