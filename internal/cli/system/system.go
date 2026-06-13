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
	// Single injection on `system` covers the `updates` sub-parent via Cobra's
	// PersistentPreRunE inheritance — sub-parents must NOT define their own.
	cmdutil.InjectClient(cmd, func() (SystemClient, error) {
		httpClient, apiURL, err := f.HTTPClient()
		if err != nil {
			return nil, err
		}
		return NewSystemClient(httpClient, apiURL)
	})
	cmd.AddCommand(newHealthCmd(f), newInfoCmd(f), newUtilizationCmd(f), newUpdatesCmd(f))
	return cmd
}
