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
	cmd.AddCommand(newDevicesCmd(f), newClientsCmd(f), newTopologyCmd(f, nil), newVlansCmd(f), newSsidsCmd(f), newWansCmd(f))
	return cmd
}
