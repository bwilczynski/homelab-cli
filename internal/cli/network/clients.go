package network

import (
	"context"
	"io"

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

func newClientsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Network clients",
	}
	cmd.AddCommand(newListClientsCmd(f))
	cmd.AddCommand(newGetClientCmd(f))
	return cmd
}

func newListClientsCmd(f *cmdutil.Factory) *cobra.Command {
	var statusFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List network clients",
	}
	cmd.RunE = watch.Wrap(func() output.Format { return f.Output() }, func(ctx context.Context, w io.Writer) error {
		params := &networkapi.ListNetworkClientsParams{}
		if statusFilter != "" {
			s := networkapi.NetworkClientStatus(statusFilter)
			params.Status = &s
		}

		resp, err := cmdutil.Client[NetworkClient](cmd).ListNetworkClientsWithResponse(ctx, params)
		if err != nil {
			return err
		}
		return clientsListView.Render(w, f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
	})
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (online|offline)")
	watch.RegisterFlags(cmd)
	return cmd
}

func newGetClientCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <client-id>",
		Short: "Show network client details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[NetworkClient](cmd).GetNetworkClientWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return clientGetView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}
