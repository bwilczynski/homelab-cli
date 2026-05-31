package cmdutil_test

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
)

func TestActionCmd_success_printsMessage(t *testing.T) {
	cmd := cmdutil.ActionCmd[*fakeClient]("start <id>", "Start it", "started",
		func(c *fakeClient, ctx context.Context, id string) (int, []byte, error) {
			if c == nil || ctx == nil || id == "" {
				t.Fatal("exec called with unexpected zero args")
			}
			return http.StatusNoContent, nil, nil
		})
	cmdutil.SetClient[*fakeClient](cmd, &fakeClient{name: "ok"})

	cmd.SetArgs([]string{"abc-1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := "abc-1 started\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestActionCmd_nonNoContent_returnsParseError(t *testing.T) {
	cmd := cmdutil.ActionCmd[*fakeClient]("stop <id>", "Stop it", "stopped",
		func(*fakeClient, context.Context, string) (int, []byte, error) {
			return http.StatusInternalServerError, []byte(`{"title":"Boom"}`), nil
		})
	cmdutil.SetClient[*fakeClient](cmd, &fakeClient{})
	cmd.SetArgs([]string{"abc-1"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "Boom") {
		t.Errorf("expected ParseError with 'Boom', got %v", err)
	}
}

func TestActionCmd_execError_propagates(t *testing.T) {
	cmd := cmdutil.ActionCmd[*fakeClient]("restart <id>", "Restart it", "restarted",
		func(*fakeClient, context.Context, string) (int, []byte, error) {
			return 0, nil, errBoom
		})
	cmdutil.SetClient[*fakeClient](cmd, &fakeClient{})
	cmd.SetArgs([]string{"abc-1"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil || err.Error() != "boom" {
		t.Errorf("expected boom, got %v", err)
	}
}

func TestActionCmd_requiresExactlyOneArg(t *testing.T) {
	cmd := cmdutil.ActionCmd[*fakeClient]("start <id>", "Start it", "started",
		func(*fakeClient, context.Context, string) (int, []byte, error) { return 204, nil, nil })
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err == nil {
		t.Error("expected error when no arg given")
	}
}
