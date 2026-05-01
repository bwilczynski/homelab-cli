package containers

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/containers"
)

// ContainersClient is the interface used by containers commands.
// It matches the subset of gen.ClientInterface that containers commands need.
type ContainersClient interface {
	ListContainers(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetContainer(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StartContainer(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StopContainer(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	RestartContainer(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

// NewContainersClient constructs a ContainersClient backed by the real API.
func NewContainersClient(httpClient *http.Client, apiURL string) (ContainersClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
