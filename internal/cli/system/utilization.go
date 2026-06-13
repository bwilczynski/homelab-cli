package system

import (
	"context"
	"fmt"
	"io"

	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	"github.com/spf13/cobra"
)

var utilizationView = cmdutil.View{Templates: systemTemplates, Name: "utilization.tmpl"}

type utilizationRow struct {
	Device string
	Cpu    string
	Memory string
	Swap   string
}

func newUtilizationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "utilization",
		Short: "Show live resource utilization",
	}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
		params := &systemapi.ListSystemUtilizationParams{}
		if *device != "" {
			params.Device = device
		}

		resp, err := cmdutil.Client[SystemClient](cmd).ListSystemUtilizationWithResponse(ctx, params)
		if err != nil {
			return err
		}
		return utilizationView.RenderWith(w, flags.GetOutputFormat(), resp.StatusCode(), resp.Body, func() (any, error) {
			items := make([]utilizationRow, 0, len(resp.JSON200.Items))
			for _, u := range resp.JSON200.Items {
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
			return struct{ Items []utilizationRow }{items}, nil
		})
	})
	watch.RegisterFlags(cmd)
	return cmd
}
