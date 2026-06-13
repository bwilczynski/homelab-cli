package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
)

func TestNewRootCmd_version(t *testing.T) {
	var buf bytes.Buffer
	f := cmdutil.TestFactory(t)
	f.Version = "v20260508.774a"
	f.IOStreams.Out = &buf

	root := NewRootCmd(f)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "v20260508.774a") {
		t.Errorf("expected version in output, got: %s", buf.String())
	}
}
