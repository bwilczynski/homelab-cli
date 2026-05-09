package docker

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/docker"
)

// DockerClient is the interface used by all docker subcommands.
type DockerClient interface {
	ListContainers(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetContainer(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StartContainer(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StopContainer(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	RestartContainer(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListDockerNetworks(ctx context.Context, params *gen.ListDockerNetworksParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetDockerNetwork(ctx context.Context, networkId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListDockerImages(ctx context.Context, params *gen.ListDockerImagesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetDockerImage(ctx context.Context, imageId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

// NewDockerClient constructs a DockerClient backed by the real API.
func NewDockerClient(httpClient *http.Client, apiURL string) (DockerClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
