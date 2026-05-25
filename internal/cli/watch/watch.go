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
