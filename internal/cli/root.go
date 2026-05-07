package cli

import (
	"github.com/bwilczynski/hlctl/internal/cli/auth"
	"github.com/bwilczynski/hlctl/internal/cli/config"
	dockercli "github.com/bwilczynski/hlctl/internal/cli/docker"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/cli/network"
	"github.com/bwilczynski/hlctl/internal/cli/storage"
	"github.com/bwilczynski/hlctl/internal/cli/system"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:          "hlctl",
	Short:        "CLI for controlling homelab services",
	Long:         "hlctl is a command-line interface for managing your homelab infrastructure via the Homelab API.",
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flags.OutputFormat, "output", "o", "table", "Output format: table or json")
	rootCmd.PersistentFlags().StringVar(&flags.APIURL, "api-url", "", "Override API base URL")
	rootCmd.AddCommand(auth.NewCmd())
	rootCmd.AddCommand(config.NewCmd())
	rootCmd.AddCommand(dockercli.NewCmd())
	rootCmd.AddCommand(network.NewCmd())
	rootCmd.AddCommand(storage.NewCmd())
	rootCmd.AddCommand(system.NewCmd())
}

func Execute() error {
	return rootCmd.Execute()
}
