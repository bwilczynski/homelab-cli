package docker

import (
	"context"
	"io"
	"net/http"

	dockerapi "github.com/bwilczynski/hlctl/internal/api/docker"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	networksListView = cmdutil.View{Templates: dockerTemplates, Name: "networks_list.tmpl"}
	networksGetView  = cmdutil.View{Templates: dockerTemplates, Name: "networks_get.tmpl"}
)

type listNetworksOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newNetworksCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "networks", Short: "Docker networks"}
	cmd.AddCommand(newListNetworksCmd(f, nil), newGetNetworkCmd(f, nil))
	return cmd
}

func newListNetworksCmd(f *cmdutil.Factory, runF func(*listNetworksOptions) error) *cobra.Command {
	opts := &listNetworksOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Docker networks",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listNetworksRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	return cmd
}

func listNetworksRun(ctx context.Context, w io.Writer, opts *listNetworksOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &dockerapi.ListDockerNetworksParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListDockerNetworksWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return networksListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

type getNetworkOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newGetNetworkCmd(f *cmdutil.Factory, runF func(*getNetworkOptions) error) *cobra.Command {
	opts := &getNetworkOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <network-id>",
		Short: "Show network details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getNetworkRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getNetworkRun(ctx context.Context, w io.Writer, opts *getNetworkOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetDockerNetworkWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return networksGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
