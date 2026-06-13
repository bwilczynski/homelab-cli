package network

import (
	"context"
	"net/http"

	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
)

// NetworkClient is the interface used by network commands.
type NetworkClient interface {
	ListNetworkDevicesWithResponse(ctx context.Context, reqEditors ...networkapi.RequestEditorFn) (*networkapi.ListNetworkDevicesResponse, error)
	GetNetworkDeviceWithResponse(ctx context.Context, deviceId string, reqEditors ...networkapi.RequestEditorFn) (*networkapi.GetNetworkDeviceResponse, error)
	ListNetworkClientsWithResponse(ctx context.Context, params *networkapi.ListNetworkClientsParams, reqEditors ...networkapi.RequestEditorFn) (*networkapi.ListNetworkClientsResponse, error)
	GetNetworkClientWithResponse(ctx context.Context, clientId string, reqEditors ...networkapi.RequestEditorFn) (*networkapi.GetNetworkClientResponse, error)
	GetNetworkTopologyWithResponse(ctx context.Context, params *networkapi.GetNetworkTopologyParams, reqEditors ...networkapi.RequestEditorFn) (*networkapi.GetNetworkTopologyResponse, error)
	ListVlansWithResponse(ctx context.Context, reqEditors ...networkapi.RequestEditorFn) (*networkapi.ListVlansResponse, error)
	GetVlanWithResponse(ctx context.Context, vlanId string, reqEditors ...networkapi.RequestEditorFn) (*networkapi.GetVlanResponse, error)
	ListSsidsWithResponse(ctx context.Context, reqEditors ...networkapi.RequestEditorFn) (*networkapi.ListSsidsResponse, error)
	GetSsidWithResponse(ctx context.Context, ssidId string, reqEditors ...networkapi.RequestEditorFn) (*networkapi.GetSsidResponse, error)
	ListWansWithResponse(ctx context.Context, reqEditors ...networkapi.RequestEditorFn) (*networkapi.ListWansResponse, error)
	GetWanWithResponse(ctx context.Context, wanId string, reqEditors ...networkapi.RequestEditorFn) (*networkapi.GetWanResponse, error)
}

// NewNetworkClient constructs a NetworkClient backed by the real API.
func NewNetworkClient(httpClient *http.Client, apiURL string) (NetworkClient, error) {
	return networkapi.NewClientWithResponses(apiURL, networkapi.WithHTTPClient(httpClient))
}
