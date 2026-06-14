package docker

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "docker", Short: "Docker resources"}
	cmd.AddCommand(
		newContainersCmd(f),
		newNetworksCmd(f),
		newImagesCmd(f),
	)
	return cmd
}

func newContainersCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "containers", Short: "Manage Docker containers"}
	cmd.AddCommand(
		newListContainersCmd(f, nil),
		newGetContainerCmd(f, nil),
		newStartContainerCmd(f, nil),
		newStopContainerCmd(f, nil),
		newRestartContainerCmd(f, nil),
	)
	return cmd
}
