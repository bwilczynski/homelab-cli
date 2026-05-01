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

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Network devices and clients",
	}

	cmd.AddCommand(newDevicesCmd(nil))
	cmd.AddCommand(newDeviceCmd(nil))
	cmd.AddCommand(newClientsCmd(nil))
	cmd.AddCommand(newClientCmd(nil))
	return cmd
}

func buildClient() (NetworkClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewNetworkClient(httpClient, apiURL)
}

func newDevicesCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "devices",
		Short: "List network devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.ListNetworkDevices(context.Background())
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
			var list gen.NetworkDeviceList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "NAME", "MAC", "IP", "TYPE", "STATUS", "CLIENTS"}
			var rows [][]string
			for _, d := range list.Items {
				clients := ""
				if d.NumClients != nil {
					clients = fmt.Sprintf("%d", *d.NumClients)
				}
				rows = append(rows, []string{
					d.Id, d.Name, d.Mac, d.Ip,
					string(d.Type), string(d.Status),
					clients,
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}
}

func newDeviceCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "device <device-id>",
		Short: "Show network device details",
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

			resp, err := c.GetNetworkDevice(context.Background(), args[0])
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
			var detail gen.NetworkDeviceDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", detail.Id},
				{"NAME", detail.Name},
				{"MAC", detail.Mac},
				{"IP", detail.Ip},
				{"TYPE", string(detail.Type)},
				{"STATUS", string(detail.Status)},
			}
			if detail.NumClients != nil {
				rows = append(rows, []string{"CLIENTS", fmt.Sprintf("%d", *detail.NumClients)})
			}
			rows = append(rows,
				[]string{"MODEL", detail.Model},
				[]string{"FIRMWARE", detail.FirmwareVersion},
				[]string{"UPTIME", output.FormatUptime(detail.Uptime)},
			)
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)
		},
	}
}

func newClientsCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "clients",
		Short: "List connected network clients",
		RunE: func(cmd *cobra.Command, args []string) error {
			panic("not yet implemented")
		},
	}
}

func newClientCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "client <client-id>",
		Short: "Show network client details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			panic("not yet implemented")
		},
	}
}
