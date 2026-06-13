package network

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
)

func okTopologyResp(data map[string]any) *networkapi.GetNetworkTopologyResponse {
	b, _ := json.Marshal(data)
	var typed networkapi.NetworkTopology
	_ = json.Unmarshal(b, &typed)
	return &networkapi.GetNetworkTopologyResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &typed}
}

func errTopologyResp(status int, body map[string]any) *networkapi.GetNetworkTopologyResponse {
	b, _ := json.Marshal(body)
	return &networkapi.GetNetworkTopologyResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func TestTopologyCmd_devicesOnly(t *testing.T) {
	stub := &StubClient{
		GetNetworkTopologyWithResponseFunc: func(_ context.Context, params *networkapi.GetNetworkTopologyParams, _ ...networkapi.RequestEditorFn) (*networkapi.GetNetworkTopologyResponse, error) {
			if params.IncludeClients != nil {
				t.Errorf("expected no IncludeClients param, got %v", *params.IncludeClients)
			}
			return okTopologyResp(map[string]any{
				"nodes": []any{
					map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG", "type": "gateway", "status": "connected"},
					map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR", "type": "switch", "status": "connected"},
					map[string]any{"kind": "device", "id": "unifi.apoffice", "uri": "/network/devices/unifi.apoffice", "name": "AP-Office", "type": "accessPoint", "status": "connected"},
					map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR", "type": "accessPoint", "status": "connected", "numClients": 2},
				},
				"edges": []any{
					map[string]any{
						"kind":      "wired",
						"source":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
						"target":    map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG"},
						"port":      1,
						"linkSpeed": "gbe1",
					},
					map[string]any{
						"kind":      "wired",
						"source":    map[string]any{"kind": "device", "id": "unifi.apoffice", "uri": "/network/devices/unifi.apoffice", "name": "AP-Office"},
						"target":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
						"port":      3,
						"linkSpeed": "gbe1",
					},
					map[string]any{
						"kind":      "wired",
						"source":    map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
						"target":    map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG"},
						"port":      2,
						"linkSpeed": "gbe1",
					},
				},
			}), nil
		},
	}

	cmd := newTopologyCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"USG", "gateway",
		"Switch LR", "switch", "port 1", "1 GbE",
		"AP-Office", "accessPoint", "port 3",
		"AP LR", "port 2", "[2 clients]",
		"├──", "└──", "│",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestTopologyCmd_includeClientsWiredOnly(t *testing.T) {
	stub := &StubClient{
		GetNetworkTopologyWithResponseFunc: func(_ context.Context, params *networkapi.GetNetworkTopologyParams, _ ...networkapi.RequestEditorFn) (*networkapi.GetNetworkTopologyResponse, error) {
			if params.IncludeClients == nil || !*params.IncludeClients {
				t.Error("expected IncludeClients=true")
			}
			return okTopologyResp(map[string]any{
				"nodes": []any{
					map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG", "type": "gateway", "status": "connected"},
					map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR", "type": "switch", "status": "connected"},
					map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR", "type": "accessPoint", "status": "connected"},
					map[string]any{"kind": "client", "id": "unifi.nas", "uri": "/network/clients/unifi.nas", "name": "nas-1", "connectionType": "wired", "status": "online"},
					map[string]any{"kind": "client", "id": "unifi.mbp", "uri": "/network/clients/unifi.mbp", "name": "MacBook Pro", "connectionType": "wireless", "status": "online"},
				},
				"edges": []any{
					map[string]any{
						"kind":      "wired",
						"source":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
						"target":    map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG"},
						"port":      1,
						"linkSpeed": "gbe1",
					},
					map[string]any{
						"kind":      "wired",
						"source":    map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
						"target":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
						"port":      7,
						"linkSpeed": "gbe2_5",
					},
					map[string]any{
						"kind":      "wired",
						"source":    map[string]any{"kind": "client", "id": "unifi.nas", "uri": "/network/clients/unifi.nas", "name": "nas-1"},
						"target":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
						"port":      8,
						"linkSpeed": "gbe1",
					},
					map[string]any{
						"kind":           "wireless",
						"source":         map[string]any{"kind": "client", "id": "unifi.mbp", "uri": "/network/clients/unifi.mbp", "name": "MacBook Pro"},
						"target":         map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
						"ssid":           "HomeNet",
						"signalStrength": -55,
					},
				},
			}), nil
		},
	}

	cmd := newTopologyCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
	cmd.SetArgs([]string{"--include-clients"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1", "online", "port 8", "1 GbE"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"MacBook Pro", "HomeNet"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected %q absent (wireless filtered), got:\n%s", absent, out)
		}
	}
}

func TestTopologyCmd_includeWireless(t *testing.T) {
	stub := &StubClient{
		GetNetworkTopologyWithResponseFunc: func(_ context.Context, params *networkapi.GetNetworkTopologyParams, _ ...networkapi.RequestEditorFn) (*networkapi.GetNetworkTopologyResponse, error) {
			if params.IncludeClients == nil || !*params.IncludeClients {
				t.Error("expected IncludeClients=true (implied by --include-wireless)")
			}
			return okTopologyResp(map[string]any{
				"nodes": []any{
					map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG", "type": "gateway", "status": "connected"},
					map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR", "type": "accessPoint", "status": "connected"},
					map[string]any{"kind": "client", "id": "unifi.mbp", "uri": "/network/clients/unifi.mbp", "name": "MacBook Pro", "connectionType": "wireless", "status": "online"},
				},
				"edges": []any{
					map[string]any{
						"kind":      "wired",
						"source":    map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
						"target":    map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG"},
						"port":      1,
						"linkSpeed": "gbe1",
					},
					map[string]any{
						"kind":           "wireless",
						"source":         map[string]any{"kind": "client", "id": "unifi.mbp", "uri": "/network/clients/unifi.mbp", "name": "MacBook Pro"},
						"target":         map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
						"ssid":           "HomeNet",
						"signalStrength": -55,
					},
				},
			}), nil
		},
	}

	cmd := newTopologyCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
	cmd.SetArgs([]string{"--include-wireless"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"MacBook Pro", "online", "HomeNet", "-55 dBm"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestTopologyCmd_jsonOutput(t *testing.T) {
	old := flags.OutputFormat
	flags.OutputFormat = "json"
	defer func() { flags.OutputFormat = old }()

	stub := &StubClient{
		GetNetworkTopologyWithResponseFunc: func(_ context.Context, _ *networkapi.GetNetworkTopologyParams, _ ...networkapi.RequestEditorFn) (*networkapi.GetNetworkTopologyResponse, error) {
			return okTopologyResp(map[string]any{
				"nodes": []any{},
				"edges": []any{},
			}), nil
		},
	}

	cmd := newTopologyCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"nodes"`) || !strings.Contains(out, `"edges"`) {
		t.Errorf("expected raw JSON with nodes/edges keys, got:\n%s", out)
	}
}

func TestTopologyCmd_apiError(t *testing.T) {
	stub := &StubClient{
		GetNetworkTopologyWithResponseFunc: func(_ context.Context, _ *networkapi.GetNetworkTopologyParams, _ ...networkapi.RequestEditorFn) (*networkapi.GetNetworkTopologyResponse, error) {
			return errTopologyResp(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}

	cmd := newTopologyCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
}
