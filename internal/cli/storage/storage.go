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

			return output.RenderTemplate(cmd.OutOrStdout(), storageTemplates, "volumes_list.tmpl", *resp.JSON200)
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

			return output.RenderTemplate(cmd.OutOrStdout(), storageTemplates, "volumes_get.tmpl", *resp.JSON200)
		},
	}
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

			return output.RenderTemplate(cmd.OutOrStdout(), storageTemplates, "backups_list.tmpl", *resp.JSON200)
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

			return output.RenderTemplate(cmd.OutOrStdout(), storageTemplates, "backups_get.tmpl", *resp.JSON200)
		},
	}
}
