package system

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System health and information",
	}
	cmd.AddCommand(newHealthCmd(f, nil), newInfoCmd(f, nil), newUtilizationCmd(f, nil), newUpdatesCmd(f))
	return cmd
}
