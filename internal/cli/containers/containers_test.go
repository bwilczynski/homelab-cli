package containers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	gen "github.com/bwilczynski/hlctl/internal/containers"
)

func TestListCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListContainersFunc: func(_ context.Context, _ *gen.ListContainersParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.ContainerList{
				Items: []gen.Container{
					{
						Id:           "nas-1.homeassistant",
						Image:        "ghcr.io/home-assistant/home-assistant:2025.4",
						Status:       gen.Running,
						RestartCount: 0,
						Resources:    gen.ContainerResources{CpuPercent: 2.5, MemoryBytes: 268435456, MemoryPercent: 6.4},
					},
				},
			}), nil
		},
	}

	cmd := newListCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "nas-1.homeassistant") {
		t.Errorf("expected container ID in output, got:\n%s", out)
	}
	if !strings.Contains(out, "256.0 MB") {
		t.Errorf("expected formatted memory in output, got:\n%s", out)
	}
	if !strings.Contains(out, "2.5%") {
		t.Errorf("expected CPU percentage in output, got:\n%s", out)
	}
}

func TestListCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListContainersFunc: func(_ context.Context, _ *gen.ListContainersParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}

	cmd := newListCmd(stub)
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

func TestGetCmd_tableOutput(t *testing.T) {
	detail := gen.ContainerDetail{
		Id:            "nas-1.homeassistant",
		Name:          "homeassistant",
		Device:        "nas-1",
		Status:        gen.Running,
		Image:         "ghcr.io/home-assistant/home-assistant:2025.4",
		RestartCount:  0,
		Resources:     gen.ContainerResources{CpuPercent: 2.5, MemoryBytes: 268435456, MemoryPercent: 6.4},
		ExitCode:      0,
		OomKilled:     false,
		RestartPolicy: "always",
		Privileged:    false,
		MemoryLimit:   0,
		PortBindings: []gen.PortBinding{
			{ContainerPort: 8123, HostPort: 8123, Protocol: "tcp"},
		},
		Networks: []gen.ContainerNetwork{
			{Name: "homeassistant_default", Driver: "bridge"},
		},
		VolumeBindings: []gen.VolumeMount{
			{Source: "/volume1/docker/homeassistant/config", Destination: "/config", Mode: "rw"},
		},
		EnvVariables: []gen.EnvVariable{
			{Key: "TZ", Value: "Europe/Warsaw"},
		},
		Entrypoint: []string{"/init"},
		Cmd:        []string{},
	}

	stub := &StubClient{
		GetContainerFunc: func(_ context.Context, id string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, detail), nil
		},
	}

	cmd := newGetCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"nas-1.homeassistant",
		"256.0 MB",
		"PORT BINDINGS",
		"8123",
		"NETWORKS",
		"homeassistant_default",
		"VOLUME BINDINGS",
		"/config",
		"ENVIRONMENT VARIABLES",
		"Europe/Warsaw",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetCmd_notFound(t *testing.T) {
	stub := &StubClient{
		GetContainerFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "container 'nas-1.foo' does not exist",
			}), nil
		},
	}

	cmd := newGetCmd(stub)
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

func TestStartCmd_success(t *testing.T) {
	stub := &StubClient{
		StartContainerFunc: func(_ context.Context, id string, _ *gen.StartContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}

	cmd := newStartCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "nas-1.homeassistant") {
		t.Errorf("expected container ID in output, got: %s", buf.String())
	}
}

func TestStopCmd_success(t *testing.T) {
	stub := &StubClient{
		StopContainerFunc: func(_ context.Context, id string, _ *gen.StopContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}

	cmd := newStopCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "nas-1.homeassistant") {
		t.Errorf("expected container ID in output, got: %s", buf.String())
	}
}

func TestRestartCmd_success(t *testing.T) {
	stub := &StubClient{
		RestartContainerFunc: func(_ context.Context, id string, _ *gen.RestartContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}

	cmd := newRestartCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "nas-1.homeassistant") {
		t.Errorf("expected container ID in output, got: %s", buf.String())
	}
}

func TestStartCmd_apiError(t *testing.T) {
	stub := &StubClient{
		StartContainerFunc: func(_ context.Context, _ string, _ *gen.StartContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "container 'nas-1.foo' does not exist",
			}), nil
		},
	}

	cmd := newStartCmd(stub)
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

func TestStopCmd_apiError(t *testing.T) {
	stub := &StubClient{
		StopContainerFunc: func(_ context.Context, _ string, _ *gen.StopContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "container 'nas-1.foo' does not exist",
			}), nil
		},
	}

	cmd := newStopCmd(stub)
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

func TestRestartCmd_apiError(t *testing.T) {
	stub := &StubClient{
		RestartContainerFunc: func(_ context.Context, _ string, _ *gen.RestartContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "container 'nas-1.foo' does not exist",
			}), nil
		},
	}

	cmd := newRestartCmd(stub)
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
