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

// Layer 1: flag/arg parsing

func TestNewListUpdatesCmd_statusFlag(t *testing.T) {
	var captured *listUpdatesOptions
	cmd := newListUpdatesCmd(cmdutil.TestFactory(t), func(o *listUpdatesOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"--status", "updateAvailable"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Status != "updateAvailable" {
		t.Errorf("expected Status=updateAvailable, got %q", captured.Status)
	}
}

func TestNewGetUpdateCmd_argParsed(t *testing.T) {
	var captured *getUpdateOptions
	cmd := newGetUpdateCmd(cmdutil.TestFactory(t), func(o *getUpdateOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.ID != "nas-1.homeassistant" {
		t.Errorf("expected ID=nas-1.homeassistant, got %q", captured.ID)
	}
}

func TestNewCheckUpdatesCmd_runFCalled(t *testing.T) {
	called := false
	cmd := newCheckUpdatesCmd(cmdutil.TestFactory(t), func(o *checkUpdatesOptions) error {
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

func TestListUpdatesRun_tableOutput(t *testing.T) {
	list := systemapi.SystemUpdateList{Items: []systemapi.SystemUpdate{{
		Id:             "nas-1.homeassistant",
		Name:           "homeassistant",
		Device:         "nas-1",
		Type:           systemapi.Container,
		Status:         systemapi.UpdateAvailable,
		CurrentVersion: "2024.1.0",
		LatestVersion:  "2024.2.0",
		CheckedAt:      time.Now(),
	}}}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/system/updates"), httpmock.JSONResponse(list))

	var out bytes.Buffer
	opts := &listUpdatesOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listUpdatesRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.homeassistant", "nas-1", "container", "updateAvailable", "2024.1.0", "2024.2.0"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetUpdateRun_containerType(t *testing.T) {
	detail := map[string]any{
		"id":             "nas-1.homeassistant",
		"name":           "homeassistant",
		"device":         "nas-1",
		"type":           "container",
		"status":         "updateAvailable",
		"currentVersion": "2024.1.0",
		"latestVersion":  "2024.2.0",
		"checkedAt":      time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
		"publishedAt":    time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
		"image":          "ghcr.io/home-assistant/home-assistant",
		"source":         "https://github.com/home-assistant/core",
		"releaseUrl":     "https://github.com/home-assistant/core/releases/tag/2024.2.0",
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/system/updates/*"), httpmock.JSONResponse(detail))

	var out bytes.Buffer
	opts := &getUpdateOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.homeassistant",
	}
	if err := getUpdateRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{
		"nas-1.homeassistant", "homeassistant", "container",
		"2024.1.0", "2024.2.0",
		"ghcr.io/home-assistant/home-assistant",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetUpdateRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/system/updates/*"), httpmock.StatusJSONResponse(http.StatusNotFound, map[string]any{
		"type": "https://homelab.local/problems/not-found", "title": "Not Found", "status": 404, "detail": "update 'nas-1.foo' not found",
	}))

	var out bytes.Buffer
	opts := &getUpdateOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.foo",
	}
	err := getUpdateRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
	reg.Verify(t)
}

func TestCheckUpdatesRun_tableOutput(t *testing.T) {
	list := systemapi.SystemUpdateList{Items: []systemapi.SystemUpdate{{
		Id:             "nas-1.homeassistant",
		Name:           "homeassistant",
		Device:         "nas-1",
		Type:           systemapi.Container,
		Status:         systemapi.UpdateAvailable,
		CurrentVersion: "2024.1.0",
		LatestVersion:  "2024.2.0",
		CheckedAt:      time.Now(),
	}}}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("POST", "/system/updates:check"), httpmock.JSONResponse(list))

	var out bytes.Buffer
	opts := &checkUpdatesOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := checkUpdatesRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.homeassistant", "updateAvailable"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetUpdateRun_jsonOutput(t *testing.T) {
	detail := map[string]any{
		"id":             "nas-1.homeassistant",
		"name":           "homeassistant",
		"device":         "nas-1",
		"type":           "container",
		"status":         "updateAvailable",
		"currentVersion": "2024.1.0",
		"latestVersion":  "2024.2.0",
		"checkedAt":      time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
		"image":          "ghcr.io/home-assistant/home-assistant",
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/system/updates/*"), httpmock.JSONResponse(detail))

	var out bytes.Buffer
	opts := &getUpdateOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatJSON },
		ID:         "nas-1.homeassistant",
	}
	if err := getUpdateRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "ghcr.io/home-assistant/home-assistant") {
		t.Errorf("expected raw JSON with image field in output, got:\n%s", out.String())
	}
	reg.Verify(t)
}
