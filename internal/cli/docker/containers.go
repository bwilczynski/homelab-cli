package docker

import (
	"context"
	"io"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	gen "github.com/bwilczynski/hlctl/internal/docker"
	"github.com/spf13/cobra"
)

var (
	containersListView = cmdutil.View{Templates: dockerTemplates, Name: "containers_list.tmpl"}
	containersGetView  = cmdutil.View{Templates: dockerTemplates, Name: "containers_get.tmpl"}
)

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
		return containersListView.Render(w, resp.StatusCode(), resp.Body, resp.JSON200)
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
		return containersGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
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
