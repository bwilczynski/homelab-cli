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

func TestNewListSsidsCmd_runFCalled(t *testing.T) {
	called := false
	cmd := newListSsidsCmd(cmdutil.TestFactory(t), func(o *listSsidsOptions) error {
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

func TestNewGetSsidCmd_argParsed(t *testing.T) {
	var captured *getSsidOptions
	cmd := newGetSsidCmd(cmdutil.TestFactory(t), func(o *getSsidOptions) error {
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

func TestListSsidsRun_tableOutput(t *testing.T) {
	fixture := map[string]any{
		"items": []any{
			map[string]any{
				"id": "unifi.home", "name": "Home", "vlanId": 1,
				"bands":      []string{"band2g", "band5g", "band6g"},
				"numClients": 12,
			},
			map[string]any{
				"id": "unifi.iot", "name": "IoT", "vlanId": 20,
				"bands":      []string{"band2g", "band5g"},
				"numClients": 8,
			},
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ssids"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &listSsidsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listSsidsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"unifi.home", "Home", "unifi.iot", "IoT", "2.4 GHz", "5 GHz", "6 GHz", "12", "8"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestListSsidsRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ssids"), httpmock.StatusJSONResponse(http.StatusUnauthorized, map[string]any{
		"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized",
		"status": 401, "detail": "Bearer token missing",
	}))

	var out bytes.Buffer
	opts := &listSsidsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	err := listSsidsRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
	reg.Verify(t)
}

func TestGetSsidRun_withClients(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.iot", "uri": "/network/ssids/unifi.iot",
		"name": "IoT", "vlanId": 20,
		"bands":            []string{"band2g", "band5g"},
		"numClients":       2,
		"securityProtocol": "wpa2",
		"clients": []map[string]any{
			{"kind": "client", "id": "unifi.sonos", "uri": "/network/clients/unifi.sonos", "name": "Sonos One SL"},
			{"kind": "client", "id": "unifi.hue", "uri": "/network/clients/unifi.hue", "name": "Philips Hue Bridge"},
		},
		"broadcastingAps": []map[string]any{
			{"kind": "device", "id": "unifi.ap-lr", "uri": "/network/devices/unifi.ap-lr", "name": "AP Living Room"},
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ssids/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getSsidOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.iot",
	}
	if err := getSsidRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"unifi.iot", "IoT", "20", "wpa2", "Sonos One SL", "Philips Hue Bridge", "AP Living Room", "CLIENTS", "BROADCASTING APs"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetSsidRun_notFound(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ssids/*"), httpmock.StatusJSONResponse(http.StatusNotFound, map[string]any{
		"type": "https://homelab.local/problems/not-found", "title": "Not Found",
		"status": 404, "detail": "ssid not found",
	}))

	var out bytes.Buffer
	opts := &getSsidOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.nonexistent",
	}
	err := getSsidRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
	reg.Verify(t)
}
