package system

import (
	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
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

func newInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show device information",
	}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		params := &systemapi.ListSystemInfoParams{}
		if *device != "" {
			params.Device = device
		}

		resp, err := cmdutil.Client[SystemClient](cmd).ListSystemInfoWithResponse(cmd.Context(), params)
		if err != nil {
			return err
		}
		return infoView.RenderWith(cmd.OutOrStdout(), flags.GetOutputFormat(), resp.StatusCode(), resp.Body, func() (any, error) {
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
	return cmd
}
