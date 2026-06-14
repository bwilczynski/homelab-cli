package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/api"
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

// --- list ---

type listContainersOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newListContainersCmd(f *cmdutil.Factory, runF func(*listContainersOptions) error) *cobra.Command {
	opts := &listContainersOptions{
		HTTPClient: f.HTTPClient,
		IO:         f.IOStreams,
		Output:     f.Output,
	}
	cmd := &cobra.Command{Use: "list", Short: "List containers"}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	cmd.RunE = watch.Wrap(f.Output, func(ctx context.Context, w io.Writer) error {
		if runF != nil {
			return runF(opts)
		}
		return listContainersRun(ctx, w, opts)
	})
	watch.RegisterFlags(cmd)
	return cmd
}

func listContainersRun(ctx context.Context, w io.Writer, opts *listContainersOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &dockerapi.ListContainersParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListContainersWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return containersListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

// --- get ---

type getContainerOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newGetContainerCmd(f *cmdutil.Factory, runF func(*getContainerOptions) error) *cobra.Command {
	opts := &getContainerOptions{
		HTTPClient: f.HTTPClient,
		IO:         f.IOStreams,
		Output:     f.Output,
	}
	return &cobra.Command{
		Use:   "get <container-id>",
		Short: "Show container details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getContainerRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getContainerRun(ctx context.Context, w io.Writer, opts *getContainerOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetContainerWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return containersGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

// --- start ---

type startContainerOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	ID         string
}

func newStartContainerCmd(f *cmdutil.Factory, runF func(*startContainerOptions) error) *cobra.Command {
	opts := &startContainerOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams}
	return &cobra.Command{
		Use:   "start <container-id>",
		Short: "Start a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return startContainerRun(cmd.Context(), opts)
		},
	}
}

func startContainerRun(ctx context.Context, opts *startContainerOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	r, err := c.StartContainerWithResponse(ctx, opts.ID, &dockerapi.StartContainerParams{})
	if err != nil {
		return err
	}
	if r.StatusCode() != http.StatusNoContent {
		return api.ParseError(r.StatusCode(), r.Body)
	}
	fmt.Fprintf(opts.IO.Out, "%s started\n", opts.ID)
	return nil
}

// --- stop ---

type stopContainerOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	ID         string
}

func newStopContainerCmd(f *cmdutil.Factory, runF func(*stopContainerOptions) error) *cobra.Command {
	opts := &stopContainerOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams}
	return &cobra.Command{
		Use:   "stop <container-id>",
		Short: "Stop a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return stopContainerRun(cmd.Context(), opts)
		},
	}
}

func stopContainerRun(ctx context.Context, opts *stopContainerOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	r, err := c.StopContainerWithResponse(ctx, opts.ID, &dockerapi.StopContainerParams{})
	if err != nil {
		return err
	}
	if r.StatusCode() != http.StatusNoContent {
		return api.ParseError(r.StatusCode(), r.Body)
	}
	fmt.Fprintf(opts.IO.Out, "%s stopped\n", opts.ID)
	return nil
}

// --- restart ---

type restartContainerOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	ID         string
}

func newRestartContainerCmd(f *cmdutil.Factory, runF func(*restartContainerOptions) error) *cobra.Command {
	opts := &restartContainerOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams}
	return &cobra.Command{
		Use:   "restart <container-id>",
		Short: "Restart a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return restartContainerRun(cmd.Context(), opts)
		},
	}
}

func restartContainerRun(ctx context.Context, opts *restartContainerOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	r, err := c.RestartContainerWithResponse(ctx, opts.ID, &dockerapi.RestartContainerParams{})
	if err != nil {
		return err
	}
	if r.StatusCode() != http.StatusNoContent {
		return api.ParseError(r.StatusCode(), r.Body)
	}
	fmt.Fprintf(opts.IO.Out, "%s restarted\n", opts.ID)
	return nil
}
