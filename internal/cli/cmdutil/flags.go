package cmdutil

import "github.com/spf13/cobra"

// DeviceFlag registers --device on cmd and returns a pointer to the bound value.
func DeviceFlag(cmd *cobra.Command) *string {
	return cmd.Flags().String("device", "", "Filter by device ID")
}
