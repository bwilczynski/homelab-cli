package login

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the Homelab API",
		Long:  "Obtain an OAuth2 token using client credentials and store it locally.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "login: not yet implemented")
			return nil
		},
	}
}
