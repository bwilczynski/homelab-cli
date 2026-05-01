package containers

import (
	"fmt"

	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "containers",
		Short: "Manage Docker containers",
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newStopCmd())
	cmd.AddCommand(newRestartCmd())
	return cmd
}

func newListCmd() *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := []map[string]string{
				{"id": "nas-1.homeassistant", "image": "homeassistant/home-assistant:2024.1", "status": "running", "cpu": "2.3%", "memory": "512MB"},
				{"id": "nas-1.plex", "image": "plexinc/pms-docker:latest", "status": "running", "cpu": "5.1%", "memory": "1.2GB"},
			}

			headers := []string{"ID", "IMAGE", "STATUS", "CPU", "MEMORY"}
			var rows [][]string
			for _, d := range data {
				rows = append(rows, []string{d["id"], d["image"], d["status"], d["cpu"], d["memory"]})
			}
			return output.Print(flags.GetOutputFormat(), data, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <container-id>",
		Short: "Show container details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data := map[string]any{
				"id":     args[0],
				"image":  "homeassistant/home-assistant:2024.1",
				"status": "running",
				"ports":  []string{"8123:8123/tcp"},
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", args[0]},
				{"Image", "homeassistant/home-assistant:2024.1"},
				{"Status", "running"},
				{"Ports", "8123:8123/tcp"},
			}
			return output.Print(flags.GetOutputFormat(), data, headers, rows)
		},
	}
}

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <container-id>",
		Short: "Start a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "Container %s started\n", args[0])
			return nil
		},
	}
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <container-id>",
		Short: "Stop a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "Container %s stopped\n", args[0])
			return nil
		},
	}
}

func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <container-id>",
		Short: "Restart a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "Container %s restarted\n", args[0])
			return nil
		},
	}
}
