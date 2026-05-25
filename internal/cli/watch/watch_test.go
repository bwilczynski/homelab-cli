package watch

import (
	"bytes"
	"context"
	"io"
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

func TestWrap_watchDisabled_callsOnce(t *testing.T) {
	calls := 0
	fn := func(_ context.Context, w io.Writer) error {
		calls++
		_, _ = w.Write([]byte("hello\n"))
		return nil
	}

	cmd := &cobra.Command{Use: "test"}
	RegisterFlags(cmd)
	cmd.RunE = Wrap(fn)

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
	if buf.String() != "hello\n" {
		t.Errorf("expected %q, got %q", "hello\n", buf.String())
	}
}
