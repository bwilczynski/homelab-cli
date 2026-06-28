package version

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	metaapi "github.com/bwilczynski/hlctl/internal/api/meta"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

func testHTTPClient(reg *httpmock.Registry) func() (*http.Client, string, error) {
	return func() (*http.Client, string, error) {
		return &http.Client{Transport: reg}, "http://localhost", nil
	}
}

// Layer 1: runF hook fires

func TestNewVersionCmd_runFCalled(t *testing.T) {
	called := false
	cmd := NewCmd(cmdutil.TestFactory(t), func(o *versionOptions) error {
		called = true
		return nil
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected runF to be called")
	}
}

// Layer 2: business logic

func TestGetVersionRun_tableOutput_serverReachable(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/meta/version"), httpmock.JSONResponse(metaapi.Version{
		ApiVersion:    "1.1.0",
		ServerVersion: "v2.0.0",
	}))

	var out, errOut bytes.Buffer
	opts := &versionOptions{
		ClientVersion: "v1.0.0",
		ClientSpec:    "1.0.0",
		HTTPClient:    testHTTPClient(reg),
		IO:            &cmdutil.IOStreams{Out: &out, ErrOut: &errOut},
		Output:        func() output.Format { return output.FormatTable },
	}
	if err := getVersionRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"v1.0.0", "1.0.0", "v2.0.0", "1.1.0"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetVersionRun_tableOutput_clientOnly(t *testing.T) {
	var httpCalled bool
	var out, errOut bytes.Buffer
	opts := &versionOptions{
		ClientVersion: "v1.0.0",
		ClientSpec:    "1.0.0",
		ClientOnly:    true,
		HTTPClient: func() (*http.Client, string, error) {
			httpCalled = true
			return nil, "", nil
		},
		IO:     &cmdutil.IOStreams{Out: &out, ErrOut: &errOut},
		Output: func() output.Format { return output.FormatTable },
	}
	if err := getVersionRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpCalled {
		t.Error("expected HTTPClient not to be called with --client flag")
	}
	for _, want := range []string{"v1.0.0", "1.0.0"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	for _, absent := range []string{"Server version", "Server spec"} {
		if strings.Contains(out.String(), absent) {
			t.Errorf("expected %q absent from output, got:\n%s", absent, out.String())
		}
	}
}

func TestGetVersionRun_tableOutput_serverUnavailable(t *testing.T) {
	var out, errOut bytes.Buffer
	opts := &versionOptions{
		ClientVersion: "v1.0.0",
		ClientSpec:    "1.0.0",
		HTTPClient: func() (*http.Client, string, error) {
			return nil, "", fmt.Errorf("connection refused")
		},
		IO:     &cmdutil.IOStreams{Out: &out, ErrOut: &errOut},
		Output: func() output.Format { return output.FormatTable },
	}
	if err := getVersionRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("expected graceful degradation, got error: %v", err)
	}
	if !strings.Contains(out.String(), "(unavailable)") {
		t.Errorf("expected '(unavailable)' in output, got:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "warning") {
		t.Errorf("expected warning on stderr, got:\n%s", errOut.String())
	}
}

func TestGetVersionRun_jsonOutput_serverReachable(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/meta/version"), httpmock.JSONResponse(metaapi.Version{
		ApiVersion:    "1.1.0",
		ServerVersion: "v2.0.0",
	}))

	var out, errOut bytes.Buffer
	opts := &versionOptions{
		ClientVersion: "v1.0.0",
		ClientSpec:    "1.0.0",
		HTTPClient:    testHTTPClient(reg),
		IO:            &cmdutil.IOStreams{Out: &out, ErrOut: &errOut},
		Output:        func() output.Format { return output.FormatJSON },
	}
	if err := getVersionRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got versionOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out.String())
	}
	if got.ClientVersion != "v1.0.0" {
		t.Errorf("clientVersion: got %q, want %q", got.ClientVersion, "v1.0.0")
	}
	if got.ServerVersion == nil || *got.ServerVersion != "v2.0.0" {
		t.Errorf("serverVersion: got %v, want %q", got.ServerVersion, "v2.0.0")
	}
	reg.Verify(t)
}
