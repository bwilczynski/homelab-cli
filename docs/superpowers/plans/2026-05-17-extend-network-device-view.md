# Extend Network Device/Client View Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update `hlctl network devices` and `hlctl network clients` commands to expose the richer fields introduced by spec PR #11 — polymorphic device detail, per-device traffic stats, switch port lists, AP client lists, and restructured wired/wireless client connection info.

**Architecture:** Regenerate the API client from the updated spec submodule, add two output helpers (`FormatBytesPerSec`, `FormatLinkSpeed`), then update the four command functions (`newListDevicesCmd`, `newGetDeviceCmd`, `newGetClientCmd`) and their tests. The `get device` command dispatches on the `type` discriminator and renders type-specific sections (`--- PORTS ---`, `--- CLIENTS ---`) in table mode after the shared base rows.

**Tech Stack:** Go, Cobra, oapi-codegen, `internal/output` table helpers.

---

## File Map

| File | Change |
|------|--------|
| `internal/network/api.gen.go` | Regenerated — new polymorphic types |
| `internal/output/output.go` | Add `FormatBytesPerSec`, `FormatLinkSpeed` |
| `internal/output/output_test.go` | Add tests for both new helpers |
| `internal/cli/network/network.go` | Update all four command functions |
| `internal/cli/network/network_test.go` | Update/add tests for all changes |

`internal/cli/network/stub.go` and `internal/cli/network/client.go` need no changes.

---

### Task 1: Regenerate API client and fix compilation

**Files:**
- Modify: `internal/network/api.gen.go` (generated, do not edit by hand)

- [ ] **Step 1: Update spec submodule and regenerate**

```bash
cd /path/to/homelab-cli
git submodule update --remote spec
make generate
```

Expected: `internal/network/api.gen.go` is rewritten with new types including `SwitchDetail`, `AccessPointDetail`, `GatewayDetail`, `UnknownDeviceDetail`, `NetworkTraffic`, `SwitchPort`, `AccessPointClient`, `NetworkConnection`, `WirelessConnection`, `NetworkDeviceRef`, `NetworkClientRef`, `NetworkConnectionRef`, `NetworkLinkSpeed`, `NetworkPortState`, `SwitchPortPoeMode`.

Note: `NetworkDeviceDetail` is now a union (anyOf) with `Discriminator()` and `AsSwitchDetail()` / `AsAccessPointDetail()` / `AsGatewayDetail()` / `AsUnknownDeviceDetail()` methods, similar to the existing `NetworkClientDetail` union. `NetworkDevice` loses `NumClients` and gains `Uri`.

- [ ] **Step 2: Verify the build fails (expected — CLI uses removed fields)**

```bash
make build
```

Expected: compile errors referencing `NumClients` on `NetworkDevice` and the flat shape of `NetworkDeviceDetail`. Note every error location — these are the exact lines to update in subsequent tasks.

- [ ] **Step 3: Commit the regenerated file**

```bash
git add internal/network/api.gen.go spec
git commit -m "chore: regenerate network API client from updated spec"
```

---

### Task 2: Add output helpers

**Files:**
- Modify: `internal/output/output.go`
- Modify: `internal/output/output_test.go`

- [ ] **Step 1: Write failing tests for `FormatBytesPerSec`**

Add to `internal/output/output_test.go` inside the existing `package output_test`:

```go
func TestFormatBytesPerSec(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B/s"},
		{500, "500 B/s"},
		{1024, "1.0 KB/s"},
		{125000, "122.1 KB/s"},
		{1048576, "1.0 MB/s"},
		{1073741824, "1.0 GB/s"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := output.FormatBytesPerSec(tt.input)
			if got != tt.expected {
				t.Errorf("FormatBytesPerSec(%d) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Write failing test for `FormatLinkSpeed`**

Append to `internal/output/output_test.go`:

```go
func TestFormatLinkSpeed(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"e", "10M"},
		{"fe", "100M"},
		{"gbe1", "1GbE"},
		{"gbe2_5", "2.5GbE"},
		{"gbe5", "5GbE"},
		{"gbe10", "10GbE"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := output.FormatLinkSpeed(tt.input)
			if got != tt.expected {
				t.Errorf("FormatLinkSpeed(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 3: Run tests to confirm they fail**

```bash
go test ./internal/output/... -run "TestFormatBytesPerSec|TestFormatLinkSpeed" -v
```

Expected: FAIL — functions not defined.

- [ ] **Step 4: Implement `FormatBytesPerSec`**

Add to `internal/output/output.go` after the `FormatBytes` function:

```go
// FormatBytesPerSec formats a bytes-per-second throughput using binary units.
func FormatBytesPerSec(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B/s", n)
	}
	div, exp := int64(unit), 0
	for n := n / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB/s", "MB/s", "GB/s", "TB/s", "PB/s"}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), units[exp])
}
```

- [ ] **Step 5: Implement `FormatLinkSpeed`**

Add to `internal/output/output.go` after `FormatBytesPerSec`:

```go
// FormatLinkSpeed maps a NetworkLinkSpeed enum value to a human-readable string.
func FormatLinkSpeed(s string) string {
	switch s {
	case "e":
		return "10M"
	case "fe":
		return "100M"
	case "gbe1":
		return "1GbE"
	case "gbe2_5":
		return "2.5GbE"
	case "gbe5":
		return "5GbE"
	case "gbe10":
		return "10GbE"
	default:
		return s
	}
}
```

- [ ] **Step 6: Run tests and confirm they pass**

```bash
go test ./internal/output/... -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/output/output.go internal/output/output_test.go
git commit -m "feat: add FormatBytesPerSec and FormatLinkSpeed output helpers"
```

---

### Task 3: Update `newListDevicesCmd` — remove CLIENTS column

**Files:**
- Modify: `internal/cli/network/network.go`
- Modify: `internal/cli/network/network_test.go`

- [ ] **Step 1: Update the existing list devices test**

In `internal/cli/network/network_test.go`, replace `TestListDevicesCmd_tableOutput`:

```go
func TestListDevicesCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListNetworkDevicesFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkDeviceList{
				Items: []gen.NetworkDevice{
					{
						Id:     "unifi.usg",
						Uri:    "/network/devices/unifi.usg",
						Name:   "USG",
						Mac:    "aa:bb:cc:dd:00:01",
						Ip:     "192.168.1.1",
						Type:   gen.Gateway,
						Status: gen.Connected,
					},
					{
						Id:     "unifi.ap-living-room",
						Uri:    "/network/devices/unifi.ap-living-room",
						Name:   "AP Living Room",
						Mac:    "aa:bb:cc:dd:00:03",
						Ip:     "192.168.1.3",
						Type:   gen.AccessPoint,
						Status: gen.Connected,
					},
				},
			}), nil
		},
	}

	cmd := newListDevicesCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"unifi.usg", "unifi.ap-living-room", "gateway", "accessPoint"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "CLIENTS") {
		t.Errorf("expected no CLIENTS column in list output, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./internal/cli/network/... -run TestListDevicesCmd_tableOutput -v
```

Expected: FAIL — `NumClients` field removed from `NetworkDevice` + assertion on no CLIENTS column.

- [ ] **Step 3: Update `newListDevicesCmd` in `network.go`**

Replace the headers and rows block inside `newListDevicesCmd`:

```go
headers := []string{"ID", "NAME", "MAC", "IP", "TYPE", "STATUS"}
var rows [][]string
for _, d := range list.Items {
    rows = append(rows, []string{
        d.Id, d.Name, d.Mac, d.Ip,
        string(d.Type), string(d.Status),
    })
}
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
```

- [ ] **Step 4: Run test to confirm it passes**

```bash
go test ./internal/cli/network/... -run TestListDevicesCmd -v
```

Expected: all list devices tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/network/network.go internal/cli/network/network_test.go
git commit -m "feat: remove CLIENTS column from network devices list"
```

---

### Task 4: Update `newGetDeviceCmd` — base fields, traffic, uplink, gateway/unknown

**Files:**
- Modify: `internal/cli/network/network.go`
- Modify: `internal/cli/network/network_test.go`

- [ ] **Step 1: Write failing test for gateway detail (base fields + traffic)**

Replace `TestGetDeviceCmd_tableOutput` and `TestGetDeviceCmd_noClientsRow` in `network_test.go` with:

```go
func TestGetDeviceCmd_gateway(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":              "unifi.usg",
				"uri":             "/network/devices/unifi.usg",
				"name":            "USG",
				"mac":             "aa:bb:cc:dd:00:01",
				"ip":              "192.168.1.1",
				"type":            "gateway",
				"status":          "connected",
				"model":           "USG-3P",
				"firmwareVersion": "4.4.57",
				"uptime":          86400,
				"traffic": map[string]any{
					"rxBytesTotal":  int64(12884901888),
					"txBytesTotal":  int64(4294967296),
					"rxBytesPerSec": int64(125000),
					"txBytesPerSec": int64(50000),
				},
			}), nil
		},
	}

	cmd := newGetDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.usg"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"unifi.usg", "USG-3P", "4.4.57", "gateway", "TRAFFIC RX", "TRAFFIC TX", "1d"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"PORTS", "CLIENTS", "UPLINK"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected %q absent for gateway, got:\n%s", absent, out)
		}
	}
}
```

- [ ] **Step 2: Write failing test for unknown device with uplink**

Append to `network_test.go`:

```go
func TestGetDeviceCmd_unknownWithUplink(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":              "unifi.mystery-device",
				"uri":             "/network/devices/unifi.mystery-device",
				"name":            "Mystery Device",
				"mac":             "aa:bb:cc:dd:00:ff",
				"ip":              "192.168.1.99",
				"type":            "unknown",
				"status":          "connected",
				"model":           "unknown-model",
				"firmwareVersion": "0.0.0",
				"uptime":          3600,
				"traffic": map[string]any{
					"rxBytesTotal":  int64(0),
					"txBytesTotal":  int64(0),
					"rxBytesPerSec": int64(0),
					"txBytesPerSec": int64(0),
				},
				"uplink": map[string]any{
					"device": map[string]any{
						"kind": "device",
						"id":   "unifi.switch-lr",
						"uri":  "/network/devices/unifi.switch-lr",
						"name": "Switch Living Room",
					},
					"port":      8,
					"linkSpeed": "gbe1",
				},
			}), nil
		},
	}

	cmd := newGetDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.mystery-device"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Mystery Device", "UPLINK", "Switch Living Room", "port 8", "1GbE"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 3: Run tests to confirm they fail**

```bash
go test ./internal/cli/network/... -run "TestGetDeviceCmd_gateway|TestGetDeviceCmd_unknownWithUplink" -v
```

Expected: FAIL.

- [ ] **Step 4: Add `deviceBaseRows` helper and rewrite `newGetDeviceCmd` in `network.go`**

Add the helper function above `newGetDeviceCmd` (not exported):

```go
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
```

Replace the body of `newGetDeviceCmd` with a version that dispatches on the discriminator. For now implement gateway and unknown only (switch and AP follow in Tasks 5–6):

```go
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
					if !allPorts && p.State != gen.Up {
						continue
					}
					speed := "-"
					if p.LinkSpeed != nil {
						speed = output.FormatLinkSpeed(string(*p.LinkSpeed))
					}
					poePower := "-"
					if p.PoePowerWatts != nil {
						poePower = fmt.Sprintf("%.1f W", *p.PoePowerWatts)
					}
					connectedTo := "-"
					if p.ConnectedTo != nil {
						kind, _ := p.ConnectedTo.Discriminator()
						switch kind {
						case "device":
							ref, _ := p.ConnectedTo.AsNetworkDeviceRef()
							connectedTo = ref.Name
						case "client":
							ref, _ := p.ConnectedTo.AsNetworkClientRef()
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

			default: // gateway, unknown
				var (
					id, name, mac, ip, typ, status string
					model, firmware                 string
					uptime                          int
					traffic                         gen.NetworkTraffic
					uplink                          *gen.NetworkConnection
				)
				switch disc {
				case "gateway":
					d, err := detail.AsGatewayDetail()
					if err != nil {
						return err
					}
					id, name, mac, ip = d.Id, d.Name, d.Mac, d.Ip
					typ, status = string(d.Type), string(d.Status)
					model, firmware, uptime = d.Model, d.FirmwareVersion, d.Uptime
					traffic, uplink = d.Traffic, d.Uplink
				case "unknown":
					d, err := detail.AsUnknownDeviceDetail()
					if err != nil {
						return err
					}
					id, name, mac, ip = d.Id, d.Name, d.Mac, d.Ip
					typ, status = string(d.Type), string(d.Status)
					model, firmware, uptime = d.Model, d.FirmwareVersion, d.Uptime
					traffic, uplink = d.Traffic, d.Uplink
				default:
					return fmt.Errorf("unknown device type: %s", disc)
				}
				rows := deviceBaseRows(id, name, mac, ip, typ, status, model, firmware, uptime, traffic, uplink)
				return output.Print(cmd.OutOrStdout(), output.FormatTable, nil, baseHeaders, rows)
			}
		},
	}
	cmd.Flags().BoolVar(&allPorts, "all-ports", false, "Show all ports (default: active ports only)")
	return cmd
}
```

Note: After `make generate`, verify the exact enum constant for the `up` port state — it is likely `gen.Up` or `gen.NetworkPortStateUp`. Check `api.gen.go` and adjust accordingly. Same for other enums (`gen.SwitchPortPoeModeOff`, etc.).

- [ ] **Step 5: Run tests to confirm gateway and unknown pass**

```bash
go test ./internal/cli/network/... -run "TestGetDeviceCmd_gateway|TestGetDeviceCmd_unknownWithUplink" -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/network/network.go internal/cli/network/network_test.go
git commit -m "feat: update get device command with polymorphic dispatch and traffic rows"
```

---

### Task 5: Add switch port section and `--all-ports` flag tests

**Files:**
- Modify: `internal/cli/network/network_test.go`

- [ ] **Step 1: Write failing test for switch with active ports (default)**

Append to `network_test.go`:

```go
func TestGetDeviceCmd_switch_activePorts(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":              "unifi.switch-lr",
				"uri":             "/network/devices/unifi.switch-lr",
				"name":            "Switch Living Room",
				"mac":             "aa:bb:cc:dd:00:10",
				"ip":              "192.168.1.10",
				"type":            "switch",
				"status":          "connected",
				"model":           "USW-24-PoE",
				"firmwareVersion": "6.2.14",
				"uptime":          86400,
				"traffic": map[string]any{
					"rxBytesTotal":  int64(12884901888),
					"txBytesTotal":  int64(4294967296),
					"rxBytesPerSec": int64(125000),
					"txBytesPerSec": int64(50000),
				},
				"ports": []map[string]any{
					{
						"number":    1,
						"state":     "up",
						"linkSpeed": "gbe1",
						"poeMode":   "auto",
						"poePowerWatts": 8.5,
						"traffic": map[string]any{
							"rxBytesTotal":  int64(0),
							"txBytesTotal":  int64(0),
							"rxBytesPerSec": int64(1200),
							"txBytesPerSec": int64(500),
						},
						"connectedTo": map[string]any{
							"kind": "device",
							"id":   "unifi.ap-living-room",
							"uri":  "/network/devices/unifi.ap-living-room",
							"name": "AP Living Room",
						},
					},
					{
						"number":  2,
						"state":   "down",
						"poeMode": "off",
						"traffic": map[string]any{
							"rxBytesTotal":  int64(0),
							"txBytesTotal":  int64(0),
							"rxBytesPerSec": int64(0),
							"txBytesPerSec": int64(0),
						},
					},
				},
			}), nil
		},
	}

	cmd := newGetDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.switch-lr"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Switch Living Room", "PORTS", "AP Living Room", "1GbE", "8.5 W", "TRAFFIC RX", "TRAFFIC TX"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	// Port 2 is down — should be hidden by default
	if strings.Contains(out, "down") {
		t.Errorf("expected down ports hidden by default, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Write failing test for `--all-ports` flag**

Append to `network_test.go`:

```go
func TestGetDeviceCmd_switch_allPorts(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":              "unifi.switch-lr",
				"uri":             "/network/devices/unifi.switch-lr",
				"name":            "Switch Living Room",
				"mac":             "aa:bb:cc:dd:00:10",
				"ip":              "192.168.1.10",
				"type":            "switch",
				"status":          "connected",
				"model":           "USW-24-PoE",
				"firmwareVersion": "6.2.14",
				"uptime":          3600,
				"traffic": map[string]any{
					"rxBytesTotal":  int64(0),
					"txBytesTotal":  int64(0),
					"rxBytesPerSec": int64(0),
					"txBytesPerSec": int64(0),
				},
				"ports": []map[string]any{
					{
						"number":  1,
						"state":   "up",
						"poeMode": "off",
						"traffic": map[string]any{
							"rxBytesTotal":  int64(0),
							"txBytesTotal":  int64(0),
							"rxBytesPerSec": int64(0),
							"txBytesPerSec": int64(0),
						},
					},
					{
						"number":  2,
						"state":   "down",
						"poeMode": "off",
						"traffic": map[string]any{
							"rxBytesTotal":  int64(0),
							"txBytesTotal":  int64(0),
							"rxBytesPerSec": int64(0),
							"txBytesPerSec": int64(0),
						},
					},
					{
						"number":   3,
						"state":    "disabled",
						"poeMode":  "off",
						"traffic": map[string]any{
							"rxBytesTotal":  int64(0),
							"txBytesTotal":  int64(0),
							"rxBytesPerSec": int64(0),
							"txBytesPerSec": int64(0),
						},
					},
				},
			}), nil
		},
	}

	cmd := newGetDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.switch-lr", "--all-ports"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"down", "disabled"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output with --all-ports, got:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 3: Run switch tests**

```bash
go test ./internal/cli/network/... -run "TestGetDeviceCmd_switch" -v
```

Expected: PASS (implementation was done in Task 4).

- [ ] **Step 4: Commit**

```bash
git add internal/cli/network/network_test.go
git commit -m "test: add switch port and --all-ports flag tests for get device command"
```

---

### Task 6: Add access point clients section test

**Files:**
- Modify: `internal/cli/network/network_test.go`

- [ ] **Step 1: Write failing test for AP with connected clients**

Append to `network_test.go`:

```go
func TestGetDeviceCmd_accessPoint(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":              "unifi.ap-living-room",
				"uri":             "/network/devices/unifi.ap-living-room",
				"name":            "AP Living Room",
				"mac":             "aa:bb:cc:dd:00:03",
				"ip":              "192.168.1.3",
				"type":            "accessPoint",
				"status":          "connected",
				"model":           "U6-Lite",
				"firmwareVersion": "6.6.77",
				"uptime":          7200,
				"numClients":      2,
				"traffic": map[string]any{
					"rxBytesTotal":  int64(1073741824),
					"txBytesTotal":  int64(536870912),
					"rxBytesPerSec": int64(50000),
					"txBytesPerSec": int64(25000),
				},
				"connectedClients": []map[string]any{
					{
						"client": map[string]any{
							"kind": "client",
							"id":   "unifi.macbook-pro-3c",
							"uri":  "/network/clients/unifi.macbook-pro-3c",
							"name": "MacBook Pro",
						},
						"ssid":           "HomeNetwork",
						"signalStrength": -62,
					},
					{
						"client": map[string]any{
							"kind": "client",
							"id":   "unifi.iphone-15-aa",
							"uri":  "/network/clients/unifi.iphone-15-aa",
							"name": "iPhone 15",
						},
						"ssid":           "HomeNetwork",
						"signalStrength": -70,
					},
				},
			}), nil
		},
	}

	cmd := newGetDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.ap-living-room"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"AP Living Room", "CLIENTS", "MacBook Pro", "iPhone 15", "HomeNetwork", "-62 dBm", "-70 dBm", "TRAFFIC RX"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "PORTS") {
		t.Errorf("expected no PORTS section for AP, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run AP test**

```bash
go test ./internal/cli/network/... -run TestGetDeviceCmd_accessPoint -v
```

Expected: PASS (implementation was done in Task 4).

- [ ] **Step 3: Commit**

```bash
git add internal/cli/network/network_test.go
git commit -m "test: add access point connected clients section test"
```

---

### Task 7: Update `newGetClientCmd` — wired client `connectedTo`

**Files:**
- Modify: `internal/cli/network/network.go`
- Modify: `internal/cli/network/network_test.go`

- [ ] **Step 1: Update the wired client test**

Replace `TestGetClientCmd_wired` in `network_test.go`:

```go
func TestGetClientCmd_wired(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:01",
				"uri":            "/network/clients/unifi.aa:bb:cc:dd:ee:01",
				"name":           "laptop",
				"mac":            "aa:bb:cc:dd:ee:01",
				"ip":             "192.168.1.50",
				"connectionType": "wired",
				"status":         "online",
				"connectedTo": map[string]any{
					"device": map[string]any{
						"kind": "device",
						"id":   "unifi.switch-lr",
						"uri":  "/network/devices/unifi.switch-lr",
						"name": "Switch Living Room",
					},
					"port":      3,
					"linkSpeed": "gbe1",
				},
				"uptime": 3600,
			}), nil
		},
	}

	cmd := newGetClientCmd(stub)
	cmd.SetArgs([]string{"unifi.aa:bb:cc:dd:ee:01"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"laptop", "Switch Living Room", "3", "1GbE", "online"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Update the offline wired client test**

Replace `TestGetClientCmd_offline_wired` in `network_test.go`:

```go
func TestGetClientCmd_offline_wired(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:03",
				"uri":            "/network/clients/unifi.aa:bb:cc:dd:ee:03",
				"name":           "printer",
				"mac":            "aa:bb:cc:dd:ee:03",
				"ip":             "192.168.1.60",
				"connectionType": "wired",
				"status":         "offline",
				"connectedTo": map[string]any{
					"device": map[string]any{
						"kind": "device",
						"id":   "unifi.switch-lr",
						"uri":  "/network/devices/unifi.switch-lr",
						"name": "Switch Living Room",
					},
				},
			}), nil
		},
	}

	cmd := newGetClientCmd(stub)
	cmd.SetArgs([]string{"unifi.aa:bb:cc:dd:ee:03"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"printer", "offline", "Switch Living Room"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"PORT", "LINK SPEED", "UPTIME"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected %q absent for offline wired client, got:\n%s", absent, out)
		}
	}
}
```

- [ ] **Step 3: Run tests to confirm they fail**

```bash
go test ./internal/cli/network/... -run "TestGetClientCmd_wired|TestGetClientCmd_offline_wired" -v
```

Expected: FAIL — old code references `d.SwitchName` / `d.SwitchPort` which no longer exist.

- [ ] **Step 4: Update the wired case in `newGetClientCmd`**

Replace the `case "wired":` block inside `newGetClientCmd` in `network.go`:

```go
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
```

- [ ] **Step 5: Run tests to confirm they pass**

```bash
go test ./internal/cli/network/... -run "TestGetClientCmd_wired|TestGetClientCmd_offline_wired" -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/network/network.go internal/cli/network/network_test.go
git commit -m "feat: update wired client detail to use connectedTo shape"
```

---

### Task 8: Update `newGetClientCmd` — wireless client `connectedTo`

**Files:**
- Modify: `internal/cli/network/network.go`
- Modify: `internal/cli/network/network_test.go`

- [ ] **Step 1: Update the wireless client test**

Replace `TestGetClientCmd_wireless` in `network_test.go`:

```go
func TestGetClientCmd_wireless(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:02",
				"uri":            "/network/clients/unifi.aa:bb:cc:dd:ee:02",
				"name":           "phone",
				"mac":            "aa:bb:cc:dd:ee:02",
				"ip":             "192.168.1.51",
				"connectionType": "wireless",
				"status":         "online",
				"connectedTo": map[string]any{
					"device": map[string]any{
						"kind": "device",
						"id":   "unifi.ap-living-room",
						"uri":  "/network/devices/unifi.ap-living-room",
						"name": "AP Living Room",
					},
					"ssid":           "HomeNet",
					"signalStrength": -65,
				},
				"uptime": 1800,
			}), nil
		},
	}

	cmd := newGetClientCmd(stub)
	cmd.SetArgs([]string{"unifi.aa:bb:cc:dd:ee:02"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"phone", "AP Living Room", "HomeNet", "-65 dBm", "online"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "SWITCH") {
		t.Errorf("expected no SWITCH row in wireless output, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Update the offline wireless client test**

Replace `TestGetClientCmd_offline_wireless` in `network_test.go`:

```go
func TestGetClientCmd_offline_wireless(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:04",
				"uri":            "/network/clients/unifi.aa:bb:cc:dd:ee:04",
				"name":           "tablet",
				"mac":            "aa:bb:cc:dd:ee:04",
				"ip":             "192.168.1.70",
				"connectionType": "wireless",
				"status":         "offline",
				"connectedTo": map[string]any{
					"device": map[string]any{
						"kind": "device",
						"id":   "unifi.ap-living-room",
						"uri":  "/network/devices/unifi.ap-living-room",
						"name": "AP Living Room",
					},
					"ssid": "HomeNet",
				},
			}), nil
		},
	}

	cmd := newGetClientCmd(stub)
	cmd.SetArgs([]string{"unifi.aa:bb:cc:dd:ee:04"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"tablet", "offline", "AP Living Room", "HomeNet"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"SIGNAL", "UPTIME"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected %q absent for offline wireless client, got:\n%s", absent, out)
		}
	}
}
```

- [ ] **Step 3: Run tests to confirm they fail**

```bash
go test ./internal/cli/network/... -run "TestGetClientCmd_wireless|TestGetClientCmd_offline_wireless" -v
```

Expected: FAIL — old code references `d.Ssid` / `d.SignalStrength` which no longer exist.

- [ ] **Step 4: Update the wireless case in `newGetClientCmd`**

Replace the `case "wireless":` block inside `newGetClientCmd` in `network.go`:

```go
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
```

- [ ] **Step 5: Run all network tests**

```bash
go test ./internal/cli/network/... -v
```

Expected: all PASS.

- [ ] **Step 6: Run full test suite and build**

```bash
go test ./... && make build
```

Expected: all tests PASS, binary builds cleanly.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/network/network.go internal/cli/network/network_test.go
git commit -m "feat: update wireless client detail to use connectedTo shape"
```
