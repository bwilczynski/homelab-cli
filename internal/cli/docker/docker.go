package docker

import (
	"context"
	"io"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	gen "github.com/bwilczynski/hlctl/internal/docker"
	"github.com/spf13/cobra"
)


func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "docker", Short: "Docker resources"}
	cmd.AddCommand(newContainersCmd(), newNetworksCmd(), newImagesCmd())
	return cmd
}

func buildClient() (DockerClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewDockerClient(httpClient, apiURL)
}

// --- containers ---

func newContainersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "containers", Short: "Manage Docker containers"}
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newListCmd(), newGetCmd(), newStartCmd(), newStopCmd(), newRestartCmd())
	return cmd
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List containers"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
		params := &gen.ListContainersParams{}
		if *device != "" {
			params.Device = device
		}
		resp, err := cmdutil.Client[DockerClient](cmd).ListContainersWithResponse(ctx, params)
		if err != nil {
			return err
		}
		return cmdutil.View{Templates: dockerTemplates, Name: "containers_list.tmpl"}.Render(w, resp.StatusCode(), resp.Body, resp.JSON200)
	})
	watch.RegisterFlags(cmd)
	return cmd
}

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "get <container-id>", Short: "Show container details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[DockerClient](cmd).GetContainerWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return cmdutil.View{Templates: dockerTemplates, Name: "containers_get.tmpl"}.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newStartCmd() *cobra.Command {
	return cmdutil.ActionCmd("start <container-id>", "Start a container", "started",
		func(c DockerClient, ctx context.Context, id string) (int, []byte, error) {
			r, err := c.StartContainerWithResponse(ctx, id, &gen.StartContainerParams{})
			if err != nil {
				return 0, nil, err
			}
			return r.StatusCode(), r.Body, nil
		})
}

func newStopCmd() *cobra.Command {
	return cmdutil.ActionCmd("stop <container-id>", "Stop a container", "stopped",
		func(c DockerClient, ctx context.Context, id string) (int, []byte, error) {
			r, err := c.StopContainerWithResponse(ctx, id, &gen.StopContainerParams{})
			if err != nil {
				return 0, nil, err
			}
			return r.StatusCode(), r.Body, nil
		})
}

func newRestartCmd() *cobra.Command {
	return cmdutil.ActionCmd("restart <container-id>", "Restart a container", "restarted",
		func(c DockerClient, ctx context.Context, id string) (int, []byte, error) {
			r, err := c.RestartContainerWithResponse(ctx, id, &gen.RestartContainerParams{})
			if err != nil {
				return 0, nil, err
			}
			return r.StatusCode(), r.Body, nil
		})
}

// --- networks ---

func newNetworksCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "networks", Short: "Docker networks"}
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newListNetworksCmd(), newGetNetworkCmd())
	return cmd
}

func newListNetworksCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List Docker networks"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		params := &gen.ListDockerNetworksParams{}
		if *device != "" {
			params.Device = device
		}
		resp, err := cmdutil.Client[DockerClient](cmd).ListDockerNetworksWithResponse(cmd.Context(), params)
		if err != nil {
			return err
		}
		return cmdutil.View{Templates: dockerTemplates, Name: "networks_list.tmpl"}.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newGetNetworkCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "get <network-id>", Short: "Show network details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[DockerClient](cmd).GetDockerNetworkWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return cmdutil.View{Templates: dockerTemplates, Name: "networks_get.tmpl"}.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

// --- images ---

func newImagesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "images", Short: "Docker images"}
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newListImagesCmd(), newGetImageCmd())
	return cmd
}

func newListImagesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List Docker images"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		params := &gen.ListDockerImagesParams{}
		if *device != "" {
			params.Device = device
		}
		resp, err := cmdutil.Client[DockerClient](cmd).ListDockerImagesWithResponse(cmd.Context(), params)
		if err != nil {
			return err
		}
		return cmdutil.View{Templates: dockerTemplates, Name: "images_list.tmpl"}.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newGetImageCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "get <image-id>", Short: "Show image details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[DockerClient](cmd).GetDockerImageWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return cmdutil.View{Templates: dockerTemplates, Name: "images_get.tmpl"}.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}
