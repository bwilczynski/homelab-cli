package system

import (
	"context"
	"io"
	"net/http"

	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var infoView = cmdutil.View{Templates: systemTemplates, Name: "info.tmpl"}

type infoRow struct {
	Device   string
	Model    string
	Firmware string
	Ram      string
	Uptime   string
}

type listInfoOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newInfoCmd(f *cmdutil.Factory, runF func(*listInfoOptions) error) *cobra.Command {
	opts := &listInfoOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{Use: "info", Short: "Show device information"}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if runF != nil {
			return runF(opts)
		}
		return listInfoRun(cmd.Context(), opts.IO.Out, opts)
	}
	return cmd
}

func listInfoRun(ctx context.Context, w io.Writer, opts *listInfoOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewSystemClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &systemapi.ListSystemInfoParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListSystemInfoWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return infoView.RenderWith(w, opts.Output(), resp.StatusCode(), resp.Body, func() (any, error) {
		items := make([]infoRow, 0, len(resp.JSON200.Items))
		for _, info := range resp.JSON200.Items {
			items = append(items, infoRow{
				Device:   info.Device,
				Model:    info.Model,
				Firmware: info.Firmware,
				Ram:      output.FormatBytes(int64(info.RamMb) * 1024 * 1024),
				Uptime:   output.FormatUptime(int(info.UptimeSeconds)),
			})
		}
		return struct{ Items []infoRow }{items}, nil
	})
}
