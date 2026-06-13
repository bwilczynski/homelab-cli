package network

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

var (
	vlansListView = cmdutil.View{Templates: networkTemplates, Name: "vlans_list.tmpl"}
	vlansGetView  = cmdutil.View{Templates: networkTemplates, Name: "vlans_get.tmpl"}
)

func newVlansCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vlans",
		Short: "VLANs",
	}
	cmd.AddCommand(newListVlansCmd(f), newGetVlanCmd(f))
	return cmd
}

func newListVlansCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List VLANs",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[NetworkClient](cmd).ListVlansWithResponse(cmd.Context())
			if err != nil {
				return err
			}
			return vlansListView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}

func newGetVlanCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <vlan-id>",
		Short: "Show VLAN details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[NetworkClient](cmd).GetVlanWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return vlansGetView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}
