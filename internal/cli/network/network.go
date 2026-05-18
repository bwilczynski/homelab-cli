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

func deviceBaseRows(id, name, mac, ip, typ, status, model, firmware string, uptime int, traffic gen.NetworkTraffic, uplink *gen.NetworkConnection) [][]string {
	rows := [][]string{
		{"ID", id},
		{"NAME", name},
		{"MAC", mac},
		{"IP", ip},
		{"TYPE", typ},
		{"STATUS", status},
		{"MODEL", model},
		{"FIRMWARE", firmware},
		{"UPTIME", output.FormatUptime(uptime)},
		{"TRAFFIC RX", fmt.Sprintf("%s (%s total)", output.FormatBytesPerSec(traffic.RxBytesPerSec), output.FormatBytes(traffic.RxBytesTotal))},
		{"TRAFFIC TX", fmt.Sprintf("%s (%s total)", output.FormatBytesPerSec(traffic.TxBytesPerSec), output.FormatBytes(traffic.TxBytesTotal))},
	}
	if uplink != nil {
		uplinkStr := uplink.Device.Name
		if uplink.Port != nil {
			uplinkStr += fmt.Sprintf(" (port %d", *uplink.Port)
			if uplink.LinkSpeed != nil {
				uplinkStr += fmt.Sprintf(", %s", output.FormatLinkSpeed(string(*uplink.LinkSpeed)))
			}
			uplinkStr += ")"
		}
		rows = append(rows, []string{"UPLINK", uplinkStr})
	}
	return rows
}

func newGetDeviceCmd(client NetworkClient) *cobra.Command {
	var allPorts bool
	cmd := &cobra.Command{
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

			baseHeaders := []string{"FIELD", "VALUE"}

			switch disc {
			case "switch":
				d, err := detail.AsSwitchDetail()
				if err != nil {
					return err
				}
				rows := deviceBaseRows(d.Id, d.Name, d.Mac, d.Ip, string(d.Type), string(d.Status), d.Model, d.FirmwareVersion, d.Uptime, d.Traffic, d.Uplink)
				if err := output.Print(cmd.OutOrStdout(), output.FormatTable, nil, baseHeaders, rows); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n--- PORTS ---\n")
				portHeaders := []string{"PORT", "STATE", "SPEED", "POE", "POE WATTS", "RX", "TX", "CONNECTED TO"}
				var portRows [][]string
				for _, p := range d.Ports {
					if !allPorts && p.State != gen.NetworkPortStateUp {
						continue
					}
					speed := "-"
					if p.State == gen.NetworkPortStateUp && p.LinkSpeed != nil {
						speed = output.FormatLinkSpeed(string(*p.LinkSpeed))
					}
					poePower := "-"
					if p.PoePowerWatts != nil {
						poePower = fmt.Sprintf("%.1f W", *p.PoePowerWatts)
					}
					connectedTo := "-"
					if p.ConnectedTo != nil {
						kind, err := p.ConnectedTo.Discriminator()
						if err != nil {
							return err
						}
						switch kind {
						case "device":
							ref, err := p.ConnectedTo.AsNetworkDeviceRef()
							if err != nil {
								return err
							}
							connectedTo = ref.Name
						case "client":
							ref, err := p.ConnectedTo.AsNetworkClientRef()
							if err != nil {
								return err
							}
							connectedTo = ref.Name
						}
					}
					portRows = append(portRows, []string{
						fmt.Sprintf("%d", p.Number),
						string(p.State),
						speed,
						string(p.PoeMode),
						poePower,
						output.FormatBytesPerSec(p.Traffic.RxBytesPerSec),
						output.FormatBytesPerSec(p.Traffic.TxBytesPerSec),
						connectedTo,
					})
				}
				return output.Print(cmd.OutOrStdout(), output.FormatTable, nil, portHeaders, portRows)

			case "accessPoint":
				d, err := detail.AsAccessPointDetail()
				if err != nil {
					return err
				}
				rows := deviceBaseRows(d.Id, d.Name, d.Mac, d.Ip, string(d.Type), string(d.Status), d.Model, d.FirmwareVersion, d.Uptime, d.Traffic, d.Uplink)
				if err := output.Print(cmd.OutOrStdout(), output.FormatTable, nil, baseHeaders, rows); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n--- CLIENTS ---\n")
				clientHeaders := []string{"CLIENT", "SSID", "SIGNAL"}
				var clientRows [][]string
				for _, cl := range d.ConnectedClients {
					clientRows = append(clientRows, []string{
						cl.Client.Name,
						cl.Ssid,
						fmt.Sprintf("%d dBm", cl.SignalStrength),
					})
				}
				return output.Print(cmd.OutOrStdout(), output.FormatTable, nil, clientHeaders, clientRows)

			case "gateway":
				d, err := detail.AsGatewayDetail()
				if err != nil {
					return err
				}
				rows := deviceBaseRows(d.Id, d.Name, d.Mac, d.Ip, string(d.Type), string(d.Status), d.Model, d.FirmwareVersion, d.Uptime, d.Traffic, d.Uplink)
				return output.Print(cmd.OutOrStdout(), output.FormatTable, nil, baseHeaders, rows)

			case "unknown":
				d, err := detail.AsUnknownDeviceDetail()
				if err != nil {
					return err
				}
				rows := deviceBaseRows(d.Id, d.Name, d.Mac, d.Ip, string(d.Type), string(d.Status), d.Model, d.FirmwareVersion, d.Uptime, d.Traffic, d.Uplink)
				return output.Print(cmd.OutOrStdout(), output.FormatTable, nil, baseHeaders, rows)

			default:
				return fmt.Errorf("unknown device type: %s", disc)
			}
		},
	}
	cmd.Flags().BoolVar(&allPorts, "all-ports", false, "Show all ports (default: active ports only)")
	return cmd
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
					{"SWITCH", d.ConnectedTo.Device.Name},
				}
				if d.ConnectedTo.Port != nil {
					rows = append(rows, []string{"PORT", fmt.Sprintf("%d", *d.ConnectedTo.Port)})
				}
				if d.ConnectedTo.LinkSpeed != nil {
					rows = append(rows, []string{"LINK SPEED", output.FormatLinkSpeed(string(*d.ConnectedTo.LinkSpeed))})
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
					{"AP", d.ConnectedTo.Device.Name},
					{"SSID", d.ConnectedTo.Ssid},
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

type childEntry struct {
	nodeID   string
	nodeDisp string
	edgeDisp string
}

func newTopologyCmd(client NetworkClient) *cobra.Command {
	var includeClients bool
	var includeWireless bool

	cmd := &cobra.Command{
		Use:   "topology",
		Short: "Show network topology",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.GetNetworkTopologyParams{}
			if includeClients || includeWireless {
				t := true
				params.IncludeClients = &t
			}

			resp, err := c.GetNetworkTopology(context.Background(), params)
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

			var topo gen.NetworkTopology
			if err := json.Unmarshal(body, &topo); err != nil {
				return err
			}

			return printTopologyTree(cmd.OutOrStdout(), topo, includeWireless)
		},
	}

	cmd.Flags().BoolVar(&includeClients, "include-clients", false, "Include wired clients in the topology")
	cmd.Flags().BoolVar(&includeWireless, "include-wireless", false, "Also include wireless clients (implies --include-clients)")
	return cmd
}


func printTopologyTree(w io.Writer, topo gen.NetworkTopology, includeWireless bool) error {
	// Build node display strings keyed by node ID.
	nodeDisp := make(map[string]string)
	var gatewayID string
	for _, n := range topo.Nodes {
		disc, err := n.Discriminator()
		if err != nil {
			return err
		}
		switch disc {
		case "device":
			d, err := n.AsTopologyDeviceNode()
			if err != nil {
				return err
			}
			disp := fmt.Sprintf("%s (%s)", d.Name, string(d.Type))
			if d.NumClients != nil && *d.NumClients > 0 {
				disp += fmt.Sprintf(" [%d clients]", *d.NumClients)
			}
			nodeDisp[d.Id] = disp
			if d.Type == gen.NetworkDeviceTypeGateway {
				gatewayID = d.Id
			}
		case "client":
			cl, err := n.AsTopologyClientNode()
			if err != nil {
				return err
			}
			nodeDisp[cl.Id] = fmt.Sprintf("%s (client, %s)", cl.Name, string(cl.ConnectionType))
		}
	}

	if gatewayID == "" {
		return fmt.Errorf("no gateway node found in topology")
	}

	// Build adjacency map: parent node ID → []childEntry.
	adjacency := make(map[string][]childEntry)
	for _, e := range topo.Edges {
		disc, err := e.Discriminator()
		if err != nil {
			return err
		}
		switch disc {
		case "wired":
			we, err := e.AsTopologyWiredEdge()
			if err != nil {
				return err
			}
			srcID, err := connectionRefID(we.Source)
			if err != nil {
				return err
			}
			edgeDisp := ""
			if we.Port != nil && we.LinkSpeed != nil {
				edgeDisp = fmt.Sprintf("[port %d, %s]", *we.Port, output.FormatLinkSpeed(string(*we.LinkSpeed)))
			} else if we.Port != nil {
				edgeDisp = fmt.Sprintf("[port %d]", *we.Port)
			}
			adjacency[we.Target.Id] = append(adjacency[we.Target.Id], childEntry{
				nodeID:   srcID,
				nodeDisp: nodeDisp[srcID],
				edgeDisp: edgeDisp,
			})
		case "wireless":
			if !includeWireless {
				continue
			}
			wire, err := e.AsTopologyWirelessEdge()
			if err != nil {
				return err
			}
			edgeDisp := fmt.Sprintf("(%s)", wire.Ssid)
			if wire.SignalStrength != nil {
				edgeDisp = fmt.Sprintf("(%s, %d dBm)", wire.Ssid, *wire.SignalStrength)
			}
			adjacency[wire.Target.Id] = append(adjacency[wire.Target.Id], childEntry{
				nodeID:   wire.Source.Id,
				nodeDisp: nodeDisp[wire.Source.Id],
				edgeDisp: edgeDisp,
			})
		}
	}

	// Print the tree rooted at the gateway.
	fmt.Fprintln(w, nodeDisp[gatewayID])
	children := adjacency[gatewayID]
	for i, child := range children {
		printTopologyNode(w, child, adjacency, "", i == len(children)-1)
	}
	return nil
}

func connectionRefID(ref gen.NetworkConnectionRef) (string, error) {
	disc, err := ref.Discriminator()
	if err != nil {
		return "", err
	}
	switch disc {
	case "device":
		r, err := ref.AsNetworkDeviceRef()
		if err != nil {
			return "", err
		}
		return r.Id, nil
	case "client":
		r, err := ref.AsNetworkClientRef()
		if err != nil {
			return "", err
		}
		return r.Id, nil
	default:
		return "", fmt.Errorf("unknown connection ref kind: %s", disc)
	}
}

func printTopologyNode(w io.Writer, entry childEntry, adjacency map[string][]childEntry, prefix string, isLast bool) {
	connector := "├── "
	childPrefix := "│   "
	if isLast {
		connector = "└── "
		childPrefix = "    "
	}

	line := entry.nodeDisp
	if entry.edgeDisp != "" {
		line += " " + entry.edgeDisp
	}
	fmt.Fprintln(w, prefix+connector+line)

	children := adjacency[entry.nodeID]
	for i, child := range children {
		printTopologyNode(w, child, adjacency, prefix+childPrefix, i == len(children)-1)
	}
}
