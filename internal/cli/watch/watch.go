// Package watch provides a reusable --watch loop for list-style hlctl commands.
//
// TTY behavior (screen clear, cursor hide, header line) uses ANSI escape
// sequences and assumes a vt100-compatible terminal. Non-TTY output is
// escape-free so piped consumers stay grep-friendly.
package watch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const minInterval = 100 * time.Millisecond

const (
	ansiClearScreen = "\x1b[H\x1b[2J\x1b[3J" // cursor home + erase display + erase scrollback
	ansiHideCursor  = "\x1b[?25l"
	ansiShowCursor  = "\x1b[?25h"
)

// TickFunc is the per-tick body executed by the watch loop. It writes its
// rendered output to w and uses ctx for any cancellable work (e.g. HTTP calls).
//
// In JSON output mode (flags.GetOutputFormat() == output.FormatJSON), fn must
// write exactly one compact JSON document with no trailing newline; the loop
// appends the NDJSON newline separator. Do NOT call output.Print in JSON mode
// — it pretty-prints with indentation, which breaks NDJSON.
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
		interval, _ := cmd.Flags().GetDuration("watch-interval")
		if interval < minInterval {
			return fmt.Errorf("--watch-interval must be at least %s", minInterval)
		}
		return loop(cmd, interval, fn)
	}
}

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
				b, _ := json.Marshal(err.Error())
				fmt.Fprintf(w, `{"error":%s}`+"\n", b)
			} else {
				fmt.Fprintf(w, "error: %v\n", err)
			}
		} else if jsonMode {
			fmt.Fprintln(w)
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

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

