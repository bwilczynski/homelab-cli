# Network Ports Listing — Design

## Problem

`hlctl network devices get <switch>` returns rich per-port `vlanConfig`, PoE, LAG, SFP, traffic, and `connectedTo` data. Auditing that data across a homelab requires fanning out one call per switch and post-processing with `jq` — there is no single command that answers questions like "which ports carry VLAN 30?" or "show me every trunk port on every switch."

The gap has two parts:

1. **API contract** — no listing endpoint for ports; ports are only reachable inside a device detail response for switches.
2. **Client** — even with a single-endpoint API, the CLI needs a first-class `network ports list` command with the filters and column set an audit workflow expects.

This document specifies both changes.

## Scope

**In scope**

- New `GET /network/ports` endpoint in `homelab-api-spec` returning a flat list of switch ports across all controllers.
- New `hlctl network ports list` command with server-side filter flags and an audit-focused table view.
- Reuse of the existing per-port decoration logic (VLAN formatting, connected-to name resolution) between `devices get` and `ports list`.

**Out of scope (explicitly deferred)**

- `GET /network/ports/{id}` singular fetch. A composite port ID model was considered and declined; single-port detail remains available via `devices get <switch>`.
- Non-switch ports (gateway LAN ports, AP uplinks as first-class rows). Only `SwitchPort` exists today; if that changes, `/network/ports` can grow a discriminator without breaking additive changes.
- Pagination. Homelab scale does not justify it; matches the precedent set by `NetworkDeviceList`.
- Watch mode (`watch.Wrap`). Can be added later without redesign.

## Non-goals

- Modifying `GET /network/devices/{id}`. It stays as-is; `/network/ports` is additive.
- Introducing a new codegen domain. The endpoint belongs to the existing `network` domain.

## API contract change (`homelab-api-spec`)

### Path — `openapi/paths/network-ports.yaml`

- `GET /network/ports`
- `operationId: listNetworkPorts`
- `x-stability-level: draft`
- `tags: [network]`
- `security: [bearerAuth: [read:network]]`

**Query parameters** (all optional). Each is extracted to `openapi/components/parameters/` per repo convention (`DeviceFilter.yaml`, `UpdateStatusFilter.yaml`, etc.):

| Name | Type | Notes |
|---|---|---|
| `switchId` | string | Composite device ID (e.g. `unifi.switch-living-room`). Filters to ports belonging to that switch. Extracted to `parameters/SwitchIdFilter.yaml`. |
| `mode` | `$ref: SwitchPortVlanMode` | References a new shared enum schema `SwitchPortVlanMode.yaml` extracted from the inline enum currently on `SwitchPortVlanConfig.mode`. Ports with no `vlanConfig` never match. Extracted to `parameters/SwitchPortModeFilter.yaml`. |
| `state` | `$ref: NetworkPortState` | References the existing `NetworkPortState.yaml` enum — accepts `up`, `down`, and `disabled` (all three values from the schema). Extracted to `parameters/NetworkPortStateFilter.yaml`. |
| `vlanId` | integer, minimum 1 | Matches when the value equals `vlanConfig.nativeVlan.vlanId`, OR appears in `vlanConfig.taggedVlans.items[*].vlanId`, OR when `vlanConfig.taggedVlans.scope == "all"` on a trunk port. Ports without a `vlanConfig` never match. Extracted to `parameters/VlanIdFilter.yaml`. |

**Enum sourcing rationale:** referencing `NetworkPortState` directly (rather than inlining `[up, down]`) keeps a single source of truth and lets operators filter for administratively-disabled ports too. Extracting `SwitchPortVlanMode.yaml` and referencing it from both `SwitchPortVlanConfig.mode` and this filter eliminates the second enum-drift risk before a third caller appears.

**Response**

- `200` — `application/json` → `NetworkPortList` (schema below). No pagination.
- Standard errors — `400`, `401`, `403`, `429`, `500` (reuse existing shared responses).

**Example payload** should mirror the switch example in `network-devices-id.yaml` so the CLI can reuse fixtures.

**Description tone:** the endpoint description acknowledges both reasons `vlanConfig` can be omitted (administratively disabled *and* insufficient controller data), matching the schema. The description explicitly notes that filters `mode` and `vlanId` never match ports without a `vlanConfig`.

### Schema — `openapi/components/schemas/network/SwitchPortVlanMode.yaml` (new, extracted)

Standalone enum extracted from the inline enum currently on `SwitchPortVlanConfig.mode`. Contains the two-value enum `[access, trunk]` with the same description text. `SwitchPortVlanConfig.yaml` is updated to reference it (`allOf: [$ref: "./SwitchPortVlanMode.yaml"]` on the `mode` property).

### Schema — `openapi/components/schemas/network/NetworkPort.yaml`

`allOf` inherits every field from `SwitchPort.yaml` (no duplication) and adds:

- `switch` — `$ref` to the existing `NetworkDeviceRef.yaml`, using the same ref shape already used by `NetworkConnectionRef` for `connectedTo`. The property carries a brief description; the fact that `kind` is always `device` is guaranteed by the ref schema itself and is NOT restated here.

`required: [number, state, poeMode, traffic, switch]` — parent required set plus `switch`.

### Schema — `openapi/components/schemas/network/NetworkPortList.yaml`

`{ items: [NetworkPort] }`, `required: [items]`. Matches `NetworkClientList` tone: `items` description reads "Switch ports matching the query. Empty array, never null." — no repeated operation-level guidance, no sort note (that lives on the operation).

### Wiring

- Add the path ref to `openapi/openapi.yaml` under `paths:`.
- No new shared responses. Reuse `Unauthorized`, `Forbidden`, `TooManyRequests`, `InternalServerError`, `BadRequest`.
- Run `make lint` before committing.

### Versioning

Commit as `feat: add /network/ports listing endpoint`. Purely additive; existing `/network/devices/{id}` unchanged. No `BREAKING CHANGE` footer. Semantic-release cuts a minor bump.

## Client change (`hlctl`)

### Command tree

```
hlctl network
  devices
    list
    get <device-id>
  ports              (new)
    list             (new)
  ssids, vlans, wans, topology, clients, ...
```

Registered in `internal/cli/network/network.go` by adding `newPortsCmd(f)` to `AddCommand`.

### File — `internal/cli/network/ports.go`

```go
var portsListView = cmdutil.View{Templates: networkTemplates, Name: "ports_list.tmpl"}

type listPortsOptions struct {
    HTTPClient func() (*http.Client, string, error)
    IO         *cmdutil.IOStreams
    Output     func() output.Format

    Switch   string // --switch
    Mode     string // --mode  (trunk|access, cobra choice)
    State    string // --state (up|down, cobra choice)
    VlanID   int    // --vlan
    Wide     bool   // --wide
    AllPorts bool   // --all-ports (overrides default state=up)
}
```

Follows the Options + `runF` pattern documented in `CLAUDE.md`:

- `newPortsCmd(f *cmdutil.Factory) *cobra.Command` — parent, only child is `list`.
- `newListPortsCmd(f *cmdutil.Factory, runF func(*listPortsOptions) error) *cobra.Command` — constructor wires flags; `RunE` invokes `runF(opts)` when non-nil (test seam) or `listPortsRun(cmd.Context(), opts.IO.Out, opts)`.

### Filter semantics in `listPortsRun`

1. Resolve the state param:
   - If `--all-ports`, omit `state` on the wire.
   - Else if `--state` was explicitly set, use its value.
   - Else default to `state=up` (parity with `devices get`).
2. Build `ListNetworkPortsParams{ SwitchId, Mode, State, VlanId }` from `opts`.
3. Call `ListNetworkPortsWithResponse(ctx, &params)`.
4. Render via `portsListView.RenderWith(w, opts.Output(), resp.StatusCode(), resp.Body, buildPortRowViews)` so decoration runs only in table mode.

### Flag conflict rules

- `--all-ports` and `--state` are mutually exclusive (`cmd.MarkFlagsMutuallyExclusive("all-ports", "state")`).
- `--mode` accepts `trunk|access`.
- `--state` accepts `up|down|disabled` — mirrors the API filter which is now `$ref: NetworkPortState`.
- Choice validation is done in `RunE` via a small `validateEnum(name, value, allowed...)` helper.

### Generated Go type shift from the spec refactor

Extracting `SwitchPortVlanMode.yaml` in the spec repo renames the generated Go enum type. Callers that currently reference `networkapi.SwitchPortVlanConfigModeTrunk` (in `internal/cli/network/devices.go`) must switch to `networkapi.SwitchPortVlanModeTrunk` (or whatever oapi-codegen actually emits from the shared schema). This is a compile-time rename to catch during Task 3.

### Decoration reuse

The existing `buildSwitchPortViews` in `devices.go` contains ~40 lines of per-port decoration (connected-to name, VLAN mode/native/tagged formatting, LAG, SFP, uptime). Extract that body into `decoratePort` that consumes a small normalized input shape, and route both `devices get` and `ports list` through it. `NetworkPort` inherits `SwitchPort` fields structurally but oapi-codegen emits a distinct Go type, so a thin adapter constructs the normalized shape from either input.

### Template — `internal/cli/network/templates/ports_list.tmpl`

**Default columns** — `SWITCH  PORT  LABEL  STATE  MODE  NATIVE VLAN  TAGGED VLANS  CONNECTED TO`

**With `--wide`** — appends `LINK SPEED  POE  LAG  SFP  RX/S  TX/S`

**Sort order** — switch name ascending, then port number ascending. Predictable and stable across runs.

**`SWITCH` column** — renders `switch.name` from the embedded ref.

### JSON output

`--output=json` echoes the raw response body via the existing `View.Render` fast path. No decoration.

## Testing

Two-layer pattern from `CLAUDE.md`.

### Layer 1 — Cobra wiring (`ports_test.go`)

- `TestListPortsCmd_RunF` — pass a `runF` that flips a bool; execute with each flag combination (no flags, `--switch=X`, `--mode=trunk`, `--state=down`, `--vlan=30`, `--wide`, `--all-ports`). Assert the bool flipped and `opts` fields populated.
- `TestListPortsCmd_FlagConflict` — `--all-ports --state=up` errors out before `runF` fires.
- `TestListPortsCmd_InvalidChoice` — `--mode=garbage` and `--state=garbage` fail choice validation.

### Layer 2 — Run function against mock registry (`ports_test.go`)

Construct `opts` directly with `testHTTPClient(reg)` and `httpmock.NewRegistry()`. Register a `GET /network/ports` response with a fixture covering:

- Two switches
- A trunk port with `taggedVlans.scope == "all"`
- A trunk port with `taggedVlans.scope == "custom"` and multiple items
- An access port
- A down port with no `vlanConfig`
- One `connectedTo` device and one `connectedTo` client
- One port with `lagMembership` set and `sfpModulePresent: true`

Cases:

- `TestListPortsRun_Table` — default table renders expected columns and rows, sorted by (switch, port).
- `TestListPortsRun_TableWide` — `Wide: true` appends the extra columns.
- `TestListPortsRun_TableStateDefault` — `AllPorts: false, State: ""` sends `state=up` on the wire (verify from captured request URL).
- `TestListPortsRun_TableAllPorts` — `AllPorts: true` omits the `state` query param.
- `TestListPortsRun_QueryParams` — all four filters set → all four params on the wire.
- `TestListPortsRun_JSON` — `Output` returns `json`; raw body echoed, no decoration.
- `TestListPortsRun_ErrorStatus` — 401 renders via the shared View error path.

The fixture payload mirrors the switch example already in `network-devices-id.yaml`, doubling as a spec-conformance check.

Behavioural tests for `decoratePort` (VLAN scope rendering, connected-to name resolution) remain on that helper directly; table tests exercise it end-to-end but do not re-cover every branch.

## Rollout sequencing

The spec (`homelab-api-spec`) is a submodule of the client; they release independently.

1. **Spec PR** — path, schemas, root-doc wiring. `make lint`. Merge. Semantic-release cuts a new minor (e.g. `1.4.0`).
2. **Server implementation** (in `homelab-api`) — out of scope here; noted because end-to-end verification of the CLI depends on it. Client unit tests do not.
3. **Client submodule bump** — `git submodule update --remote spec`, commit as `chore: update homelab-api-spec submodule` (matches existing history).
4. **Regenerate client code** — `make generate` produces `ListNetworkPortsWithResponse`, `NetworkPort`, `NetworkPortList` in the gitignored `internal/api/network/api.gen.go`.
5. **Client PR** — `internal/cli/network/ports.go`, `templates/ports_list.tmpl`, `ports_test.go`, `decoratePort` extraction, `network.NewCmd` registration. `make lint` and `go test ./...` pass.

Steps 3–5 land in a single client PR so the submodule bump, generated symbols, and consuming code are atomic. The client PR should not merge before the spec release is published — otherwise a fresh `main` clone + `make generate` would fail against a stale pinned spec commit.

## Open questions

None at design time. Any surfaced during implementation should be raised in the plan document, not this spec.
