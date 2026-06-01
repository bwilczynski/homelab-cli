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

func okNetworksResp(list gen.DockerNetworkList) *gen.ListDockerNetworksResponse {
	b, _ := json.Marshal(list)
	return &gen.ListDockerNetworksResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func okNetworkResp(detail gen.DockerNetworkDetail) *gen.GetDockerNetworkResponse {
	b, _ := json.Marshal(detail)
	return &gen.GetDockerNetworkResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &detail}
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
