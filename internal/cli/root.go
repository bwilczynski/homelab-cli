package cli

import (
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	outputFormat string
	apiURL       string
)

var rootCmd = &cobra.Command{
	Use:   "hlctl",
	Short: "CLI for controlling homelab services",
	Long:  "hlctl is a command-line interface for managing your homelab infrastructure via the Homelab API.",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table or json")
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "Override API base URL")
}

func OutputFormat() output.Format {
	return output.Format(outputFormat)
}

func APIURLOverride() string {
	return apiURL
}

func Execute() error {
	return rootCmd.Execute()
}
