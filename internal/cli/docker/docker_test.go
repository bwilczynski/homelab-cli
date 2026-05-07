package docker

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	gen "github.com/bwilczynski/hlctl/internal/docker"
)

func TestListContainersCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListContainersFunc: func(_ context.Context, _ *gen.ListContainersParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.ContainerList{
				Items: []gen.Container{
					{
						Id:     "nas-1.homeassistant",
						Image:  "homeassistant/home-assistant:latest",
						Status: gen.Running,
						Resources: gen.ContainerResources{
							CpuPercent:  1.5,
							MemoryBytes: 104857600,
						},
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
	for _, want := range []string{"nas-1.homeassistant", "homeassistant/home-assistant:latest", "running", "1.5%"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListContainersCmd_apiError(t *testing.T) {
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

func TestGetContainerCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		GetContainerFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.ContainerDetail{
				Id:            "nas-1.homeassistant",
				Name:          "homeassistant",
				Device:        "nas-1",
				Status:        gen.Running,
				Image:         "homeassistant/home-assistant:latest",
				RestartPolicy: gen.Always,
				Resources: gen.ContainerResources{
					CpuPercent:    1.5,
					MemoryBytes:   104857600,
					MemoryPercent: 5.0,
				},
			}), nil
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
	for _, want := range []string{"nas-1.homeassistant", "homeassistant", "running", "always"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestStartContainerCmd(t *testing.T) {
	stub := &StubClient{
		StartContainerFunc: func(_ context.Context, _ string, _ *gen.StartContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
		},
	}
	cmd := newStartCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "started") {
		t.Errorf("expected 'started' in output, got: %s", buf.String())
	}
}

func TestStopContainerCmd(t *testing.T) {
	stub := &StubClient{
		StopContainerFunc: func(_ context.Context, _ string, _ *gen.StopContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
		},
	}
	cmd := newStopCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "stopped") {
		t.Errorf("expected 'stopped' in output, got: %s", buf.String())
	}
}

func TestRestartContainerCmd(t *testing.T) {
	stub := &StubClient{
		RestartContainerFunc: func(_ context.Context, _ string, _ *gen.RestartContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
		},
	}
	cmd := newRestartCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "restarted") {
		t.Errorf("expected 'restarted' in output, got: %s", buf.String())
	}
}
