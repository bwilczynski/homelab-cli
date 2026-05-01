// internal/cli/backups/backups.go
package backups

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/backups"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backups",
		Short: "Backup tasks and history",
	}

	cmd.AddCommand(newTasksCmd(nil))
	cmd.AddCommand(newTaskCmd(nil))
	return cmd
}

func buildClient() (BackupsClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewBackupsClient(httpClient, apiURL)
}

func newTasksCmd(client BackupsClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "List backup tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListBackupTasksParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListBackupTasks(context.Background(), params)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return apiclient.ParseError(resp)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var list gen.BackupTaskList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "NAME", "DEVICE", "STATUS", "LAST RESULT", "TYPE"}
			var rows [][]string
			for _, t := range list.Items {
				rows = append(rows, []string{
					t.Id, t.Name, t.Device,
					string(t.Status), string(t.LastResult), t.Type,
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newTaskCmd(client BackupsClient) *cobra.Command {
	return &cobra.Command{
		Use:   "task <task-id>",
		Short: "Show backup task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.GetBackupTask(context.Background(), args[0])
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return apiclient.ParseError(resp)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var detail gen.BackupTaskDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", detail.Id},
				{"NAME", detail.Name},
				{"DEVICE", detail.Device},
				{"STATUS", string(detail.Status)},
				{"LAST RESULT", string(detail.LastResult)},
				{"TYPE", detail.Type},
			}
			if detail.LastRunAt != nil {
				rows = append(rows, []string{"LAST RUN", detail.LastRunAt.Format(time.RFC3339)})
			}
			if detail.NextRunAt != nil {
				rows = append(rows, []string{"NEXT RUN", detail.NextRunAt.Format(time.RFC3339)})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)
		},
	}
}
