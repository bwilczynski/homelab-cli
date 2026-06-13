package storage

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "storage", Short: "NAS storage resources"}
	cmd.AddCommand(newVolumesCmd(f), newBackupsCmd(f))
	return cmd
}
