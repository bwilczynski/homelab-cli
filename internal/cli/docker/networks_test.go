package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	dockerapi "github.com/bwilczynski/hlctl/internal/api/docker"
)

func okNetworksResp(list dockerapi.DockerNetworkList) *dockerapi.ListDockerNetworksResponse {
	b, _ := json.Marshal(list)
	return &dockerapi.ListDockerNetworksResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func okNetworkResp(detail dockerapi.DockerNetworkDetail) *dockerapi.GetDockerNetworkResponse {
	b, _ := json.Marshal(detail)
	return &dockerapi.GetDockerNetworkResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &detail}
}

func TestListNetworksCmd_tableOutput(t *testing.T) {
	list := dockerapi.DockerNetworkList{
		Items: []dockerapi.DockerNetwork{
			{
				Id:                  "nas-1.immich_default",
				Name:                "immich_default",
				Device:              "nas-1",
				ConnectedContainers: 4,
			},
		},
	}
	stub := &StubClient{
		ListDockerNetworksWithResponseFunc: func(_ context.Context, _ *dockerapi.ListDockerNetworksParams, _ ...dockerapi.RequestEditorFn) (*dockerapi.ListDockerNetworksResponse, error) {
			return okNetworksResp(list), nil
		},
	}

	cmd := newListNetworksCmd()
	cmdutil.SetClient[DockerClient](cmd, stub)
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
	detail := dockerapi.DockerNetworkDetail{
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
		GetDockerNetworkWithResponseFunc: func(_ context.Context, _ string, _ ...dockerapi.RequestEditorFn) (*dockerapi.GetDockerNetworkResponse, error) {
			return okNetworkResp(detail), nil
		},
	}

	cmd := newGetNetworkCmd()
	cmdutil.SetClient[DockerClient](cmd, stub)
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
