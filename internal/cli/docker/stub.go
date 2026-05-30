package docker

import (
	"context"

	gen "github.com/bwilczynski/hlctl/internal/docker"
)

// StubClient is a DockerClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListContainersWithResponseFunc    func(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*gen.ListContainersResponse, error)
	GetContainerWithResponseFunc      func(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*gen.GetContainerResponse, error)
	StartContainerWithResponseFunc    func(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.StartContainerResponse, error)
	StopContainerWithResponseFunc     func(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.StopContainerResponse, error)
	RestartContainerWithResponseFunc  func(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.RestartContainerResponse, error)
	ListDockerNetworksWithResponseFunc func(ctx context.Context, params *gen.ListDockerNetworksParams, reqEditors ...gen.RequestEditorFn) (*gen.ListDockerNetworksResponse, error)
	GetDockerNetworkWithResponseFunc   func(ctx context.Context, networkId string, reqEditors ...gen.RequestEditorFn) (*gen.GetDockerNetworkResponse, error)
	ListDockerImagesWithResponseFunc   func(ctx context.Context, params *gen.ListDockerImagesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListDockerImagesResponse, error)
	GetDockerImageWithResponseFunc     func(ctx context.Context, imageId string, reqEditors ...gen.RequestEditorFn) (*gen.GetDockerImageResponse, error)
}

func (s *StubClient) ListContainersWithResponse(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*gen.ListContainersResponse, error) {
	return s.ListContainersWithResponseFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetContainerWithResponse(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*gen.GetContainerResponse, error) {
	return s.GetContainerWithResponseFunc(ctx, containerId, reqEditors...)
}

func (s *StubClient) StartContainerWithResponse(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.StartContainerResponse, error) {
	return s.StartContainerWithResponseFunc(ctx, containerId, params, reqEditors...)
}

func (s *StubClient) StopContainerWithResponse(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.StopContainerResponse, error) {
	return s.StopContainerWithResponseFunc(ctx, containerId, params, reqEditors...)
}

func (s *StubClient) RestartContainerWithResponse(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.RestartContainerResponse, error) {
	return s.RestartContainerWithResponseFunc(ctx, containerId, params, reqEditors...)
}

func (s *StubClient) ListDockerNetworksWithResponse(ctx context.Context, params *gen.ListDockerNetworksParams, reqEditors ...gen.RequestEditorFn) (*gen.ListDockerNetworksResponse, error) {
	return s.ListDockerNetworksWithResponseFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetDockerNetworkWithResponse(ctx context.Context, networkId string, reqEditors ...gen.RequestEditorFn) (*gen.GetDockerNetworkResponse, error) {
	return s.GetDockerNetworkWithResponseFunc(ctx, networkId, reqEditors...)
}

func (s *StubClient) ListDockerImagesWithResponse(ctx context.Context, params *gen.ListDockerImagesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListDockerImagesResponse, error) {
	return s.ListDockerImagesWithResponseFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetDockerImageWithResponse(ctx context.Context, imageId string, reqEditors ...gen.RequestEditorFn) (*gen.GetDockerImageResponse, error) {
	return s.GetDockerImageWithResponseFunc(ctx, imageId, reqEditors...)
}
