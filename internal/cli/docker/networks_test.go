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

func TestNewListNetworksCmd_deviceFlag(t *testing.T) {
	var captured *listNetworksOptions
	cmd := newListNetworksCmd(cmdutil.TestFactory(t), func(o *listNetworksOptions) error {
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

func TestListNetworksRun_tableOutput(t *testing.T) {
	list := dockerapi.DockerNetworkList{
		Items: []dockerapi.DockerNetwork{{
			Id: "nas-1.immich_default", Name: "immich_default", Device: "nas-1", ConnectedContainers: 4,
		}},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/docker/networks"), httpmock.JSONResponse(list))

	var out bytes.Buffer
	opts := &listNetworksOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listNetworksRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.immich_default", "immich_default", "nas-1", "4"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetNetworkRun_tableOutput(t *testing.T) {
	subnet := "172.18.0.0/16"
	gateway := "172.18.0.1"
	detail := dockerapi.DockerNetworkDetail{
		Id: "nas-1.immich_default", Name: "immich_default", Device: "nas-1",
		ConnectedContainers: 4, Driver: "bridge",
		Subnet: &subnet, Gateway: &gateway,
		Containers: []string{"immich_server", "immich_redis"},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/docker/networks/*"), httpmock.JSONResponse(detail))

	var out bytes.Buffer
	opts := &getNetworkOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.immich_default",
	}
	if err := getNetworkRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.immich_default", "bridge", "172.18.0.0/16", "172.18.0.1", "immich_server", "immich_redis"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}
