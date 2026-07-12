package network

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var portsListView = cmdutil.View{Templates: networkTemplates, Name: "ports_list.tmpl"}

type listPortsOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format

	Device   string
	Mode     string
	State    string
	VlanID   int
	Wide     bool
	AllPorts bool
}

func newPortsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ports",
		Short: "Switch ports",
	}
	cmd.AddCommand(newListPortsCmd(f, nil))
	return cmd
}

func newListPortsCmd(f *cmdutil.Factory, runF func(*listPortsOptions) error) *cobra.Command {
	opts := &listPortsOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List switch ports across all switches",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateEnum("mode", opts.Mode, "trunk", "access"); err != nil {
				return err
			}
			if err := validateEnum("state", opts.State, "up", "down", "disabled"); err != nil {
				return err
			}
			if runF != nil {
				return runF(opts)
			}
			return listPortsRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID (switch or gateway)")
	cmd.Flags().StringVar(&opts.Mode, "mode", "", "Filter by VLAN mode (trunk|access)")
	cmd.Flags().StringVar(&opts.State, "state", "", "Filter by port state (up|down|disabled)")
	cmd.Flags().IntVar(&opts.VlanID, "vlan", 0, "Filter to ports carrying this VLAN ID")
	cmd.Flags().BoolVar(&opts.Wide, "wide", false, "Show additional columns (link speed, PoE, LAG, SFP, traffic)")
	cmd.Flags().BoolVar(&opts.AllPorts, "all-ports", false, "Show all ports (overrides default state=up filter)")
	cmd.MarkFlagsMutuallyExclusive("all-ports", "state")
	return cmd
}

func validateEnum(name, value string, allowed ...string) error {
	if value == "" {
		return nil
	}
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("invalid value for --%s: %q (allowed: %v)", name, value, allowed)
}

// portRow is the display-ready shape rendered by the ports_list template.
// It intentionally holds only string/scalar fields so the template stays
// free of Go type conversions.
type portRow struct {
	DeviceName      string
	Number          int
	Label           string
	State           string
	LinkSpeed       string
	VlanMode        string
	NativeVlan      string
	TaggedVlans     string
	PoeMode         string
	PoePowerWatts   string
	LagInfo         string
	SfpPresent      string
	LinkUptime      string
	RxPerSec        string
	TxPerSec        string
	ConnectedToName string
}

// portInput is the shared decoration input. Both NetworkPort (flat list
// response) and DevicePort (nested in device detail) can be adapted to
// this shape; the decoration logic is centralized in decoratePort.
type portInput struct {
	DeviceName string
	DevicePort networkapi.DevicePort
}

func decoratePort(in portInput) (portRow, error) {
	p := in.DevicePort

	connectedTo := "-"
	if p.ConnectedTo != nil {
		kind, err := p.ConnectedTo.Discriminator()
		if err != nil {
			return portRow{}, err
		}
		switch kind {
		case "device":
			ref, err := p.ConnectedTo.AsNetworkDeviceRef()
			if err != nil {
				return portRow{}, err
			}
			connectedTo = ref.Name
		case "client":
			ref, err := p.ConnectedTo.AsNetworkClientRef()
			if err != nil {
				return portRow{}, err
			}
			connectedTo = ref.Name
		}
	}

	label := "-"
	if p.Label != nil {
		label = *p.Label
	}

	linkUptime := "-"
	if p.LinkUptime != nil {
		linkUptime = output.FormatUptime(*p.LinkUptime)
	}

	linkSpeed := "-"
	if p.State == networkapi.NetworkPortStateUp && p.LinkSpeed != nil {
		linkSpeed = output.FormatLinkSpeed(string(*p.LinkSpeed))
	}

	lagInfo := "-"
	if p.LagMembership != nil {
		lagInfo = fmt.Sprintf("%s #%d", p.LagMembership.Role, p.LagMembership.Id)
	}

	sfpPresent := "-"
	if p.SfpModulePresent != nil {
		if *p.SfpModulePresent {
			sfpPresent = "yes"
		} else {
			sfpPresent = "no"
		}
	}

	poeMode := "-"
	if p.PoeMode != nil {
		poeMode = string(*p.PoeMode)
	}

	poePowerWatts := "-"
	if p.PoePowerWatts != nil {
		poePowerWatts = fmt.Sprintf("%.1f W", *p.PoePowerWatts)
	}

	vlanMode := "-"
	nativeVlan := "-"
	taggedVlans := "-"
	if p.VlanConfig != nil {
		vlanMode = string(p.VlanConfig.Mode)
		nativeVlan = fmt.Sprintf("%s (%d)", p.VlanConfig.NativeVlan.Name, p.VlanConfig.NativeVlan.VlanId)
		if p.VlanConfig.Mode == networkapi.DevicePortVlanModeTrunk && p.VlanConfig.TaggedVlans != nil {
			switch p.VlanConfig.TaggedVlans.Scope {
			case networkapi.DevicePortVlanConfigTaggedVlansScopeAll:
				taggedVlans = "all"
			case networkapi.DevicePortVlanConfigTaggedVlansScopeCustom:
				if p.VlanConfig.TaggedVlans.Items != nil && len(*p.VlanConfig.TaggedVlans.Items) > 0 {
					parts := make([]string, 0, len(*p.VlanConfig.TaggedVlans.Items))
					for _, v := range *p.VlanConfig.TaggedVlans.Items {
						parts = append(parts, fmt.Sprintf("%s (%d)", v.Name, v.VlanId))
					}
					taggedVlans = strings.Join(parts, ", ")
				}
			}
		}
	}

	return portRow{
		DeviceName:      in.DeviceName,
		Number:          p.Number,
		Label:           label,
		State:           string(p.State),
		LinkSpeed:       linkSpeed,
		VlanMode:        vlanMode,
		NativeVlan:      nativeVlan,
		TaggedVlans:     taggedVlans,
		PoeMode:         poeMode,
		PoePowerWatts:   poePowerWatts,
		LagInfo:         lagInfo,
		SfpPresent:      sfpPresent,
		LinkUptime:      linkUptime,
		RxPerSec:        output.FormatBytesPerSec(p.Traffic.RxBytesPerSec),
		TxPerSec:        output.FormatBytesPerSec(p.Traffic.TxBytesPerSec),
		ConnectedToName: connectedTo,
	}, nil
}

type portsListData struct {
	Wide bool
	Rows []portRow
}

func listPortsRun(ctx context.Context, w io.Writer, opts *listPortsOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := buildListPortsParams(opts)
	resp, err := c.ListNetworkPortsWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return portsListView.RenderWith(w, opts.Output(), resp.StatusCode(), resp.Body, func() (any, error) {
		if resp.JSON200 == nil {
			return nil, fmt.Errorf("empty response body")
		}
		rows := make([]portRow, 0, len(resp.JSON200.Items))
		for _, p := range resp.JSON200.Items {
			row, err := decoratePort(portInput{
				DeviceName: p.Device.Name,
				DevicePort: networkapi.DevicePort{
					Number:           p.Number,
					Label:            p.Label,
					State:            p.State,
					LinkSpeed:        p.LinkSpeed,
					LinkUptime:       p.LinkUptime,
					SfpModulePresent: p.SfpModulePresent,
					PoeMode:          p.PoeMode,
					PoePowerWatts:    p.PoePowerWatts,
					LagMembership:    p.LagMembership,
					VlanConfig:       p.VlanConfig,
					Traffic:          p.Traffic,
					ConnectedTo:      p.ConnectedTo,
				},
			})
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].DeviceName != rows[j].DeviceName {
				return rows[i].DeviceName < rows[j].DeviceName
			}
			return rows[i].Number < rows[j].Number
		})
		return portsListData{Wide: opts.Wide, Rows: rows}, nil
	})
}

func buildListPortsParams(opts *listPortsOptions) *networkapi.ListNetworkPortsParams {
	params := &networkapi.ListNetworkPortsParams{}
	if opts.Device != "" {
		s := opts.Device
		params.DeviceId = &s
	}
	if opts.Mode != "" {
		m := networkapi.DevicePortVlanMode(opts.Mode)
		params.Mode = &m
	}
	state := opts.State
	if !opts.AllPorts && state == "" {
		state = "up"
	}
	if !opts.AllPorts && state != "" {
		s := networkapi.NetworkPortState(state)
		params.State = &s
	}
	if opts.VlanID > 0 {
		v := opts.VlanID
		params.VlanId = &v
	}
	return params
}
