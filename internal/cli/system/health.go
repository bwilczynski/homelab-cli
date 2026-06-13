package system

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

var healthView = cmdutil.View{Templates: systemTemplates, Name: "health.tmpl"}

func newHealthCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Show aggregate system health",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[SystemClient](cmd).GetSystemHealthWithResponse(cmd.Context())
			if err != nil {
				return err
			}
			return healthView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}
