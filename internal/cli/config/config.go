package config

import (
	"fmt"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	appconfig "github.com/bwilczynski/hlctl/internal/config"
	"github.com/spf13/cobra"
)

func NewCmd(_ *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage hlctl configuration",
	}

	cmd.AddCommand(newSetURLCmd())
	cmd.AddCommand(newShowCmd())
	return cmd
}

func newSetURLCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-url <url>",
		Short: "Set the API base URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := appconfig.Load()
			if err != nil {
				return err
			}
			cfg.APIURL = args[0]
			if err := appconfig.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "API URL set to %s\n", args[0])
			return nil
		},
	}
}

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := appconfig.Load()
			if err != nil {
				return err
			}
			if cfg.APIURL == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "api_url: (not set)")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "api_url: %s\n", cfg.APIURL)
			}
			return nil
		},
	}
}
