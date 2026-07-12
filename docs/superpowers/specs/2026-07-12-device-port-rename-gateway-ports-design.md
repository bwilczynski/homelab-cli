# DevicePort Rename & Gateway Ports — Design

**Spec version:** homelab-api-spec v1.5.0
**Scope:** Update hlctl for the `SwitchPort*` → `DevicePort*` schema rename, the `switch` → `device` field rename on `NetworkPort`, and the new `ports`/`wans` fields on `GatewayDetail`.

---

## Background

homelab-api-spec v1.5.0 (commit `5eef858`) makes the following breaking changes:

- Renames the `SwitchPort*` schema family to `DevicePort*` so the type is not switch-specific.
- Renames `NetworkPort.switch` → `NetworkPort.device` and the corresponding `switchId` query param → `deviceId`, since gateway LAN ports now appear in the flat `/network/ports` listing alongside switch ports.
- Adds `ports: [DevicePort]` and `wans: [WanRef]` as required fields on `GatewayDetail`.

After `make generate` the generated `networkapi` package reflects these renames, causing compile errors across `ports.go` and `devices.go` that must be fixed.

---

## Changes

### 1. Mechanical renames (`ports.go`, `devices.go`)

| Old | New |
|---|---|
| `networkapi.SwitchPort` | `networkapi.DevicePort` |
| `networkapi.SwitchPortVlanMode` | `networkapi.DevicePortVlanMode` |
| `networkapi.SwitchPortVlanModeTrunk` | `networkapi.DevicePortVlanModeTrunk` |
| `networkapi.SwitchPortVlanConfig…Scope…` constants | `networkapi.DevicePortVlanConfig…Scope…` |
| `portInput.SwitchName` | `portInput.DeviceName` |
| `buildSwitchPortViews` | `buildDevicePortViews` |
| `switchPortView.SwitchPort` (embedded field) | `switchPortView.DevicePort` |
| `p.Switch.Name` (in `listPortsRun`) | `p.Device.Name` |
| `params.SwitchId` | `params.DeviceId` |
| `--switch` flag on `ports list` | `--device` flag (breaking rename, no alias) |

### 2. Gateway detail view

`GatewayDetail` now carries `Ports []DevicePort` and `Wans []WanRef`. The gateway branch in `getDeviceRun` is extended to pre-decorate ports and surface wans.

**New struct in `devices.go`:**

```go
type gatewayDetailView struct {
    networkapi.GatewayDetail
    Ports []switchPortView
}
```

`switchPortView` is reused — it already holds all decorated port fields.

**Updated gateway resolver in `getDeviceRun`:**

```go
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
```

**`devices_get_gateway.tmpl`** gains two sections appended after existing header fields:

- **Ports table** — same columns as the switch port table (Number, Label, State, Speed, VLAN Mode, Native VLAN, Connected To). Controlled by `--all-ports` flag (default: active ports only), which already exists on `newGetDeviceCmd`.
- **WANs table** — two columns: `ID` and `NAME`, sourced from `WanRef.Id` / `WanRef.Name`.

---

## Out of scope

- Adding a `--device` flag to `ports list` for filtering by gateway ID was not previously possible (only switches were listed); the rename makes it work automatically — no new logic needed.
- No changes to `ports.go` templates; port rendering is shared via `decoratePort`.
- No changes to `wans.go` — `WanRef` is display-only here; full WAN detail is fetched from `/network/wans/{id}`.
