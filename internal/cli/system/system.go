// internal/cli/system/system.go
package system

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/system"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System health and information",
	}

	cmd.AddCommand(newHealthCmd(nil))
	cmd.AddCommand(newInfoCmd(nil))
	cmd.AddCommand(newUtilizationCmd(nil))
	cmd.AddCommand(newUpdatesCmd(nil))
	cmd.AddCommand(newUpdateCmd(nil))
	cmd.AddCommand(newCheckUpdatesCmd(nil))
	return cmd
}

func buildClient() (SystemClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewSystemClient(httpClient, apiURL)
}

func newHealthCmd(client SystemClient) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Show aggregate system health",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.GetSystemHealth(context.Background())
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
			var health gen.Health
			if err := json.Unmarshal(body, &health); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"COMPONENT", "STATUS"}
			var rows [][]string
			for _, comp := range health.Components {
				rows = append(rows, []string{comp.Name, string(comp.Status)})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), health, headers, rows)
		},
	}
}

func newInfoCmd(client SystemClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show device information",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListSystemInfoParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListSystemInfo(context.Background(), params)
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
			var list gen.SystemInfoList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"DEVICE", "MODEL", "FIRMWARE", "RAM", "UPTIME"}
			var rows [][]string
			for _, info := range list.Items {
				rows = append(rows, []string{
					info.Device,
					info.Model,
					info.Firmware,
					fmt.Sprintf("%d MB", info.RamMb),
					output.FormatUptime(int(info.UptimeSeconds)),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newUtilizationCmd(client SystemClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "utilization",
		Short: "Show live resource utilization",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListSystemUtilizationParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListSystemUtilization(context.Background(), params)
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
			var list gen.SystemUtilizationList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"DEVICE", "CPU", "MEMORY", "SWAP"}
			var rows [][]string
			for _, u := range list.Items {
				swapPct := 0
				if u.Memory.SwapTotalBytes > 0 {
					swapPct = int(u.Memory.SwapUsedBytes * 100 / u.Memory.SwapTotalBytes)
				}
				rows = append(rows, []string{
					u.Device,
					fmt.Sprintf("%d%%", u.Cpu.TotalPercent),
					fmt.Sprintf("%d%%", u.Memory.UsedPercent),
					fmt.Sprintf("%d%%", swapPct),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newUpdatesCmd(client SystemClient) *cobra.Command {
	var status, updateType string

	cmd := &cobra.Command{
		Use:   "updates",
		Short: "List tracked software updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListSystemUpdatesParams{}
			if status != "" {
				s := gen.UpdateStatusFilter(status)
				params.Status = &s
			}
			if updateType != "" {
				ut := gen.UpdateTypeFilter(updateType)
				params.Type = &ut
			}

			resp, err := c.ListSystemUpdates(context.Background(), params)
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
			var list gen.SystemUpdateList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			return printUpdateList(cmd.OutOrStdout(), list)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by update status (unknown, upToDate, updateAvailable)")
	cmd.Flags().StringVar(&updateType, "type", "", "Filter by component type (container)")
	return cmd
}

func printUpdateList(w io.Writer, list gen.SystemUpdateList) error {
	headers := []string{"ID", "NAME", "DEVICE", "TYPE", "STATUS", "CURRENT", "LATEST"}
	var rows [][]string
	for _, u := range list.Items {
		rows = append(rows, []string{
			u.Id, u.Name, u.Device,
			string(u.Type), string(u.Status),
			u.CurrentVersion, u.LatestVersion,
		})
	}
	return output.Print(w, output.FormatTable, list, headers, rows)
}

func newUpdateCmd(client SystemClient) *cobra.Command {
	return &cobra.Command{
		Use:   "update <update-id>",
		Short: "Show update details for a tracked component",
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

			resp, err := c.GetSystemUpdate(context.Background(), args[0])
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
			var detail gen.SystemUpdateDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			disc, err := detail.Discriminator()
			if err != nil {
				return err
			}

			switch disc {
			case "container":
				d, err := detail.AsContainerSystemUpdateDetail()
				if err != nil {
					return err
				}
				headers := []string{"FIELD", "VALUE"}
				rows := [][]string{
					{"ID", d.Id},
					{"NAME", d.Name},
					{"DEVICE", d.Device},
					{"TYPE", string(d.Type)},
					{"STATUS", string(d.Status)},
					{"CURRENT", d.CurrentVersion},
					{"LATEST", d.LatestVersion},
					{"CHECKED AT", d.CheckedAt.Format(time.RFC3339)},
					{"PUBLISHED AT", d.PublishedAt.Format(time.RFC3339)},
					{"IMAGE", d.Image},
					{"SOURCE", d.Source},
					{"RELEASE URL", d.ReleaseUrl},
				}
				return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)
			default:
				return fmt.Errorf("unknown update type: %s", disc)
			}
		},
	}
}

func newCheckUpdatesCmd(client SystemClient) *cobra.Command {
	return &cobra.Command{
		Use:   "check-updates",
		Short: "Force check for upstream updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.CheckSystemUpdates(context.Background(), &gen.CheckSystemUpdatesParams{})
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
			var list gen.SystemUpdateList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			return printUpdateList(cmd.OutOrStdout(), list)
		},
	}
}
