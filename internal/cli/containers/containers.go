package containers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/containers"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "containers",
		Short: "Manage Docker containers",
	}

	cmd.AddCommand(newListCmd(nil))
	cmd.AddCommand(newGetCmd(nil))
	cmd.AddCommand(newStartCmd(nil))
	cmd.AddCommand(newStopCmd(nil))
	cmd.AddCommand(newRestartCmd(nil))
	return cmd
}

func buildClient() (ContainersClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewContainersClient(httpClient, apiURL)
}

func newListCmd(client ContainersClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListContainersParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListContainers(context.Background(), params)
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
			var list gen.ContainerList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "IMAGE", "STATUS", "CPU", "MEMORY"}
			var rows [][]string
			for _, c := range list.Items {
				rows = append(rows, []string{
					c.Id,
					c.Image,
					string(c.Status),
					fmt.Sprintf("%.1f%%", c.Resources.CpuPercent),
					output.FormatBytes(c.Resources.MemoryBytes),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newGetCmd(client ContainersClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <container-id>",
		Short: "Show container details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// implemented in Task 5
			return fmt.Errorf("not implemented")
		},
	}
}

func newStartCmd(client ContainersClient) *cobra.Command {
	return &cobra.Command{
		Use:   "start <container-id>",
		Short: "Start a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// implemented in Task 6
			return fmt.Errorf("not implemented")
		},
	}
}

func newStopCmd(client ContainersClient) *cobra.Command {
	return &cobra.Command{
		Use:   "stop <container-id>",
		Short: "Stop a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// implemented in Task 6
			return fmt.Errorf("not implemented")
		},
	}
}

func newRestartCmd(client ContainersClient) *cobra.Command {
	return &cobra.Command{
		Use:   "restart <container-id>",
		Short: "Restart a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// implemented in Task 6
			return fmt.Errorf("not implemented")
		},
	}
}
