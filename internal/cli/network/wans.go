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

func newWansCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wans",
		Short: "WAN interfaces",
	}
	cmd.AddCommand(newListWansCmd(nil))
	cmd.AddCommand(newGetWanCmd(nil))
	return cmd
}

func newListWansCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List WAN interfaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.ListWansWithResponse(context.Background())
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

			return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "wans_list.tmpl", *resp.JSON200)
		},
	}
}

func newGetWanCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <wan-id>",
		Short: "Show WAN interface details",
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

			resp, err := c.GetWanWithResponse(context.Background(), args[0])
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

			return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "wans_get.tmpl", *resp.JSON200)
		},
	}
}
