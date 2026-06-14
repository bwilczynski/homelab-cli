package network

import (
	"context"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	ssidsListView = cmdutil.View{Templates: networkTemplates, Name: "ssids_list.tmpl"}
	ssidsGetView  = cmdutil.View{Templates: networkTemplates, Name: "ssids_get.tmpl"}
)

type listSsidsOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
}

type getSsidOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newSsidsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssids",
		Short: "WiFi networks (SSIDs)",
	}
	cmd.AddCommand(newListSsidsCmd(f, nil), newGetSsidCmd(f, nil))
	return cmd
}

func newListSsidsCmd(f *cmdutil.Factory, runF func(*listSsidsOptions) error) *cobra.Command {
	opts := &listSsidsOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "list",
		Short: "List WiFi networks",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listSsidsRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func listSsidsRun(ctx context.Context, w io.Writer, opts *listSsidsOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.ListSsidsWithResponse(ctx)
	if err != nil {
		return err
	}
	return ssidsListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

func newGetSsidCmd(f *cmdutil.Factory, runF func(*getSsidOptions) error) *cobra.Command {
	opts := &getSsidOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <ssid-id>",
		Short: "Show WiFi network details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getSsidRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getSsidRun(ctx context.Context, w io.Writer, opts *getSsidOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetSsidWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return ssidsGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
