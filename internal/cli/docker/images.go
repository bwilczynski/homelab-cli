package docker

import (
	dockerapi "github.com/bwilczynski/hlctl/internal/api/docker"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

var (
	imagesListView = cmdutil.View{Templates: dockerTemplates, Name: "images_list.tmpl"}
	imagesGetView  = cmdutil.View{Templates: dockerTemplates, Name: "images_get.tmpl"}
)

func newImagesCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "images", Short: "Docker images"}
	cmdutil.InjectClient(cmd, func() (DockerClient, error) {
		httpClient, apiURL, err := f.HTTPClient()
		if err != nil {
			return nil, err
		}
		return NewDockerClient(httpClient, apiURL)
	})
	cmd.AddCommand(newListImagesCmd(f), newGetImageCmd(f))
	return cmd
}

func newListImagesCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List Docker images"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		params := &dockerapi.ListDockerImagesParams{}
		if *device != "" {
			params.Device = device
		}
		resp, err := cmdutil.Client[DockerClient](cmd).ListDockerImagesWithResponse(cmd.Context(), params)
		if err != nil {
			return err
		}
		return imagesListView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newGetImageCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get <image-id>", Short: "Show image details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[DockerClient](cmd).GetDockerImageWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return imagesGetView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}
