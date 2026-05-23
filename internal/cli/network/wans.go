package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/network"
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

			resp, err := c.ListWans(context.Background())
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

			var list gen.WanList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "IP", "UPTIME", "STATUS"}
			var rows [][]string
			for _, w := range list.Items {
				rows = append(rows, []string{
					w.Id, w.Name, w.IpAddress,
					output.FormatUptime(w.Uptime),
					string(w.Status),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
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

			resp, err := c.GetWan(context.Background(), args[0])
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

			var detail gen.WanDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", detail.Id},
				{"NAME", detail.Name},
				{"IP", detail.IpAddress},
				{"UPTIME", output.FormatUptime(detail.Uptime)},
				{"STATUS", string(detail.Status)},
				{"DNS", strings.Join(detail.DnsServers, ", ")},
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), nil, headers, rows)
		},
	}
}
