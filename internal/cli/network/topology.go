package network

import (
	"context"
	"fmt"
	"io"
	"net/http"

	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
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

type getTopologyOptions struct {
	HTTPClient      func() (*http.Client, string, error)
	IO              *cmdutil.IOStreams
	Output          func() output.Format
	IncludeClients  bool
	IncludeWireless bool
}

func newTopologyCmd(f *cmdutil.Factory, runF func(*getTopologyOptions) error) *cobra.Command {
	opts := &getTopologyOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "topology",
		Short: "Show network topology",
	}
	cmd.RunE = watch.Wrap(f.Output, func(ctx context.Context, w io.Writer) error {
		if runF != nil {
			return runF(opts)
		}
		return getTopologyRun(ctx, w, opts)
	})
	cmd.Flags().BoolVar(&opts.IncludeClients, "include-clients", false, "Include wired clients in the topology")
	cmd.Flags().BoolVar(&opts.IncludeWireless, "include-wireless", false, "Also include wireless clients (implies --include-clients)")
	watch.RegisterFlags(cmd)
	return cmd
}

func getTopologyRun(ctx context.Context, w io.Writer, opts *getTopologyOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &networkapi.GetNetworkTopologyParams{}
	if opts.IncludeClients || opts.IncludeWireless {
		t := true
		params.IncludeClients = &t
	}
	resp, err := c.GetNetworkTopologyWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return topologyView.RenderWith(w, opts.Output(), resp.StatusCode(), resp.Body, func() (any, error) {
		return buildTopologyTree(*resp.JSON200, opts.IncludeWireless)
	})
}

func buildTopologyTree(topo networkapi.NetworkTopology, includeWireless bool) (topologyTree, error) {
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
			if d.Type == networkapi.NetworkDeviceTypeGateway {
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

func connectionRefID(ref networkapi.NetworkConnectionRef) (string, error) {
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
