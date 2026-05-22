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

			headers := []string{"ID", "NAME", "VLAN ID", "BANDS", "CLIENTS"}
			var rows [][]string
			for _, s := range list.Items {
				rows = append(rows, []string{
					s.Id, s.Name, fmt.Sprintf("%d", s.VlanId),
					formatBands(s.Bands),
					fmt.Sprintf("%d", s.NumClients),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
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

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", detail.Id},
				{"NAME", detail.Name},
				{"VLAN ID", fmt.Sprintf("%d", detail.VlanId)},
				{"BANDS", formatBands(detail.Bands)},
				{"CLIENTS", fmt.Sprintf("%d", detail.NumClients)},
				{"SECURITY", string(detail.SecurityProtocol)},
			}
			if err := output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), nil, headers, rows); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n--- CLIENTS ---\n")
			clientHeaders := []string{"NAME"}
			var clientRows [][]string
			for _, cl := range detail.Clients {
				clientRows = append(clientRows, []string{cl.Name})
			}
			if err := output.Print(cmd.OutOrStdout(), output.FormatTable, nil, clientHeaders, clientRows); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n--- BROADCASTING APs ---\n")
			apHeaders := []string{"NAME"}
			var apRows [][]string
			for _, ap := range detail.BroadcastingAps {
				apRows = append(apRows, []string{ap.Name})
			}
			return output.Print(cmd.OutOrStdout(), output.FormatTable, nil, apHeaders, apRows)
		},
	}
}

func formatBands(bands []gen.WifiBand) string {
	var parts []string
	for _, b := range bands {
		switch b {
		case gen.WifiBandBand2g:
			parts = append(parts, "2.4 GHz")
		case gen.WifiBandBand5g:
			parts = append(parts, "5 GHz")
		case gen.WifiBandBand6g:
			parts = append(parts, "6 GHz")
		default:
			parts = append(parts, string(b))
		}
	}
	return strings.Join(parts, ", ")
}
