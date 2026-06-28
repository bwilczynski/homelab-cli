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

func TestNewRootCmd_versionSubcommand(t *testing.T) {
	var buf bytes.Buffer
	f := cmdutil.TestFactory(t)
	f.Version = "v1.0.0"
	f.SpecVersion = "1.1.0"
	f.IOStreams.Out = &buf

	root := NewRootCmd(f)
	root.SetArgs([]string{"version", "--client"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	for _, want := range []string{"v1.0.0", "1.1.0"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("expected %q in version output, got: %s", want, buf.String())
		}
	}
}
