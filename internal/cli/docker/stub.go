package docker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gen "github.com/bwilczynski/hlctl/internal/docker"
)

// StubClient is a ContainersClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListContainersFunc   func(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetContainerFunc     func(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StartContainerFunc   func(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StopContainerFunc    func(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	RestartContainerFunc func(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func (s *StubClient) ListContainers(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListContainersFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetContainer(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetContainerFunc(ctx, containerId, reqEditors...)
}

func (s *StubClient) StartContainer(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.StartContainerFunc(ctx, containerId, params, reqEditors...)
}

func (s *StubClient) StopContainer(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.StopContainerFunc(ctx, containerId, params, reqEditors...)
}

func (s *StubClient) RestartContainer(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.RestartContainerFunc(ctx, containerId, params, reqEditors...)
}

// jsonResponse builds an *http.Response with a JSON body and the given status code.
func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}
