package docker

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	dockerapi "github.com/bwilczynski/hlctl/internal/api/docker"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

func testHTTPClient(reg *httpmock.Registry) func() (*http.Client, string, error) {
	return func() (*http.Client, string, error) {
		return &http.Client{Transport: reg}, "http://localhost", nil
	}
}

// Layer 1: flag/arg parsing

func TestNewListContainersCmd_deviceFlag(t *testing.T) {
	var captured *listContainersOptions
	cmd := newListContainersCmd(cmdutil.TestFactory(t), func(o *listContainersOptions) error {
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

func TestNewGetContainerCmd_argParsed(t *testing.T) {
	var captured *getContainerOptions
	cmd := newGetContainerCmd(cmdutil.TestFactory(t), func(o *getContainerOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.ID != "nas-1.homeassistant" {
		t.Errorf("expected ID=nas-1.homeassistant, got %q", captured.ID)
	}
}

func TestNewStartContainerCmd_argParsed(t *testing.T) {
	var captured *startContainerOptions
	cmd := newStartContainerCmd(cmdutil.TestFactory(t), func(o *startContainerOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.ID != "nas-1.homeassistant" {
		t.Errorf("expected ID=nas-1.homeassistant, got %q", captured.ID)
	}
}

// Layer 2: business logic

func TestListContainersRun_tableOutput(t *testing.T) {
	list := dockerapi.ContainerList{
		Items: []dockerapi.Container{{
			Id:        "nas-1.homeassistant",
			Image:     "homeassistant/home-assistant:latest",
			Status:    dockerapi.Running,
			Resources: dockerapi.ContainerResources{CpuPercent: 1.5, MemoryBytes: 104857600},
		}},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/docker/containers"), httpmock.JSONResponse(list))

	var out bytes.Buffer
	opts := &listContainersOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listContainersRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.homeassistant", "homeassistant/home-assistant:latest", "running", "1.5%"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestListContainersRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(
		httpmock.REST("GET", "/docker/containers"),
		httpmock.StatusJSONResponse(http.StatusUnauthorized, map[string]any{
			"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized", "status": 401, "detail": "Bearer token missing",
		}),
	)
	var out bytes.Buffer
	opts := &listContainersOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	err := listContainersRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
	reg.Verify(t)
}

func TestGetContainerRun_tableOutput(t *testing.T) {
	detail := dockerapi.ContainerDetail{
		Id: "nas-1.homeassistant", Name: "homeassistant", Device: "nas-1",
		Status: dockerapi.Running, Image: "homeassistant/home-assistant:latest",
		RestartPolicy: dockerapi.Always,
		Resources:     dockerapi.ContainerResources{CpuPercent: 1.5, MemoryBytes: 104857600, MemoryPercent: 5.0},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/docker/containers/*"), httpmock.JSONResponse(detail))

	var out bytes.Buffer
	opts := &getContainerOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.homeassistant",
	}
	if err := getContainerRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.homeassistant", "homeassistant", "running", "always"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestStartContainerRun(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("POST", "/docker/containers/*:start"), httpmock.StatusStringResponse(http.StatusNoContent, ""))

	var out bytes.Buffer
	opts := &startContainerOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		ID:         "nas-1.homeassistant",
	}
	if err := startContainerRun(context.Background(), opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "nas-1.homeassistant started") {
		t.Errorf("expected 'started' in output, got: %s", out.String())
	}
	reg.Verify(t)
}

func TestStopContainerRun(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("POST", "/docker/containers/*:stop"), httpmock.StatusStringResponse(http.StatusNoContent, ""))

	var out bytes.Buffer
	opts := &stopContainerOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		ID:         "nas-1.homeassistant",
	}
	if err := stopContainerRun(context.Background(), opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "nas-1.homeassistant stopped") {
		t.Errorf("expected 'stopped' in output, got: %s", out.String())
	}
	reg.Verify(t)
}

func TestRestartContainerRun(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("POST", "/docker/containers/*:restart"), httpmock.StatusStringResponse(http.StatusNoContent, ""))

	var out bytes.Buffer
	opts := &restartContainerOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		ID:         "nas-1.homeassistant",
	}
	if err := restartContainerRun(context.Background(), opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "nas-1.homeassistant restarted") {
		t.Errorf("expected 'restarted' in output, got: %s", out.String())
	}
	reg.Verify(t)
}
