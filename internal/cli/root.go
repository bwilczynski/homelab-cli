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
	rootCmd.AddCommand(auth.NewCmd(nil)) // nil replaced by Task 14's NewRootCmd(f)
	rootCmd.AddCommand(config.NewCmd(nil))
	rootCmd.AddCommand(dockercli.NewCmd(nil)) // nil replaced by Task 14's NewRootCmd(f)
	rootCmd.AddCommand(network.NewCmd(nil))
	rootCmd.AddCommand(storage.NewCmd())
	rootCmd.AddCommand(system.NewCmd(nil))
}

func Execute(version string) error {
	rootCmd.Version = version
	return rootCmd.Execute()
}
