# Network VLANs / SSIDs / WANs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `vlans`, `ssids`, and `wans` subcommand groups under `hlctl network`, each with `list` and `get <id>` commands backed by new API endpoints in spec commit `4c71e7d`.

**Architecture:** Advance the `spec/` submodule, regenerate the oapi-codegen client, extend the `NetworkClient` interface and `StubClient`, then add three focused files (`vlans.go`, `ssids.go`, `wans.go`) in `internal/cli/network/`. Tests live in the existing `network_test.go`. Commands register in `NewCmd()` in `network.go`.

**Tech Stack:** Go, Cobra, oapi-codegen, `internal/output` for table/JSON rendering, `internal/apiclient` for error handling.

---

## File Map

| Action | Path |
|--------|------|
| Modify | `spec/` (submodule pointer) |
| Regenerate | `internal/network/api.gen.go` |
| Modify | `internal/cli/network/client.go` — add 6 interface methods |
| Modify | `internal/cli/network/stub.go` — add 6 stub method fields + forwarding methods |
| Create | `internal/cli/network/vlans.go` |
| Create | `internal/cli/network/ssids.go` |
| Create | `internal/cli/network/wans.go` |
| Modify | `internal/cli/network/network.go` — register 3 new subcommand groups in `NewCmd()` |
| Modify | `internal/cli/network/network_test.go` — add tests for all 6 commands |

---

### Task 1: Update submodule and regenerate API client

**Files:**
- Modify: `spec/` (submodule)
- Regenerate: `internal/network/api.gen.go`

- [ ] **Step 1: Advance the submodule to the commit that adds SSID/VLAN/WAN endpoints**

```bash
cd /path/to/repo/spec
git checkout 4c71e7d
cd ..
git add spec
```

- [ ] **Step 2: Regenerate the API client**

```bash
make generate
```

Expected: no errors; `internal/network/api.gen.go` is updated.

- [ ] **Step 3: Verify the new types exist in the generated file**

```bash
grep -E "VlanList|SsidList|WanList|ListVlans|ListSsids|ListWans" internal/network/api.gen.go
```

Expected: all six names appear.

- [ ] **Step 4: Note the exact field names you'll need**

Check what oapi-codegen produced for these types:

```bash
grep -A 15 "type VlanDetail struct" internal/network/api.gen.go
grep -A 10 "type Wan struct" internal/network/api.gen.go
grep -A 10 "type SsidDetail struct" internal/network/api.gen.go
grep -E "DhcpMode[A-Z]|WanStatus[A-Z]|WifiBand[A-Z]" internal/network/api.gen.go | head -20
```

Use those exact names in subsequent tasks. The plan assumes:
- `gen.VlanDetail` has fields `Id`, `Name`, `VlanId`, `Subnet`, `GatewayIp`, `BroadcastIp`, `DhcpMode`, `DhcpRange *gen.DhcpRange`, `RelayServer *string`, `DnsServers []string`
- `gen.DhcpRange` has `Start`, `End string`
- `gen.DhcpMode` enum values: `DhcpModeServer`, `DhcpModeRelay`, `DhcpModeDisabled`
- `gen.Ssid` has `Id`, `Name`, `VlanId int`, `Bands []gen.WifiBand`, `NumClients int`
- `gen.SsidDetail` has all Ssid fields plus `SecurityProtocol gen.WifiSecurityProtocol`, `Clients []gen.NetworkClientRef`, `BroadcastingAps []gen.NetworkDeviceRef`
- `gen.WifiBand` enum values: `WifiBandBand2g`, `WifiBandBand5g`, `WifiBandBand6g`
- `gen.Wan` has `Id`, `Name`, `IpAddress`, `Uptime int`, `Status gen.WanStatus`
- `gen.WanDetail` has all Wan fields plus `DnsServers []string`
- `gen.WanStatus` enum values: `WanStatusConnected`, `WanStatusDisconnected`, `WanStatusFailover`

If actual generated names differ, adjust the code in later tasks accordingly.

- [ ] **Step 5: Build to verify generated code compiles**

```bash
make build
```

Expected: successful build.

- [ ] **Step 6: Commit submodule update**

```bash
git add spec internal/network/api.gen.go
git commit -m "chore: advance spec submodule to add VLAN/SSID/WAN endpoints"
```

---

### Task 2: Extend NetworkClient interface and StubClient

**Files:**
- Modify: `internal/cli/network/client.go`
- Modify: `internal/cli/network/stub.go`

- [ ] **Step 1: Add 6 methods to the `NetworkClient` interface in `client.go`**

Open `internal/cli/network/client.go`. The current interface ends after `GetNetworkTopology`. Add these methods:

```go
ListVlans(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
GetVlan(ctx context.Context, vlanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
ListSsids(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
GetSsid(ctx context.Context, ssidId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
ListWans(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
GetWan(ctx context.Context, wanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
```

The full updated `NetworkClient` interface in `client.go`:

```go
type NetworkClient interface {
	ListNetworkDevices(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkDevice(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListNetworkClients(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkClient(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkTopology(ctx context.Context, params *gen.GetNetworkTopologyParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListVlans(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetVlan(ctx context.Context, vlanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSsids(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetSsid(ctx context.Context, ssidId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListWans(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetWan(ctx context.Context, wanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}
```

- [ ] **Step 2: Add 6 function fields + forwarding methods to `StubClient` in `stub.go`**

Add these fields to the `StubClient` struct (after `GetNetworkTopologyFunc`):

```go
ListVlansFunc func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
GetVlanFunc   func(ctx context.Context, vlanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
ListSsidsFunc func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
GetSsidFunc   func(ctx context.Context, ssidId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
ListWansFunc  func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
GetWanFunc    func(ctx context.Context, wanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
```

Add these forwarding methods at the bottom of `stub.go` (before the `jsonResponse` helper):

```go
func (s *StubClient) ListVlans(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListVlansFunc(ctx, reqEditors...)
}

func (s *StubClient) GetVlan(ctx context.Context, vlanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetVlanFunc(ctx, vlanId, reqEditors...)
}

func (s *StubClient) ListSsids(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListSsidsFunc(ctx, reqEditors...)
}

func (s *StubClient) GetSsid(ctx context.Context, ssidId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetSsidFunc(ctx, ssidId, reqEditors...)
}

func (s *StubClient) ListWans(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListWansFunc(ctx, reqEditors...)
}

func (s *StubClient) GetWan(ctx context.Context, wanId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetWanFunc(ctx, wanId, reqEditors...)
}
```

- [ ] **Step 3: Build to verify no compile errors**

```bash
make build
```

Expected: successful build.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/network/client.go internal/cli/network/stub.go
git commit -m "feat: extend NetworkClient interface with vlans, ssids, wans methods"
```

---

### Task 3: Implement vlans commands (TDD)

**Files:**
- Create: `internal/cli/network/vlans.go`
- Modify: `internal/cli/network/network_test.go`

- [ ] **Step 1: Write failing tests for vlans list and get**

Append to `internal/cli/network/network_test.go`:

```go
func TestListVlansCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListVlansFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.VlanList{
				Items: []gen.Vlan{
					{Id: "unifi.default", Name: "Default", VlanId: 1, Subnet: "192.168.1.0/24"},
					{Id: "unifi.iot", Name: "IoT", VlanId: 20, Subnet: "192.168.20.0/24"},
				},
			}), nil
		},
	}
	cmd := newListVlansCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"unifi.default", "Default", "192.168.1.0/24", "unifi.iot", "IoT", "20"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListVlansCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListVlansFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized",
				"status": 401, "detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListVlansCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
}

func TestGetVlanCmd_serverDhcp(t *testing.T) {
	stub := &StubClient{
		GetVlanFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.iot", "uri": "/network/vlans/unifi.iot",
				"name": "IoT", "vlanId": 20, "subnet": "192.168.20.0/24",
				"gatewayIp": "192.168.20.1", "broadcastIp": "192.168.20.255",
				"dhcpMode": "server",
				"dhcpRange": map[string]any{"start": "192.168.20.100", "end": "192.168.20.200"},
				"dnsServers": []string{"1.1.1.1", "8.8.8.8"},
			}), nil
		},
	}
	cmd := newGetVlanCmd(stub)
	cmd.SetArgs([]string{"unifi.iot"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"unifi.iot", "IoT", "20", "192.168.20.0/24", "192.168.20.1", "192.168.20.255", "server", "192.168.20.100", "192.168.20.200", "1.1.1.1"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "RELAY") {
		t.Errorf("expected no RELAY row for server DHCP, got:\n%s", out)
	}
}

func TestGetVlanCmd_relayDhcp(t *testing.T) {
	stub := &StubClient{
		GetVlanFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.mgmt", "uri": "/network/vlans/unifi.mgmt",
				"name": "Management", "vlanId": 99, "subnet": "10.0.99.0/24",
				"gatewayIp": "10.0.99.1", "broadcastIp": "10.0.99.255",
				"dhcpMode": "relay", "relayServer": "192.168.1.1",
				"dnsServers": []string{"192.168.1.1"},
			}), nil
		},
	}
	cmd := newGetVlanCmd(stub)
	cmd.SetArgs([]string{"unifi.mgmt"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Management", "relay", "192.168.1.1", "RELAY"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "DHCP RANGE") {
		t.Errorf("expected no DHCP RANGE row for relay DHCP, got:\n%s", out)
	}
}

func TestGetVlanCmd_notFound(t *testing.T) {
	stub := &StubClient{
		GetVlanFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type": "https://homelab.local/problems/not-found", "title": "Not Found",
				"status": 404, "detail": "vlan not found",
			}), nil
		},
	}
	cmd := newGetVlanCmd(stub)
	cmd.SetArgs([]string{"unifi.nonexistent"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail (commands not yet defined)**

```bash
go test ./internal/cli/network/... -run "TestListVlansCmd|TestGetVlanCmd" -v 2>&1 | head -30
```

Expected: compile error — `newListVlansCmd` undefined.

- [ ] **Step 3: Create `internal/cli/network/vlans.go`**

```go
package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/network"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func newVlansCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vlans",
		Short: "VLANs",
	}
	cmd.AddCommand(newListVlansCmd(nil))
	cmd.AddCommand(newGetVlanCmd(nil))
	return cmd
}

func newListVlansCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List VLANs",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.ListVlans(context.Background())
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

			var list gen.VlanList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "VLAN ID", "SUBNET"}
			var rows [][]string
			for _, v := range list.Items {
				rows = append(rows, []string{
					v.Id, v.Name, fmt.Sprintf("%d", v.VlanId), v.Subnet,
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}
}

func newGetVlanCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <vlan-id>",
		Short: "Show VLAN details",
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

			resp, err := c.GetVlan(context.Background(), args[0])
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

			var detail gen.VlanDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", detail.Id},
				{"NAME", detail.Name},
				{"VLAN ID", fmt.Sprintf("%d", detail.VlanId)},
				{"SUBNET", detail.Subnet},
				{"GATEWAY IP", detail.GatewayIp},
				{"BROADCAST", detail.BroadcastIp},
				{"DHCP MODE", string(detail.DhcpMode)},
			}
			if detail.DhcpMode == gen.DhcpModeServer && detail.DhcpRange != nil {
				rows = append(rows, []string{"DHCP RANGE", fmt.Sprintf("%s - %s", detail.DhcpRange.Start, detail.DhcpRange.End)})
			}
			if detail.DhcpMode == gen.DhcpModeRelay && detail.RelayServer != nil {
				rows = append(rows, []string{"RELAY", *detail.RelayServer})
			}
			rows = append(rows, []string{"DNS", strings.Join(detail.DnsServers, ", ")})
			return output.Print(cmd.OutOrStdout(), output.FormatTable, nil, headers, rows)
		},
	}
}
```

- [ ] **Step 4: Run the vlans tests**

```bash
go test ./internal/cli/network/... -run "TestListVlansCmd|TestGetVlanCmd" -v
```

Expected: all 4 tests pass.

- [ ] **Step 5: Run full test suite to check for regressions**

```bash
go test ./internal/cli/network/...
```

Expected: all existing tests still pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/network/vlans.go internal/cli/network/network_test.go
git commit -m "feat: add network vlans list and get commands"
```

---

### Task 4: Implement ssids commands (TDD)

**Files:**
- Create: `internal/cli/network/ssids.go`
- Modify: `internal/cli/network/network_test.go`

- [ ] **Step 1: Write failing tests for ssids list and get**

Append to `internal/cli/network/network_test.go`:

```go
func TestListSsidsCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListSsidsFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.SsidList{
				Items: []gen.Ssid{
					{
						Id: "unifi.home", Name: "Home", VlanId: 1,
						Bands:      []gen.WifiBand{gen.WifiBandBand2g, gen.WifiBandBand5g, gen.WifiBandBand6g},
						NumClients: 12,
					},
					{
						Id: "unifi.iot", Name: "IoT", VlanId: 20,
						Bands:      []gen.WifiBand{gen.WifiBandBand2g, gen.WifiBandBand5g},
						NumClients: 8,
					},
				},
			}), nil
		},
	}
	cmd := newListSsidsCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"unifi.home", "Home", "unifi.iot", "IoT", "2.4 GHz", "5 GHz", "6 GHz", "12", "8"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListSsidsCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListSsidsFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized",
				"status": 401, "detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListSsidsCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
}

func TestGetSsidCmd_withClients(t *testing.T) {
	stub := &StubClient{
		GetSsidFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.iot", "uri": "/network/ssids/unifi.iot",
				"name": "IoT", "vlanId": 20,
				"bands":      []string{"band2g", "band5g"},
				"numClients": 2,
				"securityProtocol": "wpa2",
				"clients": []map[string]any{
					{"kind": "client", "id": "unifi.sonos", "uri": "/network/clients/unifi.sonos", "name": "Sonos One SL"},
					{"kind": "client", "id": "unifi.hue", "uri": "/network/clients/unifi.hue", "name": "Philips Hue Bridge"},
				},
				"broadcastingAps": []map[string]any{
					{"kind": "device", "id": "unifi.ap-lr", "uri": "/network/devices/unifi.ap-lr", "name": "AP Living Room"},
				},
			}), nil
		},
	}
	cmd := newGetSsidCmd(stub)
	cmd.SetArgs([]string{"unifi.iot"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"unifi.iot", "IoT", "20", "wpa2", "Sonos One SL", "Philips Hue Bridge", "AP Living Room", "CLIENTS", "BROADCASTING APs"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetSsidCmd_notFound(t *testing.T) {
	stub := &StubClient{
		GetSsidFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type": "https://homelab.local/problems/not-found", "title": "Not Found",
				"status": 404, "detail": "ssid not found",
			}), nil
		},
	}
	cmd := newGetSsidCmd(stub)
	cmd.SetArgs([]string{"unifi.nonexistent"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/cli/network/... -run "TestListSsidsCmd|TestGetSsidCmd" -v 2>&1 | head -20
```

Expected: compile error — `newListSsidsCmd` undefined.

- [ ] **Step 3: Create `internal/cli/network/ssids.go`**

```go
package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/network"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func newSsidsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssids",
		Short: "WiFi networks (SSIDs)",
	}
	cmd.AddCommand(newListSsidsCmd(nil))
	cmd.AddCommand(newGetSsidCmd(nil))
	return cmd
}

func newListSsidsCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List WiFi networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.ListSsids(context.Background())
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

			var list gen.SsidList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "VLAN ID", "BANDS", "CLIENTS"}
			var rows [][]string
			for _, s := range list.Items {
				rows = append(rows, []string{
					s.Id, s.Name, fmt.Sprintf("%d", s.VlanId),
					formatBands(s.Bands),
					fmt.Sprintf("%d", s.NumClients),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}
}

func newGetSsidCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <ssid-id>",
		Short: "Show WiFi network details",
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

			resp, err := c.GetSsid(context.Background(), args[0])
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

			var detail gen.SsidDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", detail.Id},
				{"NAME", detail.Name},
				{"VLAN ID", fmt.Sprintf("%d", detail.VlanId)},
				{"BANDS", formatBands(detail.Bands)},
				{"CLIENTS", fmt.Sprintf("%d", detail.NumClients)},
				{"SECURITY", string(detail.SecurityProtocol)},
			}
			if err := output.Print(cmd.OutOrStdout(), output.FormatTable, nil, headers, rows); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n--- CLIENTS ---\n")
			clientHeaders := []string{"NAME"}
			var clientRows [][]string
			for _, cl := range detail.Clients {
				clientRows = append(clientRows, []string{cl.Name})
			}
			if err := output.Print(cmd.OutOrStdout(), output.FormatTable, nil, clientHeaders, clientRows); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n--- BROADCASTING APs ---\n")
			apHeaders := []string{"NAME"}
			var apRows [][]string
			for _, ap := range detail.BroadcastingAps {
				apRows = append(apRows, []string{ap.Name})
			}
			return output.Print(cmd.OutOrStdout(), output.FormatTable, nil, apHeaders, apRows)
		},
	}
}

func formatBands(bands []gen.WifiBand) string {
	var parts []string
	for _, b := range bands {
		switch b {
		case gen.WifiBandBand2g:
			parts = append(parts, "2.4 GHz")
		case gen.WifiBandBand5g:
			parts = append(parts, "5 GHz")
		case gen.WifiBandBand6g:
			parts = append(parts, "6 GHz")
		default:
			parts = append(parts, string(b))
		}
	}
	return strings.Join(parts, ", ")
}
```

- [ ] **Step 4: Run the ssids tests**

```bash
go test ./internal/cli/network/... -run "TestListSsidsCmd|TestGetSsidCmd" -v
```

Expected: all 4 tests pass.

- [ ] **Step 5: Run full test suite**

```bash
go test ./internal/cli/network/...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/network/ssids.go internal/cli/network/network_test.go
git commit -m "feat: add network ssids list and get commands"
```

---

### Task 5: Implement wans commands (TDD)

**Files:**
- Create: `internal/cli/network/wans.go`
- Modify: `internal/cli/network/network_test.go`

- [ ] **Step 1: Write failing tests for wans list and get**

Append to `internal/cli/network/network_test.go`:

```go
func TestListWansCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListWansFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.WanList{
				Items: []gen.Wan{
					{Id: "unifi.wan1", Name: "WAN 1", IpAddress: "203.0.113.42", Uptime: 86400, Status: gen.WanStatusConnected},
					{Id: "unifi.wan2", Name: "WAN 2", IpAddress: "198.51.100.7", Uptime: 0, Status: gen.WanStatusFailover},
				},
			}), nil
		},
	}
	cmd := newListWansCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"unifi.wan1", "WAN 1", "203.0.113.42", "connected", "unifi.wan2", "failover"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListWansCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListWansFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized",
				"status": 401, "detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListWansCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
}

func TestGetWanCmd_connected(t *testing.T) {
	stub := &StubClient{
		GetWanFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id": "unifi.wan1", "uri": "/network/wans/unifi.wan1",
				"name": "WAN 1", "ipAddress": "203.0.113.42",
				"uptime": 86400, "status": "connected",
				"dnsServers": []string{"1.1.1.1", "1.0.0.1"},
			}), nil
		},
	}
	cmd := newGetWanCmd(stub)
	cmd.SetArgs([]string{"unifi.wan1"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"unifi.wan1", "WAN 1", "203.0.113.42", "connected", "1d", "1.1.1.1", "1.0.0.1"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetWanCmd_notFound(t *testing.T) {
	stub := &StubClient{
		GetWanFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type": "https://homelab.local/problems/not-found", "title": "Not Found",
				"status": 404, "detail": "wan not found",
			}), nil
		},
	}
	cmd := newGetWanCmd(stub)
	cmd.SetArgs([]string{"unifi.nonexistent"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/cli/network/... -run "TestListWansCmd|TestGetWanCmd" -v 2>&1 | head -20
```

Expected: compile error — `newListWansCmd` undefined.

- [ ] **Step 3: Create `internal/cli/network/wans.go`**

```go
package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/network"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func newWansCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wans",
		Short: "WAN interfaces",
	}
	cmd.AddCommand(newListWansCmd(nil))
	cmd.AddCommand(newGetWanCmd(nil))
	return cmd
}

func newListWansCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List WAN interfaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.ListWans(context.Background())
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

			var list gen.WanList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			headers := []string{"ID", "NAME", "IP", "UPTIME", "STATUS"}
			var rows [][]string
			for _, w := range list.Items {
				rows = append(rows, []string{
					w.Id, w.Name, w.IpAddress,
					output.FormatUptime(w.Uptime),
					string(w.Status),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}
}

func newGetWanCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <wan-id>",
		Short: "Show WAN interface details",
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

			resp, err := c.GetWan(context.Background(), args[0])
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

			var detail gen.WanDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", detail.Id},
				{"NAME", detail.Name},
				{"IP", detail.IpAddress},
				{"UPTIME", output.FormatUptime(detail.Uptime)},
				{"STATUS", string(detail.Status)},
				{"DNS", strings.Join(detail.DnsServers, ", ")},
			}
			return output.Print(cmd.OutOrStdout(), output.FormatTable, nil, headers, rows)
		},
	}
}
```

- [ ] **Step 4: Run the wans tests**

```bash
go test ./internal/cli/network/... -run "TestListWansCmd|TestGetWanCmd" -v
```

Expected: all 4 tests pass.

- [ ] **Step 5: Run full test suite**

```bash
go test ./internal/cli/network/...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/network/wans.go internal/cli/network/network_test.go
git commit -m "feat: add network wans list and get commands"
```

---

### Task 6: Register commands and final verification

**Files:**
- Modify: `internal/cli/network/network.go`

- [ ] **Step 1: Register the three new subcommand groups in `NewCmd()`**

In `internal/cli/network/network.go`, find `NewCmd()`. It currently reads:

```go
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Network devices and clients",
	}
	cmd.AddCommand(newDevicesCmd())
	cmd.AddCommand(newClientsCmd())
	cmd.AddCommand(newTopologyCmd(nil))
	return cmd
}
```

Change it to:

```go
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
```

- [ ] **Step 2: Run the full test suite**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 3: Build and smoke-test the help output**

```bash
make build && ./bin/hlctl network --help
```

Expected: `vlans`, `ssids`, and `wans` appear in the subcommand list.

```bash
./bin/hlctl network vlans --help
./bin/hlctl network ssids --help
./bin/hlctl network wans --help
```

Expected: each shows `list` and `get` subcommands.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/network/network.go
git commit -m "feat: register vlans, ssids, and wans subcommands under network"
```
