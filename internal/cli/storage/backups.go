package storage

import (
	storageapi "github.com/bwilczynski/hlctl/internal/api/storage"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

var (
	backupsListView = cmdutil.View{Templates: storageTemplates, Name: "backups_list.tmpl"}
	backupsGetView  = cmdutil.View{Templates: storageTemplates, Name: "backups_get.tmpl"}
)

func newBackupsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "backups", Short: "Backup tasks and history"}
	cmdutil.InjectClient(cmd, func() (StorageClient, error) {
		httpClient, apiURL, err := f.HTTPClient()
		if err != nil {
			return nil, err
		}
		return NewStorageClient(httpClient, apiURL)
	})
	cmd.AddCommand(newListBackupsCmd(f), newGetBackupCmd(f))
	return cmd
}

func newListBackupsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List backups"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		params := &storageapi.ListBackupsParams{}
		if *device != "" {
			params.Device = device
		}
		resp, err := cmdutil.Client[StorageClient](cmd).ListBackupsWithResponse(cmd.Context(), params)
		if err != nil {
			return err
		}
		return backupsListView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newGetBackupCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get <backup-id>", Short: "Show backup details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[StorageClient](cmd).GetBackupWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return backupsGetView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}
