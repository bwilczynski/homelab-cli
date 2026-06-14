package system

import (
	"bytes"
	"context"
	"strings"
	"testing"

	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

// Layer 1: flag parsing

func TestNewInfoCmd_deviceFlag(t *testing.T) {
	var captured *listInfoOptions
	cmd := newInfoCmd(cmdutil.TestFactory(t), func(o *listInfoOptions) error {
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

// Layer 2: business logic

func TestListInfoRun_tableOutput(t *testing.T) {
	info := systemapi.SystemInfoList{Items: []systemapi.SystemInfo{{
		Device:        "nas-1",
		Model:         "DS920+",
		Firmware:      "7.2.1-69057",
		RamMb:         4096,
		UptimeSeconds: 3_931_200,
	}}}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/system/info"), httpmock.JSONResponse(info))

	var out bytes.Buffer
	opts := &listInfoOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listInfoRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1", "DS920+", "7.2.1-69057", "4.0 GB"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}
