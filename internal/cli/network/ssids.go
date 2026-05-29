package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/network"
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

			resp, err := c.ListSsids(context.Background())
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

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			var list gen.SsidList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "ssids_list.tmpl", list)
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

			resp, err := c.GetSsid(context.Background(), args[0])
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

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			var detail gen.SsidDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "ssids_get.tmpl", detail)
		},
	}
}
