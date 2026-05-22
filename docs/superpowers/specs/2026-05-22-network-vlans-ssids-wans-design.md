# Design: network vlans, ssids, wans commands

## Overview

Add three new subcommand groups under `hlctl network`: `vlans`, `ssids`, and `wans`. Each exposes `list` and `get <id>` commands backed by new endpoints added in spec commit `4c71e7d`.

## Submodule & Code Generation

Advance `spec/` to `4c71e7d` ("Add network SSID, VLAN, and WAN endpoints"), then run `make generate` to regenerate `internal/network/api.gen.go` with the six new operations: `listVlans`, `getVlan`, `listSsids`, `getSsid`, `listWans`, `getWan`.

## Commands

### `network vlans list`

Table output columns: `ID  NAME  VLAN-ID  SUBNET`

JSON output: raw `VlanList` response body.

### `network vlans get <vlan-id>`

Key/value table:

```
ID          unifi.iot
NAME        IoT
VLAN ID     20
SUBNET      192.168.20.0/24
GATEWAY IP  192.168.20.1
BROADCAST   192.168.20.255
DHCP MODE   server
DHCP RANGE  192.168.20.100 - 192.168.20.200   (only when dhcpMode=server)
RELAY       192.168.1.1                        (only when dhcpMode=relay)
DNS         1.1.1.1, 8.8.8.8
```

JSON output: raw `VlanDetail` response body.

### `network ssids list`

Table output columns: `ID  NAME  VLAN-ID  BANDS  CLIENTS`

Bands are joined as a comma-separated string (e.g. `2.4 GHz, 5 GHz`). Band display mapping: `band2g`→`2.4 GHz`, `band5g`→`5 GHz`, `band6g`→`6 GHz`.

JSON output: raw `SsidList` response body.

### `network ssids get <ssid-id>`

Key/value table followed by two sub-sections:

```
ID        unifi.iot
NAME      IoT
VLAN ID   20
BANDS     2.4 GHz, 5 GHz
CLIENTS   3
SECURITY  wpa2

--- CLIENTS ---
NAME
Sonos One SL
...

--- BROADCASTING APs ---
NAME
AP Living Room
...
```

JSON output: raw `SsidDetail` response body.

### `network wans list`

Table output columns: `ID  NAME  IP  UPTIME  STATUS`

Uptime formatted with the existing `output.FormatUptime` helper.

JSON output: raw `WanList` response body.

### `network wans get <wan-id>`

Key/value table:

```
ID      unifi.wan1
NAME    WAN 1
IP      203.0.113.42
UPTIME  1d 0h 0m
STATUS  connected
DNS     1.1.1.1, 1.0.0.1
```

JSON output: raw `WanDetail` response body.

## Code Structure

`network.go` is already 654 lines. New commands go in three new files in the same `network` package:

- `internal/cli/network/vlans.go` — `newVlansCmd`, `newListVlansCmd`, `newGetVlanCmd`
- `internal/cli/network/ssids.go` — `newSsidsCmd`, `newListSsidsCmd`, `newGetSsidCmd`
- `internal/cli/network/wans.go`  — `newWansCmd`, `newListWansCmd`, `newGetWanCmd`

`client.go` gains six new methods on `NetworkClient`:

```go
ListVlans(ctx context.Context, ...) (*http.Response, error)
GetVlan(ctx context.Context, vlanId string, ...) (*http.Response, error)
ListSsids(ctx context.Context, ...) (*http.Response, error)
GetSsid(ctx context.Context, ssidId string, ...) (*http.Response, error)
ListWans(ctx context.Context, ...) (*http.Response, error)
GetWan(ctx context.Context, wanId string, ...) (*http.Response, error)
```

`network.go` `NewCmd()` registers the three new subcommand groups:

```go
cmd.AddCommand(newVlansCmd())
cmd.AddCommand(newSsidsCmd())
cmd.AddCommand(newWansCmd())
```

## Conventions

- Each `list` command follows the same JSON short-circuit pattern (`flags.GetOutputFormat() == output.FormatJSON → fmt.Fprint raw body, return nil`), then table rendering.
- Each `get` command uses `cobra.ExactArgs(1)`.
- `buildClient()` is shared from `network.go`; each new file calls it the same way the existing commands do.
- No new oapi-codegen config needed — all six operations are already under the `network` tag covered by `oapi-codegen-network.yaml`.
