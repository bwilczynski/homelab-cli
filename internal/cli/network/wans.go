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
	wansListView = cmdutil.View{Templates: networkTemplates, Name: "wans_list.tmpl"}
	wansGetView  = cmdutil.View{Templates: networkTemplates, Name: "wans_get.tmpl"}
)

type listWansOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
}

type getWanOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newWansCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wans",
		Short: "WAN interfaces",
	}
	cmd.AddCommand(newListWansCmd(f, nil), newGetWanCmd(f, nil))
	return cmd
}

func newListWansCmd(f *cmdutil.Factory, runF func(*listWansOptions) error) *cobra.Command {
	opts := &listWansOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "list",
		Short: "List WAN interfaces",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listWansRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func listWansRun(ctx context.Context, w io.Writer, opts *listWansOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.ListWansWithResponse(ctx)
	if err != nil {
		return err
	}
	return wansListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

func newGetWanCmd(f *cmdutil.Factory, runF func(*getWanOptions) error) *cobra.Command {
	opts := &getWanOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <wan-id>",
		Short: "Show WAN interface details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getWanRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getWanRun(ctx context.Context, w io.Writer, opts *getWanOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetWanWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return wansGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
