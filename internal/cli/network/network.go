package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	gen "github.com/bwilczynski/hlctl/internal/network"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

// switchDetailView wraps gen.SwitchDetail with pre-resolved port data.
// The Ports field shadows gen.SwitchDetail.Ports so the template always
// sees []switchPortView, with ConnectedToName already resolved.
type switchDetailView struct {
	gen.SwitchDetail
	Ports []switchPortView
}

type switchPortView struct {
	gen.SwitchPort
	ConnectedToName string
}

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Network devices and clients",
	}
	cmd.AddCommand(newDevicesCmd())
	cmd.AddCommand(newClientsCmd())
	cmd.AddCommand(newTopologyCmd(nil))
	cmd.AddCommand(newVlansCmd())
	cmd.AddCommand(newSsidsCmd())
	cmd.AddCommand(newWansCmd())
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
				return apiclient.ParseErrorResponse(resp)
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

			return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_list.tmpl", list)
		},
	}
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
				return apiclient.ParseErrorResponse(resp)
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

			switch disc {
			case "switch":
				d, err := detail.AsSwitchDetail()
				if err != nil {
					return err
				}
				var portViews []switchPortView
				for _, p := range d.Ports {
					if !allPorts && p.State != gen.NetworkPortStateUp {
						continue
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
					portViews = append(portViews, switchPortView{SwitchPort: p, ConnectedToName: connectedTo})
				}
				return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_get_switch.tmpl",
					switchDetailView{SwitchDetail: d, Ports: portViews})

			case "accessPoint":
				d, err := detail.AsAccessPointDetail()
				if err != nil {
					return err
				}
				return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_get_accesspoint.tmpl", d)

			case "gateway":
				d, err := detail.AsGatewayDetail()
				if err != nil {
					return err
				}
				return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_get_gateway.tmpl", d)

			case "unknown":
				d, err := detail.AsUnknownDeviceDetail()
				if err != nil {
					return err
				}
				return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_get_unknown.tmpl", d)

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
	}
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
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

		resp, err := c.ListNetworkClients(ctx, params)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return apiclient.ParseErrorResponse(resp)
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
			fmt.Fprint(w, string(body))
			return nil
		}

		return output.RenderTemplate(w, networkTemplates, "clients_list.tmpl", list)
	})
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (online|offline)")
	watch.RegisterFlags(cmd)
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
				return apiclient.ParseErrorResponse(resp)
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

			switch disc {
			case "wired":
				d, err := detail.AsWiredNetworkClientDetail()
				if err != nil {
					return err
				}
				return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "clients_get_wired.tmpl", d)

			case "wireless":
				d, err := detail.AsWirelessNetworkClientDetail()
				if err != nil {
					return err
				}
				return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "clients_get_wireless.tmpl", d)

			default:
				return fmt.Errorf("unknown connection type: %s", disc)
			}
		},
	}
}

type topologyTree struct {
	GatewayID      string
	GatewayDisplay string
	Adjacency      map[string][]topologyEdge
}

type topologyEdge struct {
	ID       string
	Display  string
	EdgeDisp string
}

func newTopologyCmd(client NetworkClient) *cobra.Command {
	var includeClients bool
	var includeWireless bool

	cmd := &cobra.Command{
		Use:   "topology",
		Short: "Show network topology",
	}
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
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

		resp, err := c.GetNetworkTopology(ctx, params)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return apiclient.ParseErrorResponse(resp)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if flags.GetOutputFormat() == output.FormatJSON {
			fmt.Fprint(w, string(body))
			return nil
		}

		var topo gen.NetworkTopology
		if err := json.Unmarshal(body, &topo); err != nil {
			return err
		}
		tree, err := buildTopologyTree(topo, includeWireless)
		if err != nil {
			return err
		}
		return output.RenderTemplate(w, networkTemplates, "topology.tmpl", tree)
	})

	cmd.Flags().BoolVar(&includeClients, "include-clients", false, "Include wired clients in the topology")
	cmd.Flags().BoolVar(&includeWireless, "include-wireless", false, "Also include wireless clients (implies --include-clients)")
	watch.RegisterFlags(cmd)
	return cmd
}

func buildTopologyTree(topo gen.NetworkTopology, includeWireless bool) (topologyTree, error) {
	nodeDisp := make(map[string]string)
	var gatewayID string

	for _, n := range topo.Nodes {
		disc, err := n.Discriminator()
		if err != nil {
			return topologyTree{}, err
		}
		switch disc {
		case "device":
			d, err := n.AsTopologyDeviceNode()
			if err != nil {
				return topologyTree{}, err
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
				return topologyTree{}, err
			}
			nodeDisp[cl.Id] = fmt.Sprintf("%s (client, %s, %s)", cl.Name, string(cl.ConnectionType), string(cl.Status))
		}
	}

	if gatewayID == "" {
		return topologyTree{}, fmt.Errorf("no gateway node found in topology")
	}

	adjacency := make(map[string][]topologyEdge)

	for _, e := range topo.Edges {
		disc, err := e.Discriminator()
		if err != nil {
			return topologyTree{}, err
		}
		switch disc {
		case "wired":
			we, err := e.AsTopologyWiredEdge()
			if err != nil {
				return topologyTree{}, err
			}
			srcID, err := connectionRefID(we.Source)
			if err != nil {
				return topologyTree{}, err
			}
			edgeDisp := ""
			if we.Port != nil && we.LinkSpeed != nil {
				edgeDisp = fmt.Sprintf("[port %d, %s]", *we.Port, output.FormatLinkSpeed(string(*we.LinkSpeed)))
			} else if we.Port != nil {
				edgeDisp = fmt.Sprintf("[port %d]", *we.Port)
			}
			adjacency[we.Target.Id] = append(adjacency[we.Target.Id], topologyEdge{
				ID:       srcID,
				Display:  nodeDisp[srcID],
				EdgeDisp: edgeDisp,
			})
		case "wireless":
			if !includeWireless {
				continue
			}
			wire, err := e.AsTopologyWirelessEdge()
			if err != nil {
				return topologyTree{}, err
			}
			edgeDisp := fmt.Sprintf("[%s]", wire.Ssid)
			if wire.SignalStrength != nil {
				edgeDisp = fmt.Sprintf("[%s, %d dBm]", wire.Ssid, *wire.SignalStrength)
			}
			adjacency[wire.Target.Id] = append(adjacency[wire.Target.Id], topologyEdge{
				ID:       wire.Source.Id,
				Display:  nodeDisp[wire.Source.Id],
				EdgeDisp: edgeDisp,
			})
		}
	}

	return topologyTree{
		GatewayID:      gatewayID,
		GatewayDisplay: nodeDisp[gatewayID],
		Adjacency:      adjacency,
	}, nil
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

