package storage

import (
	storageapi "github.com/bwilczynski/hlctl/internal/api/storage"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

var (
	volumesListView = cmdutil.View{Templates: storageTemplates, Name: "volumes_list.tmpl"}
	volumesGetView  = cmdutil.View{Templates: storageTemplates, Name: "volumes_get.tmpl"}
)

func newVolumesCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "volumes", Short: "Storage volumes"}
	cmdutil.InjectClient(cmd, func() (StorageClient, error) {
		httpClient, apiURL, err := f.HTTPClient()
		if err != nil {
			return nil, err
		}
		return NewStorageClient(httpClient, apiURL)
	})
	cmd.AddCommand(newListVolumesCmd(f), newGetVolumeCmd(f))
	return cmd
}

func newListVolumesCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List storage volumes"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		params := &storageapi.ListStorageVolumesParams{}
		if *device != "" {
			params.Device = device
		}
		resp, err := cmdutil.Client[StorageClient](cmd).ListStorageVolumesWithResponse(cmd.Context(), params)
		if err != nil {
			return err
		}
		return volumesListView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newGetVolumeCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get <volume-id>", Short: "Show volume details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[StorageClient](cmd).GetStorageVolumeWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return volumesGetView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}
