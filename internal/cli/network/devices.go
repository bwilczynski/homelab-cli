package network

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
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

type switchPortView struct {
	networkapi.SwitchPort
	ConnectedToName string
}

// buildSwitchPortViews filters and decorates a switch's ports for display.
// When allPorts is false, only ports with state "up" are returned. Each
// remaining port's ConnectedTo field is resolved to a display name (device or
// client name, or "-" when nothing is connected).
func buildSwitchPortViews(ports []networkapi.SwitchPort, allPorts bool) ([]switchPortView, error) {
	var out []switchPortView
	for _, p := range ports {
		if !allPorts && p.State != networkapi.NetworkPortStateUp {
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

func newDevicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Network devices",
	}
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
							portViews, err := buildSwitchPortViews(sw.Ports, allPorts)
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
						Resolve:  func(d networkapi.NetworkDeviceDetail) (any, error) { return d.AsGatewayDetail() },
					},
					"unknown": {
						Template: "devices_get_unknown.tmpl",
						Resolve:  func(d networkapi.NetworkDeviceDetail) (any, error) { return d.AsUnknownDeviceDetail() },
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
