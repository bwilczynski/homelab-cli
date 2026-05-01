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
