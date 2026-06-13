package network

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/spf13/cobra"
)

var (
	vlansListView = cmdutil.View{Templates: networkTemplates, Name: "vlans_list.tmpl"}
	vlansGetView  = cmdutil.View{Templates: networkTemplates, Name: "vlans_get.tmpl"}
)

func newVlansCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vlans",
		Short: "VLANs",
	}
	cmd.AddCommand(newListVlansCmd(), newGetVlanCmd())
	return cmd
}

func newListVlansCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List VLANs",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[NetworkClient](cmd).ListVlansWithResponse(cmd.Context())
			if err != nil {
				return err
			}
			return vlansListView.Render(cmd.OutOrStdout(), flags.GetOutputFormat(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}

func newGetVlanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <vlan-id>",
		Short: "Show VLAN details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[NetworkClient](cmd).GetVlanWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return vlansGetView.Render(cmd.OutOrStdout(), flags.GetOutputFormat(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}
