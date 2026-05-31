package system

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	"github.com/bwilczynski/hlctl/internal/output"
	gen "github.com/bwilczynski/hlctl/internal/system"
	"github.com/spf13/cobra"
)

var (
	healthView      = cmdutil.View{Templates: systemTemplates, Name: "health.tmpl"}
	updatesListView = cmdutil.View{Templates: systemTemplates, Name: "updates_list.tmpl"}
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System health and information",
	}
	// Single injection on `system` covers the `updates` sub-parent via Cobra's
	// PersistentPreRunE inheritance — sub-parents must NOT define their own.
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newHealthCmd(), newInfoCmd(), newUtilizationCmd(), newUpdatesCmd())
	return cmd
}

func newUpdatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "updates",
		Short: "Software update tracking",
	}
	cmd.AddCommand(newListUpdatesCmd(), newGetUpdateCmd(), newCheckUpdatesCmd())
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

func newHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Show aggregate system health",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[SystemClient](cmd).GetSystemHealthWithResponse(cmd.Context())
			if err != nil {
				return err
			}
			return healthView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}

func newInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show device information",
	}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		params := &gen.ListSystemInfoParams{}
		if *device != "" {
			params.Device = device
		}

		resp, err := cmdutil.Client[SystemClient](cmd).ListSystemInfoWithResponse(cmd.Context(), params)
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
	}
	return cmd
}

func newUtilizationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "utilization",
		Short: "Show live resource utilization",
	}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
		params := &gen.ListSystemUtilizationParams{}
		if *device != "" {
			params.Device = device
		}

		resp, err := cmdutil.Client[SystemClient](cmd).ListSystemUtilizationWithResponse(ctx, params)
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
	watch.RegisterFlags(cmd)
	return cmd
}

func newListUpdatesCmd() *cobra.Command {
	var status, updateType string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tracked software updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := &gen.ListSystemUpdatesParams{}
			if status != "" {
				s := gen.UpdateStatusFilter(status)
				params.Status = &s
			}
			if updateType != "" {
				ut := gen.UpdateTypeFilter(updateType)
				params.Type = &ut
			}

			resp, err := cmdutil.Client[SystemClient](cmd).ListSystemUpdatesWithResponse(cmd.Context(), params)
			if err != nil {
				return err
			}
			return updatesListView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by update status (unknown, upToDate, updateAvailable)")
	cmd.Flags().StringVar(&updateType, "type", "", "Filter by component type (container)")
	return cmd
}

func newGetUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <update-id>",
		Short: "Show update details for a tracked component",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[SystemClient](cmd).GetSystemUpdateWithResponse(cmd.Context(), args[0])
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

func newCheckUpdatesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Force check for upstream updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[SystemClient](cmd).CheckSystemUpdatesWithResponse(cmd.Context(), &gen.CheckSystemUpdatesParams{})
			if err != nil {
				return err
			}
			return updatesListView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}
