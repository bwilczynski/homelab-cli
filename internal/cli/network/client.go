package network

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/network"
)

// NetworkClient is the interface used by network commands.
type NetworkClient interface {
	ListNetworkDevicesWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListNetworkDevicesResponse, error)
	GetNetworkDeviceWithResponse(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkDeviceResponse, error)
	ListNetworkClientsWithResponse(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*gen.ListNetworkClientsResponse, error)
	GetNetworkClientWithResponse(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkClientResponse, error)
	GetNetworkTopologyWithResponse(ctx context.Context, params *gen.GetNetworkTopologyParams, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkTopologyResponse, error)
	ListVlansWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListVlansResponse, error)
	GetVlanWithResponse(ctx context.Context, vlanId string, reqEditors ...gen.RequestEditorFn) (*gen.GetVlanResponse, error)
	ListSsidsWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListSsidsResponse, error)
	GetSsidWithResponse(ctx context.Context, ssidId string, reqEditors ...gen.RequestEditorFn) (*gen.GetSsidResponse, error)
	ListWansWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListWansResponse, error)
	GetWanWithResponse(ctx context.Context, wanId string, reqEditors ...gen.RequestEditorFn) (*gen.GetWanResponse, error)
}

// NewNetworkClient constructs a NetworkClient backed by the real API.
func NewNetworkClient(httpClient *http.Client, apiURL string) (NetworkClient, error) {
	return gen.NewClientWithResponses(apiURL, gen.WithHTTPClient(httpClient))
}
