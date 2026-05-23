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
	GetNetworkTopologyFunc func(ctx context.Context, params *gen.GetNetworkTopologyParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListVlansFunc         func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetVlanFunc           func(ctx context.Context, vlanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSsidsFunc         func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetSsidFunc           func(ctx context.Context, ssidId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListWansFunc          func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetWanFunc            func(ctx context.Context, wanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
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

func (s *StubClient) GetNetworkTopology(ctx context.Context, params *gen.GetNetworkTopologyParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetNetworkTopologyFunc(ctx, params, reqEditors...)
}

func (s *StubClient) ListVlans(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListVlansFunc(ctx, reqEditors...)
}

func (s *StubClient) GetVlan(ctx context.Context, vlanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetVlanFunc(ctx, vlanId, reqEditors...)
}

func (s *StubClient) ListSsids(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListSsidsFunc(ctx, reqEditors...)
}

func (s *StubClient) GetSsid(ctx context.Context, ssidId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetSsidFunc(ctx, ssidId, reqEditors...)
}

func (s *StubClient) ListWans(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListWansFunc(ctx, reqEditors...)
}

func (s *StubClient) GetWan(ctx context.Context, wanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetWanFunc(ctx, wanId, reqEditors...)
}

// jsonResponse builds an *http.Response with a JSON body and the given status code.
func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}
