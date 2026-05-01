package network

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	gen "github.com/bwilczynski/hlctl/internal/network"
)

func TestDevicesCmd_tableOutput(t *testing.T) {
	numClients := 5
	stub := &StubClient{
		ListNetworkDevicesFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkDeviceList{
				Items: []gen.NetworkDevice{
					{
						Id:     "unifi.usg",
						Name:   "USG",
						Mac:    "aa:bb:cc:dd:00:01",
						Ip:     "192.168.1.1",
						Type:   gen.Gateway,
						Status: gen.Connected,
					},
					{
						Id:         "unifi.ap-living-room",
						Name:       "AP Living Room",
						Mac:        "aa:bb:cc:dd:00:03",
						Ip:         "192.168.1.3",
						Type:       gen.AccessPoint,
						Status:     gen.Connected,
						NumClients: &numClients,
					},
				},
			}), nil
		},
	}

	cmd := newDevicesCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"unifi.usg", "unifi.ap-living-room", "gateway", "accessPoint", "5"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestDevicesCmd_apiError(t *testing.T) {
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

	cmd := newDevicesCmd(stub)
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

func TestDeviceCmd_accessPoint(t *testing.T) {
	numClients := 5
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkDeviceDetail{
				Id:              "unifi.ap-living-room",
				Name:            "AP Living Room",
				Mac:             "aa:bb:cc:dd:ee:ff",
				Ip:              "192.168.1.3",
				Type:            gen.AccessPoint,
				Status:          gen.Connected,
				NumClients:      &numClients,
				Model:           "U6-Lite",
				FirmwareVersion: "6.6.77.14522",
				Uptime:          86400,
			}), nil
		},
	}

	cmd := newDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.ap-living-room"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"unifi.ap-living-room", "U6-Lite", "6.6.77.14522",
		"CLIENTS", "5", "1d 0h 0m 0s",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestDeviceCmd_gateway_noClientsRow(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkDeviceDetail{
				Id:              "unifi.usg",
				Name:            "USG",
				Mac:             "aa:bb:cc:dd:00:01",
				Ip:              "192.168.1.1",
				Type:            gen.Gateway,
				Status:          gen.Connected,
				Model:           "USG-3P",
				FirmwareVersion: "4.4.57",
				Uptime:          3600,
			}), nil
		},
	}

	cmd := newDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.usg"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "CLIENTS") {
		t.Errorf("expected no CLIENTS row for gateway, got:\n%s", out)
	}
	if !strings.Contains(out, "1h 0m 0s") {
		t.Errorf("expected formatted uptime in output, got:\n%s", out)
	}
}

func TestClientsCmd_tableOutput(t *testing.T) {
	ip1 := "192.168.1.101"
	ip2 := "192.168.1.10"
	stub := &StubClient{
		ListNetworkClientsFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkClientList{
				Items: []gen.NetworkClient{
					{
						Id:             "unifi.macbook-pro-3c",
						Name:           "MacBook Pro",
						Mac:            "3c:22:fb:09:aa:b1",
						Ip:             &ip1,
						ConnectionType: "wireless",
					},
					{
						Id:             "unifi.nas-1-68",
						Name:           "nas-1",
						Mac:            "68:d7:9a:12:bb:c2",
						Ip:             &ip2,
						ConnectionType: "wired",
					},
				},
			}), nil
		},
	}

	cmd := newClientsCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"unifi.macbook-pro-3c", "MacBook Pro", "wireless",
		"unifi.nas-1-68", "nas-1", "wired",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestClientsCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListNetworkClientsFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}

	cmd := newClientsCmd(stub)
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

func TestClientCmd_wireless(t *testing.T) {
	ip := "192.168.1.101"
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.WirelessNetworkClientDetail{
				ConnectionType: gen.Wireless,
				Id:             "unifi.macbook-pro-3c",
				Name:           "MacBook Pro",
				Mac:            "3c:22:fb:09:aa:b1",
				Ip:             &ip,
				Ssid:           "HomeNetwork",
				SignalStrength:  -62,
				Uptime:         7200,
			}), nil
		},
	}

	cmd := newClientCmd(stub)
	cmd.SetArgs([]string{"unifi.macbook-pro-3c"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"HomeNetwork", "-62 dBm", "2h 0m 0s", "wireless"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"SWITCH", "SWITCH PORT"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected no %q row for wireless client, got:\n%s", absent, out)
		}
	}
}

func TestClientCmd_wired(t *testing.T) {
	ip := "192.168.1.10"
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.WiredNetworkClientDetail{
				ConnectionType: gen.WiredNetworkClientDetailConnectionTypeWired,
				Id:             "unifi.nas-1-68",
				Name:           "nas-1",
				Mac:            "68:d7:9a:12:bb:c2",
				Ip:             &ip,
				SwitchName:     "Switch Living Room",
				SwitchPort:     8,
				Uptime:         604800,
			}), nil
		},
	}

	cmd := newClientCmd(stub)
	cmd.SetArgs([]string{"unifi.nas-1-68"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Switch Living Room", "8", "7d 0h 0m 0s", "wired"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"SSID", "SIGNAL"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected no %q row for wired client, got:\n%s", absent, out)
		}
	}
}

func TestClientCmd_notFound(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "client 'unifi.foo' does not exist or is offline",
			}), nil
		},
	}

	cmd := newClientCmd(stub)
	cmd.SetArgs([]string{"unifi.foo"})
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
