package system

import (
	"context"
	"fmt"
	"io"
	"net/http"

	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var utilizationView = cmdutil.View{Templates: systemTemplates, Name: "utilization.tmpl"}

type utilizationRow struct {
	Device string
	Cpu    string
	Memory string
	Swap   string
}

type listUtilizationOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newUtilizationCmd(f *cmdutil.Factory, runF func(*listUtilizationOptions) error) *cobra.Command {
	opts := &listUtilizationOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{Use: "utilization", Short: "Show live resource utilization"}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	cmd.RunE = watch.Wrap(f.Output, func(ctx context.Context, w io.Writer) error {
		if runF != nil {
			return runF(opts)
		}
		return listUtilizationRun(ctx, w, opts)
	})
	watch.RegisterFlags(cmd)
	return cmd
}

func listUtilizationRun(ctx context.Context, w io.Writer, opts *listUtilizationOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewSystemClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &systemapi.ListSystemUtilizationParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListSystemUtilizationWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return utilizationView.RenderWith(w, opts.Output(), resp.StatusCode(), resp.Body, func() (any, error) {
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
}
