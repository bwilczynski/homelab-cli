package cli

import (
	"github.com/bwilczynski/hlctl/internal/cli/auth"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/config"
	dockercli "github.com/bwilczynski/hlctl/internal/cli/docker"
	"github.com/bwilczynski/hlctl/internal/cli/network"
	"github.com/bwilczynski/hlctl/internal/cli/storage"
	"github.com/bwilczynski/hlctl/internal/cli/system"
	"github.com/spf13/cobra"
)

// NewRootCmd builds a fresh root command tree bound to f.
// No package-level state — safe to call multiple times (tests do).
func NewRootCmd(f *cmdutil.Factory) *cobra.Command {
	root := &cobra.Command{
		Use:          "hlctl",
		Short:        "CLI for controlling homelab services",
		Long:         "hlctl is a command-line interface for managing your homelab infrastructure via the Homelab API.",
		Version:      f.Version,
		SilenceUsage: true,
	}
	root.SetOut(f.IOStreams.Out)
	root.SetErr(f.IOStreams.ErrOut)
	root.SetIn(f.IOStreams.In)
	root.AddCommand(
		auth.NewCmd(f),
		config.NewCmd(f),
		dockercli.NewCmd(f),
		network.NewCmd(f),
		storage.NewCmd(f),
		system.NewCmd(f),
	)
	return root
}
