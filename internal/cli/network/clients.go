package network

import (
	"context"
	"io"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	gen "github.com/bwilczynski/hlctl/internal/network"
	"github.com/spf13/cobra"
)

var (
	clientsListView = cmdutil.View{Templates: networkTemplates, Name: "clients_list.tmpl"}

	clientGetView = cmdutil.PolymorphicView[gen.NetworkClientDetail]{
		Templates: networkTemplates,
		Variants: map[string]cmdutil.Variant[gen.NetworkClientDetail]{
			"wired": {
				Template: "clients_get_wired.tmpl",
				Resolve:  func(d gen.NetworkClientDetail) (any, error) { return d.AsWiredNetworkClientDetail() },
			},
			"wireless": {
				Template: "clients_get_wireless.tmpl",
				Resolve:  func(d gen.NetworkClientDetail) (any, error) { return d.AsWirelessNetworkClientDetail() },
			},
		},
	}
)

func newClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Network clients",
	}
	cmd.AddCommand(newListClientsCmd())
	cmd.AddCommand(newGetClientCmd())
	return cmd
}

func newListClientsCmd() *cobra.Command {
	var statusFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List network clients",
	}
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
		params := &gen.ListNetworkClientsParams{}
		if statusFilter != "" {
			s := gen.NetworkClientStatus(statusFilter)
			params.Status = &s
		}

		resp, err := cmdutil.Client[NetworkClient](cmd).ListNetworkClientsWithResponse(ctx, params)
		if err != nil {
			return err
		}
		return clientsListView.Render(w, resp.StatusCode(), resp.Body, resp.JSON200)
	})
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (online|offline)")
	watch.RegisterFlags(cmd)
	return cmd
}

func newGetClientCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <client-id>",
		Short: "Show network client details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[NetworkClient](cmd).GetNetworkClientWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return clientGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}
