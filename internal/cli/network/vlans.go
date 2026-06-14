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
	vlansListView = cmdutil.View{Templates: networkTemplates, Name: "vlans_list.tmpl"}
	vlansGetView  = cmdutil.View{Templates: networkTemplates, Name: "vlans_get.tmpl"}
)

type listVlansOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
}

type getVlanOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newVlansCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vlans",
		Short: "VLANs",
	}
	cmd.AddCommand(newListVlansCmd(f, nil), newGetVlanCmd(f, nil))
	return cmd
}

func newListVlansCmd(f *cmdutil.Factory, runF func(*listVlansOptions) error) *cobra.Command {
	opts := &listVlansOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "list",
		Short: "List VLANs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listVlansRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func listVlansRun(ctx context.Context, w io.Writer, opts *listVlansOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.ListVlansWithResponse(ctx)
	if err != nil {
		return err
	}
	return vlansListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

func newGetVlanCmd(f *cmdutil.Factory, runF func(*getVlanOptions) error) *cobra.Command {
	opts := &getVlanOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <vlan-id>",
		Short: "Show VLAN details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getVlanRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getVlanRun(ctx context.Context, w io.Writer, opts *getVlanOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetVlanWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return vlansGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
