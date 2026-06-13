package network

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
)

func okVlansResp(list networkapi.VlanList) *networkapi.ListVlansResponse {
	b, _ := json.Marshal(list)
	return &networkapi.ListVlansResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func errVlansResp(status int, body map[string]any) *networkapi.ListVlansResponse {
	b, _ := json.Marshal(body)
	return &networkapi.ListVlansResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func okVlanResp(data map[string]any) *networkapi.GetVlanResponse {
	b, _ := json.Marshal(data)
	var typed networkapi.VlanDetail
	_ = json.Unmarshal(b, &typed)
	return &networkapi.GetVlanResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &typed}
}

func errVlanResp(status int, body map[string]any) *networkapi.GetVlanResponse {
	b, _ := json.Marshal(body)
	return &networkapi.GetVlanResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func TestListVlansCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListVlansWithResponseFunc: func(_ context.Context, _ ...networkapi.RequestEditorFn) (*networkapi.ListVlansResponse, error) {
			return okVlansResp(networkapi.VlanList{
				Items: []networkapi.Vlan{
					{Id: "unifi.default", Name: "Default", VlanId: 1, Subnet: "192.168.1.0/24"},
					{Id: "unifi.iot", Name: "IoT", VlanId: 20, Subnet: "192.168.20.0/24"},
				},
			}), nil
		},
	}
	cmd := newListVlansCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
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
		ListVlansWithResponseFunc: func(_ context.Context, _ ...networkapi.RequestEditorFn) (*networkapi.ListVlansResponse, error) {
			return errVlansResp(http.StatusUnauthorized, map[string]any{
				"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized",
				"status": 401, "detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListVlansCmd()
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

func TestGetVlanCmd_serverDhcp(t *testing.T) {
	stub := &StubClient{
		GetVlanWithResponseFunc: func(_ context.Context, _ string, _ ...networkapi.RequestEditorFn) (*networkapi.GetVlanResponse, error) {
			return okVlanResp(map[string]any{
				"id": "unifi.iot", "uri": "/network/vlans/unifi.iot",
				"name": "IoT", "vlanId": 20, "subnet": "192.168.20.0/24",
				"gatewayIp": "192.168.20.1", "broadcastIp": "192.168.20.255",
				"dhcpMode": "server",
				"dhcpRange": map[string]any{"start": "192.168.20.100", "end": "192.168.20.200"},
				"dnsServers": []string{"1.1.1.1", "8.8.8.8"},
			}), nil
		},
	}
	cmd := newGetVlanCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
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
		GetVlanWithResponseFunc: func(_ context.Context, _ string, _ ...networkapi.RequestEditorFn) (*networkapi.GetVlanResponse, error) {
			return okVlanResp(map[string]any{
				"id": "unifi.mgmt", "uri": "/network/vlans/unifi.mgmt",
				"name": "Management", "vlanId": 99, "subnet": "10.0.99.0/24",
				"gatewayIp": "10.0.99.1", "broadcastIp": "10.0.99.255",
				"dhcpMode": "relay", "relayServer": "192.168.1.1",
				"dnsServers": []string{"192.168.1.1"},
			}), nil
		},
	}
	cmd := newGetVlanCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
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
		GetVlanWithResponseFunc: func(_ context.Context, _ string, _ ...networkapi.RequestEditorFn) (*networkapi.GetVlanResponse, error) {
			return errVlanResp(http.StatusNotFound, map[string]any{
				"type": "https://homelab.local/problems/not-found", "title": "Not Found",
				"status": 404, "detail": "vlan not found",
			}), nil
		},
	}
	cmd := newGetVlanCmd()
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

func TestGetVlanCmd_disabledDhcp(t *testing.T) {
	stub := &StubClient{
		GetVlanWithResponseFunc: func(_ context.Context, _ string, _ ...networkapi.RequestEditorFn) (*networkapi.GetVlanResponse, error) {
			return okVlanResp(map[string]any{
				"id": "unifi.servers", "uri": "/network/vlans/unifi.servers",
				"name": "Servers", "vlanId": 10, "subnet": "192.168.10.0/24",
				"gatewayIp": "192.168.10.1", "broadcastIp": "192.168.10.255",
				"dhcpMode": "disabled",
				"dnsServers": []string{"1.1.1.1"},
			}), nil
		},
	}
	cmd := newGetVlanCmd()
	cmdutil.SetClient[NetworkClient](cmd, stub)
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
