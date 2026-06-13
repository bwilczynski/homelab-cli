package docker

import (
	"context"
	"net/http"

	dockerapi "github.com/bwilczynski/hlctl/internal/api/docker"
)

// DockerClient is the interface used by all docker subcommands.
type DockerClient interface {
	ListContainersWithResponse(ctx context.Context, params *dockerapi.ListContainersParams, reqEditors ...dockerapi.RequestEditorFn) (*dockerapi.ListContainersResponse, error)
	GetContainerWithResponse(ctx context.Context, containerId string, reqEditors ...dockerapi.RequestEditorFn) (*dockerapi.GetContainerResponse, error)
	StartContainerWithResponse(ctx context.Context, containerId string, params *dockerapi.StartContainerParams, reqEditors ...dockerapi.RequestEditorFn) (*dockerapi.StartContainerResponse, error)
	StopContainerWithResponse(ctx context.Context, containerId string, params *dockerapi.StopContainerParams, reqEditors ...dockerapi.RequestEditorFn) (*dockerapi.StopContainerResponse, error)
	RestartContainerWithResponse(ctx context.Context, containerId string, params *dockerapi.RestartContainerParams, reqEditors ...dockerapi.RequestEditorFn) (*dockerapi.RestartContainerResponse, error)
	ListDockerNetworksWithResponse(ctx context.Context, params *dockerapi.ListDockerNetworksParams, reqEditors ...dockerapi.RequestEditorFn) (*dockerapi.ListDockerNetworksResponse, error)
	GetDockerNetworkWithResponse(ctx context.Context, networkId string, reqEditors ...dockerapi.RequestEditorFn) (*dockerapi.GetDockerNetworkResponse, error)
	ListDockerImagesWithResponse(ctx context.Context, params *dockerapi.ListDockerImagesParams, reqEditors ...dockerapi.RequestEditorFn) (*dockerapi.ListDockerImagesResponse, error)
	GetDockerImageWithResponse(ctx context.Context, imageId string, reqEditors ...dockerapi.RequestEditorFn) (*dockerapi.GetDockerImageResponse, error)
}

// NewDockerClient constructs a DockerClient backed by the real API.
func NewDockerClient(httpClient *http.Client, apiURL string) (DockerClient, error) {
	return dockerapi.NewClientWithResponses(apiURL, dockerapi.WithHTTPClient(httpClient))
}
