package network

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Network devices and clients",
	}
	cmdutil.InjectClient(cmd, func() (NetworkClient, error) {
		httpClient, apiURL, err := f.HTTPClient()
		if err != nil {
			return nil, err
		}
		return NewNetworkClient(httpClient, apiURL)
	})
	cmd.AddCommand(newDevicesCmd(f))
	cmd.AddCommand(newClientsCmd(f))
	cmd.AddCommand(newTopologyCmd(f))
	cmd.AddCommand(newVlansCmd(f))
	cmd.AddCommand(newSsidsCmd(f))
	cmd.AddCommand(newWansCmd(f))
	return cmd
}
