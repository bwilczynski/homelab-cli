package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

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

func okNetworksResp(list gen.DockerNetworkList) *gen.ListDockerNetworksResponse {
	b, _ := json.Marshal(list)
	return &gen.ListDockerNetworksResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func okNetworkResp(detail gen.DockerNetworkDetail) *gen.GetDockerNetworkResponse {
	b, _ := json.Marshal(detail)
	return &gen.GetDockerNetworkResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &detail}
}

func okImagesResp(list gen.DockerImageList) *gen.ListDockerImagesResponse {
	b, _ := json.Marshal(list)
	return &gen.ListDockerImagesResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func okImageResp(detail gen.DockerImageDetail) *gen.GetDockerImageResponse {
	b, _ := json.Marshal(detail)
	return &gen.GetDockerImageResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &detail}
}

func strPtr(s string) *string { return &s }

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
		ListContainersWithResponseFunc: func(_ context.Context, _ *gen.ListContainersParams, _ ...gen.RequestEditorFn) (*gen.ListContainersResponse, error) {
			return errContainersResp(http.StatusUnauthorized, map[string]any{
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
		StartContainerWithResponseFunc: func(_ context.Context, _ string, _ *gen.StartContainerParams, _ ...gen.RequestEditorFn) (*gen.StartContainerResponse, error) {
			return noContentStartResp(), nil
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
		StopContainerWithResponseFunc: func(_ context.Context, _ string, _ *gen.StopContainerParams, _ ...gen.RequestEditorFn) (*gen.StopContainerResponse, error) {
			return noContentStopResp(), nil
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
		RestartContainerWithResponseFunc: func(_ context.Context, _ string, _ *gen.RestartContainerParams, _ ...gen.RequestEditorFn) (*gen.RestartContainerResponse, error) {
			return noContentRestartResp(), nil
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

func TestListNetworksCmd_tableOutput(t *testing.T) {
	list := gen.DockerNetworkList{
		Items: []gen.DockerNetwork{
			{
				Id:                  "nas-1.immich_default",
				Name:                "immich_default",
				Device:              "nas-1",
				ConnectedContainers: 4,
			},
		},
	}
	stub := &StubClient{
		ListDockerNetworksWithResponseFunc: func(_ context.Context, _ *gen.ListDockerNetworksParams, _ ...gen.RequestEditorFn) (*gen.ListDockerNetworksResponse, error) {
			return okNetworksResp(list), nil
		},
	}

	cmd := newListNetworksCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.immich_default", "immich_default", "nas-1", "4"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetNetworkCmd_tableOutput(t *testing.T) {
	subnet := "172.18.0.0/16"
	gateway := "172.18.0.1"
	detail := gen.DockerNetworkDetail{
		Id:                  "nas-1.immich_default",
		Name:                "immich_default",
		Device:              "nas-1",
		ConnectedContainers: 4,
		Driver:              "bridge",
		Subnet:              &subnet,
		Gateway:             &gateway,
		Containers:          []string{"immich_server", "immich_redis"},
	}
	stub := &StubClient{
		GetDockerNetworkWithResponseFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*gen.GetDockerNetworkResponse, error) {
			return okNetworkResp(detail), nil
		},
	}

	cmd := newGetNetworkCmd(stub)
	cmd.SetArgs([]string{"nas-1.immich_default"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.immich_default", "bridge", "172.18.0.0/16", "172.18.0.1", "immich_server", "immich_redis"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListImagesCmd_tableOutput(t *testing.T) {
	list := gen.DockerImageList{
		Items: []gen.DockerImage{
			{
				Id:         "nas-1.925ff61909ae",
				Device:     "nas-1",
				Repository: "ghcr.io/immich-app/immich-server",
				Tags:       []string{"v1.120.0"},
				Size:       524288000,
			},
		},
	}
	stub := &StubClient{
		ListDockerImagesWithResponseFunc: func(_ context.Context, _ *gen.ListDockerImagesParams, _ ...gen.RequestEditorFn) (*gen.ListDockerImagesResponse, error) {
			return okImagesResp(list), nil
		},
	}

	cmd := newListImagesCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.925ff61909ae", "ghcr.io/immich-app/immich-server", "v1.120.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetImageCmd_tableOutput(t *testing.T) {
	detail := gen.DockerImageDetail{
		Id:          "nas-1.925ff61909ae",
		Device:      "nas-1",
		Repository:  "ghcr.io/immich-app/immich-server",
		Tags:        []string{"v1.120.0"},
		Size:        524288000,
		VirtualSize: 1073741824,
	}
	stub := &StubClient{
		GetDockerImageWithResponseFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*gen.GetDockerImageResponse, error) {
			return okImageResp(detail), nil
		},
	}

	cmd := newGetImageCmd(stub)
	cmd.SetArgs([]string{"nas-1.925ff61909ae"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.925ff61909ae", "ghcr.io/immich-app/immich-server", "v1.120.0", "500.0 MB", "1.0 GB"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
