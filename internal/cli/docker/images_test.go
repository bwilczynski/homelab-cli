package docker

import (
	"bytes"
	"context"
	"strings"
	"testing"

	dockerapi "github.com/bwilczynski/hlctl/internal/api/docker"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

func TestNewListImagesCmd_deviceFlag(t *testing.T) {
	var captured *listImagesOptions
	cmd := newListImagesCmd(cmdutil.TestFactory(t), func(o *listImagesOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"--device", "nas-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Device != "nas-1" {
		t.Errorf("expected Device=nas-1, got %q", captured.Device)
	}
}

func TestNewGetImageCmd_argParsed(t *testing.T) {
	var captured *getImageOptions
	cmd := newGetImageCmd(cmdutil.TestFactory(t), func(o *getImageOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"nas-1.925ff61909ae"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.ID != "nas-1.925ff61909ae" {
		t.Errorf("expected ID=nas-1.925ff61909ae, got %q", captured.ID)
	}
}

func TestListImagesRun_tableOutput(t *testing.T) {
	list := dockerapi.DockerImageList{
		Items: []dockerapi.DockerImage{{
			Id:         "nas-1.925ff61909ae",
			Device:     "nas-1",
			Repository: "ghcr.io/immich-app/immich-server",
			Tags:       []string{"v1.120.0"},
			Size:       524288000,
		}},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/docker/images"), httpmock.JSONResponse(list))

	var out bytes.Buffer
	opts := &listImagesOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listImagesRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.925ff61909ae", "ghcr.io/immich-app/immich-server", "v1.120.0"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetImageRun_tableOutput(t *testing.T) {
	detail := dockerapi.DockerImageDetail{
		Id:          "nas-1.925ff61909ae",
		Device:      "nas-1",
		Repository:  "ghcr.io/immich-app/immich-server",
		Tags:        []string{"v1.120.0"},
		Size:        524288000,
		VirtualSize: 1073741824,
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/docker/images/*"), httpmock.JSONResponse(detail))

	var out bytes.Buffer
	opts := &getImageOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.925ff61909ae",
	}
	if err := getImageRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.925ff61909ae", "ghcr.io/immich-app/immich-server", "v1.120.0", "500.0 MB", "1.0 GB"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}
