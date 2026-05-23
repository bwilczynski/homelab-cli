package network

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/network"
)

// NetworkClient is the interface used by network commands.
// It matches the subset of gen.ClientInterface that network commands need.
type NetworkClient interface {
	ListNetworkDevices(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkDevice(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListNetworkClients(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkClient(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkTopology(ctx context.Context, params *gen.GetNetworkTopologyParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListVlans(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetVlan(ctx context.Context, vlanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSsids(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetSsid(ctx context.Context, ssidId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListWans(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetWan(ctx context.Context, wanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

// NewNetworkClient constructs a NetworkClient backed by the real API.
func NewNetworkClient(httpClient *http.Client, apiURL string) (NetworkClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
