package network

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	gen "github.com/bwilczynski/hlctl/internal/network"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	devicesListView = cmdutil.View{Templates: networkTemplates, Name: "devices_list.tmpl"}
	clientsListView = cmdutil.View{Templates: networkTemplates, Name: "clients_list.tmpl"}
)

var clientGetView = cmdutil.PolymorphicView[gen.NetworkClientDetail]{
	Templates: networkTemplates,
	Variants: map[string]cmdutil.Variant[gen.NetworkClientDetail]{
		"wired": {
			Template: "clients_get_wired.tmpl",
			Resolve:  func(d gen.NetworkClientDetail) (any, error) { return d.AsWiredNetworkClientDetail() },
		},
		"wireless": {
			Template: "clients_get_wireless.tmpl",
			Resolve:  func(d gen.NetworkClientDetail) (any, error) { return d.AsWirelessNetworkClientDetail() },
		},
	},
}

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

// buildSwitchPortViews filters and decorates a switch's ports for display.
// When allPorts is false, only ports with state "up" are returned. Each
// remaining port's ConnectedTo field is resolved to a display name (device or
// client name, or "-" when nothing is connected).
func buildSwitchPortViews(ports []gen.SwitchPort, allPorts bool) ([]switchPortView, error) {
	var out []switchPortView
	for _, p := range ports {
		if !allPorts && p.State != gen.NetworkPortStateUp {
			continue
		}
		connectedTo := "-"
		if p.ConnectedTo != nil {
			kind, err := p.ConnectedTo.Discriminator()
			if err != nil {
				return nil, err
			}
			switch kind {
			case "device":
				ref, err := p.ConnectedTo.AsNetworkDeviceRef()
				if err != nil {
					return nil, err
				}
				connectedTo = ref.Name
			case "client":
				ref, err := p.ConnectedTo.AsNetworkClientRef()
				if err != nil {
					return nil, err
				}
				connectedTo = ref.Name
			}
		}
		out = append(out, switchPortView{SwitchPort: p, ConnectedToName: connectedTo})
	}
	return out, nil
}

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Network devices and clients",
	}
	// Needed for `topology`, which is a direct child of root with no sub-group
	// parent. Cobra runs the closest PersistentPreRunE up the chain, so the
	// sub-group InjectClient calls below shadow this for their own leaves.
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newDevicesCmd())
	cmd.AddCommand(newClientsCmd())
	cmd.AddCommand(newTopologyCmd())
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
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newListDevicesCmd())
	cmd.AddCommand(newGetDeviceCmd())
	return cmd
}

func newListDevicesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List network devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[NetworkClient](cmd).ListNetworkDevicesWithResponse(cmd.Context())
			if err != nil {
				return err
			}
			return devicesListView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}

func newGetDeviceCmd() *cobra.Command {
	var allPorts bool
	cmd := &cobra.Command{
		Use:   "get <device-id>",
		Short: "Show network device details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Inline so the "switch" Resolve closure can capture the --all-ports flag.
			view := cmdutil.PolymorphicView[gen.NetworkDeviceDetail]{
				Templates: networkTemplates,
				Variants: map[string]cmdutil.Variant[gen.NetworkDeviceDetail]{
					"switch": {
						Template: "devices_get_switch.tmpl",
						Resolve: func(d gen.NetworkDeviceDetail) (any, error) {
							sw, err := d.AsSwitchDetail()
							if err != nil {
								return nil, err
							}
							portViews, err := buildSwitchPortViews(sw.Ports, allPorts)
							if err != nil {
								return nil, err
							}
							return switchDetailView{SwitchDetail: sw, Ports: portViews}, nil
						},
					},
					"accessPoint": {
						Template: "devices_get_accesspoint.tmpl",
						Resolve:  func(d gen.NetworkDeviceDetail) (any, error) { return d.AsAccessPointDetail() },
					},
					"gateway": {
						Template: "devices_get_gateway.tmpl",
						Resolve:  func(d gen.NetworkDeviceDetail) (any, error) { return d.AsGatewayDetail() },
					},
					"unknown": {
						Template: "devices_get_unknown.tmpl",
						Resolve:  func(d gen.NetworkDeviceDetail) (any, error) { return d.AsUnknownDeviceDetail() },
					},
				},
			}

			resp, err := cmdutil.Client[NetworkClient](cmd).GetNetworkDeviceWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return view.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
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
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newListClientsCmd())
	cmd.AddCommand(newGetClientCmd())
	return cmd
}

func newListClientsCmd() *cobra.Command {
	var statusFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List network clients",
	}
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
		params := &gen.ListNetworkClientsParams{}
		if statusFilter != "" {
			s := gen.NetworkClientStatus(statusFilter)
			params.Status = &s
		}

		resp, err := cmdutil.Client[NetworkClient](cmd).ListNetworkClientsWithResponse(ctx, params)
		if err != nil {
			return err
		}
		return clientsListView.Render(w, resp.StatusCode(), resp.Body, resp.JSON200)
	})
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (online|offline)")
	watch.RegisterFlags(cmd)
	return cmd
}

func newGetClientCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <client-id>",
		Short: "Show network client details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[NetworkClient](cmd).GetNetworkClientWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return clientGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
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

func newTopologyCmd() *cobra.Command {
	var includeClients bool
	var includeWireless bool

	cmd := &cobra.Command{
		Use:   "topology",
		Short: "Show network topology",
	}
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
		params := &gen.GetNetworkTopologyParams{}
		if includeClients || includeWireless {
			t := true
			params.IncludeClients = &t
		}

		resp, err := cmdutil.Client[NetworkClient](cmd).GetNetworkTopologyWithResponse(ctx, params)
		if err != nil {
			return err
		}
		if resp.StatusCode() != http.StatusOK {
			return apiclient.ParseError(resp.StatusCode(), resp.Body)
		}

		if flags.GetOutputFormat() == output.FormatJSON {
			fmt.Fprint(w, string(resp.Body))
			return nil
		}

		tree, err := buildTopologyTree(*resp.JSON200, includeWireless)
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
