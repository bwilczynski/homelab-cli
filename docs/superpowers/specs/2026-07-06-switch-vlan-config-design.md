# Switch Port VLAN Config & Detail Fields — Design

**Date:** 2026-07-06
**Spec version:** homelab-api-spec v1.3.0
**Branch:** feat/switch-vlan-config

## Overview

Extend the `hlctl network devices get <id>` switch view to surface five new fields introduced in API spec v1.3.0: per-port VLAN policy (`vlanConfig`), link uptime (`linkUptime`), LAG membership (`lagMembership`), operator label (`label`), and SFP module presence (`sfpModulePresent`).

## API Changes (v1.3.0)

New optional fields on `SwitchPort`:

| Field | Type | Description |
|---|---|---|
| `label` | string | Operator-assigned port label |
| `linkUptime` | integer (seconds) | Seconds since link came up; only when state=up |
| `sfpModulePresent` | boolean | SFP module inserted; only on SFP-capable ports |
| `lagMembership` | `SwitchPortLagMembership` | LAG bond id + role (master\|member) |
| `vlanConfig` | `SwitchPortVlanConfig` | VLAN policy: mode (access\|trunk), nativeVlan, taggedVlans |

New schemas: `SwitchPortVlanConfig`, `SwitchPortLagMembership`, `NetworkVlanRef`.

## Layout

Option B: two-section layout in a single template, both sections controlled by `--all-ports`.

### Summary table (existing, extended)

```
PORT  LABEL  STATE  SPEED  UPTIME  SFP  LAG  POE  POE W  RX  TX  CONNECTED TO
```

New columns added after SPEED, before POE.

### VLAN CONFIG block (new)

Appended after the port table, separated by `{{ flush }}`:

```
--- VLAN CONFIG ---
PORT  MODE    NATIVE VLAN   TAGGED VLANS
1     trunk   Default (1)   all
7     trunk   Default (1)   IoT (20), Guest (30)
2     access  Default (1)   -
```

Both sections iterate the same `range .Ports` slice — the `--all-ports` flag controls which ports are included in both.

## Data Layer

`switchPortView` (in `internal/cli/network/devices.go`) gains pre-resolved string fields, populated in `buildSwitchPortViews`:

| Field | Source | Nil/absent renders as |
|---|---|---|
| `Label` | `SwitchPort.Label` | `-` |
| `LinkUptime` | `SwitchPort.LinkUptime` | `-` |
| `LagInfo` | `SwitchPort.LagMembership` | `-` |
| `SfpPresent` | `SwitchPort.SfpModulePresent` | `-` |
| `VlanMode` | `SwitchPort.VlanConfig.Mode` | `-` |
| `NativeVlan` | `VlanConfig.NativeVlan.Name + ID` | `-` |
| `TaggedVlans` | `VlanConfig.TaggedVlans` | `-` |

All fields are pre-resolved strings — no logic in the template.

**Formatting rules:**

- `LinkUptime`: pre-formatted string in `buildSwitchPortViews` by calling `output.FormatUptime()` directly — consistent with the rest of the pre-resolved string fields on `switchPortView`
- `LagInfo`: `master #<id>` or `member #<id>`
- `SfpPresent`: `yes` / `no` (omitted ports render `-`)
- `NativeVlan`: `<Name> (<vlanId>)`, e.g. `Default (1)`
- `TaggedVlans`: `all` when scope=all; comma-joined `<Name> (<vlanId>)` when scope=custom with items; `-` for access mode or empty custom list

## Template

File: `internal/cli/network/templates/devices_get_switch.tmpl`

Single template with two sections separated by `{{ flush }}`:

1. Device base header (unchanged)
2. Port summary table — extended with LABEL, UPTIME, SFP, LAG columns
3. VLAN CONFIG block — new section iterating the same `.Ports` slice

## Template Helpers

All required helpers already exist in `internal/output/output.go` and are registered in the global template func map:

- `formatUptime(seconds int) string` — already used in `device_base.tmpl`, `wans_list.tmpl`, and client templates
- `formatLinkSpeed`, `formatBytesPerSec`, `derefStr`, `derefFloat` — unchanged

No new helpers needed.

## Testing

File: `internal/cli/network/devices_test.go`

Layer 2 tests (run function directly with mock HTTP):

- Trunk port, `scope: all` — VLAN CONFIG shows `all` in TAGGED VLANS
- Trunk port, `scope: custom` with items — shows comma-joined VLAN names+IDs
- Access port — TAGGED VLANS shows `-`
- Port with no `vlanConfig` — all VLAN columns show `-`
- Port with `lagMembership`, role=master — LAG shows `master #<id>`
- Port with `lagMembership`, role=member — LAG shows `member #<id>`
- Port with `linkUptime` — UPTIME formatted correctly
- Port with `sfpModulePresent: true` — SFP shows `yes`

## Implementation Steps

1. Run `make generate` to regenerate API client with v1.3.0 schemas
2. Extend `switchPortView` with new string fields
3. Update `buildSwitchPortViews` to populate new fields (add `formatUptime` helper)
4. Update `devices_get_switch.tmpl` — add columns to summary table, append VLAN CONFIG block
5. Add/extend tests in `devices_test.go`
