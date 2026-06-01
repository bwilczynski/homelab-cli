package system

import (
	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System health and information",
	}
	// Single injection on `system` covers the `updates` sub-parent via Cobra's
	// PersistentPreRunE inheritance — sub-parents must NOT define their own.
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newHealthCmd(), newInfoCmd(), newUtilizationCmd(), newUpdatesCmd())
	return cmd
}

func buildClient() (SystemClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewSystemClient(httpClient, apiURL)
}
