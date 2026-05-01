package network

import (
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Network devices and clients",
	}

	cmd.AddCommand(newDevicesCmd())
	cmd.AddCommand(newDeviceCmd())
	cmd.AddCommand(newClientsCmd())
	cmd.AddCommand(newClientCmd())
	return cmd
}

func newDevicesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "devices",
		Short: "List network devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := []map[string]string{
				{"id": "unifi.ap-living-room", "model": "U6-Pro", "status": "connected", "clients": "12"},
				{"id": "unifi.usw-24", "model": "USW-24-PoE", "status": "connected", "clients": "18"},
			}
			headers := []string{"ID", "MODEL", "STATUS", "CLIENTS"}
			var rows [][]string
			for _, d := range data {
				rows = append(rows, []string{d["id"], d["model"], d["status"], d["clients"]})
			}
			return output.Print(flags.GetOutputFormat(), data, headers, rows)
		},
	}
}

func newDeviceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "device <device-id>",
		Short: "Show network device details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data := map[string]any{
				"id":       args[0],
				"model":    "U6-Pro",
				"firmware": "7.0.83",
				"status":   "connected",
				"uptime":   "30d 5h",
			}
			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", args[0]},
				{"Model", "U6-Pro"},
				{"Firmware", "7.0.83"},
				{"Status", "connected"},
				{"Uptime", "30d 5h"},
			}
			return output.Print(flags.GetOutputFormat(), data, headers, rows)
		},
	}
}

func newClientsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clients",
		Short: "List connected network clients",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := []map[string]string{
				{"id": "unifi.macbook-pro", "hostname": "macbook-pro", "ip": "192.168.1.100", "connection": "wireless"},
				{"id": "unifi.nas-1", "hostname": "nas-1", "ip": "192.168.1.10", "connection": "wired"},
			}
			headers := []string{"ID", "HOSTNAME", "IP", "CONNECTION"}
			var rows [][]string
			for _, d := range data {
				rows = append(rows, []string{d["id"], d["hostname"], d["ip"], d["connection"]})
			}
			return output.Print(flags.GetOutputFormat(), data, headers, rows)
		},
	}
}

func newClientCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "client <client-id>",
		Short: "Show network client details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data := map[string]any{
				"id":         args[0],
				"hostname":   "macbook-pro",
				"ip":         "192.168.1.100",
				"connection": "wireless",
				"ssid":       "HomeNet",
				"signal":     "-42 dBm",
			}
			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", args[0]},
				{"Hostname", "macbook-pro"},
				{"IP", "192.168.1.100"},
				{"Connection", "wireless"},
				{"SSID", "HomeNet"},
				{"Signal", "-42 dBm"},
			}
			return output.Print(flags.GetOutputFormat(), data, headers, rows)
		},
	}
}
