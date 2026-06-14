package system

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

// Layer 1: flag parsing

func TestNewUtilizationCmd_deviceFlag(t *testing.T) {
	var captured *listUtilizationOptions
	cmd := newUtilizationCmd(cmdutil.TestFactory(t), func(o *listUtilizationOptions) error {
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

func TestListUtilizationRun_tableOutput(t *testing.T) {
	util := systemapi.SystemUtilizationList{Items: []systemapi.SystemUtilization{{
		Device:    "nas-1",
		SampledAt: time.Now(),
		Cpu:       systemapi.CpuUsage{TotalPercent: 12},
		Memory: systemapi.MemoryUsage{
			UsedPercent:    68,
			SwapTotalBytes: 2_147_483_648,
			SwapUsedBytes:  0,
		},
	}}}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/system/utilization"), httpmock.JSONResponse(util))

	var out bytes.Buffer
	opts := &listUtilizationOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listUtilizationRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1", "12%", "68%"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}
