package docker

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/docker"
)

// DockerClient is the interface used by all docker subcommands.
type DockerClient interface {
	ListContainersWithResponse(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*gen.ListContainersResponse, error)
	GetContainerWithResponse(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*gen.GetContainerResponse, error)
	StartContainerWithResponse(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.StartContainerResponse, error)
	StopContainerWithResponse(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.StopContainerResponse, error)
	RestartContainerWithResponse(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.RestartContainerResponse, error)
	ListDockerNetworksWithResponse(ctx context.Context, params *gen.ListDockerNetworksParams, reqEditors ...gen.RequestEditorFn) (*gen.ListDockerNetworksResponse, error)
	GetDockerNetworkWithResponse(ctx context.Context, networkId string, reqEditors ...gen.RequestEditorFn) (*gen.GetDockerNetworkResponse, error)
	ListDockerImagesWithResponse(ctx context.Context, params *gen.ListDockerImagesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListDockerImagesResponse, error)
	GetDockerImageWithResponse(ctx context.Context, imageId string, reqEditors ...gen.RequestEditorFn) (*gen.GetDockerImageResponse, error)
}

// NewDockerClient constructs a DockerClient backed by the real API.
func NewDockerClient(httpClient *http.Client, apiURL string) (DockerClient, error) {
	return gen.NewClientWithResponses(apiURL, gen.WithHTTPClient(httpClient))
}
