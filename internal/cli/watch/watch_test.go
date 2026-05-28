package watch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/flags"
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

	root := &cobra.Command{Use: "hlctl"}
	cmd := &cobra.Command{Use: "test"}
	root.AddCommand(cmd)
	RegisterFlags(cmd)
	cmd.RunE = Wrap(fn)
	root.SetContext(ctx)
	root.SetArgs([]string{"test", "--watch", "--watch-interval=100ms"})

	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)

	if err := root.Execute(); err != nil {
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

func TestWrap_jsonMode_emitsNDJSON(t *testing.T) {
	prev := flags.OutputFormat
	flags.OutputFormat = "json"
	t.Cleanup(func() { flags.OutputFormat = prev })

	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fn := func(_ context.Context, w io.Writer) error {
		n := calls.Add(1)
		_, _ = fmt.Fprintf(w, `{"tick":%d}`, n)
		if n == 3 {
			cancel()
		}
		return nil
	}

	root := &cobra.Command{Use: "hlctl"}
	cmd := &cobra.Command{Use: "test"}
	root.AddCommand(cmd)
	RegisterFlags(cmd)
	cmd.RunE = Wrap(fn)
	root.SetContext(ctx)
	root.SetArgs([]string{"test", "--watch", "--watch-interval=100ms"})

	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("expected 3 calls, got %d", got)
	}
	if strings.Contains(buf.String(), "---") {
		t.Errorf("JSON mode must not emit text header; got:\n%s", buf.String())
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), buf.String())
	}
	for i, line := range lines {
		var got map[string]any
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Errorf("line %d not valid JSON: %v (%q)", i, err, line)
		}
	}
}

func TestWrap_jsonMode_errorIsValidJSON(t *testing.T) {
	prev := flags.OutputFormat
	flags.OutputFormat = "json"
	t.Cleanup(func() { flags.OutputFormat = prev })

	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Error containing a control byte and an invalid UTF-8 byte. %q would
	// emit \x01 / \xff which are NOT valid JSON escapes. json.Marshal must
	// be used instead, which encodes invalid UTF-8 as U+FFFD.
	fn := func(_ context.Context, _ io.Writer) error {
		calls.Add(1)
		cancel()
		return errors.New("boom \x01 \xff end")
	}

	root := &cobra.Command{Use: "hlctl"}
	cmd := &cobra.Command{Use: "test"}
	root.AddCommand(cmd)
	RegisterFlags(cmd)
	cmd.RunE = Wrap(fn)
	root.SetContext(ctx)
	root.SetArgs([]string{"test", "--watch", "--watch-interval=100ms"})

	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	line := strings.TrimRight(buf.String(), "\n")
	if line == "" {
		t.Fatal("expected one JSON error line, got empty output")
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("error line is not valid JSON: %v (%q)", err, line)
	}
	if _, ok := got["error"]; !ok {
		t.Errorf("expected error field, got: %v", got)
	}
}

func TestWrap_intervalBelowMinimum_returnsError(t *testing.T) {
	fn := func(_ context.Context, _ io.Writer) error { return nil }
	cmd := &cobra.Command{Use: "test"}
	RegisterFlags(cmd)
	cmd.RunE = Wrap(fn)
	cmd.SetArgs([]string{"--watch", "--watch-interval=10ms"})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "100ms") {
		t.Errorf("expected error to mention 100ms minimum, got: %v", err)
	}
}

func TestWrap_nonTTY_tickError_continues(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fn := func(_ context.Context, w io.Writer) error {
		n := calls.Add(1)
		switch n {
		case 1:
			_, _ = w.Write([]byte("ok-1\n"))
		case 2:
			return errors.New("boom")
		case 3:
			_, _ = w.Write([]byte("ok-3\n"))
			cancel()
		}
		return nil
	}

	root := &cobra.Command{Use: "hlctl"}
	cmd := &cobra.Command{Use: "test"}
	root.AddCommand(cmd)
	RegisterFlags(cmd)
	cmd.RunE = Wrap(fn)
	root.SetContext(ctx)
	root.SetArgs([]string{"test", "--watch", "--watch-interval=100ms"})

	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error from loop: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"ok-1", "error: boom", "ok-3"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
