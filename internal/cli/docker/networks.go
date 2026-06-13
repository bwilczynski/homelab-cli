package docker

import (
	dockerapi "github.com/bwilczynski/hlctl/internal/api/docker"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

var (
	networksListView = cmdutil.View{Templates: dockerTemplates, Name: "networks_list.tmpl"}
	networksGetView  = cmdutil.View{Templates: dockerTemplates, Name: "networks_get.tmpl"}
)

func newNetworksCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "networks", Short: "Docker networks"}
	cmdutil.InjectClient(cmd, func() (DockerClient, error) {
		httpClient, apiURL, err := f.HTTPClient()
		if err != nil {
			return nil, err
		}
		return NewDockerClient(httpClient, apiURL)
	})
	cmd.AddCommand(newListNetworksCmd(f), newGetNetworkCmd(f))
	return cmd
}

func newListNetworksCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List Docker networks"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		params := &dockerapi.ListDockerNetworksParams{}
		if *device != "" {
			params.Device = device
		}
		resp, err := cmdutil.Client[DockerClient](cmd).ListDockerNetworksWithResponse(cmd.Context(), params)
		if err != nil {
			return err
		}
		return networksListView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newGetNetworkCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get <network-id>", Short: "Show network details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[DockerClient](cmd).GetDockerNetworkWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return networksGetView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}
