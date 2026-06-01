package storage

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	gen "github.com/bwilczynski/hlctl/internal/storage"
	"github.com/spf13/cobra"
)

var (
	backupsListView = cmdutil.View{Templates: storageTemplates, Name: "backups_list.tmpl"}
	backupsGetView  = cmdutil.View{Templates: storageTemplates, Name: "backups_get.tmpl"}
)

func newBackupsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "backups", Short: "Backup tasks and history"}
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newListBackupsCmd(), newGetBackupCmd())
	return cmd
}

func newListBackupsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List backups"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		params := &gen.ListBackupsParams{}
		if *device != "" {
			params.Device = device
		}
		resp, err := cmdutil.Client[StorageClient](cmd).ListBackupsWithResponse(cmd.Context(), params)
		if err != nil {
			return err
		}
		return backupsListView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newGetBackupCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "get <backup-id>", Short: "Show backup details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[StorageClient](cmd).GetBackupWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return backupsGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}
