package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hlctl",
	Short: "CLI for controlling homelab services",
}

func Execute() error {
	return rootCmd.Execute()
}
