// internal/cli/system/system_test.go
package system

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	gen "github.com/bwilczynski/hlctl/internal/system"
)

func TestHealthCmd_tableOutput(t *testing.T) {
	msg := "disk failing"
	stub := &StubClient{
		GetSystemHealthFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.Health{
				Status:    gen.Healthy,
				CheckedAt: time.Now(),
				Components: []gen.ComponentHealth{
					{Name: "nas-1", Status: gen.Healthy},
					{Name: "unifi", Status: gen.Degraded, Message: &msg},
				},
			}), nil
		},
	}

	cmd := newHealthCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1", "healthy", "unifi", "degraded"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestHealthCmd_apiError(t *testing.T) {
	stub := &StubClient{
		GetSystemHealthFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}

	cmd := newHealthCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
}

func TestInfoCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListSystemInfoFunc: func(_ context.Context, _ *gen.ListSystemInfoParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.SystemInfoList{
				Items: []gen.SystemInfo{
					{
						Device:        "nas-1",
						Model:         "DS920+",
						Firmware:      "7.2.1-69057",
						RamMb:         4096,
						UptimeSeconds: 3_931_200,
					},
				},
			}), nil
		},
	}

	cmd := newInfoCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1", "DS920+", "7.2.1-69057", "4.0 GB"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestUtilizationCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListSystemUtilizationFunc: func(_ context.Context, _ *gen.ListSystemUtilizationParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.SystemUtilizationList{
				Items: []gen.SystemUtilization{
					{
						Device:    "nas-1",
						SampledAt: time.Now(),
						Cpu:       gen.CpuUsage{TotalPercent: 12},
						Memory: gen.MemoryUsage{
							UsedPercent:    68,
							SwapTotalBytes: 2_147_483_648,
							SwapUsedBytes:  0,
						},
					},
				},
			}), nil
		},
	}

	cmd := newUtilizationCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1", "12%", "68%"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListUpdatesCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListSystemUpdatesFunc: func(_ context.Context, _ *gen.ListSystemUpdatesParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.SystemUpdateList{
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

	cmd := newListUpdatesCmd(stub)
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
		GetSystemUpdateFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.ContainerSystemUpdateDetail{
				Id:             "nas-1.homeassistant",
				Name:           "homeassistant",
				Device:         "nas-1",
				Type:           gen.ContainerSystemUpdateDetailTypeContainer,
				Status:         gen.UpdateAvailable,
				CurrentVersion: "2024.1.0",
				LatestVersion:  "2024.2.0",
				CheckedAt:      time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
				PublishedAt:    time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
				Image:          "ghcr.io/home-assistant/home-assistant",
				Source:         "https://github.com/home-assistant/core",
				ReleaseUrl:     "https://github.com/home-assistant/core/releases/tag/2024.2.0",
			}), nil
		},
	}

	cmd := newGetUpdateCmd(stub)
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
		GetSystemUpdateFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "update 'nas-1.foo' not found",
			}), nil
		},
	}

	cmd := newGetUpdateCmd(stub)
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
		CheckSystemUpdatesFunc: func(_ context.Context, _ *gen.CheckSystemUpdatesParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.SystemUpdateList{
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

	cmd := newCheckUpdatesCmd(stub)
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
