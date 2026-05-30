package storage

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
	gen "github.com/bwilczynski/hlctl/internal/storage"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "NAS storage resources",
	}
	cmd.AddCommand(newVolumesCmd())
	cmd.AddCommand(newBackupsCmd())
	return cmd
}

func buildClient() (StorageClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewStorageClient(httpClient, apiURL)
}

func newVolumesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volumes",
		Short: "Storage volumes",
	}
	cmd.AddCommand(newListVolumesCmd(nil))
	cmd.AddCommand(newGetVolumeCmd(nil))
	return cmd
}

func newListVolumesCmd(client StorageClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "list",
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

			resp, err := c.ListStorageVolumesWithResponse(context.Background(), params)
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

func newGetVolumeCmd(client StorageClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <volume-id>",
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

			resp, err := c.GetStorageVolumeWithResponse(context.Background(), args[0])
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

			return printVolumeDetail(cmd, *resp.JSON200)
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

func newBackupsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backups",
		Short: "Backup tasks and history",
	}
	cmd.AddCommand(newListBackupsCmd(nil))
	cmd.AddCommand(newGetBackupCmd(nil))
	return cmd
}

func newListBackupsCmd(client StorageClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListBackupsParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListBackupsWithResponse(context.Background(), params)
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
			headers := []string{"ID", "NAME", "DEVICE", "STATUS", "LAST RESULT", "TYPE"}
			var rows [][]string
			for _, t := range list.Items {
				rows = append(rows, []string{
					t.Id, t.Name, t.Device,
					string(t.Status), string(t.LastResult), t.Type,
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newGetBackupCmd(client StorageClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <backup-id>",
		Short: "Show backup details",
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

			resp, err := c.GetBackupWithResponse(context.Background(), args[0])
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
			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", detail.Id},
				{"NAME", detail.Name},
				{"DEVICE", detail.Device},
				{"STATUS", string(detail.Status)},
				{"LAST RESULT", string(detail.LastResult)},
				{"TYPE", detail.Type},
			}
			if detail.LastRunAt != nil {
				rows = append(rows, []string{"LAST RUN", output.FormatTime(*detail.LastRunAt)})
			}
			if detail.NextRunAt != nil {
				rows = append(rows, []string{"NEXT RUN", output.FormatTime(*detail.NextRunAt)})
			}
			if detail.Size != nil {
				rows = append(rows, []string{"SIZE", output.FormatBytes(*detail.Size)})
			}
			if detail.Folders != nil && len(*detail.Folders) > 0 {
				for i, folder := range *detail.Folders {
					label := "FOLDERS"
					if i > 0 {
						label = ""
					}
					rows = append(rows, []string{label, folder})
				}
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)
		},
	}
}
