package login

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/bwilczynski/hlctl/internal/auth"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the Homelab API",
		Long:  "Store a bearer token for use with the Homelab API.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(cmd.OutOrStdout(), "Token: ")
			scanner := bufio.NewScanner(cmd.InOrStdin())
			if !scanner.Scan() {
				return fmt.Errorf("no token provided")
			}
			token := strings.TrimSpace(scanner.Text())
			if token == "" {
				return fmt.Errorf("token cannot be empty")
			}

			creds := &auth.Credentials{
				AccessToken: token,
				TokenType:   "Bearer",
				ExpiresAt:   time.Now().Add(365 * 24 * time.Hour),
			}
			if err := auth.SaveCredentials(creds); err != nil {
				return fmt.Errorf("saving credentials: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Login successful.")
			return nil
		},
	}
}
