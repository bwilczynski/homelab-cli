// internal/cli/backups/stub.go
package backups

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gen "github.com/bwilczynski/hlctl/internal/backups"
)

// StubClient is a BackupsClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListBackupTasksFunc func(ctx context.Context, params *gen.ListBackupTasksParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetBackupTaskFunc   func(ctx context.Context, taskId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func (s *StubClient) ListBackupTasks(ctx context.Context, params *gen.ListBackupTasksParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListBackupTasksFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetBackupTask(ctx context.Context, taskId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetBackupTaskFunc(ctx, taskId, reqEditors...)
}

// jsonResponse builds an *http.Response with a JSON body and the given status code.
func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}
