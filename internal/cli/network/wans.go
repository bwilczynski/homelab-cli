package network

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

var (
	wansListView = cmdutil.View{Templates: networkTemplates, Name: "wans_list.tmpl"}
	wansGetView  = cmdutil.View{Templates: networkTemplates, Name: "wans_get.tmpl"}
)

func newWansCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wans",
		Short: "WAN interfaces",
	}
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newListWansCmd(), newGetWanCmd())
	return cmd
}

func newListWansCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List WAN interfaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[NetworkClient](cmd).ListWansWithResponse(cmd.Context())
			if err != nil {
				return err
			}
			return wansListView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}

func newGetWanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <wan-id>",
		Short: "Show WAN interface details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[NetworkClient](cmd).GetWanWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return wansGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}
