package network

import (
	"context"
	"io"
	"net/http"

	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	clientsListView = cmdutil.View{Templates: networkTemplates, Name: "clients_list.tmpl"}

	clientGetView = cmdutil.PolymorphicView[networkapi.NetworkClientDetail]{
		Templates: networkTemplates,
		Variants: map[string]cmdutil.Variant[networkapi.NetworkClientDetail]{
			"wired": {
				Template: "clients_get_wired.tmpl",
				Resolve:  func(d networkapi.NetworkClientDetail) (any, error) { return d.AsWiredNetworkClientDetail() },
			},
			"wireless": {
				Template: "clients_get_wireless.tmpl",
				Resolve:  func(d networkapi.NetworkClientDetail) (any, error) { return d.AsWirelessNetworkClientDetail() },
			},
		},
	}
)

type listClientsOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Status     string
}

type getClientOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newClientsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Network clients",
	}
	cmd.AddCommand(newListClientsCmd(f, nil), newGetClientCmd(f, nil))
	return cmd
}

func newListClientsCmd(f *cmdutil.Factory, runF func(*listClientsOptions) error) *cobra.Command {
	opts := &listClientsOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List network clients",
	}
	cmd.RunE = watch.Wrap(f.Output, func(ctx context.Context, w io.Writer) error {
		if runF != nil {
			return runF(opts)
		}
		return listClientsRun(ctx, w, opts)
	})
	cmd.Flags().StringVar(&opts.Status, "status", "", "Filter by status (online|offline)")
	watch.RegisterFlags(cmd)
	return cmd
}

func listClientsRun(ctx context.Context, w io.Writer, opts *listClientsOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &networkapi.ListNetworkClientsParams{}
	if opts.Status != "" {
		s := networkapi.NetworkClientStatus(opts.Status)
		params.Status = &s
	}
	resp, err := c.ListNetworkClientsWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return clientsListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

func newGetClientCmd(f *cmdutil.Factory, runF func(*getClientOptions) error) *cobra.Command {
	opts := &getClientOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <client-id>",
		Short: "Show network client details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getClientRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getClientRun(ctx context.Context, w io.Writer, opts *getClientOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetNetworkClientWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return clientGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
