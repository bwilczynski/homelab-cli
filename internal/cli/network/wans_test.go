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

func TestNewListWansCmd_runFCalled(t *testing.T) {
	called := false
	cmd := newListWansCmd(cmdutil.TestFactory(t), func(o *listWansOptions) error {
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

func TestNewGetWanCmd_argParsed(t *testing.T) {
	var captured *getWanOptions
	cmd := newGetWanCmd(cmdutil.TestFactory(t), func(o *getWanOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"unifi.wan1"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected runF to be called")
	}
	if captured.ID != "unifi.wan1" {
		t.Errorf("expected ID=unifi.wan1, got %q", captured.ID)
	}
}

// Layer 2: business logic via httpmock

func TestListWansRun_tableOutput(t *testing.T) {
	fixture := map[string]any{
		"items": []any{
			map[string]any{"id": "unifi.wan1", "name": "WAN 1", "ipAddress": "203.0.113.42", "uptime": 86400, "status": "connected"},
			map[string]any{"id": "unifi.wan2", "name": "WAN 2", "ipAddress": "198.51.100.7", "uptime": 0, "status": "failover"},
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/wans"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &listWansOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listWansRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"unifi.wan1", "WAN 1", "203.0.113.42", "connected", "unifi.wan2", "failover"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestListWansRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/wans"), httpmock.StatusJSONResponse(http.StatusUnauthorized, map[string]any{
		"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized",
		"status": 401, "detail": "Bearer token missing",
	}))

	var out bytes.Buffer
	opts := &listWansOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	err := listWansRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
	reg.Verify(t)
}

func TestGetWanRun_connected(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.wan1", "uri": "/network/wans/unifi.wan1",
		"name": "WAN 1", "ipAddress": "203.0.113.42",
		"uptime": 86400, "status": "connected",
		"dnsServers": []string{"1.1.1.1", "1.0.0.1"},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/wans/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getWanOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.wan1",
	}
	if err := getWanRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"unifi.wan1", "WAN 1", "203.0.113.42", "connected", "1d", "1.1.1.1", "1.0.0.1"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetWanRun_notFound(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/wans/*"), httpmock.StatusJSONResponse(http.StatusNotFound, map[string]any{
		"type": "https://homelab.local/problems/not-found", "title": "Not Found",
		"status": 404, "detail": "wan not found",
	}))

	var out bytes.Buffer
	opts := &getWanOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.nonexistent",
	}
	err := getWanRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
	reg.Verify(t)
}
