package network

import (
	"context"
	"fmt"
	"io"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	gen "github.com/bwilczynski/hlctl/internal/network"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var topologyView = cmdutil.View{Templates: networkTemplates, Name: "topology.tmpl"}

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
		return topologyView.RenderWith(w, resp.StatusCode(), resp.Body, func() (any, error) {
			return buildTopologyTree(*resp.JSON200, includeWireless)
		})
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
