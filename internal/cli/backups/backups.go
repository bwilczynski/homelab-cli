package backups

import (
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backups",
		Short: "Backup tasks and history",
	}

	cmd.AddCommand(newTasksCmd())
	cmd.AddCommand(newTaskCmd())
	return cmd
}

func newTasksCmd() *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "List backup tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := []map[string]string{
				{"id": "nas-1.daily-backup", "status": "completed", "lastResult": "success", "type": "hyper_backup"},
			}
			headers := []string{"ID", "STATUS", "LAST RESULT", "TYPE"}
			var rows [][]string
			for _, d := range data {
				rows = append(rows, []string{d["id"], d["status"], d["lastResult"], d["type"]})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), data, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newTaskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "task <task-id>",
		Short: "Show backup task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data := map[string]any{
				"id":         args[0],
				"status":     "completed",
				"lastResult": "success",
				"type":       "hyper_backup",
				"lastRunAt":  "2026-04-30T03:00:00Z",
				"nextRunAt":  "2026-05-01T03:00:00Z",
			}
			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", args[0]},
				{"Status", "completed"},
				{"Last Result", "success"},
				{"Type", "hyper_backup"},
				{"Last Run", "2026-04-30T03:00:00Z"},
				{"Next Run", "2026-05-01T03:00:00Z"},
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), data, headers, rows)
		},
	}
}
