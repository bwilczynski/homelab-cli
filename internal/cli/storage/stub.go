// internal/cli/storage/stub.go
package storage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gen "github.com/bwilczynski/hlctl/internal/storage"
)

// StubClient is a StorageClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListStorageVolumesFunc func(ctx context.Context, params *gen.ListStorageVolumesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetStorageVolumeFunc   func(ctx context.Context, volumeId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func (s *StubClient) ListStorageVolumes(ctx context.Context, params *gen.ListStorageVolumesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListStorageVolumesFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetStorageVolume(ctx context.Context, volumeId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetStorageVolumeFunc(ctx, volumeId, reqEditors...)
}

// jsonResponse builds an *http.Response with a JSON body and the given status code.
func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}
