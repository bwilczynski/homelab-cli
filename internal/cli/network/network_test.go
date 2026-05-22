package network

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/network"
)

func TestListDevicesCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListNetworkDevicesFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkDeviceList{
				Items: []gen.NetworkDevice{
					{
						Id:     "unifi.usg",
						Uri:    "/network/devices/unifi.usg",
						Name:   "USG",
						Mac:    "aa:bb:cc:dd:00:01",
						Ip:     "192.168.1.1",
						Type:   gen.NetworkDeviceTypeGateway,
						Status: gen.NetworkDeviceStatusConnected,
					},
					{
						Id:     "unifi.ap-living-room",
						Uri:    "/network/devices/unifi.ap-living-room",
						Name:   "AP Living Room",
						Mac:    "aa:bb:cc:dd:00:03",
						Ip:     "192.168.1.3",
						Type:   gen.NetworkDeviceTypeAccessPoint,
						Status: gen.NetworkDeviceStatusConnected,
					},
				},
			}), nil
		},
	}

	cmd := newListDevicesCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"unifi.usg", "unifi.ap-living-room", "gateway", "accessPoint"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "CLIENTS") {
		t.Errorf("expected no CLIENTS column in list output, got:\n%s", out)
	}
}

func TestListDevicesCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListNetworkDevicesFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListDevicesCmd(stub)
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

func TestGetDeviceCmd_gateway(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.usg", "uri": "/network/devices/unifi.usg",
				"name": "USG", "mac": "aa:bb:cc:dd:00:01", "ip": "192.168.1.1",
				"type": "gateway", "status": "connected",
				"model": "USG-3P", "firmwareVersion": "4.4.57", "uptime": 86400,
				"traffic": map[string]any{
					"rxBytesTotal": int64(12884901888), "txBytesTotal": int64(4294967296),
					"rxBytesPerSec": int64(125000), "txBytesPerSec": int64(50000),
				},
			}), nil
		},
	}
	cmd := newGetDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.usg"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"unifi.usg", "USG-3P", "4.4.57", "gateway", "TRAFFIC RX", "TRAFFIC TX", "1d"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"PORTS", "CLIENTS", "UPLINK"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected %q absent for gateway, got:\n%s", absent, out)
		}
	}
}

func TestListClientsCmd_tableOutput(t *testing.T) {
	ip := "192.168.1.50"
	stub := &StubClient{
		ListNetworkClientsFunc: func(_ context.Context, _ *gen.ListNetworkClientsParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkClientList{
				Items: []gen.NetworkClient{
					{
						Id:             "unifi.aa:bb:cc:dd:ee:01",
						Name:           "laptop",
						Mac:            "aa:bb:cc:dd:ee:01",
						Ip:             &ip,
						ConnectionType: gen.NetworkClientConnectionTypeWired,
						Status:         gen.NetworkClientStatusOnline,
					},
				},
			}), nil
		},
	}

	cmd := newListClientsCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"unifi.aa:bb:cc:dd:ee:01", "laptop", "192.168.1.50", "wired", "online"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetClientCmd_wired(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.aa:bb:cc:dd:ee:01", "uri": "/network/clients/unifi.aa:bb:cc:dd:ee:01",
				"name": "laptop", "mac": "aa:bb:cc:dd:ee:01", "ip": "192.168.1.50",
				"connectionType": "wired", "status": "online",
				"connectedTo": map[string]any{
					"device": map[string]any{"kind": "device", "id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr", "name": "Switch Living Room"},
					"port": 3, "linkSpeed": "gbe1",
				},
				"uptime": 3600,
			}), nil
		},
	}
	cmd := newGetClientCmd(stub)
	cmd.SetArgs([]string{"unifi.aa:bb:cc:dd:ee:01"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"laptop", "Switch Living Room", "3", "1 GbE", "online"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "SWITCH PORT") {
		t.Errorf("expected PORT not SWITCH PORT, got:\n%s", out)
	}
}

func TestGetDeviceCmd_unknownWithUplink(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.mystery", "uri": "/network/devices/unifi.mystery",
				"name": "Mystery Device", "mac": "aa:bb:cc:dd:00:ff", "ip": "192.168.1.99",
				"type": "unknown", "status": "connected",
				"model": "unknown-model", "firmwareVersion": "0.0.0", "uptime": 3600,
				"traffic": map[string]any{
					"rxBytesTotal": int64(0), "txBytesTotal": int64(0),
					"rxBytesPerSec": int64(0), "txBytesPerSec": int64(0),
				},
				"uplink": map[string]any{
					"device": map[string]any{"kind": "device", "id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr", "name": "Switch Living Room"},
					"port": 8, "linkSpeed": "gbe1",
				},
			}), nil
		},
	}
	cmd := newGetDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.mystery"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Mystery Device", "UPLINK", "Switch Living Room", "port 8", "1 GbE"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetDeviceCmd_switch_activePorts(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr",
				"name": "Switch Living Room", "mac": "aa:bb:cc:dd:00:10", "ip": "192.168.1.10",
				"type": "switch", "status": "connected",
				"model": "USW-24-PoE", "firmwareVersion": "6.2.14", "uptime": 86400,
				"traffic": map[string]any{
					"rxBytesTotal": int64(12884901888), "txBytesTotal": int64(4294967296),
					"rxBytesPerSec": int64(125000), "txBytesPerSec": int64(50000),
				},
				"ports": []map[string]any{
					{
						"number": 1, "state": "up", "linkSpeed": "gbe1", "poeMode": "auto",
						"poePowerWatts": 8.5,
						"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(1200), "txBytesPerSec": int64(500)},
						"connectedTo": map[string]any{"kind": "device", "id": "unifi.ap-living-room", "uri": "/network/devices/unifi.ap-living-room", "name": "AP Living Room"},
					},
					{
						"number": 2, "state": "down", "poeMode": "off",
						"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
					},
				},
			}), nil
		},
	}
	cmd := newGetDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.switch-lr"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Switch Living Room", "PORTS", "AP Living Room", "1 GbE", "8.5 W", "TRAFFIC RX", "TRAFFIC TX"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "down") {
		t.Errorf("expected down port hidden by default, got:\n%s", out)
	}
}

func TestGetDeviceCmd_switch_allPorts(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr",
				"name": "Switch Living Room", "mac": "aa:bb:cc:dd:00:10", "ip": "192.168.1.10",
				"type": "switch", "status": "connected",
				"model": "USW-24-PoE", "firmwareVersion": "6.2.14", "uptime": 3600,
				"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
				"ports": []map[string]any{
					{"number": 1, "state": "up", "poeMode": "off", "traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)}},
					{"number": 2, "state": "down", "poeMode": "off", "traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)}},
					{"number": 3, "state": "disabled", "poeMode": "off", "traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)}},
				},
			}), nil
		},
	}
	cmd := newGetDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.switch-lr", "--all-ports"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"down", "disabled"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output with --all-ports, got:\n%s", want, out)
		}
	}
}

func TestGetDeviceCmd_accessPoint(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.ap-living-room", "uri": "/network/devices/unifi.ap-living-room",
				"name": "AP Living Room", "mac": "aa:bb:cc:dd:00:03", "ip": "192.168.1.3",
				"type": "accessPoint", "status": "connected",
				"model": "U6-Lite", "firmwareVersion": "6.6.77", "uptime": 7200,
				"traffic": map[string]any{
					"rxBytesTotal": int64(1073741824), "txBytesTotal": int64(536870912),
					"rxBytesPerSec": int64(50000), "txBytesPerSec": int64(25000),
				},
				"connectedClients": []map[string]any{
					{"client": map[string]any{"kind": "client", "id": "unifi.macbook-pro", "uri": "/network/clients/unifi.macbook-pro", "name": "MacBook Pro"}, "ssid": "HomeNetwork", "signalStrength": -62},
					{"client": map[string]any{"kind": "client", "id": "unifi.iphone-15", "uri": "/network/clients/unifi.iphone-15", "name": "iPhone 15"}, "ssid": "HomeNetwork", "signalStrength": -70},
				},
			}), nil
		},
	}
	cmd := newGetDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.ap-living-room"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"AP Living Room", "CLIENTS", "MacBook Pro", "iPhone 15", "HomeNetwork", "-62 dBm", "-70 dBm", "TRAFFIC RX"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "PORTS") {
		t.Errorf("expected no PORTS section for AP, got:\n%s", out)
	}
}

func TestGetClientCmd_wireless(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.aa:bb:cc:dd:ee:02", "uri": "/network/clients/unifi.aa:bb:cc:dd:ee:02",
				"name": "phone", "mac": "aa:bb:cc:dd:ee:02", "ip": "192.168.1.51",
				"connectionType": "wireless", "status": "online",
				"connectedTo": map[string]any{
					"device": map[string]any{"kind": "device", "id": "unifi.ap-living-room", "uri": "/network/devices/unifi.ap-living-room", "name": "AP Living Room"},
					"ssid": "HomeNet", "signalStrength": -65,
				},
				"uptime": 1800,
			}), nil
		},
	}
	cmd := newGetClientCmd(stub)
	cmd.SetArgs([]string{"unifi.aa:bb:cc:dd:ee:02"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"phone", "AP Living Room", "HomeNet", "-65 dBm", "online"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "SWITCH") {
		t.Errorf("expected no SWITCH row in wireless output, got:\n%s", out)
	}
}

func TestGetClientCmd_offline_wired(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.aa:bb:cc:dd:ee:03", "uri": "/network/clients/unifi.aa:bb:cc:dd:ee:03",
				"name": "printer", "mac": "aa:bb:cc:dd:ee:03", "ip": "192.168.1.60",
				"connectionType": "wired", "status": "offline",
				"connectedTo": map[string]any{
					"device": map[string]any{"kind": "device", "id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr", "name": "Switch Living Room"},
				},
			}), nil
		},
	}
	cmd := newGetClientCmd(stub)
	cmd.SetArgs([]string{"unifi.aa:bb:cc:dd:ee:03"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"printer", "offline", "Switch Living Room"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"PORT", "LINK SPEED", "UPTIME"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected %q absent for offline wired client, got:\n%s", absent, out)
		}
	}
}

func TestGetClientCmd_offline_wireless(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.aa:bb:cc:dd:ee:04", "uri": "/network/clients/unifi.aa:bb:cc:dd:ee:04",
				"name": "tablet", "mac": "aa:bb:cc:dd:ee:04", "ip": "192.168.1.70",
				"connectionType": "wireless", "status": "offline",
				"connectedTo": map[string]any{
					"device": map[string]any{"kind": "device", "id": "unifi.ap-living-room", "uri": "/network/devices/unifi.ap-living-room", "name": "AP Living Room"},
					"ssid": "HomeNet",
				},
			}), nil
		},
	}
	cmd := newGetClientCmd(stub)
	cmd.SetArgs([]string{"unifi.aa:bb:cc:dd:ee:04"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"tablet", "offline", "AP Living Room", "HomeNet"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"SIGNAL", "UPTIME"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected %q absent for offline wireless client, got:\n%s", absent, out)
		}
	}
}

func TestListClientsCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListNetworkClientsFunc: func(_ context.Context, _ *gen.ListNetworkClientsParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListClientsCmd(stub)
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

func TestListClientsCmd_statusFilter(t *testing.T) {
	var capturedParams *gen.ListNetworkClientsParams
	stub := &StubClient{
		ListNetworkClientsFunc: func(_ context.Context, params *gen.ListNetworkClientsParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			capturedParams = params
			return jsonResponse(http.StatusOK, gen.NetworkClientList{Items: []gen.NetworkClient{}}), nil
		},
	}

	cmd := newListClientsCmd(stub)
	cmd.SetArgs([]string{"--status", "online"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedParams == nil || capturedParams.Status == nil {
		t.Fatal("expected Status param to be set")
	}
	if *capturedParams.Status != gen.NetworkClientStatusOnline {
		t.Errorf("expected status=online, got %q", *capturedParams.Status)
	}
}

func TestGetClientCmd_notFound(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "client not found",
			}), nil
		},
	}
	cmd := newGetClientCmd(stub)
	cmd.SetArgs([]string{"unifi.nonexistent"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
}

func TestTopologyCmd_devicesOnly(t *testing.T) {
	// Topology:
	//   USG (gateway)
	//   ├── Switch LR (switch) [port 1, 1 GbE]
	//   │   └── AP-Office (accessPoint) [port 3, 1 GbE]
	//   └── AP LR (accessPoint) [port 2, 1 GbE] [2 clients]
	//
	// USG has two direct children → exercises ├──, └──, and │ (continuation bar).
	stub := &StubClient{
		GetNetworkTopologyFunc: func(_ context.Context, params *gen.GetNetworkTopologyParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			if params.IncludeClients != nil {
				t.Errorf("expected no IncludeClients param, got %v", *params.IncludeClients)
			}
			return jsonResponse(http.StatusOK, map[string]any{
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

	cmd := newTopologyCmd(stub)
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
		GetNetworkTopologyFunc: func(_ context.Context, params *gen.GetNetworkTopologyParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			if params.IncludeClients == nil || !*params.IncludeClients {
				t.Error("expected IncludeClients=true")
			}
			return jsonResponse(http.StatusOK, map[string]any{
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

	cmd := newTopologyCmd(stub)
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
		GetNetworkTopologyFunc: func(_ context.Context, params *gen.GetNetworkTopologyParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			if params.IncludeClients == nil || !*params.IncludeClients {
				t.Error("expected IncludeClients=true (implied by --include-wireless)")
			}
			return jsonResponse(http.StatusOK, map[string]any{
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

	cmd := newTopologyCmd(stub)
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
		GetNetworkTopologyFunc: func(_ context.Context, _ *gen.GetNetworkTopologyParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"nodes": []any{},
				"edges": []any{},
			}), nil
		},
	}

	cmd := newTopologyCmd(stub)
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
		GetNetworkTopologyFunc: func(_ context.Context, _ *gen.GetNetworkTopologyParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}

	cmd := newTopologyCmd(stub)
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

func TestListVlansCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListVlansFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.VlanList{
				Items: []gen.Vlan{
					{Id: "unifi.default", Name: "Default", VlanId: 1, Subnet: "192.168.1.0/24"},
					{Id: "unifi.iot", Name: "IoT", VlanId: 20, Subnet: "192.168.20.0/24"},
				},
			}), nil
		},
	}
	cmd := newListVlansCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"unifi.default", "Default", "192.168.1.0/24", "unifi.iot", "IoT", "20"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListVlansCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListVlansFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized",
				"status": 401, "detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListVlansCmd(stub)
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

func TestGetVlanCmd_serverDhcp(t *testing.T) {
	stub := &StubClient{
		GetVlanFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.iot", "uri": "/network/vlans/unifi.iot",
				"name": "IoT", "vlanId": 20, "subnet": "192.168.20.0/24",
				"gatewayIp": "192.168.20.1", "broadcastIp": "192.168.20.255",
				"dhcpMode": "server",
				"dhcpRange": map[string]any{"start": "192.168.20.100", "end": "192.168.20.200"},
				"dnsServers": []string{"1.1.1.1", "8.8.8.8"},
			}), nil
		},
	}
	cmd := newGetVlanCmd(stub)
	cmd.SetArgs([]string{"unifi.iot"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"unifi.iot", "IoT", "20", "192.168.20.0/24", "192.168.20.1", "192.168.20.255", "server", "192.168.20.100", "192.168.20.200", "1.1.1.1"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "RELAY") {
		t.Errorf("expected no RELAY row for server DHCP, got:\n%s", out)
	}
}

func TestGetVlanCmd_relayDhcp(t *testing.T) {
	stub := &StubClient{
		GetVlanFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.mgmt", "uri": "/network/vlans/unifi.mgmt",
				"name": "Management", "vlanId": 99, "subnet": "10.0.99.0/24",
				"gatewayIp": "10.0.99.1", "broadcastIp": "10.0.99.255",
				"dhcpMode": "relay", "relayServer": "192.168.1.1",
				"dnsServers": []string{"192.168.1.1"},
			}), nil
		},
	}
	cmd := newGetVlanCmd(stub)
	cmd.SetArgs([]string{"unifi.mgmt"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Management", "relay", "192.168.1.1", "RELAY"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "DHCP RANGE") {
		t.Errorf("expected no DHCP RANGE row for relay DHCP, got:\n%s", out)
	}
}

func TestGetVlanCmd_notFound(t *testing.T) {
	stub := &StubClient{
		GetVlanFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type": "https://homelab.local/problems/not-found", "title": "Not Found",
				"status": 404, "detail": "vlan not found",
			}), nil
		},
	}
	cmd := newGetVlanCmd(stub)
	cmd.SetArgs([]string{"unifi.nonexistent"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
}

func TestGetVlanCmd_disabledDhcp(t *testing.T) {
	stub := &StubClient{
		GetVlanFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.servers", "uri": "/network/vlans/unifi.servers",
				"name": "Servers", "vlanId": 10, "subnet": "192.168.10.0/24",
				"gatewayIp": "192.168.10.1", "broadcastIp": "192.168.10.255",
				"dhcpMode": "disabled",
				"dnsServers": []string{"1.1.1.1"},
			}), nil
		},
	}
	cmd := newGetVlanCmd(stub)
	cmd.SetArgs([]string{"unifi.servers"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Servers", "disabled", "1.1.1.1"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"DHCP RANGE", "RELAY"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected %q absent for disabled DHCP, got:\n%s", absent, out)
		}
	}
}
