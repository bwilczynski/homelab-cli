package system

import (
	"fmt"

	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System health and information",
	}

	cmd.AddCommand(newHealthCmd())
	cmd.AddCommand(newInfoCmd())
	cmd.AddCommand(newUtilizationCmd())
	cmd.AddCommand(newUpdatesCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newCheckUpdatesCmd())
	return cmd
}

func newHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Show aggregate system health",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := map[string]any{
				"status": "healthy",
				"components": []map[string]string{
					{"name": "nas-1", "status": "healthy"},
					{"name": "unifi", "status": "healthy"},
				},
			}
			headers := []string{"COMPONENT", "STATUS"}
			rows := [][]string{
				{"nas-1", "healthy"},
				{"unifi", "healthy"},
			}
			return output.Print(flags.GetOutputFormat(), data, headers, rows)
		},
	}
}

func newInfoCmd() *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show device information",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := []map[string]string{
				{"device": "nas-1", "model": "DS920+", "firmware": "7.2.1-69057", "uptime": "45d 12h"},
			}
			headers := []string{"DEVICE", "MODEL", "FIRMWARE", "UPTIME"}
			var rows [][]string
			for _, d := range data {
				rows = append(rows, []string{d["device"], d["model"], d["firmware"], d["uptime"]})
			}
			return output.Print(flags.GetOutputFormat(), data, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newUtilizationCmd() *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "utilization",
		Short: "Show live resource utilization",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := []map[string]string{
				{"device": "nas-1", "cpu": "12%", "memory": "68%", "swap": "0%"},
			}
			headers := []string{"DEVICE", "CPU", "MEMORY", "SWAP"}
			var rows [][]string
			for _, d := range data {
				rows = append(rows, []string{d["device"], d["cpu"], d["memory"], d["swap"]})
			}
			return output.Print(flags.GetOutputFormat(), data, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newUpdatesCmd() *cobra.Command {
	var status, updateType string

	cmd := &cobra.Command{
		Use:   "updates",
		Short: "List tracked software updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := []map[string]string{
				{"id": "nas-1.homeassistant", "type": "container", "current": "2024.1.0", "latest": "2024.2.0", "status": "available"},
			}
			headers := []string{"ID", "TYPE", "CURRENT", "LATEST", "STATUS"}
			var rows [][]string
			for _, d := range data {
				rows = append(rows, []string{d["id"], d["type"], d["current"], d["latest"], d["status"]})
			}
			return output.Print(flags.GetOutputFormat(), data, headers, rows)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by update status")
	cmd.Flags().StringVar(&updateType, "type", "", "Filter by component type")
	return cmd
}

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <update-id>",
		Short: "Show update details for a tracked component",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data := map[string]any{
				"id":      args[0],
				"type":    "container",
				"current": "2024.1.0",
				"latest":  "2024.2.0",
				"status":  "available",
			}
			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", args[0]},
				{"Type", "container"},
				{"Current", "2024.1.0"},
				{"Latest", "2024.2.0"},
				{"Status", "available"},
			}
			return output.Print(flags.GetOutputFormat(), data, headers, rows)
		},
	}
}

func newCheckUpdatesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check-updates",
		Short: "Force check for upstream updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "Update check triggered")
			return nil
		},
	}
}
