package network

import (
	"context"

	gen "github.com/bwilczynski/hlctl/internal/network"
)

// StubClient is a NetworkClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListNetworkDevicesWithResponseFunc func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListNetworkDevicesResponse, error)
	GetNetworkDeviceWithResponseFunc   func(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkDeviceResponse, error)
	ListNetworkClientsWithResponseFunc func(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*gen.ListNetworkClientsResponse, error)
	GetNetworkClientWithResponseFunc   func(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkClientResponse, error)
	GetNetworkTopologyWithResponseFunc func(ctx context.Context, params *gen.GetNetworkTopologyParams, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkTopologyResponse, error)
	ListVlansWithResponseFunc          func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListVlansResponse, error)
	GetVlanWithResponseFunc            func(ctx context.Context, vlanId string, reqEditors ...gen.RequestEditorFn) (*gen.GetVlanResponse, error)
	ListSsidsWithResponseFunc          func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListSsidsResponse, error)
	GetSsidWithResponseFunc            func(ctx context.Context, ssidId string, reqEditors ...gen.RequestEditorFn) (*gen.GetSsidResponse, error)
	ListWansWithResponseFunc           func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListWansResponse, error)
	GetWanWithResponseFunc             func(ctx context.Context, wanId string, reqEditors ...gen.RequestEditorFn) (*gen.GetWanResponse, error)
}

func (s *StubClient) ListNetworkDevicesWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListNetworkDevicesResponse, error) {
	return s.ListNetworkDevicesWithResponseFunc(ctx, reqEditors...)
}
func (s *StubClient) GetNetworkDeviceWithResponse(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkDeviceResponse, error) {
	return s.GetNetworkDeviceWithResponseFunc(ctx, deviceId, reqEditors...)
}
func (s *StubClient) ListNetworkClientsWithResponse(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*gen.ListNetworkClientsResponse, error) {
	return s.ListNetworkClientsWithResponseFunc(ctx, params, reqEditors...)
}
func (s *StubClient) GetNetworkClientWithResponse(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkClientResponse, error) {
	return s.GetNetworkClientWithResponseFunc(ctx, clientId, reqEditors...)
}
func (s *StubClient) GetNetworkTopologyWithResponse(ctx context.Context, params *gen.GetNetworkTopologyParams, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkTopologyResponse, error) {
	return s.GetNetworkTopologyWithResponseFunc(ctx, params, reqEditors...)
}
func (s *StubClient) ListVlansWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListVlansResponse, error) {
	return s.ListVlansWithResponseFunc(ctx, reqEditors...)
}
func (s *StubClient) GetVlanWithResponse(ctx context.Context, vlanId string, reqEditors ...gen.RequestEditorFn) (*gen.GetVlanResponse, error) {
	return s.GetVlanWithResponseFunc(ctx, vlanId, reqEditors...)
}
func (s *StubClient) ListSsidsWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListSsidsResponse, error) {
	return s.ListSsidsWithResponseFunc(ctx, reqEditors...)
}
func (s *StubClient) GetSsidWithResponse(ctx context.Context, ssidId string, reqEditors ...gen.RequestEditorFn) (*gen.GetSsidResponse, error) {
	return s.GetSsidWithResponseFunc(ctx, ssidId, reqEditors...)
}
func (s *StubClient) ListWansWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListWansResponse, error) {
	return s.ListWansWithResponseFunc(ctx, reqEditors...)
}
func (s *StubClient) GetWanWithResponse(ctx context.Context, wanId string, reqEditors ...gen.RequestEditorFn) (*gen.GetWanResponse, error) {
	return s.GetWanWithResponseFunc(ctx, wanId, reqEditors...)
}
