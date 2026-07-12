package network

import (
	"context"
	"io"
	"net/http"

	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var devicesListView = cmdutil.View{Templates: networkTemplates, Name: "devices_list.tmpl"}

// switchDetailView wraps networkapi.SwitchDetail with pre-resolved port data.
// The Ports field shadows networkapi.SwitchDetail.Ports so the template always
// sees []switchPortView, with ConnectedToName already resolved.
type switchDetailView struct {
	networkapi.SwitchDetail
	Ports []switchPortView
}

type gatewayDetailView struct {
	networkapi.GatewayDetail
	Ports []switchPortView
}

type switchPortView struct {
	networkapi.DevicePort
	ConnectedToName string
	Label           string
	LinkUptime      string
	LagInfo         string
	SfpPresent      string
	PoeMode         string
	VlanMode        string
	NativeVlan      string
	TaggedVlans     string
}

// buildDevicePortViews filters and decorates a device's ports for display.
// When allPorts is false, only ports with state "up" are returned. Delegates
// per-port decoration to decoratePort so devices get and ports list share
// the same rendering rules.
func buildDevicePortViews(ports []networkapi.DevicePort, allPorts bool) ([]switchPortView, error) {
	var out []switchPortView
	for _, p := range ports {
		if !allPorts && p.State != networkapi.NetworkPortStateUp {
			continue
		}
		row, err := decoratePort(portInput{DeviceName: "", DevicePort: p})
		if err != nil {
			return nil, err
		}
		out = append(out, switchPortView{
			DevicePort:      p,
			ConnectedToName: row.ConnectedToName,
			Label:           row.Label,
			LinkUptime:      row.LinkUptime,
			LagInfo:         row.LagInfo,
			SfpPresent:      row.SfpPresent,
			PoeMode:         row.PoeMode,
			VlanMode:        row.VlanMode,
			NativeVlan:      row.NativeVlan,
			TaggedVlans:     row.TaggedVlans,
		})
	}
	return out, nil
}

type listDevicesOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
}

type getDeviceOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
	AllPorts   bool
}

func newDevicesCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Network devices",
	}
	cmd.AddCommand(newListDevicesCmd(f, nil), newGetDeviceCmd(f, nil))
	return cmd
}

func newListDevicesCmd(f *cmdutil.Factory, runF func(*listDevicesOptions) error) *cobra.Command {
	opts := &listDevicesOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "list",
		Short: "List network devices",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listDevicesRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func listDevicesRun(ctx context.Context, w io.Writer, opts *listDevicesOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.ListNetworkDevicesWithResponse(ctx)
	if err != nil {
		return err
	}
	return devicesListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

func newGetDeviceCmd(f *cmdutil.Factory, runF func(*getDeviceOptions) error) *cobra.Command {
	opts := &getDeviceOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "get <device-id>",
		Short: "Show network device details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getDeviceRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.AllPorts, "all-ports", false, "Show all ports (default: active ports only)")
	return cmd
}

func getDeviceRun(ctx context.Context, w io.Writer, opts *getDeviceOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	// PolymorphicView constructed inline so Resolve closes over opts.AllPorts
	view := cmdutil.PolymorphicView[networkapi.NetworkDeviceDetail]{
		Templates: networkTemplates,
		Variants: map[string]cmdutil.Variant[networkapi.NetworkDeviceDetail]{
			"switch": {
				Template: "devices_get_switch.tmpl",
				Resolve: func(d networkapi.NetworkDeviceDetail) (any, error) {
					sw, err := d.AsSwitchDetail()
					if err != nil {
						return nil, err
					}
					portViews, err := buildDevicePortViews(sw.Ports, opts.AllPorts)
					if err != nil {
						return nil, err
					}
					return switchDetailView{SwitchDetail: sw, Ports: portViews}, nil
				},
			},
			"accessPoint": {
				Template: "devices_get_accesspoint.tmpl",
				Resolve:  func(d networkapi.NetworkDeviceDetail) (any, error) { return d.AsAccessPointDetail() },
			},
			"gateway": {
				Template: "devices_get_gateway.tmpl",
				Resolve: func(d networkapi.NetworkDeviceDetail) (any, error) {
					gw, err := d.AsGatewayDetail()
					if err != nil {
						return nil, err
					}
					portViews, err := buildDevicePortViews(gw.Ports, opts.AllPorts)
					if err != nil {
						return nil, err
					}
					return gatewayDetailView{GatewayDetail: gw, Ports: portViews}, nil
				},
			},
			"unknown": {
				Template: "devices_get_unknown.tmpl",
				Resolve:  func(d networkapi.NetworkDeviceDetail) (any, error) { return d.AsUnknownDeviceDetail() },
			},
		},
	}
	resp, err := c.GetNetworkDeviceWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return view.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
