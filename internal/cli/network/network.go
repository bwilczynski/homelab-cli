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
	cmd.AddCommand(newDevicesCmd())
	cmd.AddCommand(newClientsCmd())
	return cmd
}

func buildClient() (NetworkClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewNetworkClient(httpClient, apiURL)
}

func newDevicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Network devices",
	}
	cmd.AddCommand(newListDevicesCmd(nil))
	cmd.AddCommand(newGetDeviceCmd(nil))
	return cmd
}

func newListDevicesCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
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

			headers := []string{"ID", "NAME", "MAC", "IP", "TYPE", "STATUS"}
			var rows [][]string
			for _, d := range list.Items {
				rows = append(rows, []string{
					d.Id, d.Name, d.Mac, d.Ip,
					string(d.Type), string(d.Status),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}
}

func newGetDeviceCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <device-id>",
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

			disc, err := detail.Discriminator()
			if err != nil {
				return err
			}

			headers := []string{"FIELD", "VALUE"}
			var rows [][]string

			switch disc {
			case "accessPoint":
				d, err := detail.AsAccessPointDetail()
				if err != nil {
					return err
				}
				rows = [][]string{
					{"ID", d.Id},
					{"NAME", d.Name},
					{"MAC", d.Mac},
					{"IP", d.Ip},
					{"TYPE", string(d.Type)},
					{"STATUS", string(d.Status)},
					{"CLIENTS", fmt.Sprintf("%d", d.NumClients)},
					{"MODEL", d.Model},
					{"FIRMWARE", d.FirmwareVersion},
					{"UPTIME", output.FormatUptime(d.Uptime)},
				}
			case "gateway":
				d, err := detail.AsGatewayDetail()
				if err != nil {
					return err
				}
				rows = [][]string{
					{"ID", d.Id},
					{"NAME", d.Name},
					{"MAC", d.Mac},
					{"IP", d.Ip},
					{"TYPE", string(d.Type)},
					{"STATUS", string(d.Status)},
					{"MODEL", d.Model},
					{"FIRMWARE", d.FirmwareVersion},
					{"UPTIME", output.FormatUptime(d.Uptime)},
				}
			case "switch":
				d, err := detail.AsSwitchDetail()
				if err != nil {
					return err
				}
				rows = [][]string{
					{"ID", d.Id},
					{"NAME", d.Name},
					{"MAC", d.Mac},
					{"IP", d.Ip},
					{"TYPE", string(d.Type)},
					{"STATUS", string(d.Status)},
					{"MODEL", d.Model},
					{"FIRMWARE", d.FirmwareVersion},
					{"UPTIME", output.FormatUptime(d.Uptime)},
				}
			case "unknown":
				d, err := detail.AsUnknownDeviceDetail()
				if err != nil {
					return err
				}
				rows = [][]string{
					{"ID", d.Id},
					{"NAME", d.Name},
					{"MAC", d.Mac},
					{"IP", d.Ip},
					{"TYPE", string(d.Type)},
					{"STATUS", string(d.Status)},
					{"MODEL", d.Model},
					{"FIRMWARE", d.FirmwareVersion},
					{"UPTIME", output.FormatUptime(d.Uptime)},
				}
			default:
				return fmt.Errorf("unknown device type: %s", disc)
			}

			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)
		},
	}
}

func newClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Network clients",
	}
	cmd.AddCommand(newListClientsCmd(nil))
	cmd.AddCommand(newGetClientCmd(nil))
	return cmd
}

func newListClientsCmd(client NetworkClient) *cobra.Command {
	var statusFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List network clients",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListNetworkClientsParams{}
			if statusFilter != "" {
				s := gen.NetworkClientStatus(statusFilter)
				params.Status = &s
			}

			resp, err := c.ListNetworkClients(context.Background(), params)
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
			var list gen.NetworkClientList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "NAME", "MAC", "IP", "STATUS", "CONNECTION"}
			var rows [][]string
			for _, cl := range list.Items {
				ip := ""
				if cl.Ip != nil {
					ip = *cl.Ip
				}
				rows = append(rows, []string{
					cl.Id, cl.Name, cl.Mac, ip,
					string(cl.Status),
					string(cl.ConnectionType),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (online|offline)")
	return cmd
}

func newGetClientCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <client-id>",
		Short: "Show network client details",
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

			resp, err := c.GetNetworkClient(context.Background(), args[0])
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
			var detail gen.NetworkClientDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			disc, err := detail.Discriminator()
			if err != nil {
				return err
			}

			headers := []string{"FIELD", "VALUE"}
			var rows [][]string

			switch disc {
			case "wired":
				d, err := detail.AsWiredNetworkClientDetail()
				if err != nil {
					return err
				}
				ip := ""
				if d.Ip != nil {
					ip = *d.Ip
				}
				rows = [][]string{
					{"ID", d.Id},
					{"NAME", d.Name},
					{"MAC", d.Mac},
					{"IP", ip},
					{"CONNECTION", string(d.ConnectionType)},
					{"STATUS", string(d.Status)},
				}
				if d.ConnectedTo.Device.Name != "" {
					rows = append(rows, []string{"SWITCH", d.ConnectedTo.Device.Name})
				}
				if d.ConnectedTo.Port != nil {
					rows = append(rows, []string{"SWITCH PORT", fmt.Sprintf("%d", *d.ConnectedTo.Port)})
				}
				if d.Uptime != nil {
					rows = append(rows, []string{"UPTIME", output.FormatUptime(*d.Uptime)})
				}
			case "wireless":
				d, err := detail.AsWirelessNetworkClientDetail()
				if err != nil {
					return err
				}
				ip := ""
				if d.Ip != nil {
					ip = *d.Ip
				}
				rows = [][]string{
					{"ID", d.Id},
					{"NAME", d.Name},
					{"MAC", d.Mac},
					{"IP", ip},
					{"CONNECTION", string(d.ConnectionType)},
					{"STATUS", string(d.Status)},
				}
				if d.ConnectedTo.Ssid != "" {
					rows = append(rows, []string{"SSID", d.ConnectedTo.Ssid})
				}
				if d.ConnectedTo.SignalStrength != nil {
					rows = append(rows, []string{"SIGNAL", fmt.Sprintf("%d dBm", *d.ConnectedTo.SignalStrength)})
				}
				if d.Uptime != nil {
					rows = append(rows, []string{"UPTIME", output.FormatUptime(*d.Uptime)})
				}
			default:
				return fmt.Errorf("unknown connection type: %s", disc)
			}

			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)
		},
	}
}
