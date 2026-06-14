package network

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

// Layer 1: runF hook

func TestNewListVlansCmd_runFCalled(t *testing.T) {
	called := false
	cmd := newListVlansCmd(cmdutil.TestFactory(t), func(o *listVlansOptions) error {
		called = true
		return nil
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected runF to be called")
	}
}

func TestNewGetVlanCmd_argParsed(t *testing.T) {
	var captured *getVlanOptions
	cmd := newGetVlanCmd(cmdutil.TestFactory(t), func(o *getVlanOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"unifi.iot"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected runF to be called")
	}
	if captured.ID != "unifi.iot" {
		t.Errorf("expected ID=unifi.iot, got %q", captured.ID)
	}
}

// Layer 2: business logic via httpmock

func TestListVlansRun_tableOutput(t *testing.T) {
	fixture := map[string]any{
		"items": []any{
			map[string]any{"id": "unifi.default", "name": "Default", "vlanId": 1, "subnet": "192.168.1.0/24"},
			map[string]any{"id": "unifi.iot", "name": "IoT", "vlanId": 20, "subnet": "192.168.20.0/24"},
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/vlans"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &listVlansOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listVlansRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"unifi.default", "Default", "192.168.1.0/24", "unifi.iot", "IoT", "20"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestListVlansRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/vlans"), httpmock.StatusJSONResponse(http.StatusUnauthorized, map[string]any{
		"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized",
		"status": 401, "detail": "Bearer token missing",
	}))

	var out bytes.Buffer
	opts := &listVlansOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	err := listVlansRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
	reg.Verify(t)
}

func TestGetVlanRun_serverDhcp(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.iot", "uri": "/network/vlans/unifi.iot",
		"name": "IoT", "vlanId": 20, "subnet": "192.168.20.0/24",
		"gatewayIp": "192.168.20.1", "broadcastIp": "192.168.20.255",
		"dhcpMode":   "server",
		"dhcpRange":  map[string]any{"start": "192.168.20.100", "end": "192.168.20.200"},
		"dnsServers": []string{"1.1.1.1", "8.8.8.8"},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/vlans/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getVlanOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.iot",
	}
	if err := getVlanRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"unifi.iot", "IoT", "20", "192.168.20.0/24", "192.168.20.1", "192.168.20.255", "server", "192.168.20.100", "192.168.20.200", "1.1.1.1"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "RELAY") {
		t.Errorf("expected no RELAY row for server DHCP, got:\n%s", out.String())
	}
	reg.Verify(t)
}

func TestGetVlanRun_relayDhcp(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.mgmt", "uri": "/network/vlans/unifi.mgmt",
		"name": "Management", "vlanId": 99, "subnet": "10.0.99.0/24",
		"gatewayIp": "10.0.99.1", "broadcastIp": "10.0.99.255",
		"dhcpMode": "relay", "relayServer": "192.168.1.1",
		"dnsServers": []string{"192.168.1.1"},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/vlans/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getVlanOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.mgmt",
	}
	if err := getVlanRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"Management", "relay", "192.168.1.1", "RELAY"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "DHCP RANGE") {
		t.Errorf("expected no DHCP RANGE row for relay DHCP, got:\n%s", out.String())
	}
	reg.Verify(t)
}

func TestGetVlanRun_notFound(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/vlans/*"), httpmock.StatusJSONResponse(http.StatusNotFound, map[string]any{
		"type": "https://homelab.local/problems/not-found", "title": "Not Found",
		"status": 404, "detail": "vlan not found",
	}))

	var out bytes.Buffer
	opts := &getVlanOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.nonexistent",
	}
	err := getVlanRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
	reg.Verify(t)
}

func TestGetVlanRun_disabledDhcp(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.servers", "uri": "/network/vlans/unifi.servers",
		"name": "Servers", "vlanId": 10, "subnet": "192.168.10.0/24",
		"gatewayIp": "192.168.10.1", "broadcastIp": "192.168.10.255",
		"dhcpMode":   "disabled",
		"dnsServers": []string{"1.1.1.1"},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/vlans/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getVlanOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.servers",
	}
	if err := getVlanRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"Servers", "disabled", "1.1.1.1"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	for _, absent := range []string{"DHCP RANGE", "RELAY"} {
		if strings.Contains(out.String(), absent) {
			t.Errorf("expected %q absent for disabled DHCP, got:\n%s", absent, out.String())
		}
	}
	reg.Verify(t)
}
