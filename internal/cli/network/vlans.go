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

func newVlansCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vlans",
		Short: "VLANs",
	}
	cmd.AddCommand(newListVlansCmd(nil))
	cmd.AddCommand(newGetVlanCmd(nil))
	return cmd
}

func newListVlansCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List VLANs",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.ListVlans(context.Background())
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

			var list gen.VlanList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "VLAN ID", "SUBNET"}
			var rows [][]string
			for _, v := range list.Items {
				rows = append(rows, []string{
					v.Id, v.Name, fmt.Sprintf("%d", v.VlanId), v.Subnet,
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}
}

func newGetVlanCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <vlan-id>",
		Short: "Show VLAN details",
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

			resp, err := c.GetVlan(context.Background(), args[0])
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

			var detail gen.VlanDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", detail.Id},
				{"NAME", detail.Name},
				{"VLAN ID", fmt.Sprintf("%d", detail.VlanId)},
				{"SUBNET", detail.Subnet},
				{"GATEWAY IP", detail.GatewayIp},
				{"BROADCAST", detail.BroadcastIp},
				{"DHCP MODE", string(detail.DhcpMode)},
			}
			if detail.DhcpMode == gen.DhcpModeServer && detail.DhcpRange != nil {
				rows = append(rows, []string{"DHCP RANGE", fmt.Sprintf("%s - %s", detail.DhcpRange.Start, detail.DhcpRange.End)})
			}
			if detail.DhcpMode == gen.DhcpModeRelay && detail.RelayServer != nil {
				rows = append(rows, []string{"RELAY", *detail.RelayServer})
			}
			rows = append(rows, []string{"DNS", strings.Join(detail.DnsServers, ", ")})
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), nil, headers, rows)
		},
	}
}
