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

func testHTTPClient(reg *httpmock.Registry) func() (*http.Client, string, error) {
	return func() (*http.Client, string, error) {
		return &http.Client{Transport: reg}, "http://localhost", nil
	}
}

// Layer 1: runF hook / flag parsing

func TestNewListClientsCmd_statusFlag(t *testing.T) {
	var captured *listClientsOptions
	cmd := newListClientsCmd(cmdutil.TestFactory(t), func(o *listClientsOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"--status", "online"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected runF to be called")
	}
	if captured.Status != "online" {
		t.Errorf("expected Status=online, got %q", captured.Status)
	}
}

func TestNewGetClientCmd_argParsed(t *testing.T) {
	var captured *getClientOptions
	cmd := newGetClientCmd(cmdutil.TestFactory(t), func(o *getClientOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"unifi.aa:bb:cc:dd:ee:01"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected runF to be called")
	}
	if captured.ID != "unifi.aa:bb:cc:dd:ee:01" {
		t.Errorf("expected ID=unifi.aa:bb:cc:dd:ee:01, got %q", captured.ID)
	}
}

// Layer 2: business logic via httpmock

func TestListClientsRun_tableOutput(t *testing.T) {
	fixture := map[string]any{
		"items": []any{
			map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:01",
				"name":           "laptop",
				"mac":            "aa:bb:cc:dd:ee:01",
				"ip":             "192.168.1.50",
				"connectionType": "wired",
				"status":         "online",
			},
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/clients"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &listClientsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listClientsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"unifi.aa:bb:cc:dd:ee:01", "laptop", "192.168.1.50", "wired", "online"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestListClientsRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/clients"), httpmock.StatusJSONResponse(http.StatusUnauthorized, map[string]any{
		"type":   "https://homelab.local/problems/unauthorized",
		"title":  "Unauthorized",
		"status": 401,
		"detail": "Bearer token missing",
	}))

	var out bytes.Buffer
	opts := &listClientsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	err := listClientsRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
	reg.Verify(t)
}

func TestGetClientRun_wired(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.aa:bb:cc:dd:ee:01", "uri": "/network/clients/unifi.aa:bb:cc:dd:ee:01",
		"name": "laptop", "mac": "aa:bb:cc:dd:ee:01", "ip": "192.168.1.50",
		"connectionType": "wired", "status": "online",
		"connectedTo": map[string]any{
			"device": map[string]any{"kind": "device", "id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr", "name": "Switch Living Room"},
			"port":   3, "linkSpeed": "gbe1",
		},
		"uptime": 3600,
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/clients/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getClientOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.aa:bb:cc:dd:ee:01",
	}
	if err := getClientRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"laptop", "Switch Living Room", "3", "1 GbE", "online"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "SWITCH PORT") {
		t.Errorf("expected PORT not SWITCH PORT, got:\n%s", out.String())
	}
	reg.Verify(t)
}

func TestGetClientRun_wireless(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.aa:bb:cc:dd:ee:02", "uri": "/network/clients/unifi.aa:bb:cc:dd:ee:02",
		"name": "phone", "mac": "aa:bb:cc:dd:ee:02", "ip": "192.168.1.51",
		"connectionType": "wireless", "status": "online",
		"connectedTo": map[string]any{
			"device": map[string]any{"kind": "device", "id": "unifi.ap-living-room", "uri": "/network/devices/unifi.ap-living-room", "name": "AP Living Room"},
			"ssid":   "HomeNet", "signalStrength": -65,
		},
		"uptime": 1800,
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/clients/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getClientOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.aa:bb:cc:dd:ee:02",
	}
	if err := getClientRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"phone", "AP Living Room", "HomeNet", "-65 dBm", "online"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "SWITCH") {
		t.Errorf("expected no SWITCH row in wireless output, got:\n%s", out.String())
	}
	reg.Verify(t)
}

func TestGetClientRun_offline_wired(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.aa:bb:cc:dd:ee:03", "uri": "/network/clients/unifi.aa:bb:cc:dd:ee:03",
		"name": "printer", "mac": "aa:bb:cc:dd:ee:03", "ip": "192.168.1.60",
		"connectionType": "wired", "status": "offline",
		"connectedTo": map[string]any{
			"device": map[string]any{"kind": "device", "id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr", "name": "Switch Living Room"},
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/clients/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getClientOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.aa:bb:cc:dd:ee:03",
	}
	if err := getClientRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"printer", "offline", "Switch Living Room"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	for _, absent := range []string{"PORT", "LINK SPEED", "UPTIME"} {
		if strings.Contains(out.String(), absent) {
			t.Errorf("expected %q absent for offline wired client, got:\n%s", absent, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetClientRun_offline_wireless(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.aa:bb:cc:dd:ee:04", "uri": "/network/clients/unifi.aa:bb:cc:dd:ee:04",
		"name": "tablet", "mac": "aa:bb:cc:dd:ee:04", "ip": "192.168.1.70",
		"connectionType": "wireless", "status": "offline",
		"connectedTo": map[string]any{
			"device": map[string]any{"kind": "device", "id": "unifi.ap-living-room", "uri": "/network/devices/unifi.ap-living-room", "name": "AP Living Room"},
			"ssid":   "HomeNet",
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/clients/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getClientOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.aa:bb:cc:dd:ee:04",
	}
	if err := getClientRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"tablet", "offline", "AP Living Room", "HomeNet"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	for _, absent := range []string{"SIGNAL", "UPTIME"} {
		if strings.Contains(out.String(), absent) {
			t.Errorf("expected %q absent for offline wireless client, got:\n%s", absent, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetClientRun_notFound(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/clients/*"), httpmock.StatusJSONResponse(http.StatusNotFound, map[string]any{
		"type":   "https://homelab.local/problems/not-found",
		"title":  "Not Found",
		"status": 404,
		"detail": "client not found",
	}))

	var out bytes.Buffer
	opts := &getClientOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.nonexistent",
	}
	err := getClientRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
	reg.Verify(t)
}
