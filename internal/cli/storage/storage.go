package storage

import (
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "NAS storage volumes",
	}

	cmd.AddCommand(newVolumesCmd())
	cmd.AddCommand(newVolumeCmd())
	return cmd
}

func newVolumesCmd() *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "volumes",
		Short: "List storage volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := []map[string]string{
				{"id": "nas-1.volume1", "size": "14.5TB", "used": "9.2TB", "raid": "SHR-2", "health": "healthy"},
			}
			headers := []string{"ID", "SIZE", "USED", "RAID", "HEALTH"}
			var rows [][]string
			for _, d := range data {
				rows = append(rows, []string{d["id"], d["size"], d["used"], d["raid"], d["health"]})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), data, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newVolumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "volume <volume-id>",
		Short: "Show volume details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data := map[string]any{
				"id":     args[0],
				"size":   "14.5TB",
				"used":   "9.2TB",
				"raid":   "SHR-2",
				"health": "healthy",
			}
			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", args[0]},
				{"Size", "14.5TB"},
				{"Used", "9.2TB"},
				{"RAID", "SHR-2"},
				{"Health", "healthy"},
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), data, headers, rows)
		},
	}
}
