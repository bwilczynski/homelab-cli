package docker

import (
	"context"
	"io"

	dockerapi "github.com/bwilczynski/hlctl/internal/api/docker"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	containersListView = cmdutil.View{Templates: dockerTemplates, Name: "containers_list.tmpl"}
	containersGetView  = cmdutil.View{Templates: dockerTemplates, Name: "containers_get.tmpl"}
)

func newContainersCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "containers", Short: "Manage Docker containers"}
	cmdutil.InjectClient(cmd, func() (DockerClient, error) {
		httpClient, apiURL, err := f.HTTPClient()
		if err != nil {
			return nil, err
		}
		return NewDockerClient(httpClient, apiURL)
	})
	cmd.AddCommand(
		newListContainersCmd(f),
		newGetContainerCmd(f),
		newStartContainerCmd(),
		newStopContainerCmd(),
		newRestartContainerCmd(),
	)
	return cmd
}

func newListContainersCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List containers"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = watch.Wrap(
		func() output.Format { return f.Output() },
		func(ctx context.Context, w io.Writer) error {
			params := &dockerapi.ListContainersParams{}
			if *device != "" {
				params.Device = device
			}
			resp, err := cmdutil.Client[DockerClient](cmd).ListContainersWithResponse(ctx, params)
			if err != nil {
				return err
			}
			return containersListView.Render(w, f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
		})
	watch.RegisterFlags(cmd)
	return cmd
}

func newGetContainerCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get <container-id>", Short: "Show container details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[DockerClient](cmd).GetContainerWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return containersGetView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newStartContainerCmd() *cobra.Command {
	return cmdutil.ActionCmd("start <container-id>", "Start a container", "started",
		func(c DockerClient, ctx context.Context, id string) (int, []byte, error) {
			r, err := c.StartContainerWithResponse(ctx, id, &dockerapi.StartContainerParams{})
			if err != nil {
				return 0, nil, err
			}
			return r.StatusCode(), r.Body, nil
		})
}

func newStopContainerCmd() *cobra.Command {
	return cmdutil.ActionCmd("stop <container-id>", "Stop a container", "stopped",
		func(c DockerClient, ctx context.Context, id string) (int, []byte, error) {
			r, err := c.StopContainerWithResponse(ctx, id, &dockerapi.StopContainerParams{})
			if err != nil {
				return 0, nil, err
			}
			return r.StatusCode(), r.Body, nil
		})
}

func newRestartContainerCmd() *cobra.Command {
	return cmdutil.ActionCmd("restart <container-id>", "Restart a container", "restarted",
		func(c DockerClient, ctx context.Context, id string) (int, []byte, error) {
			r, err := c.RestartContainerWithResponse(ctx, id, &dockerapi.RestartContainerParams{})
			if err != nil {
				return 0, nil, err
			}
			return r.StatusCode(), r.Body, nil
		})
}
