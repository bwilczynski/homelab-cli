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

func okImagesResp(list gen.DockerImageList) *gen.ListDockerImagesResponse {
	b, _ := json.Marshal(list)
	return &gen.ListDockerImagesResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func okImageResp(detail gen.DockerImageDetail) *gen.GetDockerImageResponse {
	b, _ := json.Marshal(detail)
	return &gen.GetDockerImageResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &detail}
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

	cmd := newListImagesCmd()
	cmdutil.SetClient[DockerClient](cmd, stub)
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

	cmd := newGetImageCmd()
	cmdutil.SetClient[DockerClient](cmd, stub)
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
