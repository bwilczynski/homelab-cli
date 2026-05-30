package system

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	"github.com/bwilczynski/hlctl/internal/output"
	gen "github.com/bwilczynski/hlctl/internal/system"
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
	cmd.AddCommand(newUpdatesCmd())
	return cmd
}

func newUpdatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "updates",
		Short: "Software update tracking",
	}
	cmd.AddCommand(newListUpdatesCmd(nil))
	cmd.AddCommand(newGetUpdateCmd(nil))
	cmd.AddCommand(newCheckUpdatesCmd(nil))
	return cmd
}

type infoRow struct {
	Device   string
	Model    string
	Firmware string
	Ram      string
	Uptime   string
}

type utilizationRow struct {
	Device string
	Cpu    string
	Memory string
	Swap   string
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

			resp, err := c.GetSystemHealthWithResponse(context.Background())
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusOK {
				return apiclient.ParseError(resp.StatusCode(), resp.Body)
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
				return nil
			}

			return output.RenderTemplate(cmd.OutOrStdout(), systemTemplates, "health.tmpl", *resp.JSON200)
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

			resp, err := c.ListSystemInfoWithResponse(context.Background(), params)
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusOK {
				return apiclient.ParseError(resp.StatusCode(), resp.Body)
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
				return nil
			}

			list := resp.JSON200
			var items []infoRow
			for _, info := range list.Items {
				items = append(items, infoRow{
					Device:   info.Device,
					Model:    info.Model,
					Firmware: info.Firmware,
					Ram:      output.FormatBytes(int64(info.RamMb) * 1024 * 1024),
					Uptime:   output.FormatUptime(int(info.UptimeSeconds)),
				})
			}
			return output.RenderTemplate(cmd.OutOrStdout(), systemTemplates, "info.tmpl", struct{ Items []infoRow }{items})
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
	}
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
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

		resp, err := c.ListSystemUtilizationWithResponse(ctx, params)
		if err != nil {
			return err
		}
		if resp.StatusCode() != http.StatusOK {
			return apiclient.ParseError(resp.StatusCode(), resp.Body)
		}

		if flags.GetOutputFormat() == output.FormatJSON {
			fmt.Fprint(w, string(resp.Body))
			return nil
		}

		list := resp.JSON200
		var items []utilizationRow
		for _, u := range list.Items {
			swapPct := 0
			if u.Memory.SwapTotalBytes > 0 {
				swapPct = int(u.Memory.SwapUsedBytes * 100 / u.Memory.SwapTotalBytes)
			}
			items = append(items, utilizationRow{
				Device: u.Device,
				Cpu:    fmt.Sprintf("%d%%", u.Cpu.TotalPercent),
				Memory: fmt.Sprintf("%d%%", u.Memory.UsedPercent),
				Swap:   fmt.Sprintf("%d%%", swapPct),
			})
		}
		return output.RenderTemplate(w, systemTemplates, "utilization.tmpl", struct{ Items []utilizationRow }{items})
	})

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	watch.RegisterFlags(cmd)
	return cmd
}

func newListUpdatesCmd(client SystemClient) *cobra.Command {
	var status, updateType string

	cmd := &cobra.Command{
		Use:   "list",
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

			resp, err := c.ListSystemUpdatesWithResponse(context.Background(), params)
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusOK {
				return apiclient.ParseError(resp.StatusCode(), resp.Body)
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
				return nil
			}

			return output.RenderTemplate(cmd.OutOrStdout(), systemTemplates, "updates_list.tmpl", *resp.JSON200)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by update status (unknown, upToDate, updateAvailable)")
	cmd.Flags().StringVar(&updateType, "type", "", "Filter by component type (container)")
	return cmd
}

func newGetUpdateCmd(client SystemClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <update-id>",
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

			resp, err := c.GetSystemUpdateWithResponse(context.Background(), args[0])
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusOK {
				return apiclient.ParseError(resp.StatusCode(), resp.Body)
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
				return nil
			}

			detail := resp.JSON200
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
				return output.RenderTemplate(cmd.OutOrStdout(), systemTemplates, "updates_get_container.tmpl", d)
			default:
				return fmt.Errorf("unknown update type: %s", disc)
			}
		},
	}
}

func newCheckUpdatesCmd(client SystemClient) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
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

			resp, err := c.CheckSystemUpdatesWithResponse(context.Background(), &gen.CheckSystemUpdatesParams{})
			if err != nil {
				return err
			}
			if resp.StatusCode() != http.StatusOK {
				return apiclient.ParseError(resp.StatusCode(), resp.Body)
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
				return nil
			}

			return output.RenderTemplate(cmd.OutOrStdout(), systemTemplates, "updates_list.tmpl", *resp.JSON200)
		},
	}
}
