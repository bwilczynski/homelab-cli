package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	gen "github.com/bwilczynski/hlctl/internal/docker"
)

func okContainersResp(list gen.ContainerList) *gen.ListContainersResponse {
	b, _ := json.Marshal(list)
	return &gen.ListContainersResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func errContainersResp(status int, body map[string]any) *gen.ListContainersResponse {
	b, _ := json.Marshal(body)
	return &gen.ListContainersResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func okContainerResp(detail gen.ContainerDetail) *gen.GetContainerResponse {
	b, _ := json.Marshal(detail)
	return &gen.GetContainerResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &detail}
}

func noContentStartResp() *gen.StartContainerResponse {
	return &gen.StartContainerResponse{HTTPResponse: &http.Response{StatusCode: http.StatusNoContent}}
}

func noContentStopResp() *gen.StopContainerResponse {
	return &gen.StopContainerResponse{HTTPResponse: &http.Response{StatusCode: http.StatusNoContent}}
}

func noContentRestartResp() *gen.RestartContainerResponse {
	return &gen.RestartContainerResponse{HTTPResponse: &http.Response{StatusCode: http.StatusNoContent}}
}

func TestListContainersCmd_tableOutput(t *testing.T) {
	list := gen.ContainerList{
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
	}
	stub := &StubClient{
		ListContainersWithResponseFunc: func(_ context.Context, _ *gen.ListContainersParams, _ ...gen.RequestEditorFn) (*gen.ListContainersResponse, error) {
			return okContainersResp(list), nil
		},
	}

	cmd := newListCmd()
	cmdutil.SetClient[DockerClient](cmd, stub)
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
		ListContainersWithResponseFunc: func(_ context.Context, _ *gen.ListContainersParams, _ ...gen.RequestEditorFn) (*gen.ListContainersResponse, error) {
			return errContainersResp(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListCmd()
	cmdutil.SetClient[DockerClient](cmd, stub)
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
	detail := gen.ContainerDetail{
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
	}
	stub := &StubClient{
		GetContainerWithResponseFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*gen.GetContainerResponse, error) {
			return okContainerResp(detail), nil
		},
	}

	cmd := newGetCmd()
	cmdutil.SetClient[DockerClient](cmd, stub)
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
		StartContainerWithResponseFunc: func(_ context.Context, _ string, _ *gen.StartContainerParams, _ ...gen.RequestEditorFn) (*gen.StartContainerResponse, error) {
			return noContentStartResp(), nil
		},
	}
	cmd := newStartCmd()
	cmdutil.SetClient[DockerClient](cmd, stub)
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
		StopContainerWithResponseFunc: func(_ context.Context, _ string, _ *gen.StopContainerParams, _ ...gen.RequestEditorFn) (*gen.StopContainerResponse, error) {
			return noContentStopResp(), nil
		},
	}
	cmd := newStopCmd()
	cmdutil.SetClient[DockerClient](cmd, stub)
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
		RestartContainerWithResponseFunc: func(_ context.Context, _ string, _ *gen.RestartContainerParams, _ ...gen.RequestEditorFn) (*gen.RestartContainerResponse, error) {
			return noContentRestartResp(), nil
		},
	}
	cmd := newRestartCmd()
	cmdutil.SetClient[DockerClient](cmd, stub)
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
