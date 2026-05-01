package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/storage"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "NAS storage volumes",
	}

	cmd.AddCommand(newVolumesCmd(nil))
	cmd.AddCommand(newVolumeCmd(nil))
	return cmd
}

func buildClient() (StorageClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewStorageClient(httpClient, apiURL)
}

func newVolumesCmd(client StorageClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "volumes",
		Short: "List storage volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListStorageVolumesParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListStorageVolumes(context.Background(), params)
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
			var list gen.VolumeList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "NAME", "DEVICE", "RAID", "STATUS", "SIZE", "USED"}
			var rows [][]string
			for _, v := range list.Items {
				rows = append(rows, []string{
					v.Id, v.Name, v.Device, v.RaidType,
					string(v.Status),
					output.FormatBytes(v.TotalBytes),
					output.FormatBytes(v.UsedBytes),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newVolumeCmd(client StorageClient) *cobra.Command {
	return &cobra.Command{
		Use:   "volume <volume-id>",
		Short: "Show volume details",
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

			resp, err := c.GetStorageVolume(context.Background(), args[0])
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
			var detail gen.VolumeDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			return printVolumeDetail(cmd, detail)
		},
	}
}

func printVolumeDetail(cmd *cobra.Command, d gen.VolumeDetail) error {
	w := cmd.OutOrStdout()

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", d.Id},
		{"NAME", d.Name},
		{"DEVICE", d.Device},
		{"FILESYSTEM", d.FileSystem},
		{"RAID", d.RaidType},
		{"STATUS", string(d.Status)},
		{"POOL STATUS", string(d.PoolStatus)},
		{"MOUNT PATH", d.MountPath},
		{"SIZE", output.FormatBytes(d.TotalBytes)},
		{"USED", output.FormatBytes(d.UsedBytes)},
	}
	if err := output.Print(w, output.FormatTable, nil, headers, rows); err != nil {
		return err
	}

	if len(d.Disks) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "DISKS")
		var diskRows [][]string
		for _, disk := range d.Disks {
			diskRows = append(diskRows, []string{
				disk.Id,
				disk.Model,
				string(disk.Status),
				fmt.Sprintf("%d°C", disk.TemperatureCelsius),
				output.FormatBytes(disk.TotalBytes),
			})
		}
		if err := output.Print(w, output.FormatTable, nil, []string{"ID", "MODEL", "STATUS", "TEMP", "SIZE"}, diskRows); err != nil {
			return err
		}
	}

	return nil
}
