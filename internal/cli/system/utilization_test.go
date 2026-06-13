package system

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
)

func okUtilizationResp(list systemapi.SystemUtilizationList) *systemapi.ListSystemUtilizationResponse {
	b, _ := json.Marshal(list)
	return &systemapi.ListSystemUtilizationResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func TestUtilizationCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListSystemUtilizationWithResponseFunc: func(_ context.Context, _ *systemapi.ListSystemUtilizationParams, _ ...systemapi.RequestEditorFn) (*systemapi.ListSystemUtilizationResponse, error) {
			return okUtilizationResp(systemapi.SystemUtilizationList{
				Items: []systemapi.SystemUtilization{
					{
						Device:    "nas-1",
						SampledAt: time.Now(),
						Cpu:       systemapi.CpuUsage{TotalPercent: 12},
						Memory: systemapi.MemoryUsage{
							UsedPercent:    68,
							SwapTotalBytes: 2_147_483_648,
							SwapUsedBytes:  0,
						},
					},
				},
			}), nil
		},
	}

	cmd := newUtilizationCmd()
	cmdutil.SetClient[SystemClient](cmd, stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1", "12%", "68%"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
