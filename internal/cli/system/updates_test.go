package system

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/system"
)

func okListUpdatesResp(list gen.SystemUpdateList) *gen.ListSystemUpdatesResponse {
	b, _ := json.Marshal(list)
	return &gen.ListSystemUpdatesResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func okGetUpdateResp(data map[string]any) *gen.GetSystemUpdateResponse {
	b, _ := json.Marshal(data)
	var typed gen.SystemUpdateDetail
	_ = json.Unmarshal(b, &typed)
	return &gen.GetSystemUpdateResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &typed}
}

func errGetUpdateResp(status int, body map[string]any) *gen.GetSystemUpdateResponse {
	b, _ := json.Marshal(body)
	return &gen.GetSystemUpdateResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func okCheckUpdatesResp(list gen.SystemUpdateList) *gen.CheckSystemUpdatesResponse {
	b, _ := json.Marshal(list)
	return &gen.CheckSystemUpdatesResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func TestListUpdatesCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListSystemUpdatesWithResponseFunc: func(_ context.Context, _ *gen.ListSystemUpdatesParams, _ ...gen.RequestEditorFn) (*gen.ListSystemUpdatesResponse, error) {
			return okListUpdatesResp(gen.SystemUpdateList{
				Items: []gen.SystemUpdate{
					{
						Id:             "nas-1.homeassistant",
						Name:           "homeassistant",
						Device:         "nas-1",
						Type:           gen.Container,
						Status:         gen.UpdateAvailable,
						CurrentVersion: "2024.1.0",
						LatestVersion:  "2024.2.0",
						CheckedAt:      time.Now(),
					},
				},
			}), nil
		},
	}

	cmd := newListUpdatesCmd()
	cmdutil.SetClient[SystemClient](cmd, stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.homeassistant", "nas-1", "container", "updateAvailable", "2024.1.0", "2024.2.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetUpdateCmd_containerType(t *testing.T) {
	stub := &StubClient{
		GetSystemUpdateWithResponseFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*gen.GetSystemUpdateResponse, error) {
			return okGetUpdateResp(map[string]any{
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
			}), nil
		},
	}

	cmd := newGetUpdateCmd()
	cmdutil.SetClient[SystemClient](cmd, stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"nas-1.homeassistant", "homeassistant", "container",
		"2024.1.0", "2024.2.0",
		"ghcr.io/home-assistant/home-assistant",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetUpdateCmd_apiError(t *testing.T) {
	stub := &StubClient{
		GetSystemUpdateWithResponseFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*gen.GetSystemUpdateResponse, error) {
			return errGetUpdateResp(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "update 'nas-1.foo' not found",
			}), nil
		},
	}

	cmd := newGetUpdateCmd()
	cmdutil.SetClient[SystemClient](cmd, stub)
	cmd.SetArgs([]string{"nas-1.foo"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
}

func TestCheckUpdatesCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		CheckSystemUpdatesWithResponseFunc: func(_ context.Context, _ *gen.CheckSystemUpdatesParams, _ ...gen.RequestEditorFn) (*gen.CheckSystemUpdatesResponse, error) {
			return okCheckUpdatesResp(gen.SystemUpdateList{
				Items: []gen.SystemUpdate{
					{
						Id:             "nas-1.homeassistant",
						Name:           "homeassistant",
						Device:         "nas-1",
						Type:           gen.Container,
						Status:         gen.UpdateAvailable,
						CurrentVersion: "2024.1.0",
						LatestVersion:  "2024.2.0",
						CheckedAt:      time.Now(),
					},
				},
			}), nil
		},
	}

	cmd := newCheckUpdatesCmd()
	cmdutil.SetClient[SystemClient](cmd, stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.homeassistant", "updateAvailable"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetUpdateCmd_jsonOutput(t *testing.T) {
	stub := &StubClient{
		GetSystemUpdateWithResponseFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*gen.GetSystemUpdateResponse, error) {
			return okGetUpdateResp(map[string]any{
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
			}), nil
		},
	}

	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "json"

	cmd := newGetUpdateCmd()
	cmdutil.SetClient[SystemClient](cmd, stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ghcr.io/home-assistant/home-assistant") {
		t.Errorf("expected raw JSON with image field in output, got:\n%s", out)
	}
	if strings.Contains(out, "Type") && strings.Contains(out, "Version") {
		t.Errorf("unexpected template rendering detected in JSON output:\n%s", out)
	}
}
