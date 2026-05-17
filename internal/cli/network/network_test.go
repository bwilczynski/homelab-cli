package network

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

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
	for _, want := range []string{"laptop", "Switch Living Room", "3", "1GbE", "online"} {
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
	for _, want := range []string{"Mystery Device", "UPLINK", "Switch Living Room", "port 8", "1GbE"} {
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
	for _, want := range []string{"Switch Living Room", "PORTS", "AP Living Room", "1GbE", "8.5 W", "TRAFFIC RX", "TRAFFIC TX"} {
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
				"numClients": 2,
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
