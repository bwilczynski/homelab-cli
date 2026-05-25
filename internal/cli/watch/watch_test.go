package watch

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRegisterFlags_addsBothFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	RegisterFlags(cmd)

	if cmd.Flags().Lookup("watch") == nil {
		t.Error("expected --watch flag to be registered")
	}
	if cmd.Flags().ShorthandLookup("w") == nil {
		t.Error("expected -w shorthand to be registered")
	}
	if cmd.Flags().Lookup("watch-interval") == nil {
		t.Error("expected --watch-interval flag to be registered")
	}
}
