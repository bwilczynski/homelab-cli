package network

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

// Layer 1: Cobra wiring — runF hook

func TestNewListPortsCmd_runFCalled(t *testing.T) {
	called := false
	cmd := newListPortsCmd(cmdutil.TestFactory(t), func(o *listPortsOptions) error {
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

func TestNewListPortsCmd_flagsCaptured(t *testing.T) {
	var captured *listPortsOptions
	cmd := newListPortsCmd(cmdutil.TestFactory(t), func(o *listPortsOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{
		"--device", "unifi.switch-living-room",
		"--mode", "trunk",
		"--state", "up",
		"--vlan", "20",
		"--wide",
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected runF to be called")
	}
	if captured.Device != "unifi.switch-living-room" {
		t.Errorf("Device: got %q", captured.Device)
	}
	if captured.Mode != "trunk" {
		t.Errorf("Mode: got %q", captured.Mode)
	}
	if captured.State != "up" {
		t.Errorf("State: got %q", captured.State)
	}
	if captured.VlanID != 20 {
		t.Errorf("VlanID: got %d", captured.VlanID)
	}
	if !captured.Wide {
		t.Error("Wide: expected true")
	}
}

func TestNewListPortsCmd_allPortsAndStateMutuallyExclusive(t *testing.T) {
	cmd := newListPortsCmd(cmdutil.TestFactory(t), func(o *listPortsOptions) error {
		t.Fatal("runF should not be called when flags conflict")
		return nil
	})
	cmd.SetArgs([]string{"--all-ports", "--state", "up"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "none of the others can be") && !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutual exclusion error, got: %v", err)
	}
}

func TestNewListPortsCmd_invalidModeRejected(t *testing.T) {
	cmd := newListPortsCmd(cmdutil.TestFactory(t), func(o *listPortsOptions) error {
		t.Fatal("runF should not be called on invalid mode")
		return nil
	})
	cmd.SetArgs([]string{"--mode", "garbage"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNewListPortsCmd_invalidStateRejected(t *testing.T) {
	cmd := newListPortsCmd(cmdutil.TestFactory(t), func(o *listPortsOptions) error {
		t.Fatal("runF should not be called on invalid state")
		return nil
	})
	cmd.SetArgs([]string{"--state", "garbage"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Layer 2: business logic via httpmock

var portsFixture = map[string]any{
	"items": []any{
		map[string]any{
			"number":     1,
			"state":      "up",
			"linkSpeed":  "gbe1",
			"linkUptime": 172800,
			"poeMode":    "off",
			"vlanConfig": map[string]any{
				"mode": "trunk",
				"nativeVlan": map[string]any{
					"id": "unifi.default", "uri": "/network/vlans/unifi.default",
					"name": "Default", "vlanId": 1,
				},
				"taggedVlans": map[string]any{"scope": "all"},
			},
			"traffic": map[string]any{
				"rxBytesTotal": 43980465111, "txBytesTotal": 87960930222,
				"rxBytesPerSec": 600000, "txBytesPerSec": 1250000,
			},
			"connectedTo": map[string]any{
				"kind": "device", "id": "unifi.usg",
				"uri": "/network/devices/unifi.usg", "name": "USG",
			},
			"device": map[string]any{
				"kind": "device", "id": "unifi.switch-living-room",
				"uri": "/network/devices/unifi.switch-living-room",
				"name": "Switch Living Room",
			},
		},
		map[string]any{
			"number": 7, "label": "AP uplink",
			"state": "up", "linkSpeed": "gbe2_5", "linkUptime": 86400,
			"poeMode": "auto", "poePowerWatts": 4.5,
			"vlanConfig": map[string]any{
				"mode": "trunk",
				"nativeVlan": map[string]any{
					"id": "unifi.default", "uri": "/network/vlans/unifi.default",
					"name": "Default", "vlanId": 1,
				},
				"taggedVlans": map[string]any{
					"scope": "custom",
					"items": []any{
						map[string]any{
							"id": "unifi.iot", "uri": "/network/vlans/unifi.iot",
							"name": "IoT", "vlanId": 20,
						},
					},
				},
			},
			"traffic": map[string]any{
				"rxBytesTotal": 12884901888, "txBytesTotal": 4294967296,
				"rxBytesPerSec": 125000, "txBytesPerSec": 50000,
			},
			"connectedTo": map[string]any{
				"kind": "device", "id": "unifi.ap-living-room",
				"uri": "/network/devices/unifi.ap-living-room", "name": "AP Living Room",
			},
			"device": map[string]any{
				"kind": "device", "id": "unifi.switch-living-room",
				"uri": "/network/devices/unifi.switch-living-room",
				"name": "Switch Living Room",
			},
		},
		map[string]any{
			"number": 8, "state": "up", "linkSpeed": "gbe10", "linkUptime": 3600,
			"sfpModulePresent": true, "poeMode": "off",
			"lagMembership": map[string]any{"id": 8, "role": "master"},
			"vlanConfig": map[string]any{
				"mode": "access",
				"nativeVlan": map[string]any{
					"id": "unifi.servers", "uri": "/network/vlans/unifi.servers",
					"name": "Servers", "vlanId": 100,
				},
			},
			"traffic": map[string]any{
				"rxBytesTotal": 5497558138, "txBytesTotal": 2748779069,
				"rxBytesPerSec": 800000, "txBytesPerSec": 100000,
			},
			"connectedTo": map[string]any{
				"kind": "client", "id": "unifi.nas-1-68",
				"uri": "/network/clients/unifi.nas-1-68", "name": "nas-1",
			},
			"device": map[string]any{
				"kind": "device", "id": "unifi.switch-attic",
				"uri": "/network/devices/unifi.switch-attic",
				"name": "Switch Attic",
			},
		},
	},
}

func TestListPortsRun_tableDefault(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), httpmock.JSONResponse(portsFixture))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listPortsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"DEVICE", "PORT", "LABEL", "STATE", "MODE", "NATIVE VLAN", "TAGGED VLANS", "CONNECTED TO",
		"Switch Living Room", "Switch Attic",
		"AP uplink", "trunk", "access",
		"Default (1)", "Servers (100)",
		"all", "IoT (20)",
		"USG", "AP Living Room", "nas-1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
	// --wide columns must be absent by default
	for _, absent := range []string{"LINK SPEED", "SFP", "RX/S", "TX/S"} {
		if strings.Contains(got, absent) {
			t.Errorf("expected %q absent in default output, got:\n%s", absent, got)
		}
	}
	// Sort order: Switch Attic must appear before Switch Living Room.
	if strings.Index(got, "Switch Attic") > strings.Index(got, "Switch Living Room") {
		t.Errorf("expected switches sorted by name (attic before living room), got:\n%s", got)
	}
	reg.Verify(t)
}

func TestListPortsRun_tableWide(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), httpmock.JSONResponse(portsFixture))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		Wide:       true,
	}
	if err := listPortsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	for _, want := range []string{"LINK SPEED", "POE", "LAG", "SFP", "RX/S", "TX/S", "master #8"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
	reg.Verify(t)
}

// captureURL wraps httpmock.JSONResponse to record the request URL for
// later query-param assertions. httpmock.Registry does not expose a
// Requests() accessor, so the test closes over a local variable.
func captureURL(store **url.URL, body any) httpmock.Responder {
	inner := httpmock.JSONResponse(body)
	return func(req *http.Request) (*http.Response, error) {
		*store = req.URL
		return inner(req)
	}
}

func TestListPortsRun_defaultStateFilter(t *testing.T) {
	var reqURL *url.URL
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), captureURL(&reqURL, portsFixture))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listPortsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqURL == nil {
		t.Fatal("expected request URL to be captured")
	}
	if got := reqURL.Query().Get("state"); got != "up" {
		t.Errorf("expected default state=up on the wire, got %q", got)
	}
	reg.Verify(t)
}

func TestListPortsRun_allPortsOmitsState(t *testing.T) {
	var reqURL *url.URL
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), captureURL(&reqURL, portsFixture))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		AllPorts:   true,
	}
	if err := listPortsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqURL == nil {
		t.Fatal("expected request URL to be captured")
	}
	if reqURL.Query().Has("state") {
		t.Errorf("expected no state query param with --all-ports, got %q", reqURL.Query().Get("state"))
	}
	reg.Verify(t)
}

func TestListPortsRun_allQueryParams(t *testing.T) {
	var reqURL *url.URL
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), captureURL(&reqURL, portsFixture))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		Device:     "unifi.switch-living-room",
		Mode:       "trunk",
		State:      "down",
		VlanID:     20,
	}
	if err := listPortsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqURL == nil {
		t.Fatal("expected request URL to be captured")
	}
	q := reqURL.Query()
	cases := map[string]string{
		"deviceId": "unifi.switch-living-room",
		"mode":     "trunk",
		"state":    "down",
		"vlanId":   "20",
	}
	for k, want := range cases {
		if got := q.Get(k); got != want {
			t.Errorf("query param %s: got %q, want %q", k, got, want)
		}
	}
	reg.Verify(t)
}

func TestListPortsRun_jsonPassthrough(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), httpmock.JSONResponse(portsFixture))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatJSON },
	}
	if err := listPortsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	// Raw body echoed, table headers absent.
	if strings.Contains(got, "DEVICE") || strings.Contains(got, "NATIVE VLAN") {
		t.Errorf("expected JSON passthrough (no table headers), got:\n%s", got)
	}
	if !strings.Contains(got, `"device"`) || !strings.Contains(got, "unifi.switch-living-room") {
		t.Errorf("expected raw JSON body, got:\n%s", got)
	}
	reg.Verify(t)
}

func TestListPortsRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), httpmock.StatusJSONResponse(http.StatusUnauthorized, map[string]any{
		"type":   "https://homelab.local/problems/unauthorized",
		"title":  "Unauthorized",
		"status": 401,
		"detail": "Bearer token missing",
	}))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	err := listPortsRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
	reg.Verify(t)
}
