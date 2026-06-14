package system

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

func testHTTPClient(reg *httpmock.Registry) func() (*http.Client, string, error) {
	return func() (*http.Client, string, error) {
		return &http.Client{Transport: reg}, "http://localhost", nil
	}
}

// Layer 1: runF hook fires

func TestNewHealthCmd_runFCalled(t *testing.T) {
	called := false
	cmd := newHealthCmd(cmdutil.TestFactory(t), func(o *getHealthOptions) error {
		called = true
		return nil
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected runF to be called")
	}
}

// Layer 2: business logic

func TestGetHealthRun_tableOutput(t *testing.T) {
	msg := "disk failing"
	health := systemapi.Health{
		Status:    systemapi.Healthy,
		CheckedAt: time.Now(),
		Components: []systemapi.ComponentHealth{
			{Name: "nas-1", Status: systemapi.Healthy},
			{Name: "unifi", Status: systemapi.Degraded, Message: &msg},
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/system/health"), httpmock.JSONResponse(health))

	var out bytes.Buffer
	opts := &getHealthOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := getHealthRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1", "healthy", "unifi", "degraded"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetHealthRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/system/health"), httpmock.StatusJSONResponse(http.StatusUnauthorized, map[string]any{
		"type":   "https://homelab.local/problems/unauthorized",
		"title":  "Unauthorized",
		"status": 401,
		"detail": "Bearer token missing",
	}))

	var out bytes.Buffer
	opts := &getHealthOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	err := getHealthRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
	reg.Verify(t)
}
