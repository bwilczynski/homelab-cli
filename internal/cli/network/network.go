package network

import (
	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Network devices and clients",
	}
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newDevicesCmd())
	cmd.AddCommand(newClientsCmd())
	cmd.AddCommand(newTopologyCmd())
	cmd.AddCommand(newVlansCmd())
	cmd.AddCommand(newSsidsCmd())
	cmd.AddCommand(newWansCmd())
	return cmd
}

func buildClient() (NetworkClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewNetworkClient(httpClient, apiURL)
}
