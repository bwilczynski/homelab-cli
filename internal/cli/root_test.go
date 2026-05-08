package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestExecute_version(t *testing.T) {
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--version"})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.Version = ""
	})

	_ = Execute("v20260508.774a")

	if !strings.Contains(buf.String(), "v20260508.774a") {
		t.Errorf("expected version in output, got: %s", buf.String())
	}
}
