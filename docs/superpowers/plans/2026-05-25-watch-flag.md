# `--watch` Flag Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--watch`/`-w` (with `--watch-interval`) to four `hlctl` commands via a reusable helper in a new `internal/cli/watch` package.

**Architecture:** Build the helper TDD-first against the non-TTY path (where `*bytes.Buffer` makes isTerminal return false naturally). Layer the TTY-only behavior (clear screen, hidden cursor, header) on top — verified manually. Each integration site rewrites its `RunE` as a closure passed to `watch.Wrap`.

**Tech Stack:** Go, Cobra, `golang.org/x/term` (new dep), `signal.NotifyContext`, `time.Ticker`, ANSI escape sequences.

---

## File Map

| Action | Path |
|--------|------|
| Modify | `go.mod`, `go.sum` — add `golang.org/x/term` |
| Create | `internal/cli/watch/watch.go` |
| Create | `internal/cli/watch/watch_test.go` |
| Modify | `internal/cli/system/system.go` — `newUtilizationCmd` |
| Modify | `internal/cli/docker/docker.go` — `newListCmd` |
| Modify | `internal/cli/network/network.go` — `newListClientsCmd`, `newTopologyCmd` |

---

### Task 1: Add `golang.org/x/term` dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the dependency**

Run from the repo root:

```bash
go get golang.org/x/term@latest
go mod tidy
```

Expected: `go.mod` gains `golang.org/x/term` under `require`; `go.sum` updates accordingly.

- [ ] **Step 2: Verify the build still passes**

```bash
make build
```

Expected: produces `bin/hlctl` with no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add golang.org/x/term dependency"
```

---

### Task 2: Create `watch` package with `RegisterFlags`

**Files:**
- Create: `internal/cli/watch/watch.go`
- Create: `internal/cli/watch/watch_test.go`

- [ ] **Step 1: Write a failing test for `RegisterFlags`**

Create `internal/cli/watch/watch_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/cli/watch/...
```

Expected: FAIL — `watch` package does not exist (or `RegisterFlags` undefined).

- [ ] **Step 3: Create the package with `RegisterFlags`**

Create `internal/cli/watch/watch.go`:

```go
// Package watch provides a reusable --watch loop for list-style hlctl commands.
//
// TTY behavior (screen clear, cursor hide, header line) uses ANSI escape
// sequences and assumes a vt100-compatible terminal. Non-TTY output is
// escape-free so piped consumers stay grep-friendly.
package watch

import (
	"time"

	"github.com/spf13/cobra"
)

// RegisterFlags adds --watch/-w and --watch-interval to cmd.
func RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP("watch", "w", false, "Re-run command on an interval until interrupted")
	cmd.Flags().Duration("watch-interval", 2*time.Second, "Interval between watch ticks (minimum 100ms)")
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./internal/cli/watch/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/watch/watch.go internal/cli/watch/watch_test.go
git commit -m "feat(watch): add package skeleton with flag registration"
```

---

### Task 3: Implement once-through path (watch disabled)

**Files:**
- Modify: `internal/cli/watch/watch.go`
- Modify: `internal/cli/watch/watch_test.go`

- [ ] **Step 1: Write a failing test for the once-through path**

Append to `internal/cli/watch/watch_test.go`:

```go
import (
	"bytes"
	"context"
	"io"
)

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
```

(Update the existing imports block at the top of the file to merge in `bytes`, `context`, `io` rather than duplicating.)

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/cli/watch/... -run TestWrap_watchDisabled_callsOnce
```

Expected: FAIL — `Wrap` undefined.

- [ ] **Step 3: Implement `Wrap` with the once-through path**

Update `internal/cli/watch/watch.go` to add the `TickFunc` type and `Wrap`:

```go
package watch

import (
	"context"
	"io"
	"time"

	"github.com/spf13/cobra"
)

// TickFunc is the per-tick body executed by the watch loop. It writes its
// rendered output to w and uses ctx for any cancellable work (e.g. HTTP calls).
type TickFunc func(ctx context.Context, w io.Writer) error

// RegisterFlags adds --watch/-w and --watch-interval to cmd.
func RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP("watch", "w", false, "Re-run command on an interval until interrupted")
	cmd.Flags().Duration("watch-interval", 2*time.Second, "Interval between watch ticks (minimum 100ms)")
}

// Wrap returns a cobra RunE. When --watch is false, it calls fn once with
// cmd.OutOrStdout(). When --watch is true, it runs fn on an interval until
// the context is cancelled by SIGINT/SIGTERM.
func Wrap(fn TickFunc) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		watching, _ := cmd.Flags().GetBool("watch")
		if !watching {
			return fn(cmd.Context(), cmd.OutOrStdout())
		}
		// Loop path implemented in later tasks.
		return nil
	}
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./internal/cli/watch/...
```

Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add internal/cli/watch/watch.go internal/cli/watch/watch_test.go
git commit -m "feat(watch): implement once-through path"
```

---

### Task 4: Implement non-TTY append loop (table mode)

**Files:**
- Modify: `internal/cli/watch/watch.go`
- Modify: `internal/cli/watch/watch_test.go`

- [ ] **Step 1: Write a failing test**

Append to `internal/cli/watch/watch_test.go`:

```go
import (
	"strings"
	"sync/atomic"
)

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
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/cli/watch/... -run TestWrap_nonTTY_appendsSnapshots
```

Expected: FAIL (loop returns immediately without producing output).

- [ ] **Step 3: Implement the loop with non-TTY append**

Replace the `Wrap` function body (the `// Loop path implemented in later tasks.` portion) with the loop. Final `watch.go`:

```go
package watch

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const minInterval = 100 * time.Millisecond

type TickFunc func(ctx context.Context, w io.Writer) error

func RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP("watch", "w", false, "Re-run command on an interval until interrupted")
	cmd.Flags().Duration("watch-interval", 2*time.Second, "Interval between watch ticks (minimum 100ms)")
}

func Wrap(fn TickFunc) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		watching, _ := cmd.Flags().GetBool("watch")
		if !watching {
			return fn(cmd.Context(), cmd.OutOrStdout())
		}
		interval, _ := cmd.Flags().GetDuration("watch-interval")
		return loop(cmd, interval, fn)
	}
}

func loop(cmd *cobra.Command, interval time.Duration, fn TickFunc) error {
	w := cmd.OutOrStdout()
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tick := func() {
		writeHeader(w, cmd, interval)
		if err := fn(ctx, w); err != nil {
			fmt.Fprintf(w, "error: %v\n", err)
		}
		fmt.Fprintln(w)
	}

	tick()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			tick()
		}
	}
}

func writeHeader(w io.Writer, cmd *cobra.Command, interval time.Duration) {
	if isTerminal(w) {
		// TTY header implemented in Task 8.
		return
	}
	fmt.Fprintf(w, "--- %s  %s ---\n", time.Now().Format(time.RFC3339), cmd.CommandPath())
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// unused-but-kept imports placeholders — remove once TTY/NDJSON paths land.
var _ = flags.GetOutputFormat
var _ = output.FormatJSON
```

Note: the `flags` and `output` imports are placed now so Tasks 5 and 8 only add code, not imports. Drop the `var _` lines as you actually use them in those tasks (both are consumed by Task 5).

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./internal/cli/watch/... -run TestWrap_nonTTY_appendsSnapshots
```

Expected: PASS.

- [ ] **Step 5: Run the whole package to make sure earlier tests still pass**

```bash
go test ./internal/cli/watch/...
```

Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/cli/watch/watch.go internal/cli/watch/watch_test.go
git commit -m "feat(watch): implement non-TTY append loop"
```

---

### Task 5: Implement NDJSON path for JSON output

**Files:**
- Modify: `internal/cli/watch/watch.go`
- Modify: `internal/cli/watch/watch_test.go`

- [ ] **Step 1: Write a failing test**

Append to `internal/cli/watch/watch_test.go`:

```go
import "github.com/bwilczynski/hlctl/internal/cli/flags"

func TestWrap_jsonMode_emitsNDJSON(t *testing.T) {
	prev := flags.OutputFormat
	flags.OutputFormat = "json"
	t.Cleanup(func() { flags.OutputFormat = prev })

	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fn := func(_ context.Context, w io.Writer) error {
		n := calls.Add(1)
		_, _ = fmt.Fprintf(w, `{"tick":%d}`+"\n", n)
		if n == 3 {
			cancel()
		}
		return nil
	}

	cmd := &cobra.Command{Use: "test"}
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
```

Add `encoding/json` and `fmt` to the test file's imports.

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/cli/watch/... -run TestWrap_jsonMode_emitsNDJSON
```

Expected: FAIL — current `loop` prefixes `--- ...` headers regardless of format.

- [ ] **Step 3: Branch the loop on output format**

Edit `internal/cli/watch/watch.go`. Replace the `tick` closure body so JSON mode skips the header and the trailing blank line. Remove the `var _ = flags.GetOutputFormat` placeholder:

```go
	jsonMode := flags.GetOutputFormat() == output.FormatJSON
	tick := func() {
		if !jsonMode {
			writeHeader(w, cmd, interval)
		}
		if err := fn(ctx, w); err != nil {
			if jsonMode {
				fmt.Fprintf(w, `{"error":%q}`+"\n", err.Error())
			} else {
				fmt.Fprintf(w, "error: %v\n", err)
			}
		}
		if !jsonMode {
			fmt.Fprintln(w)
		}
	}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./internal/cli/watch/... -run TestWrap_jsonMode_emitsNDJSON
```

Expected: PASS.

- [ ] **Step 5: Run the whole package**

```bash
go test ./internal/cli/watch/...
```

Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/cli/watch/watch.go internal/cli/watch/watch_test.go
git commit -m "feat(watch): emit NDJSON in JSON output mode"
```

---

### Task 6: Tolerate per-tick errors

**Files:**
- Modify: `internal/cli/watch/watch_test.go`

- [ ] **Step 1: Write a failing test (it may already pass — that's fine; the test pins the behavior)**

Append to `internal/cli/watch/watch_test.go`:

```go
import "errors"

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

	cmd := &cobra.Command{Use: "test"}
	RegisterFlags(cmd)
	cmd.RunE = Wrap(fn)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--watch", "--watch-interval=100ms"})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error from loop: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"ok-1", "error: boom", "ok-3"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run the test**

```bash
go test ./internal/cli/watch/... -run TestWrap_nonTTY_tickError_continues
```

Expected: PASS (Task 4's implementation already prints `error: <msg>` on tick error and keeps looping). If it does not pass, the regression is in `loop` — fix it there.

- [ ] **Step 3: Commit**

```bash
git add internal/cli/watch/watch_test.go
git commit -m "test(watch): pin tick-error continue behavior"
```

---

### Task 7: Enforce minimum watch interval

**Files:**
- Modify: `internal/cli/watch/watch.go`
- Modify: `internal/cli/watch/watch_test.go`

- [ ] **Step 1: Write a failing test**

Append to `internal/cli/watch/watch_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/cli/watch/... -run TestWrap_intervalBelowMinimum_returnsError
```

Expected: FAIL (the loop runs and hangs / passes without error).

- [ ] **Step 3: Add the guard**

In `internal/cli/watch/watch.go`, in `Wrap`, insert the check before calling `loop`:

```go
		interval, _ := cmd.Flags().GetDuration("watch-interval")
		if interval < minInterval {
			return fmt.Errorf("--watch-interval must be at least %s", minInterval)
		}
		return loop(cmd, interval, fn)
```

Add `"fmt"` to imports if not already present (it is — used by `loop`).

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./internal/cli/watch/... -run TestWrap_intervalBelowMinimum_returnsError
```

Expected: PASS.

- [ ] **Step 5: Run the whole package**

```bash
go test ./internal/cli/watch/...
```

Expected: PASS (6 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/cli/watch/watch.go internal/cli/watch/watch_test.go
git commit -m "feat(watch): reject intervals below 100ms"
```

---

### Task 8: Add TTY behavior (clear screen, header, cursor)

**Files:**
- Modify: `internal/cli/watch/watch.go`

This task has no automated test (TTY-only paths). Verify manually after implementing.

- [ ] **Step 1: Implement the TTY branches**

In `internal/cli/watch/watch.go`:

1. Add ANSI escape constants near the top of the file:

```go
const (
	ansiClearScreen = "\x1b[H\x1b[2J\x1b[3J" // cursor home + erase display + erase scrollback
	ansiHideCursor  = "\x1b[?25l"
	ansiShowCursor  = "\x1b[?25h"
)
```

2. Replace `writeHeader` with TTY + non-TTY paths:

```go
func writeHeader(w io.Writer, cmd *cobra.Command, interval time.Duration) {
	if !isTerminal(w) {
		fmt.Fprintf(w, "--- %s  %s ---\n", time.Now().Format(time.RFC3339), cmd.CommandPath())
		return
	}
	left := fmt.Sprintf("Every %s: %s", interval, cmd.CommandPath())
	right := time.Now().Format("15:04:05")
	cols, _, err := term.GetSize(int(w.(*os.File).Fd()))
	if err != nil || cols < len(left)+len(right)+1 {
		fmt.Fprintf(w, "%s    %s\n\n", left, right)
		return
	}
	pad := cols - len(left) - len(right)
	fmt.Fprintf(w, "%s%s%s\n\n", left, strings.Repeat(" ", pad), right)
}
```

Add `"strings"` to imports.

3. In `loop`, wrap the loop with cursor hide/show and screen clear before each TTY tick:

```go
func loop(cmd *cobra.Command, interval time.Duration, fn TickFunc) error {
	w := cmd.OutOrStdout()
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tty := isTerminal(w)
	jsonMode := flags.GetOutputFormat() == output.FormatJSON

	if tty && !jsonMode {
		fmt.Fprint(w, ansiHideCursor)
		defer fmt.Fprint(w, ansiShowCursor)
	}

	tick := func() {
		if tty && !jsonMode {
			fmt.Fprint(w, ansiClearScreen)
		}
		if !jsonMode {
			writeHeader(w, cmd, interval)
		}
		if err := fn(ctx, w); err != nil {
			if jsonMode {
				fmt.Fprintf(w, `{"error":%q}`+"\n", err.Error())
			} else {
				fmt.Fprintf(w, "error: %v\n", err)
			}
		}
		if !jsonMode && !tty {
			fmt.Fprintln(w)
		}
	}

	tick()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			tick()
		}
	}
}
```

- [ ] **Step 2: Run all watch tests to confirm no regression**

```bash
go test ./internal/cli/watch/...
```

Expected: PASS (6 tests).

- [ ] **Step 3: Run `go vet` and the build**

```bash
make lint
make build
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/watch/watch.go
git commit -m "feat(watch): add TTY clear-screen, header, and cursor handling"
```

- [ ] **Step 5: Manual verification (do this once the integrations land in Task 12)**

Note this verification step for later — it requires an integrated command. See Task 12.

---

### Task 9: Integrate `--watch` into `system utilization`

**Files:**
- Modify: `internal/cli/system/system.go` — `newUtilizationCmd`

- [ ] **Step 1: Rewrite `newUtilizationCmd` to use `watch.Wrap`**

In `internal/cli/system/system.go`, replace the existing `newUtilizationCmd` (lines around 160–224) with:

```go
func newUtilizationCmd(client SystemClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "utilization",
		Short: "Show live resource utilization",
	}
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
		c := client
		if c == nil {
			var err error
			c, err = buildClient()
			if err != nil {
				return err
			}
		}

		params := &gen.ListSystemUtilizationParams{}
		if device != "" {
			params.Device = &device
		}

		resp, err := c.ListSystemUtilization(ctx, params)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return apiclient.ParseError(resp)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var list gen.SystemUtilizationList
		if err := json.Unmarshal(body, &list); err != nil {
			return err
		}

		if flags.GetOutputFormat() == output.FormatJSON {
			fmt.Fprint(w, string(body))
			return nil
		}

		headers := []string{"DEVICE", "CPU", "MEMORY", "SWAP"}
		var rows [][]string
		for _, u := range list.Items {
			swapPct := 0
			if u.Memory.SwapTotalBytes > 0 {
				swapPct = int(u.Memory.SwapUsedBytes * 100 / u.Memory.SwapTotalBytes)
			}
			rows = append(rows, []string{
				u.Device,
				fmt.Sprintf("%d%%", u.Cpu.TotalPercent),
				fmt.Sprintf("%d%%", u.Memory.UsedPercent),
				fmt.Sprintf("%d%%", swapPct),
			})
		}
		return output.Print(w, flags.GetOutputFormat(), list, headers, rows)
	})

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	watch.RegisterFlags(cmd)
	return cmd
}
```

Two changes from the original:
1. Body wrapped in `watch.Wrap(func(ctx, w) error { ... })`.
2. `context.Background()` → `ctx`; `cmd.OutOrStdout()` → `w`.

- [ ] **Step 2: Add the `watch` import**

In `internal/cli/system/system.go`, add to the import block:

```go
	"github.com/bwilczynski/hlctl/internal/cli/watch"
```

- [ ] **Step 3: Run the system tests — existing once-through path must still pass**

```bash
go test ./internal/cli/system/...
```

Expected: PASS (all existing tests; they invoke `cmd.Execute()` without `--watch`, hitting the once-through branch).

- [ ] **Step 4: Build and smoke-check the help**

```bash
make build
./bin/hlctl system utilization --help
```

Expected: help text includes `-w, --watch` and `--watch-interval`.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/system/system.go
git commit -m "feat(system): add --watch to utilization"
```

---

### Task 10: Integrate `--watch` into `docker containers list`

**Files:**
- Modify: `internal/cli/docker/docker.go` — `newListCmd`

- [ ] **Step 1: Rewrite `newListCmd`**

In `internal/cli/docker/docker.go`, replace `newListCmd` (lines around 51–112) with:

```go
func newListCmd(client DockerClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List containers",
	}
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
		c := client
		if c == nil {
			var err error
			c, err = buildClient()
			if err != nil {
				return err
			}
		}

		params := &gen.ListContainersParams{}
		if device != "" {
			params.Device = &device
		}

		resp, err := c.ListContainers(ctx, params)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return apiclient.ParseError(resp)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var list gen.ContainerList
		if err := json.Unmarshal(body, &list); err != nil {
			return err
		}

		if flags.GetOutputFormat() == output.FormatJSON {
			fmt.Fprint(w, string(body))
			return nil
		}

		headers := []string{"ID", "IMAGE", "STATUS", "CPU", "MEMORY"}
		var rows [][]string
		for _, c := range list.Items {
			rows = append(rows, []string{
				c.Id,
				c.Image,
				string(c.Status),
				fmt.Sprintf("%.1f%%", c.Resources.CpuPercent),
				output.FormatBytes(c.Resources.MemoryBytes),
			})
		}
		return output.Print(w, flags.GetOutputFormat(), list, headers, rows)
	})

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	watch.RegisterFlags(cmd)
	return cmd
}
```

- [ ] **Step 2: Add the `watch` import**

Add to the import block:

```go
	"github.com/bwilczynski/hlctl/internal/cli/watch"
```

- [ ] **Step 3: Run docker tests**

```bash
go test ./internal/cli/docker/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/docker/docker.go
git commit -m "feat(docker): add --watch to containers list"
```

---

### Task 11: Integrate `--watch` into `network clients list`

**Files:**
- Modify: `internal/cli/network/network.go` — `newListClientsCmd`

- [ ] **Step 1: Rewrite `newListClientsCmd`**

In `internal/cli/network/network.go`, replace `newListClientsCmd` (lines around 288–350) with:

```go
func newListClientsCmd(client NetworkClient) *cobra.Command {
	var statusFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List network clients",
	}
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
		c := client
		if c == nil {
			var err error
			c, err = buildClient()
			if err != nil {
				return err
			}
		}

		params := &gen.ListNetworkClientsParams{}
		if statusFilter != "" {
			s := gen.NetworkClientStatus(statusFilter)
			params.Status = &s
		}

		resp, err := c.ListNetworkClients(ctx, params)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return apiclient.ParseError(resp)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var list gen.NetworkClientList
		if err := json.Unmarshal(body, &list); err != nil {
			return err
		}

		if flags.GetOutputFormat() == output.FormatJSON {
			fmt.Fprint(w, string(body))
			return nil
		}

		headers := []string{"ID", "NAME", "MAC", "IP", "STATUS", "CONNECTION"}
		var rows [][]string
		for _, cl := range list.Items {
			ip := ""
			if cl.Ip != nil {
				ip = *cl.Ip
			}
			rows = append(rows, []string{
				cl.Id, cl.Name, cl.Mac, ip,
				string(cl.Status),
				string(cl.ConnectionType),
			})
		}
		return output.Print(w, flags.GetOutputFormat(), list, headers, rows)
	})
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (online|offline)")
	watch.RegisterFlags(cmd)
	return cmd
}
```

- [ ] **Step 2: Add the `watch` import**

Add to the import block of `internal/cli/network/network.go`:

```go
	"github.com/bwilczynski/hlctl/internal/cli/watch"
```

- [ ] **Step 3: Run network tests**

```bash
go test ./internal/cli/network/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/network/network.go
git commit -m "feat(network): add --watch to clients list"
```

---

### Task 12: Integrate `--watch` into `network topology`

**Files:**
- Modify: `internal/cli/network/network.go` — `newTopologyCmd`

- [ ] **Step 1: Rewrite `newTopologyCmd`**

In `internal/cli/network/network.go`, replace `newTopologyCmd` (lines around 466–520) with:

```go
func newTopologyCmd(client NetworkClient) *cobra.Command {
	var includeClients bool
	var includeWireless bool

	cmd := &cobra.Command{
		Use:   "topology",
		Short: "Show network topology",
	}
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
		c := client
		if c == nil {
			var err error
			c, err = buildClient()
			if err != nil {
				return err
			}
		}

		params := &gen.GetNetworkTopologyParams{}
		if includeClients || includeWireless {
			t := true
			params.IncludeClients = &t
		}

		resp, err := c.GetNetworkTopology(ctx, params)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return apiclient.ParseError(resp)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if flags.GetOutputFormat() == output.FormatJSON {
			fmt.Fprint(w, string(body))
			return nil
		}

		var topo gen.NetworkTopology
		if err := json.Unmarshal(body, &topo); err != nil {
			return err
		}

		return printTopologyTree(w, topo, includeWireless)
	})

	cmd.Flags().BoolVar(&includeClients, "include-clients", false, "Include wired clients in the topology")
	cmd.Flags().BoolVar(&includeWireless, "include-wireless", false, "Also include wireless clients (implies --include-clients)")
	watch.RegisterFlags(cmd)
	return cmd
}
```

- [ ] **Step 2: Run network tests**

```bash
go test ./internal/cli/network/...
```

Expected: PASS.

- [ ] **Step 3: Run the full test suite and the build**

```bash
go test ./...
make build
make lint
```

Expected: PASS, builds clean, `go vet` reports nothing.

- [ ] **Step 4: Manual TTY verification**

Against a configured `hlctl` (login already done) or against a stub server, run each command in an interactive terminal and confirm behavior:

```bash
./bin/hlctl system utilization --watch
./bin/hlctl docker containers list -w --watch-interval=1s
./bin/hlctl network clients list -w
./bin/hlctl network topology -w --include-clients
```

For each:
1. Screen clears between ticks.
2. Header line shows `Every Ns: hlctl ...` with the time on the right.
3. Cursor is hidden during the loop and restored after Ctrl-C.
4. Ctrl-C exits with status 0 (`echo $?`).

Then pipe one through `tee` to confirm non-TTY behavior (no escapes, `--- ts hlctl ... ---` separators):

```bash
./bin/hlctl system utilization -w --watch-interval=1s | head -40
```

Then JSON NDJSON:

```bash
./bin/hlctl system utilization -wo json --watch-interval=1s | head -3 | jq .
```

Expected: each line is valid JSON.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/network/network.go
git commit -m "feat(network): add --watch to topology"
```

---

## Verification Checklist

After all tasks complete, confirm:

- [ ] `go test ./...` passes
- [ ] `make build` succeeds
- [ ] `make lint` reports nothing
- [ ] `hlctl system utilization --help` shows `--watch`, `-w`, `--watch-interval`
- [ ] Same for `docker containers list`, `network clients list`, `network topology`
- [ ] TTY watch redraws cleanly; Ctrl-C restores cursor
- [ ] Non-TTY watch appends with `--- ts cmd ---` separators
- [ ] `-o json --watch` emits one valid JSON document per line
- [ ] `--watch-interval=10ms` returns an error mentioning the minimum
