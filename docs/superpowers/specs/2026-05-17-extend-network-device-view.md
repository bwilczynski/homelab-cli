# Design: Extend Network Device/Client View

**Date:** 2026-05-17
**Spec change:** homelab-api-spec PR #11 — extended network models (switch ports, AP clients, traffic, topology)

## Overview

Update `hlctl network devices` and `hlctl network clients` CLI commands to expose the new fields introduced by spec PR #11: polymorphic device detail, per-device traffic stats, switch port lists, AP connected-client lists, and restructured wired/wireless client connection info.

## Code Generation

Run `make generate` after updating the spec submodule to regenerate `internal/network/api.gen.go`. New types required:

- `SwitchDetail`, `AccessPointDetail`, `GatewayDetail`, `UnknownDeviceDetail`
- `NetworkTraffic`, `SwitchPort`, `AccessPointClient`
- `NetworkConnection`, `WirelessConnection`
- `NetworkDeviceRef`, `NetworkClientRef`, `NetworkConnectionRef`
- `NetworkLinkSpeed`, `NetworkPortState`, `SwitchPortPoeMode`

## Commands

### `hlctl network devices list`

Remove the `CLIENTS` column. `numClients` no longer exists on the base `NetworkDevice` schema (moved to `AccessPointDetail`).

Columns: `ID | NAME | MAC | IP | TYPE | STATUS`

### `hlctl network devices get <id>`

**New flag:** `--all-ports` (bool, default `false`) — shows all switch ports; by default only `state: up` ports are shown.

**Table output — shared fields (all device types):**

```
FIELD           VALUE
ID              unifi.switch-living-room
NAME            Switch Living Room
MAC             aa:bb:cc:dd:ee:ff
IP              192.168.1.5
TYPE            switch
STATUS          connected
MODEL           USW-24-PoE
FIRMWARE        6.2.14
UPTIME          1d 2h 15m 0s
TRAFFIC RX      125.0 KB/s (12.0 GB total)
TRAFFIC TX      50.0 KB/s (4.0 GB total)
UPLINK          Switch Core (port 2, 1GbE)
```

`UPLINK` is omitted for gateways (root of topology). Format: `<device name> (port <n>, <link speed>)`. Port and link speed omitted when absent.

**Switch — `--- PORTS ---` section:**

Default: only ports with `state: up`. With `--all-ports`: all ports.

```
--- PORTS ---
PORT  STATE  SPEED   POE   POE WATTS  RX          TX          CONNECTED TO
1     up     1GbE    auto  8.5 W      1.2 KB/s    500 B/s     AP Living Room
7     up     2.5GbE  off   -          125.0 KB/s  50.0 KB/s   MacBook Pro
8     up     1GbE    auto  -          10.0 KB/s   5.0 KB/s    nas-1
```

- `SPEED` omitted (shown as `-`) when port is not `up`
- `POE WATTS` shown as `-` when poeMode is `off` or no device attached
- `CONNECTED TO` shows device or client name from `NetworkConnectionRef`; `-` when absent
- Traffic columns: current rate only in ports table (both rate and total in device header)

**Access point — `--- CLIENTS ---` section:**

```
--- CLIENTS ---
CLIENT       SSID         SIGNAL
MacBook Pro  HomeNetwork  -62 dBm
iPhone 15    HomeNetwork  -70 dBm
```

**Gateway / unknown:** no type-specific section.

### `hlctl network clients get <id>`

**Wired client** — replace `SWITCH`/`SWITCH PORT` rows with `connectedTo` fields:

```
FIELD       VALUE
...
SWITCH      Switch Living Room
PORT        8
LINK SPEED  1GbE
UPTIME      7d 0h 0m 0s
```

`PORT` and `LINK SPEED` omitted for offline clients (spec makes them optional). `SWITCH` always present (last-known device).

**Wireless client** — replace top-level `SSID`/`SIGNAL` with `connectedTo` fields:

```
FIELD       VALUE
...
AP          AP Living Room
SSID        HomeNetwork
SIGNAL      -62 dBm
UPTIME      2h 0m 0s
```

`SIGNAL` omitted for offline clients.

## Output Helpers (`internal/output/output.go`)

**`FormatBytesPerSec(n int64) string`** — formats bytes/sec using binary units with `/s` suffix (e.g. `125.0 KB/s`, `1.2 MB/s`). Reuses `FormatBytes` logic.

**`FormatLinkSpeed(s string) string`** — maps `NetworkLinkSpeed` enum values to human-readable strings:

| Enum    | Display  |
|---------|----------|
| `e`     | `10M`    |
| `fe`    | `100M`   |
| `gbe1`  | `1GbE`   |
| `gbe2_5`| `2.5GbE` |
| `gbe5`  | `5GbE`   |
| `gbe10` | `10GbE`  |

## Tests

- Update `TestListDevicesCmd_tableOutput`: remove `NumClients` from stub, remove `CLIENTS` column assertions
- Update `TestGetDeviceCmd`: use new polymorphic types (switch stub with ports, AP stub with connectedClients)
- Add `TestGetDeviceCmd_switchAllPorts`: verify `--all-ports` flag shows down ports, default hides them
- Update `TestGetClientCmd` wired/wireless: use `connectedTo` shape instead of `SwitchName`/`SwitchPort`/`Ssid`/`SignalStrength`
