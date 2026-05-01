package containers

import (
	"bytes"
	"context"
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
