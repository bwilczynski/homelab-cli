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

func TestNewTopologyCmd_includeClientsFlag(t *testing.T) {
	var captured *getTopologyOptions
	cmd := newTopologyCmd(cmdutil.TestFactory(t), func(o *getTopologyOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"--include-clients"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected runF to be called")
	}
	if !captured.IncludeClients {
		t.Error("expected IncludeClients=true")
	}
}

func TestNewTopologyCmd_includeWirelessFlag(t *testing.T) {
	var captured *getTopologyOptions
	cmd := newTopologyCmd(cmdutil.TestFactory(t), func(o *getTopologyOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"--include-wireless"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected runF to be called")
	}
	if !captured.IncludeWireless {
		t.Error("expected IncludeWireless=true")
	}
}

// Layer 2: business logic via httpmock

var topologyFixtureDevicesOnly = map[string]any{
	"nodes": []any{
		map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG", "type": "gateway", "status": "connected"},
		map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR", "type": "switch", "status": "connected"},
		map[string]any{"kind": "device", "id": "unifi.apoffice", "uri": "/network/devices/unifi.apoffice", "name": "AP-Office", "type": "accessPoint", "status": "connected"},
		map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR", "type": "accessPoint", "status": "connected", "numClients": 2},
	},
	"edges": []any{
		map[string]any{
			"kind":      "wired",
			"source":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
			"target":    map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG"},
			"port":      1,
			"linkSpeed": "gbe1",
		},
		map[string]any{
			"kind":      "wired",
			"source":    map[string]any{"kind": "device", "id": "unifi.apoffice", "uri": "/network/devices/unifi.apoffice", "name": "AP-Office"},
			"target":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
			"port":      3,
			"linkSpeed": "gbe1",
		},
		map[string]any{
			"kind":      "wired",
			"source":    map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
			"target":    map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG"},
			"port":      2,
			"linkSpeed": "gbe1",
		},
	},
}

func TestGetTopologyRun_devicesOnly(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/topology"), httpmock.JSONResponse(topologyFixtureDevicesOnly))

	var out bytes.Buffer
	opts := &getTopologyOptions{
		IO:              &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient:      testHTTPClient(reg),
		Output:          func() output.Format { return output.FormatTable },
		IncludeClients:  false,
		IncludeWireless: false,
	}
	if err := getTopologyRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		"USG", "gateway",
		"Switch LR", "switch", "port 1", "1 GbE",
		"AP-Office", "accessPoint", "port 3",
		"AP LR", "port 2", "[2 clients]",
		"├──", "└──", "│",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetTopologyRun_includeClientsWiredOnly(t *testing.T) {
	fixture := map[string]any{
		"nodes": []any{
			map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG", "type": "gateway", "status": "connected"},
			map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR", "type": "switch", "status": "connected"},
			map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR", "type": "accessPoint", "status": "connected"},
			map[string]any{"kind": "client", "id": "unifi.nas", "uri": "/network/clients/unifi.nas", "name": "nas-1", "connectionType": "wired", "status": "online"},
			map[string]any{"kind": "client", "id": "unifi.mbp", "uri": "/network/clients/unifi.mbp", "name": "MacBook Pro", "connectionType": "wireless", "status": "online"},
		},
		"edges": []any{
			map[string]any{
				"kind":      "wired",
				"source":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
				"target":    map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG"},
				"port":      1,
				"linkSpeed": "gbe1",
			},
			map[string]any{
				"kind":      "wired",
				"source":    map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
				"target":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
				"port":      7,
				"linkSpeed": "gbe2_5",
			},
			map[string]any{
				"kind":      "wired",
				"source":    map[string]any{"kind": "client", "id": "unifi.nas", "uri": "/network/clients/unifi.nas", "name": "nas-1"},
				"target":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
				"port":      8,
				"linkSpeed": "gbe1",
			},
			map[string]any{
				"kind":           "wireless",
				"source":         map[string]any{"kind": "client", "id": "unifi.mbp", "uri": "/network/clients/unifi.mbp", "name": "MacBook Pro"},
				"target":         map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
				"ssid":           "HomeNet",
				"signalStrength": -55,
			},
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/topology"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getTopologyOptions{
		IO:              &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient:      testHTTPClient(reg),
		Output:          func() output.Format { return output.FormatTable },
		IncludeClients:  true,
		IncludeWireless: false,
	}
	if err := getTopologyRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1", "online", "port 8", "1 GbE"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	for _, absent := range []string{"MacBook Pro", "HomeNet"} {
		if strings.Contains(out.String(), absent) {
			t.Errorf("expected %q absent (wireless filtered), got:\n%s", absent, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetTopologyRun_includeWireless(t *testing.T) {
	fixture := map[string]any{
		"nodes": []any{
			map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG", "type": "gateway", "status": "connected"},
			map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR", "type": "accessPoint", "status": "connected"},
			map[string]any{"kind": "client", "id": "unifi.mbp", "uri": "/network/clients/unifi.mbp", "name": "MacBook Pro", "connectionType": "wireless", "status": "online"},
		},
		"edges": []any{
			map[string]any{
				"kind":      "wired",
				"source":    map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
				"target":    map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG"},
				"port":      1,
				"linkSpeed": "gbe1",
			},
			map[string]any{
				"kind":           "wireless",
				"source":         map[string]any{"kind": "client", "id": "unifi.mbp", "uri": "/network/clients/unifi.mbp", "name": "MacBook Pro"},
				"target":         map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
				"ssid":           "HomeNet",
				"signalStrength": -55,
			},
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/topology"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getTopologyOptions{
		IO:              &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient:      testHTTPClient(reg),
		Output:          func() output.Format { return output.FormatTable },
		IncludeClients:  false,
		IncludeWireless: true,
	}
	if err := getTopologyRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"MacBook Pro", "online", "HomeNet", "-55 dBm"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetTopologyRun_jsonOutput(t *testing.T) {
	fixture := map[string]any{
		"nodes": []any{},
		"edges": []any{},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/topology"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getTopologyOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatJSON },
	}
	if err := getTopologyRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"nodes"`) || !strings.Contains(out.String(), `"edges"`) {
		t.Errorf("expected raw JSON with nodes/edges keys, got:\n%s", out.String())
	}
	reg.Verify(t)
}

func TestGetTopologyRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/topology"), httpmock.StatusJSONResponse(http.StatusUnauthorized, map[string]any{
		"type":   "https://homelab.local/problems/unauthorized",
		"title":  "Unauthorized",
		"status": 401,
		"detail": "Bearer token missing",
	}))

	var out bytes.Buffer
	opts := &getTopologyOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	err := getTopologyRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
	reg.Verify(t)
}
