// Package watch provides a reusable --watch loop for list-style hlctl commands.
//
// TTY behavior (screen clear, cursor hide, header line) uses ANSI escape
// sequences and assumes a vt100-compatible terminal. Non-TTY output is
// escape-free so piped consumers stay grep-friendly.
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
