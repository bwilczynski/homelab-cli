// internal/cli/backups/client.go
package backups

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/backups"
)

// BackupsClient is the interface used by backups commands.
type BackupsClient interface {
	ListBackupTasks(ctx context.Context, params *gen.ListBackupTasksParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetBackupTask(ctx context.Context, taskId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

// NewBackupsClient constructs a BackupsClient backed by the real API.
func NewBackupsClient(httpClient *http.Client, apiURL string) (BackupsClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
