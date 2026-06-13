package network

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
)

func okClientsResp(list networkapi.NetworkClientList) *networkapi.ListNetworkClientsResponse {
	b, _ := json.Marshal(list)
	return &networkapi.ListNetworkClientsResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func errClientsResp(status int, body map[string]any) *networkapi.ListNetworkClientsResponse {
	b, _ := json.Marshal(body)
	return &networkapi.ListNetworkClientsResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func okClientResp(data map[string]any) *networkapi.GetNetworkClientResponse {
	b, _ := json.Marshal(data)
	var typed networkapi.NetworkClientDetail
	_ = json.Unmarshal(b, &typed)
	return &networkapi.GetNetworkClientResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &typed}
}

func errClientResp(status int, body map[string]any) *networkapi.GetNetworkClientResponse {
	b, _ := json.Marshal(body)
	return &networkapi.GetNetworkClientResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func TestListClientsCmd_tableOutput(t *testing.T) {
	ip := "192.168.1.50"
	stub := &StubClient{
		ListNetworkClientsWithResponseFunc: func(_ context.Context, _ *networkapi.ListNetworkClientsParams, _ ...networkapi.RequestEditorFn) (*networkapi.ListNetworkClientsResponse, error) {
			return okClientsResp(networkapi.NetworkClientList{
				Items: []networkapi.NetworkClient{
					{
						Id:             "unifi.aa:bb:cc:dd:ee:01",
						Name:           "laptop",
						Mac:            "aa:bb:cc:dd:ee:01",
						Ip:             &ip,
						ConnectionType: networkapi.NetworkClientConnectionTypeWired,
						Status:         networkapi.NetworkClientStatusOnline,
					},
				},
			}), nil
		},
	}

	f := cmdutil.TestFactory(t)
	cmd := newListClientsCmd(f)
	cmdutil.SetClient[NetworkClient](cmd, stub)
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

func TestListClientsCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListNetworkClientsWithResponseFunc: func(_ context.Context, _ *networkapi.ListNetworkClientsParams, _ ...networkapi.RequestEditorFn) (*networkapi.ListNetworkClientsResponse, error) {
			return errClientsResp(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}
	f := cmdutil.TestFactory(t)
	cmd := newListClientsCmd(f)
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

func TestListClientsCmd_statusFilter(t *testing.T) {
	var capturedParams *networkapi.ListNetworkClientsParams
	stub := &StubClient{
		ListNetworkClientsWithResponseFunc: func(_ context.Context, params *networkapi.ListNetworkClientsParams, _ ...networkapi.RequestEditorFn) (*networkapi.ListNetworkClientsResponse, error) {
			capturedParams = params
			return okClientsResp(networkapi.NetworkClientList{Items: []networkapi.NetworkClient{}}), nil
		},
	}

	f := cmdutil.TestFactory(t)
	cmd := newListClientsCmd(f)
	cmdutil.SetClient[NetworkClient](cmd, stub)
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
	if *capturedParams.Status != networkapi.NetworkClientStatusOnline {
		t.Errorf("expected status=online, got %q", *capturedParams.Status)
	}
}

func TestGetClientCmd_wired(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientWithResponseFunc: func(_ context.Context, _ string, _ ...networkapi.RequestEditorFn) (*networkapi.GetNetworkClientResponse, error) {
			return okClientResp(map[string]any{
				"id": "unifi.aa:bb:cc:dd:ee:01", "uri": "/network/clients/unifi.aa:bb:cc:dd:ee:01",
				"name": "laptop", "mac": "aa:bb:cc:dd:ee:01", "ip": "192.168.1.50",
				"connectionType": "wired", "status": "online",
				"connectedTo": map[string]any{
					"device": map[string]any{"kind": "device", "id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr", "name": "Switch Living Room"},
					"port":   3, "linkSpeed": "gbe1",
				},
				"uptime": 3600,
			}), nil
		},
	}
	f := cmdutil.TestFactory(t)
	cmd := newGetClientCmd(f)
	cmdutil.SetClient[NetworkClient](cmd, stub)
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

func TestGetClientCmd_wireless(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientWithResponseFunc: func(_ context.Context, _ string, _ ...networkapi.RequestEditorFn) (*networkapi.GetNetworkClientResponse, error) {
			return okClientResp(map[string]any{
				"id": "unifi.aa:bb:cc:dd:ee:02", "uri": "/network/clients/unifi.aa:bb:cc:dd:ee:02",
				"name": "phone", "mac": "aa:bb:cc:dd:ee:02", "ip": "192.168.1.51",
				"connectionType": "wireless", "status": "online",
				"connectedTo": map[string]any{
					"device": map[string]any{"kind": "device", "id": "unifi.ap-living-room", "uri": "/network/devices/unifi.ap-living-room", "name": "AP Living Room"},
					"ssid":   "HomeNet", "signalStrength": -65,
				},
				"uptime": 1800,
			}), nil
		},
	}
	f := cmdutil.TestFactory(t)
	cmd := newGetClientCmd(f)
	cmdutil.SetClient[NetworkClient](cmd, stub)
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
		GetNetworkClientWithResponseFunc: func(_ context.Context, _ string, _ ...networkapi.RequestEditorFn) (*networkapi.GetNetworkClientResponse, error) {
			return okClientResp(map[string]any{
				"id": "unifi.aa:bb:cc:dd:ee:03", "uri": "/network/clients/unifi.aa:bb:cc:dd:ee:03",
				"name": "printer", "mac": "aa:bb:cc:dd:ee:03", "ip": "192.168.1.60",
				"connectionType": "wired", "status": "offline",
				"connectedTo": map[string]any{
					"device": map[string]any{"kind": "device", "id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr", "name": "Switch Living Room"},
				},
			}), nil
		},
	}
	f := cmdutil.TestFactory(t)
	cmd := newGetClientCmd(f)
	cmdutil.SetClient[NetworkClient](cmd, stub)
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
		GetNetworkClientWithResponseFunc: func(_ context.Context, _ string, _ ...networkapi.RequestEditorFn) (*networkapi.GetNetworkClientResponse, error) {
			return okClientResp(map[string]any{
				"id": "unifi.aa:bb:cc:dd:ee:04", "uri": "/network/clients/unifi.aa:bb:cc:dd:ee:04",
				"name": "tablet", "mac": "aa:bb:cc:dd:ee:04", "ip": "192.168.1.70",
				"connectionType": "wireless", "status": "offline",
				"connectedTo": map[string]any{
					"device": map[string]any{"kind": "device", "id": "unifi.ap-living-room", "uri": "/network/devices/unifi.ap-living-room", "name": "AP Living Room"},
					"ssid":   "HomeNet",
				},
			}), nil
		},
	}
	f := cmdutil.TestFactory(t)
	cmd := newGetClientCmd(f)
	cmdutil.SetClient[NetworkClient](cmd, stub)
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

func TestGetClientCmd_notFound(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientWithResponseFunc: func(_ context.Context, _ string, _ ...networkapi.RequestEditorFn) (*networkapi.GetNetworkClientResponse, error) {
			return errClientResp(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "client not found",
			}), nil
		},
	}
	f := cmdutil.TestFactory(t)
	cmd := newGetClientCmd(f)
	cmdutil.SetClient[NetworkClient](cmd, stub)
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
