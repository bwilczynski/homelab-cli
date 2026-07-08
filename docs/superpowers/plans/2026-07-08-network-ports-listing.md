# Network Ports Listing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a flat `/network/ports` endpoint to the API spec and a matching `hlctl network ports list` command with server-side filters (`--switch`, `--mode`, `--state`, `--vlan`, `--all-ports`, `--wide`) that audits switch ports across all controllers in one call.

**Architecture:** Additive. New OpenAPI path + two schemas in the `homelab-api-spec` submodule. New CLI file `internal/cli/network/ports.go` following the existing Options + `runF` pattern. Per-port decoration currently in `devices.go` is extracted into `decoratePort` and reused by both `devices get` and `ports list`. A new text template `ports_list.tmpl` renders the audit-focused columns.

**Tech Stack:** Go 1.x, Cobra, oapi-codegen v2, `text/template` via `internal/output`, Redocly + Spectral for spec lint.

## Global Constraints

- Design doc — `docs/superpowers/specs/2026-07-08-network-ports-listing-design.md` (authoritative source; consult when a task feels ambiguous).
- Commit style — Conventional Commits (`feat:`, `fix:`, `chore:`, `refactor:`). No `Co-Authored-By` trailer (per `CLAUDE.md`).
- Spec changes — path files reference component files via **relative** `$ref` (e.g. `"../components/schemas/network/NetworkPort.yaml"`), never `#/components/...`.
- Spec versioning — do NOT edit `openapi/openapi.yaml`'s `info.version` by hand; semantic-release patches it.
- Spec commits — use `feat:` for additive endpoints/fields. No `BREAKING CHANGE` footer for this plan (endpoint is additive; existing endpoints unchanged).
- Client generated files — `internal/api/network/api.gen.go` is gitignored; regenerate with `make generate` after any submodule bump. Never edit it by hand.
- Client test pattern — Layer 1 (`runF` hook, verifies Cobra wiring) + Layer 2 (direct `opts` struct + `httpmock.NewRegistry()`). See `internal/cli/network/vlans_test.go` for the canonical example.
- Template style — tab-separated columns, `{{ flush }}` between sections. See `internal/cli/network/templates/devices_get_switch.tmpl`.
- Column set (design decision, Q5) — default `SWITCH  PORT  LABEL  STATE  MODE  NATIVE VLAN  TAGGED VLANS  CONNECTED TO`; `--wide` appends `LINK SPEED  POE  LAG  SFP  RX/S  TX/S`.
- Sort order — switch name ascending, then port number ascending.
- Default state filter — if neither `--state` nor `--all-ports` is set, wire `state=up` (parity with `devices get`). `--all-ports` omits the param entirely. `--state` and `--all-ports` are mutually exclusive.
- Absolute paths in this plan are anchored at the repo root `.` (this repository). Spec work uses paths inside `spec/` (submodule) unless noted.

---

## Task Map

| # | Deliverable | Repo / Directory |
|---|---|---|
| 1 | Spec — path + schemas + root-doc wiring | `spec/` submodule |
| 2 | Client — submodule bump, regenerate, extend `NetworkClient` interface | client repo root |
| 3 | Client — `ports list` command, template, tests, decoration extraction | client repo root |

Tasks 2 and 3 land in a single client PR; Task 1 lands in a spec PR that must be merged and released before the client PR.

---

## Task 1: Spec — `/network/ports` path and schemas

**Repo:** `homelab-api-spec` (accessed via the `spec/` submodule).

**Files:**
- Create: `spec/openapi/components/schemas/network/NetworkPort.yaml`
- Create: `spec/openapi/components/schemas/network/NetworkPortList.yaml`
- Create: `spec/openapi/paths/network-ports.yaml`
- Modify: `spec/openapi/openapi.yaml` (insert one `paths:` entry)

**Interfaces:**
- Produces: `GET /network/ports` → `NetworkPortList = { items: [NetworkPort] }`. Each `NetworkPort` inherits every field of `SwitchPort` and adds a required `switch: NetworkDeviceRef`. Query params: `switchId` (string), `mode` (`trunk|access`), `state` (`up|down`), `vlanId` (integer).

- [ ] **Step 1: Create `NetworkPort.yaml` schema**

Path: `spec/openapi/components/schemas/network/NetworkPort.yaml`

```yaml
allOf:
  - $ref: "./SwitchPort.yaml"
  - type: object
    description: |
      A physical switch port surfaced in the flat `/network/ports` listing.
      Inherits every field from `SwitchPort` and carries a reference to its
      parent switch so consumers can audit ports across all switches in a
      single call.
    properties:
      switch:
        allOf:
          - $ref: "./NetworkDeviceRef.yaml"
        description: |
          Reference to the switch this port belongs to. Always populated;
          `switch.kind` is always `device`.
    required:
      - switch
```

- [ ] **Step 2: Create `NetworkPortList.yaml` schema**

Path: `spec/openapi/components/schemas/network/NetworkPortList.yaml`

```yaml
type: object
description: List of switch ports across all managed switches.
properties:
  items:
    type: array
    description: |
      All switch ports across all managed switches from all configured
      controllers. Empty array, never null. Order is not guaranteed;
      clients should sort as needed.
    items:
      $ref: "./NetworkPort.yaml"
required:
  - items
```

- [ ] **Step 3: Create `network-ports.yaml` path**

Path: `spec/openapi/paths/network-ports.yaml`

```yaml
get:
  operationId: listNetworkPorts
  x-stability-level: draft
  summary: List switch ports across all switches
  description: |
    Returns physical ports from every managed switch across all configured
    UniFi controllers as a single flat list. Each port carries a reference
    to its parent switch so consumers can audit VLAN policy, PoE state,
    LAG membership, and connected endpoints without fanning out one call
    per switch.

    A homelab typically has a small number of switches (each with tens of
    ports), so this endpoint returns all results without pagination.

    Filters are AND-composed and applied server-side; any combination is
    valid. Ports without a `vlanConfig` (administratively disabled ports)
    never match the `mode` or `vlanId` filters.
  tags:
    - network
  security:
    - bearerAuth: [read:network]
  parameters:
    - name: switchId
      in: query
      required: false
      description: |
        Filter to ports belonging to a specific switch, matched against
        the composite device identifier (`{controller}.{name}`).
      schema:
        type: string
      example: "unifi.switch-living-room"
    - name: mode
      in: query
      required: false
      description: |
        Filter by VLAN mode. Ports without a `vlanConfig` never match.
      schema:
        type: string
        enum: [access, trunk]
      example: trunk
    - name: state
      in: query
      required: false
      description: Filter by link state.
      schema:
        type: string
        enum: [up, down]
      example: up
    - name: vlanId
      in: query
      required: false
      description: |
        Filter to ports that carry the given VLAN ID. A port matches when
        its `vlanConfig.nativeVlan.vlanId` equals this value, OR when the
        value appears in `vlanConfig.taggedVlans.items[*].vlanId`, OR when
        the port is a trunk with `vlanConfig.taggedVlans.scope == "all"`.
        Ports without a `vlanConfig` never match.
      schema:
        type: integer
        minimum: 1
      example: 20
  responses:
    "200":
      description: List of switch ports across all switches.
      content:
        application/json:
          schema:
            $ref: "../components/schemas/network/NetworkPortList.yaml"
          examples:
            typicalHomelab:
              summary: One switch, four ports (trunk uplink, trunk AP downlink, idle access port, access LAG member).
              value:
                items:
                  - number: 1
                    state: up
                    linkSpeed: gbe1
                    linkUptime: 172800
                    poeMode: 'off'
                    vlanConfig:
                      mode: trunk
                      nativeVlan:
                        id: "unifi.default"
                        uri: "/network/vlans/unifi.default"
                        name: "Default"
                        vlanId: 1
                      taggedVlans:
                        scope: all
                    traffic:
                      rxBytesTotal: 43980465111
                      txBytesTotal: 87960930222
                      rxBytesPerSec: 600000
                      txBytesPerSec: 1250000
                    connectedTo:
                      kind: device
                      id: "unifi.usg"
                      uri: "/network/devices/unifi.usg"
                      name: "USG"
                    switch:
                      kind: device
                      id: "unifi.switch-living-room"
                      uri: "/network/devices/unifi.switch-living-room"
                      name: "Switch Living Room"
                  - number: 7
                    label: "AP Living Room uplink"
                    state: up
                    linkSpeed: gbe2_5
                    linkUptime: 86400
                    poeMode: auto
                    poePowerWatts: 4.5
                    vlanConfig:
                      mode: trunk
                      nativeVlan:
                        id: "unifi.default"
                        uri: "/network/vlans/unifi.default"
                        name: "Default"
                        vlanId: 1
                      taggedVlans:
                        scope: custom
                        items:
                          - id: "unifi.iot"
                            uri: "/network/vlans/unifi.iot"
                            name: "IoT"
                            vlanId: 20
                    traffic:
                      rxBytesTotal: 12884901888
                      txBytesTotal: 4294967296
                      rxBytesPerSec: 125000
                      txBytesPerSec: 50000
                    connectedTo:
                      kind: device
                      id: "unifi.ap-living-room"
                      uri: "/network/devices/unifi.ap-living-room"
                      name: "AP Living Room"
                    switch:
                      kind: device
                      id: "unifi.switch-living-room"
                      uri: "/network/devices/unifi.switch-living-room"
                      name: "Switch Living Room"
                  - number: 2
                    state: down
                    poeMode: 'off'
                    vlanConfig:
                      mode: access
                      nativeVlan:
                        id: "unifi.servers"
                        uri: "/network/vlans/unifi.servers"
                        name: "Servers"
                        vlanId: 100
                    traffic:
                      rxBytesTotal: 0
                      txBytesTotal: 0
                      rxBytesPerSec: 0
                      txBytesPerSec: 0
                    switch:
                      kind: device
                      id: "unifi.switch-living-room"
                      uri: "/network/devices/unifi.switch-living-room"
                      name: "Switch Living Room"
                  - number: 8
                    state: up
                    linkSpeed: gbe10
                    linkUptime: 3600
                    sfpModulePresent: true
                    poeMode: 'off'
                    lagMembership:
                      id: 8
                      role: master
                    vlanConfig:
                      mode: access
                      nativeVlan:
                        id: "unifi.servers"
                        uri: "/network/vlans/unifi.servers"
                        name: "Servers"
                        vlanId: 100
                    traffic:
                      rxBytesTotal: 5497558138
                      txBytesTotal: 2748779069
                      rxBytesPerSec: 800000
                      txBytesPerSec: 100000
                    connectedTo:
                      kind: client
                      id: "unifi.nas-1-68"
                      uri: "/network/clients/unifi.nas-1-68"
                      name: "nas-1"
                    switch:
                      kind: device
                      id: "unifi.switch-living-room"
                      uri: "/network/devices/unifi.switch-living-room"
                      name: "Switch Living Room"
    "400":
      $ref: "../components/responses/BadRequest.yaml"
    "401":
      $ref: "../components/responses/Unauthorized.yaml"
    "403":
      $ref: "../components/responses/Forbidden.yaml"
    "429":
      $ref: "../components/responses/TooManyRequests.yaml"
    "500":
      $ref: "../components/responses/InternalServerError.yaml"
```

- [ ] **Step 4: Wire the new path into the root document**

Modify `spec/openapi/openapi.yaml`. Under `paths:`, insert `/network/ports` immediately after the existing `/network/devices/{deviceId}` entry so ports live next to devices:

```yaml
  /network/devices/{deviceId}:
    $ref: "./paths/network-devices-id.yaml"
  /network/ports:
    $ref: "./paths/network-ports.yaml"
  /network/clients:
    $ref: "./paths/network-clients.yaml"
```

- [ ] **Step 5: Run spec lint and verify clean**

Run inside the spec submodule:

```bash
cd spec && make lint
```

Expected: Redocly lint of the source spec and Spectral lint of the bundled artifact both pass with no errors. Warnings that already existed on `main` are acceptable; new warnings tied to the added files are not.

If lint reports an issue on the new files, fix it before proceeding. Common failure modes: missing `required:` on a schema, wrong `$ref` relative path, missing `description` on a parameter.

- [ ] **Step 6: Commit inside the spec submodule**

```bash
cd spec
git add openapi/components/schemas/network/NetworkPort.yaml \
        openapi/components/schemas/network/NetworkPortList.yaml \
        openapi/paths/network-ports.yaml \
        openapi/openapi.yaml
git commit -m "feat: add /network/ports listing endpoint"
```

- [ ] **Step 7: Open a spec PR, merge, and wait for release**

Open the PR against `main` in the `homelab-api-spec` repo. Once merged, semantic-release will cut a new minor version (e.g. `1.4.0`) and publish a Git tag. Note the resulting commit SHA on the spec's `main` — Task 2 will bump the submodule to that SHA.

---

## Task 2: Client — submodule bump, regenerate, extend `NetworkClient` interface

**Repo:** client repo (this repository).

**Files:**
- Modify: `spec` (submodule pointer only)
- Modify: `internal/api/network/api.gen.go` (regenerated; verified but not hand-edited)
- Modify: `internal/cli/network/client.go` (add one method to the `NetworkClient` interface)

**Interfaces:**
- Consumes: the spec release from Task 1 (specifically, the commit SHA on the spec's `main`).
- Produces: `networkapi.ListNetworkPortsWithResponse(ctx, params, editors...)` callable on the generated client, and the same method exposed on the `network.NetworkClient` interface so consumers can be typed against it. Also produces the generated types `networkapi.ListNetworkPortsParams`, `networkapi.NetworkPort`, `networkapi.NetworkPortList`, and `networkapi.ListNetworkPortsResponse` (with `JSON200 *NetworkPortList`).

- [ ] **Step 1: Bump the submodule pointer**

```bash
git submodule update --remote spec
git -C spec log --oneline -1
```

Verify the top spec commit is the merge commit from Task 1. If it's older, ensure the spec release workflow has completed and re-run.

- [ ] **Step 2: Regenerate the network client**

```bash
make generate
```

Expected: no errors; `internal/api/network/api.gen.go` is rewritten (gitignored; not part of the commit).

- [ ] **Step 3: Verify the generated symbols exist**

```bash
grep -E 'func \(c \*ClientWithResponses\) ListNetworkPortsWithResponse|type ListNetworkPortsParams struct|type NetworkPort struct|type NetworkPortList struct|type ListNetworkPortsResponse struct' internal/api/network/api.gen.go
```

Expected: five matches, one per grep alternative. If any is missing, revisit Task 1 (the schema/path may not have wired correctly) or the codegen config in `codegen/network.yaml`.

Also note the field names on `ListNetworkPortsParams` — oapi-codegen normalizes query param names to `SwitchId`, `Mode`, `State`, `VlanId` (Go export case). If the exact spelling differs, Task 3 code must match the generator's output.

- [ ] **Step 4: Extend the `NetworkClient` interface**

Modify `internal/cli/network/client.go`. Add one line to the interface, next to the other network methods (alphabetical is not required; place it after `GetNetworkDeviceWithResponse`):

```go
	GetNetworkDeviceWithResponse(ctx context.Context, deviceId string, reqEditors ...networkapi.RequestEditorFn) (*networkapi.GetNetworkDeviceResponse, error)
	ListNetworkPortsWithResponse(ctx context.Context, params *networkapi.ListNetworkPortsParams, reqEditors ...networkapi.RequestEditorFn) (*networkapi.ListNetworkPortsResponse, error)
	ListNetworkClientsWithResponse(ctx context.Context, params *networkapi.ListNetworkClientsParams, reqEditors ...networkapi.RequestEditorFn) (*networkapi.ListNetworkClientsResponse, error)
```

- [ ] **Step 5: Verify `go vet` and existing tests still pass**

```bash
make lint && go test ./internal/cli/network/...
```

Expected: both pass. The generated concrete client satisfies the extended interface automatically; no other callers of `NewNetworkClient` should regress.

- [ ] **Step 6: Commit**

```bash
git add spec internal/cli/network/client.go
git commit -m "chore: update homelab-api-spec submodule"
```

The commit message intentionally does not mention the new endpoint — the next task's commit describes the feature. This matches existing history (`d8cd47b chore: update homelab-api-spec submodule`).

---

## Task 3: Client — `ports list` command, template, tests, shared decoration

**Repo:** client repo.

**Files:**
- Create: `internal/cli/network/ports.go`
- Create: `internal/cli/network/ports_test.go`
- Create: `internal/cli/network/templates/ports_list.tmpl`
- Modify: `internal/cli/network/network.go` (register `newPortsCmd`)
- Modify: `internal/cli/network/devices.go` (route through the extracted `decoratePort`)

**Interfaces:**
- Consumes: `networkapi.ListNetworkPortsWithResponse`, `networkapi.ListNetworkPortsParams`, `networkapi.NetworkPort`, `networkapi.NetworkPortList`, `networkapi.SwitchPort` (existing), all from Task 2.
- Produces: the `hlctl network ports list` subcommand and the internal helper `func decoratePort(p portInput) (portRow, error)` used by both `ports.go` and `devices.go`.

TDD flow: Layer 1 wiring tests first (fails at "runF hook not invoked"), then implementation to green, then Layer 2 behavioural tests with `httpmock`, then implementation to green, then extract the shared `decoratePort` and route `devices.go` through it (existing device tests keep passing).

- [ ] **Step 1: Write Layer 1 wiring tests (failing)**

Create `internal/cli/network/ports_test.go` with only the Layer 1 cases first. This file will grow in later steps.

```go
package network

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

// Layer 1: Cobra wiring — runF hook

func TestNewListPortsCmd_runFCalled(t *testing.T) {
	called := false
	cmd := newListPortsCmd(cmdutil.TestFactory(t), func(o *listPortsOptions) error {
		called = true
		return nil
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected runF to be called")
	}
}

func TestNewListPortsCmd_flagsCaptured(t *testing.T) {
	var captured *listPortsOptions
	cmd := newListPortsCmd(cmdutil.TestFactory(t), func(o *listPortsOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{
		"--switch", "unifi.switch-living-room",
		"--mode", "trunk",
		"--state", "up",
		"--vlan", "20",
		"--wide",
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected runF to be called")
	}
	if captured.Switch != "unifi.switch-living-room" {
		t.Errorf("Switch: got %q", captured.Switch)
	}
	if captured.Mode != "trunk" {
		t.Errorf("Mode: got %q", captured.Mode)
	}
	if captured.State != "up" {
		t.Errorf("State: got %q", captured.State)
	}
	if captured.VlanID != 20 {
		t.Errorf("VlanID: got %d", captured.VlanID)
	}
	if !captured.Wide {
		t.Error("Wide: expected true")
	}
}

func TestNewListPortsCmd_allPortsAndStateMutuallyExclusive(t *testing.T) {
	cmd := newListPortsCmd(cmdutil.TestFactory(t), func(o *listPortsOptions) error {
		t.Fatal("runF should not be called when flags conflict")
		return nil
	})
	cmd.SetArgs([]string{"--all-ports", "--state", "up"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %v", err)
	}
}

func TestNewListPortsCmd_invalidModeRejected(t *testing.T) {
	cmd := newListPortsCmd(cmdutil.TestFactory(t), func(o *listPortsOptions) error {
		t.Fatal("runF should not be called on invalid mode")
		return nil
	})
	cmd.SetArgs([]string{"--mode", "garbage"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNewListPortsCmd_invalidStateRejected(t *testing.T) {
	cmd := newListPortsCmd(cmdutil.TestFactory(t), func(o *listPortsOptions) error {
		t.Fatal("runF should not be called on invalid state")
		return nil
	})
	cmd.SetArgs([]string{"--state", "garbage"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
```

- [ ] **Step 2: Run wiring tests to verify they fail**

```bash
go test ./internal/cli/network/... -run 'TestNewListPortsCmd' -v
```

Expected: fail with `undefined: newListPortsCmd`, `undefined: listPortsOptions`. This proves the harness sees no ports command yet.

- [ ] **Step 3: Implement the command wiring**

Create `internal/cli/network/ports.go`:

```go
package network

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"

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

	Switch   string
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
			if err := validateEnum("state", opts.State, "up", "down"); err != nil {
				return err
			}
			if runF != nil {
				return runF(opts)
			}
			return listPortsRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
	cmd.Flags().StringVar(&opts.Switch, "switch", "", "Filter by switch device ID")
	cmd.Flags().StringVar(&opts.Mode, "mode", "", "Filter by VLAN mode (trunk|access)")
	cmd.Flags().StringVar(&opts.State, "state", "", "Filter by link state (up|down)")
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
```

- [ ] **Step 4: Run wiring tests to verify they pass**

```bash
go test ./internal/cli/network/... -run 'TestNewListPortsCmd' -v
```

Expected: all pass. If `TestNewListPortsCmd_runFCalled` fails with "template not found" or similar, it means the harness fell through to `listPortsRun`; recheck that `runF` is honoured before touching HTTP.

- [ ] **Step 5: Write Layer 2 behavioural tests (failing)**

Append to `internal/cli/network/ports_test.go`. The fixture mirrors the switch example in `network-devices-id.yaml`, adapted to the flat listing shape with `switch` refs.

```go
// Layer 2: business logic via httpmock

var portsFixture = map[string]any{
	"items": []any{
		map[string]any{
			"number":    1,
			"state":     "up",
			"linkSpeed": "gbe1",
			"linkUptime": 172800,
			"poeMode":    "off",
			"vlanConfig": map[string]any{
				"mode": "trunk",
				"nativeVlan": map[string]any{
					"id": "unifi.default", "uri": "/network/vlans/unifi.default",
					"name": "Default", "vlanId": 1,
				},
				"taggedVlans": map[string]any{"scope": "all"},
			},
			"traffic": map[string]any{
				"rxBytesTotal": 43980465111, "txBytesTotal": 87960930222,
				"rxBytesPerSec": 600000, "txBytesPerSec": 1250000,
			},
			"connectedTo": map[string]any{
				"kind": "device", "id": "unifi.usg",
				"uri": "/network/devices/unifi.usg", "name": "USG",
			},
			"switch": map[string]any{
				"kind": "device", "id": "unifi.switch-living-room",
				"uri": "/network/devices/unifi.switch-living-room",
				"name": "Switch Living Room",
			},
		},
		map[string]any{
			"number": 7, "label": "AP uplink",
			"state": "up", "linkSpeed": "gbe2_5", "linkUptime": 86400,
			"poeMode": "auto", "poePowerWatts": 4.5,
			"vlanConfig": map[string]any{
				"mode": "trunk",
				"nativeVlan": map[string]any{
					"id": "unifi.default", "uri": "/network/vlans/unifi.default",
					"name": "Default", "vlanId": 1,
				},
				"taggedVlans": map[string]any{
					"scope": "custom",
					"items": []any{
						map[string]any{
							"id": "unifi.iot", "uri": "/network/vlans/unifi.iot",
							"name": "IoT", "vlanId": 20,
						},
					},
				},
			},
			"traffic": map[string]any{
				"rxBytesTotal": 12884901888, "txBytesTotal": 4294967296,
				"rxBytesPerSec": 125000, "txBytesPerSec": 50000,
			},
			"connectedTo": map[string]any{
				"kind": "device", "id": "unifi.ap-living-room",
				"uri": "/network/devices/unifi.ap-living-room", "name": "AP Living Room",
			},
			"switch": map[string]any{
				"kind": "device", "id": "unifi.switch-living-room",
				"uri": "/network/devices/unifi.switch-living-room",
				"name": "Switch Living Room",
			},
		},
		map[string]any{
			"number": 8, "state": "up", "linkSpeed": "gbe10", "linkUptime": 3600,
			"sfpModulePresent": true, "poeMode": "off",
			"lagMembership": map[string]any{"id": 8, "role": "master"},
			"vlanConfig": map[string]any{
				"mode": "access",
				"nativeVlan": map[string]any{
					"id": "unifi.servers", "uri": "/network/vlans/unifi.servers",
					"name": "Servers", "vlanId": 100,
				},
			},
			"traffic": map[string]any{
				"rxBytesTotal": 5497558138, "txBytesTotal": 2748779069,
				"rxBytesPerSec": 800000, "txBytesPerSec": 100000,
			},
			"connectedTo": map[string]any{
				"kind": "client", "id": "unifi.nas-1-68",
				"uri": "/network/clients/unifi.nas-1-68", "name": "nas-1",
			},
			"switch": map[string]any{
				"kind": "device", "id": "unifi.switch-attic",
				"uri": "/network/devices/unifi.switch-attic",
				"name": "Switch Attic",
			},
		},
	},
}

func TestListPortsRun_tableDefault(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), httpmock.JSONResponse(portsFixture))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listPortsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"SWITCH", "PORT", "LABEL", "STATE", "MODE", "NATIVE VLAN", "TAGGED VLANS", "CONNECTED TO",
		"Switch Living Room", "Switch Attic",
		"AP uplink", "trunk", "access",
		"Default (1)", "Servers (100)",
		"all", "IoT (20)",
		"USG", "AP Living Room", "nas-1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
	// --wide columns must be absent by default
	for _, absent := range []string{"LINK SPEED", "SFP", "RX/S", "TX/S"} {
		if strings.Contains(got, absent) {
			t.Errorf("expected %q absent in default output, got:\n%s", absent, got)
		}
	}
	// Sort order: Switch Attic must appear before Switch Living Room.
	if strings.Index(got, "Switch Attic") > strings.Index(got, "Switch Living Room") {
		t.Errorf("expected switches sorted by name (attic before living room), got:\n%s", got)
	}
	reg.Verify(t)
}

func TestListPortsRun_tableWide(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), httpmock.JSONResponse(portsFixture))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		Wide:       true,
	}
	if err := listPortsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	for _, want := range []string{"LINK SPEED", "POE", "LAG", "SFP", "RX/S", "TX/S", "master #8"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
	reg.Verify(t)
}

// captureURL wraps httpmock.JSONResponse to record the request URL for
// later query-param assertions. httpmock.Registry does not expose a
// Requests() accessor, so the test closes over a local variable.
func captureURL(store **url.URL, body any) httpmock.Responder {
	inner := httpmock.JSONResponse(body)
	return func(req *http.Request) (*http.Response, error) {
		*store = req.URL
		return inner(req)
	}
}

func TestListPortsRun_defaultStateFilter(t *testing.T) {
	var reqURL *url.URL
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), captureURL(&reqURL, portsFixture))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listPortsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqURL == nil {
		t.Fatal("expected request URL to be captured")
	}
	if got := reqURL.Query().Get("state"); got != "up" {
		t.Errorf("expected default state=up on the wire, got %q", got)
	}
	reg.Verify(t)
}

func TestListPortsRun_allPortsOmitsState(t *testing.T) {
	var reqURL *url.URL
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), captureURL(&reqURL, portsFixture))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		AllPorts:   true,
	}
	if err := listPortsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqURL == nil {
		t.Fatal("expected request URL to be captured")
	}
	if reqURL.Query().Has("state") {
		t.Errorf("expected no state query param with --all-ports, got %q", reqURL.Query().Get("state"))
	}
	reg.Verify(t)
}

func TestListPortsRun_allQueryParams(t *testing.T) {
	var reqURL *url.URL
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), captureURL(&reqURL, portsFixture))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		Switch:     "unifi.switch-living-room",
		Mode:       "trunk",
		State:      "down",
		VlanID:     20,
	}
	if err := listPortsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqURL == nil {
		t.Fatal("expected request URL to be captured")
	}
	q := reqURL.Query()
	cases := map[string]string{
		"switchId": "unifi.switch-living-room",
		"mode":     "trunk",
		"state":    "down",
		"vlanId":   "20",
	}
	for k, want := range cases {
		if got := q.Get(k); got != want {
			t.Errorf("query param %s: got %q, want %q", k, got, want)
		}
	}
	reg.Verify(t)
}

func TestListPortsRun_jsonPassthrough(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), httpmock.JSONResponse(portsFixture))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatJSON },
	}
	if err := listPortsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	// Raw body echoed, table headers absent.
	if strings.Contains(got, "SWITCH") || strings.Contains(got, "NATIVE VLAN") {
		t.Errorf("expected JSON passthrough (no table headers), got:\n%s", got)
	}
	if !strings.Contains(got, `"switch"`) || !strings.Contains(got, "unifi.switch-living-room") {
		t.Errorf("expected raw JSON body, got:\n%s", got)
	}
	reg.Verify(t)
}

func TestListPortsRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/ports"), httpmock.StatusJSONResponse(http.StatusUnauthorized, map[string]any{
		"type":   "https://homelab.local/problems/unauthorized",
		"title":  "Unauthorized",
		"status": 401,
		"detail": "Bearer token missing",
	}))

	var out bytes.Buffer
	opts := &listPortsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	err := listPortsRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
	reg.Verify(t)
}
```

Verify that `httpmock.Registry` exposes a `Requests()` method and that mock requests capture their URLs. If the method name differs (check `internal/cli/cmdutil/httpmock/`), adapt the assertion (an alternative is a custom matcher that records URLs, but the existing package likely already supports this).

- [ ] **Step 6: Run Layer 2 tests to verify they fail**

```bash
go test ./internal/cli/network/... -run 'TestListPortsRun' -v
```

Expected: fail with `undefined: listPortsRun` and missing template. This confirms the tests can't accidentally pass before implementation.

- [ ] **Step 7: Implement `listPortsRun`, the port row shape, and `decoratePort`**

Append to `internal/cli/network/ports.go`:

```go
// portRow is the display-ready shape rendered by the ports_list template.
// It intentionally holds only string/scalar fields so the template stays
// free of Go type conversions.
type portRow struct {
	SwitchName      string
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
// response) and SwitchPort (nested in device detail) can be adapted to
// this shape; the decoration logic is centralized in decoratePort.
type portInput struct {
	SwitchName  string
	SwitchPort  networkapi.SwitchPort
}

func decoratePort(in portInput) (portRow, error) {
	p := in.SwitchPort

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
		if p.VlanConfig.Mode == networkapi.SwitchPortVlanConfigModeTrunk && p.VlanConfig.TaggedVlans != nil {
			switch p.VlanConfig.TaggedVlans.Scope {
			case networkapi.SwitchPortVlanConfigTaggedVlansScopeAll:
				taggedVlans = "all"
			case networkapi.SwitchPortVlanConfigTaggedVlansScopeCustom:
				if p.VlanConfig.TaggedVlans.Items != nil && len(*p.VlanConfig.TaggedVlans.Items) > 0 {
					parts := make([]string, 0, len(*p.VlanConfig.TaggedVlans.Items))
					for _, v := range *p.VlanConfig.TaggedVlans.Items {
						parts = append(parts, fmt.Sprintf("%s (%d)", v.Name, v.VlanId))
					}
					taggedVlans = joinComma(parts)
				}
			}
		}
	}

	return portRow{
		SwitchName:      in.SwitchName,
		Number:          p.Number,
		Label:           label,
		State:           string(p.State),
		LinkSpeed:       linkSpeed,
		VlanMode:        vlanMode,
		NativeVlan:      nativeVlan,
		TaggedVlans:     taggedVlans,
		PoeMode:         string(p.PoeMode),
		PoePowerWatts:   poePowerWatts,
		LagInfo:         lagInfo,
		SfpPresent:      sfpPresent,
		LinkUptime:      linkUptime,
		RxPerSec:        output.FormatBytesPerSec(p.Traffic.RxBytesPerSec),
		TxPerSec:        output.FormatBytesPerSec(p.Traffic.TxBytesPerSec),
		ConnectedToName: connectedTo,
	}, nil
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
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
				SwitchName: p.Switch.Name,
				SwitchPort: networkapi.SwitchPort{
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
			if rows[i].SwitchName != rows[j].SwitchName {
				return rows[i].SwitchName < rows[j].SwitchName
			}
			return rows[i].Number < rows[j].Number
		})
		return portsListData{Wide: opts.Wide, Rows: rows}, nil
	})
}

func buildListPortsParams(opts *listPortsOptions) *networkapi.ListNetworkPortsParams {
	params := &networkapi.ListNetworkPortsParams{}
	if opts.Switch != "" {
		s := opts.Switch
		params.SwitchId = &s
	}
	if opts.Mode != "" {
		m := networkapi.ListNetworkPortsParamsMode(opts.Mode)
		params.Mode = &m
	}
	// State resolution: --all-ports omits state; explicit --state honoured;
	// otherwise default to "up".
	state := opts.State
	if !opts.AllPorts && state == "" {
		state = "up"
	}
	if !opts.AllPorts && state != "" {
		s := networkapi.ListNetworkPortsParamsState(state)
		params.State = &s
	}
	if opts.VlanID > 0 {
		v := opts.VlanID
		params.VlanId = &v
	}
	return params
}
```

Notes on generated types: `networkapi.ListNetworkPortsParamsMode` and `networkapi.ListNetworkPortsParamsState` are the typical inline-enum types oapi-codegen emits for query-param enums. If the actual generator output differs (e.g. the enum type is reused across params, giving a different type name), adjust the type name in `buildListPortsParams` to match — the semantics stay the same. Similarly, the `portInput` adapter's struct-literal field list must match the actual `networkapi.SwitchPort` field set; drop or rename fields to match the generated code.

- [ ] **Step 8: Create the template**

Create `internal/cli/network/templates/ports_list.tmpl`:

```
{{ if .Wide -}}
SWITCH	PORT	LABEL	STATE	LINK SPEED	MODE	NATIVE VLAN	TAGGED VLANS	POE	LAG	SFP	RX/S	TX/S	CONNECTED TO
{{ range .Rows -}}
{{ .SwitchName }}	{{ .Number }}	{{ .Label }}	{{ .State }}	{{ .LinkSpeed }}	{{ .VlanMode }}	{{ .NativeVlan }}	{{ .TaggedVlans }}	{{ .PoeMode }}	{{ .LagInfo }}	{{ .SfpPresent }}	{{ .RxPerSec }}	{{ .TxPerSec }}	{{ .ConnectedToName }}
{{ end -}}
{{ else -}}
SWITCH	PORT	LABEL	STATE	MODE	NATIVE VLAN	TAGGED VLANS	CONNECTED TO
{{ range .Rows -}}
{{ .SwitchName }}	{{ .Number }}	{{ .Label }}	{{ .State }}	{{ .VlanMode }}	{{ .NativeVlan }}	{{ .TaggedVlans }}	{{ .ConnectedToName }}
{{ end -}}
{{ end -}}
```

Tabs between columns (do not use spaces — the tabwriter aligns on tab). Ensure a trailing newline at end of file.

- [ ] **Step 9: Register the command**

Modify `internal/cli/network/network.go`:

```go
func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Network devices and clients",
	}
	cmd.AddCommand(newDevicesCmd(f), newClientsCmd(f), newTopologyCmd(f, nil), newVlansCmd(f), newSsidsCmd(f), newWansCmd(f), newPortsCmd(f))
	return cmd
}
```

- [ ] **Step 10: Run Layer 2 tests to verify they pass**

```bash
go test ./internal/cli/network/... -run 'TestListPortsRun' -v
```

Expected: all pass. If a `wide` test fails on missing `master #8`, check the `--wide` branch renders `LagInfo`. If sort test fails, re-check the `sort.SliceStable` comparator.

- [ ] **Step 11: Route `devices.go` through the shared `decoratePort`**

The existing `buildSwitchPortViews` in `internal/cli/network/devices.go` duplicates the decoration logic now in `decoratePort`. Replace its body so it becomes a thin wrapper: filter for `--all-ports`, then call `decoratePort` per port.

Modify `internal/cli/network/devices.go` — replace the current `buildSwitchPortViews` implementation with:

```go
// buildSwitchPortViews filters and decorates a switch's ports for display.
// When allPorts is false, only ports with state "up" are returned. Delegates
// per-port decoration to decoratePort so devices get and ports list share
// the same rendering rules.
func buildSwitchPortViews(ports []networkapi.SwitchPort, allPorts bool) ([]switchPortView, error) {
	var out []switchPortView
	for _, p := range ports {
		if !allPorts && p.State != networkapi.NetworkPortStateUp {
			continue
		}
		row, err := decoratePort(portInput{SwitchName: "", SwitchPort: p})
		if err != nil {
			return nil, err
		}
		out = append(out, switchPortView{
			SwitchPort:      p,
			ConnectedToName: row.ConnectedToName,
			Label:           row.Label,
			LinkUptime:      row.LinkUptime,
			LagInfo:         row.LagInfo,
			SfpPresent:      row.SfpPresent,
			VlanMode:        row.VlanMode,
			NativeVlan:      row.NativeVlan,
			TaggedVlans:     row.TaggedVlans,
		})
	}
	return out, nil
}
```

Delete the `strings` import from `devices.go` (`joinComma` in `ports.go` now replaces `strings.Join`). If any other function in `devices.go` still uses `strings`, keep the import.

- [ ] **Step 12: Run all network tests to verify no regressions**

```bash
go test ./internal/cli/network/... -v
```

Expected: every test (ports, devices, vlans, clients, topology, ssids, wans) passes. If `devices_test.go` fails, the delegated behaviour drifted from the original — compare the `switchPortView` values field-by-field.

- [ ] **Step 13: Run full lint and full test suite**

```bash
make lint && go test ./...
```

Expected: `go vet` clean; every package's tests pass.

- [ ] **Step 14: Manual smoke (optional but recommended)**

Build the binary and issue a `--help` to confirm the command surfaces:

```bash
make build
./bin/hlctl network ports list --help
```

Expected output includes the flags `--switch`, `--mode`, `--state`, `--vlan`, `--wide`, `--all-ports`.

- [ ] **Step 15: Commit**

```bash
git add internal/cli/network/ports.go \
        internal/cli/network/ports_test.go \
        internal/cli/network/templates/ports_list.tmpl \
        internal/cli/network/network.go \
        internal/cli/network/devices.go
git commit -m "feat: add network ports list command"
```

---

## Post-plan verification

After Task 3 lands, confirm end-to-end:

- `hlctl network ports list` returns a table when the server is running the new endpoint.
- `hlctl network ports list --vlan 20 --output=json` returns only VLAN-20-bearing ports as raw JSON.
- `hlctl network devices get unifi.switch-living-room` renders identically to before this plan (regression sanity check on the shared decoration).

If any of these fail, roll back the client PR — the spec release is safe to keep since it's additive.
