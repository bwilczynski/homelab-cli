package network

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

var (
	ssidsListView = cmdutil.View{Templates: networkTemplates, Name: "ssids_list.tmpl"}
	ssidsGetView  = cmdutil.View{Templates: networkTemplates, Name: "ssids_get.tmpl"}
)

func newSsidsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssids",
		Short: "WiFi networks (SSIDs)",
	}
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newListSsidsCmd(), newGetSsidCmd())
	return cmd
}

func newListSsidsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List WiFi networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[NetworkClient](cmd).ListSsidsWithResponse(cmd.Context())
			if err != nil {
				return err
			}
			return ssidsListView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}

func newGetSsidCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <ssid-id>",
		Short: "Show WiFi network details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[NetworkClient](cmd).GetSsidWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return ssidsGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}
