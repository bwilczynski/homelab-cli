package network

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gen "github.com/bwilczynski/hlctl/internal/network"
)

// StubClient is a NetworkClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListNetworkDevicesFunc func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkDeviceFunc   func(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListNetworkClientsFunc func(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkClientFunc   func(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func (s *StubClient) ListNetworkDevices(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListNetworkDevicesFunc(ctx, reqEditors...)
}

func (s *StubClient) GetNetworkDevice(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetNetworkDeviceFunc(ctx, deviceId, reqEditors...)
}

func (s *StubClient) ListNetworkClients(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListNetworkClientsFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetNetworkClient(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetNetworkClientFunc(ctx, clientId, reqEditors...)
}

// jsonResponse builds an *http.Response with a JSON body and the given status code.
func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}
