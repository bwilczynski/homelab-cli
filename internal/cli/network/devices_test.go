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
	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
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

func TestBuildDevicePortViews_newFields(t *testing.T) {
	label := "Backhaul"
	uptime := 90061 // 1d 1h 1m 1s
	sfp := true
	poeOff := networkapi.DevicePortPoeMode("off")
	iotVlan := networkapi.NetworkVlanRef{Id: "unifi.iot", Uri: "/network/vlans/unifi.iot", Name: "IoT", VlanId: 20}
	guestVlan := networkapi.NetworkVlanRef{Id: "unifi.guest", Uri: "/network/vlans/unifi.guest", Name: "Guest", VlanId: 30}
	ports := []networkapi.DevicePort{
		{
			Number:  1,
			State:   networkapi.NetworkPortStateUp,
			PoeMode: &poeOff,
			Label:   &label,
			LinkUptime: &uptime,
			SfpModulePresent: &sfp,
			LagMembership: &networkapi.DevicePortLagMembership{
				Id:   3,
				Role: networkapi.DevicePortLagMembershipRoleMaster,
			},
			VlanConfig: &networkapi.DevicePortVlanConfig{
				Mode:       networkapi.DevicePortVlanModeTrunk,
				NativeVlan: networkapi.NetworkVlanRef{Id: "unifi.default", Uri: "/network/vlans/unifi.default", Name: "Default", VlanId: 1},
				TaggedVlans: &struct {
					Items *[]networkapi.NetworkVlanRef                         `json:"items,omitempty"`
					Scope networkapi.DevicePortVlanConfigTaggedVlansScope `json:"scope"`
				}{
					Scope: networkapi.DevicePortVlanConfigTaggedVlansScopeCustom,
					Items: &[]networkapi.NetworkVlanRef{iotVlan, guestVlan},
				},
			},
			Traffic: networkapi.NetworkTraffic{},
		},
		{
			Number:  2,
			State:   networkapi.NetworkPortStateUp,
			PoeMode: &poeOff,
			VlanConfig: &networkapi.DevicePortVlanConfig{
				Mode:       networkapi.DevicePortVlanModeTrunk,
				NativeVlan: networkapi.NetworkVlanRef{Id: "unifi.default", Uri: "/network/vlans/unifi.default", Name: "Default", VlanId: 1},
				TaggedVlans: &struct {
					Items *[]networkapi.NetworkVlanRef                         `json:"items,omitempty"`
					Scope networkapi.DevicePortVlanConfigTaggedVlansScope `json:"scope"`
				}{
					Scope: networkapi.DevicePortVlanConfigTaggedVlansScopeAll,
				},
			},
			Traffic: networkapi.NetworkTraffic{},
		},
		{
			Number:  3,
			State:   networkapi.NetworkPortStateUp,
			PoeMode: &poeOff,
			VlanConfig: &networkapi.DevicePortVlanConfig{
				Mode:       networkapi.DevicePortVlanModeAccess,
				NativeVlan: networkapi.NetworkVlanRef{Id: "unifi.default", Uri: "/network/vlans/unifi.default", Name: "Default", VlanId: 1},
			},
			Traffic: networkapi.NetworkTraffic{},
		},
		{
			Number:  4,
			State:   networkapi.NetworkPortStateUp,
			PoeMode: &poeOff,
			Traffic: networkapi.NetworkTraffic{},
		},
	}

	views, err := buildDevicePortViews(ports, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(views) != 4 {
		t.Fatalf("expected 4 views, got %d", len(views))
	}

	// Port 1: all new fields set
	p1 := views[0]
	if p1.Label != "Backhaul" {
		t.Errorf("port 1 Label: got %q, want %q", p1.Label, "Backhaul")
	}
	if p1.LinkUptime != "1d 1h 1m 1s" {
		t.Errorf("port 1 LinkUptime: got %q, want %q", p1.LinkUptime, "1d 1h 1m 1s")
	}
	if p1.SfpPresent != "yes" {
		t.Errorf("port 1 SfpPresent: got %q, want %q", p1.SfpPresent, "yes")
	}
	if p1.LagInfo != "master #3" {
		t.Errorf("port 1 LagInfo: got %q, want %q", p1.LagInfo, "master #3")
	}
	if p1.VlanMode != "trunk" {
		t.Errorf("port 1 VlanMode: got %q, want %q", p1.VlanMode, "trunk")
	}
	if p1.NativeVlan != "Default (1)" {
		t.Errorf("port 1 NativeVlan: got %q, want %q", p1.NativeVlan, "Default (1)")
	}
	if p1.TaggedVlans != "IoT (20), Guest (30)" {
		t.Errorf("port 1 TaggedVlans: got %q, want %q", p1.TaggedVlans, "IoT (20), Guest (30)")
	}

	// Port 2: scope=all
	p2 := views[1]
	if p2.TaggedVlans != "all" {
		t.Errorf("port 2 TaggedVlans (scope=all): got %q, want %q", p2.TaggedVlans, "all")
	}

	// Port 3: access mode → tagged vlans = "-"
	p3 := views[2]
	if p3.TaggedVlans != "-" {
		t.Errorf("port 3 TaggedVlans (access): got %q, want %q", p3.TaggedVlans, "-")
	}

	// Port 4: no vlanConfig → all VLAN fields "-"
	p4 := views[3]
	if p4.VlanMode != "-" {
		t.Errorf("port 4 VlanMode (nil): got %q, want %q", p4.VlanMode, "-")
	}
	if p4.NativeVlan != "-" {
		t.Errorf("port 4 NativeVlan (nil): got %q, want %q", p4.NativeVlan, "-")
	}
	if p4.Label != "-" {
		t.Errorf("port 4 Label (nil): got %q, want %q", p4.Label, "-")
	}
	if p4.LinkUptime != "-" {
		t.Errorf("port 4 LinkUptime (nil): got %q, want %q", p4.LinkUptime, "-")
	}
	if p4.LagInfo != "-" {
		t.Errorf("port 4 LagInfo (nil): got %q, want %q", p4.LagInfo, "-")
	}
	if p4.SfpPresent != "-" {
		t.Errorf("port 4 SfpPresent (nil): got %q, want %q", p4.SfpPresent, "-")
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
		"ports": []any{},
		"wans":  []any{},
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
	for _, absent := range []string{"CLIENTS", "UPLINK"} {
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

func TestGetDeviceRun_switch_vlanConfig(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr",
		"name": "Switch LR", "mac": "aa:bb:cc:dd:00:10", "ip": "192.168.1.10",
		"type": "switch", "status": "connected",
		"model": "USW-24-PoE", "firmwareVersion": "6.2.14", "uptime": 86400,
		"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
		"ports": []map[string]any{
			{
				"number": 1, "state": "up", "poeMode": "auto",
				"label": "Uplink",
				"linkUptime": 3661,
				"lagMembership": map[string]any{"id": 3, "role": "master"},
				"sfpModulePresent": true,
				"vlanConfig": map[string]any{
					"mode": "trunk",
					"nativeVlan": map[string]any{"id": "unifi.default", "uri": "/network/vlans/unifi.default", "name": "Default", "vlanId": 1},
					"taggedVlans": map[string]any{
						"scope": "custom",
						"items": []map[string]any{
							{"id": "unifi.iot", "uri": "/network/vlans/unifi.iot", "name": "IoT", "vlanId": 20},
							{"id": "unifi.guest", "uri": "/network/vlans/unifi.guest", "name": "Guest", "vlanId": 30},
						},
					},
				},
				"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(1000), "txBytesPerSec": int64(500)},
			},
			{
				"number": 2, "state": "up", "poeMode": "off",
				"vlanConfig": map[string]any{
					"mode": "trunk",
					"nativeVlan": map[string]any{"id": "unifi.default", "uri": "/network/vlans/unifi.default", "name": "Default", "vlanId": 1},
					"taggedVlans": map[string]any{"scope": "all"},
				},
				"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
			},
			{
				"number": 3, "state": "up", "poeMode": "off",
				"vlanConfig": map[string]any{
					"mode": "access",
					"nativeVlan": map[string]any{"id": "unifi.iot", "uri": "/network/vlans/unifi.iot", "name": "IoT", "vlanId": 20},
				},
				"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
			},
			{
				"number": 4, "state": "down", "poeMode": "off",
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
	got := out.String()

	// Summary table columns
	for _, want := range []string{"LABEL", "UPTIME", "SFP", "LAG", "Uplink", "1h 1m 1s", "yes", "master #3"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}

	// VLAN CONFIG section
	for _, want := range []string{
		"VLAN CONFIG",
		"trunk", "access",
		"Default (1)", "IoT (20)",
		"IoT (20), Guest (30)",
		"all",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in VLAN CONFIG output, got:\n%s", want, got)
		}
	}

	// Port 4 (down) should be hidden by default
	if strings.Contains(got, "down") {
		t.Errorf("expected down port 4 hidden by default, got:\n%s", got)
	}

	reg.Verify(t)
}

func TestGetDeviceRun_switch_vlanConfig_allPorts(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr",
		"name": "Switch LR", "mac": "aa:bb:cc:dd:00:10", "ip": "192.168.1.10",
		"type": "switch", "status": "connected",
		"model": "USW-24-PoE", "firmwareVersion": "6.2.14", "uptime": 3600,
		"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
		"ports": []map[string]any{
			{
				"number": 1, "state": "up", "poeMode": "off",
				"vlanConfig": map[string]any{
					"mode":       "access",
					"nativeVlan": map[string]any{"id": "unifi.default", "uri": "/network/vlans/unifi.default", "name": "Default", "vlanId": 1},
				},
				"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
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
		AllPorts:   true,
	}
	if err := getDeviceRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()

	// Both ports appear in VLAN CONFIG section when --all-ports
	if !strings.Contains(got, "down") {
		t.Errorf("expected down port visible with --all-ports, got:\n%s", got)
	}
	// Port 2 has no vlanConfig → "-" in VLAN columns
	if !strings.Contains(got, "VLAN CONFIG") {
		t.Errorf("expected VLAN CONFIG section, got:\n%s", got)
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
