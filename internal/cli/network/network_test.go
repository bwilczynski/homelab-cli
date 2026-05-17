package network

import (
	"bytes"
	"context"
	"fmt"
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

func TestGetDeviceCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.GatewayDetail{
				Id:              "unifi.usg",
				Name:            "USG",
				Mac:             "aa:bb:cc:dd:00:01",
				Ip:              "192.168.1.1",
				Type:            gen.GatewayDetailTypeGateway,
				Status:          gen.NetworkDeviceStatusConnected,
				Model:           "USG-3P",
				FirmwareVersion: "4.4.57",
				Uptime:          86400,
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
	for _, want := range []string{"unifi.usg", "USG-3P", "4.4.57", "gateway"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
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
			port := 3
			return jsonResponse(http.StatusOK, map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:01",
				"name":           "laptop",
				"mac":            "aa:bb:cc:dd:ee:01",
				"ip":             "192.168.1.50",
				"connectionType": "wired",
				"status":         "online",
				"connectedTo": map[string]any{
					"device": map[string]any{
						"id":   "unifi.switch-1",
						"kind": "device",
						"name": "switch-1",
						"uri":  "/network/devices/unifi.switch-1",
					},
					"port":      &port,
					"linkSpeed": "gbe1",
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
	for _, want := range []string{"laptop", "switch-1", fmt.Sprintf("%d", 3), "online"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetDeviceCmd_noClientsRow(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.GatewayDetail{
				Id:              "unifi.usg",
				Name:            "USG",
				Mac:             "aa:bb:cc:dd:00:01",
				Ip:              "192.168.1.1",
				Type:            gen.GatewayDetailTypeGateway,
				Status:          gen.NetworkDeviceStatusConnected,
				Model:           "USG-3P",
				FirmwareVersion: "4.4.57",
				Uptime:          86400,
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

	if strings.Contains(buf.String(), "CLIENTS") {
		t.Errorf("expected no CLIENTS row for gateway device, got:\n%s", buf.String())
	}
}

func TestGetClientCmd_wireless(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			signalStrength := -65
			return jsonResponse(http.StatusOK, map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:02",
				"name":           "phone",
				"mac":            "aa:bb:cc:dd:ee:02",
				"ip":             "192.168.1.51",
				"connectionType": "wireless",
				"status":         "online",
				"connectedTo": map[string]any{
					"device": map[string]any{
						"id":   "unifi.ap-living-room",
						"kind": "device",
						"name": "AP Living Room",
						"uri":  "/network/devices/unifi.ap-living-room",
					},
					"ssid":           "HomeNet",
					"signalStrength": &signalStrength,
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
	for _, want := range []string{"phone", "HomeNet", "-65 dBm", "online"} {
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
				"id":             "unifi.aa:bb:cc:dd:ee:03",
				"name":           "printer",
				"mac":            "aa:bb:cc:dd:ee:03",
				"ip":             "192.168.1.60",
				"connectionType": "wired",
				"status":         "offline",
				"connectedTo": map[string]any{
					"device": map[string]any{
						"id":   "unifi.switch-1",
						"kind": "device",
						"name": "switch-1",
						"uri":  "/network/devices/unifi.switch-1",
					},
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
	for _, want := range []string{"printer", "offline"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"SWITCH PORT", "UPTIME"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected %q to be absent for offline wired client, got:\n%s", absent, out)
		}
	}
}

func TestGetClientCmd_offline_wireless(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:04",
				"name":           "tablet",
				"mac":            "aa:bb:cc:dd:ee:04",
				"ip":             "192.168.1.70",
				"connectionType": "wireless",
				"status":         "offline",
				"connectedTo": map[string]any{
					"device": map[string]any{
						"id":   "unifi.ap-living-room",
						"kind": "device",
						"name": "AP Living Room",
						"uri":  "/network/devices/unifi.ap-living-room",
					},
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
	for _, want := range []string{"tablet", "offline"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"SIGNAL", "UPTIME"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected %q to be absent for offline wireless client, got:\n%s", absent, out)
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
