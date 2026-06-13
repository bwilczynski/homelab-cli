package auth

import (
	"context"
	"fmt"

	authpkg "github.com/bwilczynski/hlctl/internal/auth"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with the Homelab API",
	}
	cmd.AddCommand(newLoginCmd(f))
	cmd.AddCommand(newLogoutCmd())
	return cmd
}

func newLoginCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log in via device authorization flow",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			apiURL, err := f.APIURL()
			if err != nil {
				return err
			}

			info, err := authpkg.DiscoverHomelab(apiURL)
			if err != nil {
				return fmt.Errorf("discovery failed: %w", err)
			}
			if !info.Enabled {
				fmt.Fprintln(cmd.OutOrStdout(), "Server does not require authentication.")
				return nil
			}

			endpoints, err := authpkg.DiscoverOIDC(info.Issuer)
			if err != nil {
				return fmt.Errorf("OIDC discovery failed: %w", err)
			}

			creds, err := authpkg.Login(context.Background(), endpoints, cfg.ClientID(), cmd.OutOrStdout())
			if err != nil {
				return err
			}

			if err := authpkg.SaveCredentials(creds); err != nil {
				return fmt.Errorf("saving credentials: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Login successful.")
			return nil
		},
	}
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := authpkg.DeleteCredentials(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
			return nil
		},
	}
}
