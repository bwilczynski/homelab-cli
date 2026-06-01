package system

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	gen "github.com/bwilczynski/hlctl/internal/system"
)

func okInfoResp(list gen.SystemInfoList) *gen.ListSystemInfoResponse {
	b, _ := json.Marshal(list)
	return &gen.ListSystemInfoResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func TestInfoCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListSystemInfoWithResponseFunc: func(_ context.Context, _ *gen.ListSystemInfoParams, _ ...gen.RequestEditorFn) (*gen.ListSystemInfoResponse, error) {
			return okInfoResp(gen.SystemInfoList{
				Items: []gen.SystemInfo{
					{
						Device:        "nas-1",
						Model:         "DS920+",
						Firmware:      "7.2.1-69057",
						RamMb:         4096,
						UptimeSeconds: 3_931_200,
					},
				},
			}), nil
		},
	}

	cmd := newInfoCmd()
	cmdutil.SetClient[SystemClient](cmd, stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1", "DS920+", "7.2.1-69057", "4.0 GB"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
