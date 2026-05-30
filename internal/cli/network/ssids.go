package network

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func newSsidsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssids",
		Short: "WiFi networks (SSIDs)",
	}
	cmd.AddCommand(newListSsidsCmd(nil))
	cmd.AddCommand(newGetSsidCmd(nil))
	return cmd
}

func newListSsidsCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List WiFi networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.ListSsidsWithResponse(context.Background())
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

			return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "ssids_list.tmpl", *resp.JSON200)
		},
	}
}

func newGetSsidCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <ssid-id>",
		Short: "Show WiFi network details",
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

			resp, err := c.GetSsidWithResponse(context.Background(), args[0])
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

			return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "ssids_get.tmpl", *resp.JSON200)
		},
	}
}
