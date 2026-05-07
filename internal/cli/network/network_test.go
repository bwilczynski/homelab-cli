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

	cmd := newListDevicesCmd(stub)
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
	numClients := 3
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkDeviceDetail{
				Id:              "unifi.usg",
				Name:            "USG",
				Mac:             "aa:bb:cc:dd:00:01",
				Ip:              "192.168.1.1",
				Type:            gen.Gateway,
				Status:          gen.Connected,
				NumClients:      &numClients,
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
		ListNetworkClientsFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkClientList{
				Items: []gen.NetworkClient{
					{
						Id:             "unifi.aa:bb:cc:dd:ee:01",
						Name:           "laptop",
						Mac:            "aa:bb:cc:dd:ee:01",
						Ip:             &ip,
						ConnectionType: gen.NetworkClientConnectionTypeWired,
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
	for _, want := range []string{"unifi.aa:bb:cc:dd:ee:01", "laptop", "192.168.1.50", "wired"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetClientCmd_wired(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:01",
				"name":           "laptop",
				"mac":            "aa:bb:cc:dd:ee:01",
				"ip":             "192.168.1.50",
				"connectionType": "wired",
				"switchName":     "switch-1",
				"switchPort":     3,
				"uptime":         3600,
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
	for _, want := range []string{"laptop", "switch-1", fmt.Sprintf("%d", 3)} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
