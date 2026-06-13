package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	dockerapi "github.com/bwilczynski/hlctl/internal/api/docker"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
)

func okContainersResp(list dockerapi.ContainerList) *dockerapi.ListContainersResponse {
	b, _ := json.Marshal(list)
	return &dockerapi.ListContainersResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func errContainersResp(status int, body map[string]any) *dockerapi.ListContainersResponse {
	b, _ := json.Marshal(body)
	return &dockerapi.ListContainersResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func okContainerResp(detail dockerapi.ContainerDetail) *dockerapi.GetContainerResponse {
	b, _ := json.Marshal(detail)
	return &dockerapi.GetContainerResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &detail}
}

func noContentStartResp() *dockerapi.StartContainerResponse {
	return &dockerapi.StartContainerResponse{HTTPResponse: &http.Response{StatusCode: http.StatusNoContent}}
}

func noContentStopResp() *dockerapi.StopContainerResponse {
	return &dockerapi.StopContainerResponse{HTTPResponse: &http.Response{StatusCode: http.StatusNoContent}}
}

func noContentRestartResp() *dockerapi.RestartContainerResponse {
	return &dockerapi.RestartContainerResponse{HTTPResponse: &http.Response{StatusCode: http.StatusNoContent}}
}

func TestListContainersCmd_tableOutput(t *testing.T) {
	list := dockerapi.ContainerList{
		Items: []dockerapi.Container{
			{
				Id:     "nas-1.homeassistant",
				Image:  "homeassistant/home-assistant:latest",
				Status: dockerapi.Running,
				Resources: dockerapi.ContainerResources{
					CpuPercent:  1.5,
					MemoryBytes: 104857600,
				},
			},
		},
	}
	stub := &StubClient{
		ListContainersWithResponseFunc: func(_ context.Context, _ *dockerapi.ListContainersParams, _ ...dockerapi.RequestEditorFn) (*dockerapi.ListContainersResponse, error) {
			return okContainersResp(list), nil
		},
	}

	cmd := newListContainersCmd(cmdutil.TestFactory(t))
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
		ListContainersWithResponseFunc: func(_ context.Context, _ *dockerapi.ListContainersParams, _ ...dockerapi.RequestEditorFn) (*dockerapi.ListContainersResponse, error) {
			return errContainersResp(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListContainersCmd(cmdutil.TestFactory(t))
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
	detail := dockerapi.ContainerDetail{
		Id:            "nas-1.homeassistant",
		Name:          "homeassistant",
		Device:        "nas-1",
		Status:        dockerapi.Running,
		Image:         "homeassistant/home-assistant:latest",
		RestartPolicy: dockerapi.Always,
		Resources: dockerapi.ContainerResources{
			CpuPercent:    1.5,
			MemoryBytes:   104857600,
			MemoryPercent: 5.0,
		},
	}
	stub := &StubClient{
		GetContainerWithResponseFunc: func(_ context.Context, _ string, _ ...dockerapi.RequestEditorFn) (*dockerapi.GetContainerResponse, error) {
			return okContainerResp(detail), nil
		},
	}

	cmd := newGetContainerCmd(cmdutil.TestFactory(t))
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
		StartContainerWithResponseFunc: func(_ context.Context, _ string, _ *dockerapi.StartContainerParams, _ ...dockerapi.RequestEditorFn) (*dockerapi.StartContainerResponse, error) {
			return noContentStartResp(), nil
		},
	}
	cmd := newStartContainerCmd()
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
		StopContainerWithResponseFunc: func(_ context.Context, _ string, _ *dockerapi.StopContainerParams, _ ...dockerapi.RequestEditorFn) (*dockerapi.StopContainerResponse, error) {
			return noContentStopResp(), nil
		},
	}
	cmd := newStopContainerCmd()
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
		RestartContainerWithResponseFunc: func(_ context.Context, _ string, _ *dockerapi.RestartContainerParams, _ ...dockerapi.RequestEditorFn) (*dockerapi.RestartContainerResponse, error) {
			return noContentRestartResp(), nil
		},
	}
	cmd := newRestartContainerCmd()
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
