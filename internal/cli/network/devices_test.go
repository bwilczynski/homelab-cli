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

// Layer 1: runF hook / flag parsing

func TestNewListDevicesCmd_runFCalled(t *testing.T) {
	called := false
	cmd := newListDevicesCmd(cmdutil.TestFactory(t), func(o *listDevicesOptions) error {
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

func TestNewGetDeviceCmd_argParsed(t *testing.T) {
	var captured *getDeviceOptions
	cmd := newGetDeviceCmd(cmdutil.TestFactory(t), func(o *getDeviceOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"unifi.usg"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected runF to be called")
	}
	if captured.ID != "unifi.usg" {
		t.Errorf("expected ID=unifi.usg, got %q", captured.ID)
	}
}

func TestNewGetDeviceCmd_allPortsFlag(t *testing.T) {
	var captured *getDeviceOptions
	cmd := newGetDeviceCmd(cmdutil.TestFactory(t), func(o *getDeviceOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"unifi.switch-lr", "--all-ports"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected runF to be called")
	}
	if !captured.AllPorts {
		t.Error("expected AllPorts=true")
	}
}

// Layer 2: business logic via httpmock

func TestListDevicesRun_tableOutput(t *testing.T) {
	fixture := map[string]any{
		"items": []any{
			map[string]any{"id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG", "mac": "aa:bb:cc:dd:00:01", "ip": "192.168.1.1", "type": "gateway", "status": "connected"},
			map[string]any{"id": "unifi.ap-living-room", "uri": "/network/devices/unifi.ap-living-room", "name": "AP Living Room", "mac": "aa:bb:cc:dd:00:03", "ip": "192.168.1.3", "type": "accessPoint", "status": "connected"},
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/devices"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &listDevicesOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listDevicesRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"unifi.usg", "unifi.ap-living-room", "gateway", "accessPoint"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "CLIENTS") {
		t.Errorf("expected no CLIENTS column in list output, got:\n%s", out.String())
	}
	reg.Verify(t)
}

func TestListDevicesRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/devices"), httpmock.StatusJSONResponse(http.StatusUnauthorized, map[string]any{
		"type":   "https://homelab.local/problems/unauthorized",
		"title":  "Unauthorized",
		"status": 401,
		"detail": "Bearer token missing",
	}))

	var out bytes.Buffer
	opts := &listDevicesOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	err := listDevicesRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
	reg.Verify(t)
}

func TestGetDeviceRun_gateway(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.usg", "uri": "/network/devices/unifi.usg",
		"name": "USG", "mac": "aa:bb:cc:dd:00:01", "ip": "192.168.1.1",
		"type": "gateway", "status": "connected",
		"model": "USG-3P", "firmwareVersion": "4.4.57", "uptime": 86400,
		"traffic": map[string]any{
			"rxBytesTotal": int64(12884901888), "txBytesTotal": int64(4294967296),
			"rxBytesPerSec": int64(125000), "txBytesPerSec": int64(50000),
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/devices/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getDeviceOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.usg",
	}
	if err := getDeviceRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"unifi.usg", "USG-3P", "4.4.57", "gateway", "TRAFFIC RX", "TRAFFIC TX", "1d"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	for _, absent := range []string{"PORTS", "CLIENTS", "UPLINK"} {
		if strings.Contains(out.String(), absent) {
			t.Errorf("expected %q absent for gateway, got:\n%s", absent, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetDeviceRun_unknownWithUplink(t *testing.T) {
	fixture := map[string]any{
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
			"port":   8, "linkSpeed": "gbe1",
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/devices/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getDeviceOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.mystery",
	}
	if err := getDeviceRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"Mystery Device", "UPLINK", "Switch Living Room", "port 8", "1 GbE"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetDeviceRun_switch_activePorts(t *testing.T) {
	fixture := map[string]any{
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
				"traffic":       map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(1200), "txBytesPerSec": int64(500)},
				"connectedTo":   map[string]any{"kind": "device", "id": "unifi.ap-living-room", "uri": "/network/devices/unifi.ap-living-room", "name": "AP Living Room"},
			},
			{
				"number": 2, "state": "down", "poeMode": "off",
				"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
			},
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/devices/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getDeviceOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.switch-lr",
	}
	if err := getDeviceRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"Switch Living Room", "PORTS", "AP Living Room", "1 GbE", "8.5 W", "TRAFFIC RX", "TRAFFIC TX"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "down") {
		t.Errorf("expected down port hidden by default, got:\n%s", out.String())
	}
	reg.Verify(t)
}

func TestGetDeviceRun_switch_allPorts(t *testing.T) {
	fixture := map[string]any{
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
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/devices/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getDeviceOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.switch-lr",
		AllPorts:   true,
	}
	if err := getDeviceRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"down", "disabled"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output with AllPorts=true, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetDeviceRun_accessPoint(t *testing.T) {
	fixture := map[string]any{
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
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/devices/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getDeviceOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.ap-living-room",
	}
	if err := getDeviceRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"AP Living Room", "CLIENTS", "MacBook Pro", "iPhone 15", "HomeNetwork", "-62 dBm", "-70 dBm", "TRAFFIC RX"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "PORTS") {
		t.Errorf("expected no PORTS section for AP, got:\n%s", out.String())
	}
	reg.Verify(t)
}
