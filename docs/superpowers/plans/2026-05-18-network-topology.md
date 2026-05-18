# Network Topology Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `hlctl network topology` that calls `GET /network/topology` and renders the result as an ASCII tree rooted at the gateway, with flags to include wired and wireless clients.

**Architecture:** The command follows the existing pattern in `internal/cli/network/`: a stub-based `NetworkClient` interface, commands wired up in `network.go`, and table-driven tests using `StubClient`. Tree rendering builds a parent→children adjacency map from the API's edges, then recursively prints with `├──`/`└──`/`│` connectors. Wireless edges are filtered client-side when `--include-wireless` is absent.

**Tech Stack:** Go, Cobra, oapi-codegen (client+models from OpenAPI spec), tabwriter not needed (custom tree rendering with `fmt.Fprintln`).

---

### Task 1: Commit the submodule update and regenerate the API client

**Files:**
- Modify: `spec` (submodule pointer, already advanced to `4121624`)
- Modify: `internal/network/api.gen.go` (regenerated — gitignored, not committed)

- [ ] **Step 1: Commit the submodule pointer update**

The submodule is already checked out at the new commit. Stage and commit it:

```bash
git add spec
git commit -m "chore: update spec submodule to network topology endpoint"
```

Expected: commit succeeds with message about spec submodule.

- [ ] **Step 2: Run make generate**

```bash
make generate
```

This runs `make bundle` inside the spec submodule first (requires `npx` / node), then runs `oapi-codegen`. Expected: `internal/network/api.gen.go` updated with no errors.

- [ ] **Step 3: Verify new types exist in generated code**

```bash
grep -n "GetNetworkTopology\|NetworkTopology\|TopologyNode\|TopologyEdge\|TopologyDeviceNode\|TopologyClientNode\|TopologyWiredEdge\|TopologyWirelessEdge" internal/network/api.gen.go | head -30
```

Expected: all these identifiers appear. Also verify the params type:

```bash
grep -A3 "GetNetworkTopologyParams" internal/network/api.gen.go
```

Expected: struct with `IncludeClients *bool` field.

- [ ] **Step 4: Verify the project still builds**

```bash
make build
```

Expected: `bin/hlctl` built successfully with no errors.

---

### Task 2: Extend NetworkClient interface and StubClient

**Files:**
- Modify: `internal/cli/network/client.go`
- Modify: `internal/cli/network/stub.go`

- [ ] **Step 1: Add `GetNetworkTopology` to the NetworkClient interface**

In `internal/cli/network/client.go`, add one line to the `NetworkClient` interface:

```go
type NetworkClient interface {
	ListNetworkDevices(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkDevice(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListNetworkClients(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkClient(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkTopology(ctx context.Context, params *gen.GetNetworkTopologyParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}
```

- [ ] **Step 2: Add `GetNetworkTopologyFunc` to StubClient**

In `internal/cli/network/stub.go`, add the field and method. The full updated file:

```go
package network

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gen "github.com/bwilczynski/hlctl/internal/network"
)

// StubClient is a NetworkClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListNetworkDevicesFunc   func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkDeviceFunc     func(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListNetworkClientsFunc   func(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkClientFunc     func(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkTopologyFunc   func(ctx context.Context, params *gen.GetNetworkTopologyParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func (s *StubClient) ListNetworkDevices(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListNetworkDevicesFunc(ctx, reqEditors...)
}

func (s *StubClient) GetNetworkDevice(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetNetworkDeviceFunc(ctx, deviceId, reqEditors...)
}

func (s *StubClient) ListNetworkClients(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListNetworkClientsFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetNetworkClient(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetNetworkClientFunc(ctx, clientId, reqEditors...)
}

func (s *StubClient) GetNetworkTopology(ctx context.Context, params *gen.GetNetworkTopologyParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetNetworkTopologyFunc(ctx, params, reqEditors...)
}

// jsonResponse builds an *http.Response with a JSON body and the given status code.
func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}
```

- [ ] **Step 3: Verify the project compiles**

```bash
make build
```

Expected: success. The generated `gen.Client` satisfies the updated interface because it already implements `GetNetworkTopology`.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/network/client.go internal/cli/network/stub.go
git commit -m "feat: extend NetworkClient interface with GetNetworkTopology"
```

---

### Task 3: Write topology command tests (TDD — write failing tests first)

**Files:**
- Modify: `internal/cli/network/network_test.go`

All tests use `StubClient.GetNetworkTopologyFunc`. Add the following test functions to `network_test.go`.

- [ ] **Step 1: Add TestTopologyCmd_devicesOnly**

This test covers the default (no flags) case: gateway → switch → AP, no clients.

```go
func TestTopologyCmd_devicesOnly(t *testing.T) {
	stub := &StubClient{
		GetNetworkTopologyFunc: func(_ context.Context, params *gen.GetNetworkTopologyParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			if params.IncludeClients != nil {
				t.Errorf("expected no IncludeClients param, got %v", *params.IncludeClients)
			}
			return jsonResponse(http.StatusOK, map[string]any{
				"nodes": []any{
					map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG", "type": "gateway", "status": "connected"},
					map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR", "type": "switch", "status": "connected"},
					map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR", "type": "accessPoint", "status": "connected", "numClients": 2},
				},
				"edges": []any{
					map[string]any{
						"kind":      "wired",
						"source":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
						"target":    map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG"},
						"port":      1,
						"linkSpeed": "gbe1",
					},
					map[string]any{
						"kind":      "wired",
						"source":    map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
						"target":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
						"port":      7,
						"linkSpeed": "gbe2_5",
					},
				},
			}), nil
		},
	}

	cmd := newTopologyCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"USG", "gateway", "Switch LR", "switch", "AP LR", "accessPoint", "├──", "└──", "port 1", "1 GbE", "port 7", "2.5 GbE", "[2 clients]"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Add TestTopologyCmd_includeClientsWiredOnly**

`--include-clients` sets `IncludeClients=true`; wireless client is omitted from the rendered tree.

```go
func TestTopologyCmd_includeClientsWiredOnly(t *testing.T) {
	stub := &StubClient{
		GetNetworkTopologyFunc: func(_ context.Context, params *gen.GetNetworkTopologyParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			if params.IncludeClients == nil || !*params.IncludeClients {
				t.Error("expected IncludeClients=true")
			}
			return jsonResponse(http.StatusOK, map[string]any{
				"nodes": []any{
					map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG", "type": "gateway", "status": "connected"},
					map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR", "type": "switch", "status": "connected"},
					map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR", "type": "accessPoint", "status": "connected"},
					map[string]any{"kind": "client", "id": "unifi.nas", "uri": "/network/clients/unifi.nas", "name": "nas-1", "connectionType": "wired", "status": "online"},
					map[string]any{"kind": "client", "id": "unifi.mbp", "uri": "/network/clients/unifi.mbp", "name": "MacBook Pro", "connectionType": "wireless", "status": "online"},
				},
				"edges": []any{
					map[string]any{
						"kind":      "wired",
						"source":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
						"target":    map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG"},
						"port":      1,
						"linkSpeed": "gbe1",
					},
					map[string]any{
						"kind":      "wired",
						"source":    map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
						"target":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
						"port":      7,
						"linkSpeed": "gbe2_5",
					},
					map[string]any{
						"kind":      "wired",
						"source":    map[string]any{"kind": "client", "id": "unifi.nas", "uri": "/network/clients/unifi.nas", "name": "nas-1"},
						"target":    map[string]any{"kind": "device", "id": "unifi.sw", "uri": "/network/devices/unifi.sw", "name": "Switch LR"},
						"port":      8,
						"linkSpeed": "gbe1",
					},
					map[string]any{
						"kind":   "wireless",
						"source": map[string]any{"kind": "client", "id": "unifi.mbp", "uri": "/network/clients/unifi.mbp", "name": "MacBook Pro"},
						"target": map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
						"ssid":            "HomeNet",
						"signalStrength":  -55,
					},
				},
			}), nil
		},
	}

	cmd := newTopologyCmd(stub)
	cmd.SetArgs([]string{"--include-clients"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1", "port 8", "1 GbE"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"MacBook Pro", "HomeNet"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected %q absent (wireless filtered), got:\n%s", absent, out)
		}
	}
}
```

- [ ] **Step 3: Add TestTopologyCmd_includeWireless**

`--include-wireless` implies `--include-clients`; wireless client appears in the tree.

```go
func TestTopologyCmd_includeWireless(t *testing.T) {
	stub := &StubClient{
		GetNetworkTopologyFunc: func(_ context.Context, params *gen.GetNetworkTopologyParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			if params.IncludeClients == nil || !*params.IncludeClients {
				t.Error("expected IncludeClients=true (implied by --include-wireless)")
			}
			return jsonResponse(http.StatusOK, map[string]any{
				"nodes": []any{
					map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG", "type": "gateway", "status": "connected"},
					map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR", "type": "accessPoint", "status": "connected"},
					map[string]any{"kind": "client", "id": "unifi.mbp", "uri": "/network/clients/unifi.mbp", "name": "MacBook Pro", "connectionType": "wireless", "status": "online"},
				},
				"edges": []any{
					map[string]any{
						"kind":      "wired",
						"source":    map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
						"target":    map[string]any{"kind": "device", "id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG"},
						"port":      1,
						"linkSpeed": "gbe1",
					},
					map[string]any{
						"kind":           "wireless",
						"source":         map[string]any{"kind": "client", "id": "unifi.mbp", "uri": "/network/clients/unifi.mbp", "name": "MacBook Pro"},
						"target":         map[string]any{"kind": "device", "id": "unifi.ap", "uri": "/network/devices/unifi.ap", "name": "AP LR"},
						"ssid":           "HomeNet",
						"signalStrength": -55,
					},
				},
			}), nil
		},
	}

	cmd := newTopologyCmd(stub)
	cmd.SetArgs([]string{"--include-wireless"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"MacBook Pro", "HomeNet", "-55 dBm"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 4: Add TestTopologyCmd_jsonOutput**

JSON output bypasses tree rendering and prints raw body. The `--output` flag is a persistent flag on the root command, so in isolated tests set `flags.OutputFormat` directly (then restore it).

Note: the test file will need `"github.com/bwilczynski/hlctl/internal/cli/flags"` in its import block.

```go
func TestTopologyCmd_jsonOutput(t *testing.T) {
	old := flags.OutputFormat
	flags.OutputFormat = "json"
	defer func() { flags.OutputFormat = old }()

	stub := &StubClient{
		GetNetworkTopologyFunc: func(_ context.Context, _ *gen.GetNetworkTopologyParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"nodes": []any{},
				"edges": []any{},
			}), nil
		},
	}

	cmd := newTopologyCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"nodes"`) || !strings.Contains(out, `"edges"`) {
		t.Errorf("expected raw JSON with nodes/edges keys, got:\n%s", out)
	}
}
```

- [ ] **Step 5: Add TestTopologyCmd_apiError**

Non-200 response propagates as an error.

```go
func TestTopologyCmd_apiError(t *testing.T) {
	stub := &StubClient{
		GetNetworkTopologyFunc: func(_ context.Context, _ *gen.GetNetworkTopologyParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}

	cmd := newTopologyCmd(stub)
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
```

- [ ] **Step 6: Run tests — verify they fail because `newTopologyCmd` does not exist**

```bash
go test ./internal/cli/network/... 2>&1 | head -20
```

Expected: compilation errors referencing `newTopologyCmd` undefined.

- [ ] **Step 7: Commit the tests**

```bash
git add internal/cli/network/network_test.go
git commit -m "test: add topology command tests (failing)"
```

---

### Task 4: Implement the topology command

**Files:**
- Modify: `internal/cli/network/network.go`

Add the following to `network.go`. Import `sort` is not needed — entries will render in edge-list order.

- [ ] **Step 1: Add the childEntry type and newTopologyCmd to network.go**

Add directly before the final closing of the file (after `newGetClientCmd`). The full addition:

```go
type childEntry struct {
	nodeID   string
	nodeDisp string
	edgeDisp string
}

func newTopologyCmd(client NetworkClient) *cobra.Command {
	var includeClients bool
	var includeWireless bool

	cmd := &cobra.Command{
		Use:   "topology",
		Short: "Show network topology",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			resp, err := c.GetNetworkTopology(context.Background(), params)
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

			var topo gen.NetworkTopology
			if err := json.Unmarshal(body, &topo); err != nil {
				return err
			}

			return printTopologyTree(cmd.OutOrStdout(), topo, includeWireless)
		},
	}

	cmd.Flags().BoolVar(&includeClients, "include-clients", false, "Include wired clients in the topology")
	cmd.Flags().BoolVar(&includeWireless, "include-wireless", false, "Also include wireless clients (implies --include-clients)")
	return cmd
}

func printTopologyTree(w io.Writer, topo gen.NetworkTopology, includeWireless bool) error {
	// Build node display strings keyed by node ID.
	nodeDisp := make(map[string]string)
	var gatewayID string
	for _, n := range topo.Nodes {
		disc, err := n.Discriminator()
		if err != nil {
			return err
		}
		switch disc {
		case "device":
			d, err := n.AsTopologyDeviceNode()
			if err != nil {
				return err
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
				return err
			}
			nodeDisp[cl.Id] = fmt.Sprintf("%s (client, %s)", cl.Name, string(cl.ConnectionType))
		}
	}

	if gatewayID == "" {
		return fmt.Errorf("no gateway node found in topology")
	}

	// Build adjacency map: parent node ID → []childEntry.
	adjacency := make(map[string][]childEntry)
	for _, e := range topo.Edges {
		disc, err := e.Discriminator()
		if err != nil {
			return err
		}
		switch disc {
		case "wired":
			we, err := e.AsTopologyWiredEdge()
			if err != nil {
				return err
			}
			srcID, err := connectionRefID(we.Source)
			if err != nil {
				return err
			}
			edgeDisp := ""
			if we.Port != nil && we.LinkSpeed != nil {
				edgeDisp = fmt.Sprintf("[port %d, %s]", *we.Port, output.FormatLinkSpeed(string(*we.LinkSpeed)))
			} else if we.Port != nil {
				edgeDisp = fmt.Sprintf("[port %d]", *we.Port)
			}
			adjacency[we.Target.Id] = append(adjacency[we.Target.Id], childEntry{
				nodeID:   srcID,
				nodeDisp: nodeDisp[srcID],
				edgeDisp: edgeDisp,
			})
		case "wireless":
			if !includeWireless {
				continue
			}
			wire, err := e.AsTopologyWirelessEdge()
			if err != nil {
				return err
			}
			edgeDisp := fmt.Sprintf("(%s)", wire.Ssid)
			if wire.SignalStrength != nil {
				edgeDisp = fmt.Sprintf("(%s, %d dBm)", wire.Ssid, *wire.SignalStrength)
			}
			adjacency[wire.Target.Id] = append(adjacency[wire.Target.Id], childEntry{
				nodeID:   wire.Source.Id,
				nodeDisp: nodeDisp[wire.Source.Id],
				edgeDisp: edgeDisp,
			})
		}
	}

	// Print the tree rooted at the gateway.
	fmt.Fprintln(w, nodeDisp[gatewayID])
	children := adjacency[gatewayID]
	for i, child := range children {
		printTopologyNode(w, child, adjacency, "", i == len(children)-1)
	}
	return nil
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

func printTopologyNode(w io.Writer, entry childEntry, adjacency map[string][]childEntry, prefix string, isLast bool) {
	connector := "├── "
	childPrefix := "│   "
	if isLast {
		connector = "└── "
		childPrefix = "    "
	}

	line := entry.nodeDisp
	if entry.edgeDisp != "" {
		line += " " + entry.edgeDisp
	}
	fmt.Fprintln(w, prefix+connector+line)

	children := adjacency[entry.nodeID]
	for i, child := range children {
		printTopologyNode(w, child, adjacency, prefix+childPrefix, i == len(children)-1)
	}
}
```

- [ ] **Step 2: Run tests — verify they pass**

```bash
go test ./internal/cli/network/... -v -run TestTopology 2>&1
```

Expected: all `TestTopologyCmd_*` tests PASS.

If you get a compilation error about `TopologyDeviceNode` field names or method names, check the generated code:

```bash
grep -A20 "type TopologyDeviceNode\|func.*TopologyNode.*Discriminator\|func.*AsTopologyDeviceNode" internal/network/api.gen.go | head -40
```

Adjust field names to match what oapi-codegen actually generated (e.g. `d.Id` might be `d.ID` if oapi-codegen uses Go conventions).

- [ ] **Step 3: Run the full test suite**

```bash
go test ./... 2>&1
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/network/network.go
git commit -m "feat: implement network topology command with ASCII tree rendering"
```

---

### Task 5: Register the topology command and do a final build check

**Files:**
- Modify: `internal/cli/network/network.go` (the `NewCmd` function)

- [ ] **Step 1: Register `newTopologyCmd` in `NewCmd`**

Update `NewCmd()` in `network.go`:

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

- [ ] **Step 2: Build and verify help text**

```bash
make build && ./bin/hlctl network topology --help
```

Expected output contains:
```
Show network topology

Usage:
  hlctl network topology [flags]

Flags:
      --include-clients    Include wired clients in the topology
      --include-wireless   Also include wireless clients (implies --include-clients)
  -o, --output string      ...
```

- [ ] **Step 3: Run all tests one final time**

```bash
go test ./... 2>&1
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/network/network.go
git commit -m "feat: register network topology command"
```
