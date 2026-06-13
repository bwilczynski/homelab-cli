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

func newVolumesCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "volumes", Short: "Storage volumes"}
	cmd.AddCommand(newListVolumesCmd(f, nil), newGetVolumeCmd(f, nil))
	return cmd
}

func newBackupsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "backups", Short: "Backup tasks and history"}
	cmd.AddCommand(newListBackupsCmd(f, nil), newGetBackupCmd(f, nil))
	return cmd
}
