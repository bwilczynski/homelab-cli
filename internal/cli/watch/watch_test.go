package watch

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync/atomic"
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

func TestWrap_nonTTY_appendsSnapshots(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fn := func(_ context.Context, w io.Writer) error {
		n := calls.Add(1)
		_, _ = w.Write([]byte("tick-" + string(rune('0'+n)) + "\n"))
		if n == 3 {
			cancel()
		}
		return nil
	}

	cmd := &cobra.Command{Use: "hlctl test"}
	RegisterFlags(cmd)
	cmd.RunE = Wrap(fn)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--watch", "--watch-interval=100ms"})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("expected 3 calls, got %d", got)
	}
	out := buf.String()
	for _, want := range []string{"tick-1", "tick-2", "tick-3", "hlctl test"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	headerCount := strings.Count(out, "--- ")
	if headerCount != 3 {
		t.Errorf("expected 3 header separators, got %d", headerCount)
	}
}
