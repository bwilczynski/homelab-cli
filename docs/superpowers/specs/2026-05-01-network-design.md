# Network Command Implementation Design

**Date:** 2026-05-01

## Overview

Implement the `hlctl network` subcommands (`devices`, `device <id>`, `clients`, `client <id>`) with real API calls to the Homelab API. Follows the same patterns established in the containers domain: shared `apiclient` package, interface-based testability, `StubClient` for unit tests.

## Package Structure

```
internal/
  cli/network/
    network.go        ← refactored: NewCmd() + command constructors accept a NetworkClient
    client.go         ← NetworkClient interface + NewNetworkClient() factory
    stub.go           ← StubClient implementing NetworkClient
    network_test.go   ← unit tests using StubClient
```

Generated client lives in `internal/network/api.gen.go` (already configured via `oapi-codegen-network.yaml`).

## `NetworkClient` Interface (`client.go`)

```go
type NetworkClient interface {
    ListNetworkDevices(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
    GetNetworkDevice(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
    ListNetworkClients(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
    GetNetworkClient(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}
```

Neither list endpoint has query params in the spec, so no `Params` struct is needed. `NewNetworkClient(httpClient *http.Client, apiURL string) (NetworkClient, error)` wraps the generated client.

## Command Output

### `network devices`

Flat table, one device per row:

```
ID                      NAME              MAC                IP           TYPE         STATUS      CLIENTS
unifi.usg               USG               aa:bb:cc:dd:00:01  192.168.1.1  gateway      connected
unifi.ap-living-room    AP Living Room    aa:bb:cc:dd:00:03  192.168.1.3  accessPoint  connected   5
```

`CLIENTS` column: numeric value from `numClients` for access points; empty string for switches and gateways (field absent in spec).

### `network device <id>`

Flat key/value table. `CLIENTS` row omitted for non-access-point devices. `UPTIME` formatted as `Xd Yh Zm Zs`.

```
FIELD    VALUE
ID       unifi.ap-living-room
NAME     AP Living Room
MAC      aa:bb:cc:dd:ee:ff
IP       192.168.1.3
TYPE     accessPoint
STATUS   connected
CLIENTS  5
MODEL    U6-Lite
FIRMWARE 6.6.77.14522
UPTIME   1d 0h 0m 0s
```

### `network clients`

Flat table, one client per row:

```
ID                    NAME        MAC                IP             CONNECTION
unifi.macbook-pro-3c  MacBook Pro 3c:22:fb:09:aa:b1  192.168.1.101  wireless
unifi.nas-1-68        nas-1       68:d7:9a:12:bb:c2  192.168.1.10   wired
```

### `network client <id>`

Flat key/value table. Connection-type-conditional fields: only the fields relevant to the detected `connectionType` are shown — no empty rows.

**Wireless client:**

```
FIELD       VALUE
ID          unifi.macbook-pro-3c
NAME        MacBook Pro
MAC         3c:22:fb:09:aa:b1
IP          192.168.1.101
CONNECTION  wireless
SSID        HomeNetwork
SIGNAL      -62 dBm
UPTIME      2h 0m 0s
```

**Wired client:**

```
FIELD       VALUE
ID          unifi.nas-1-68
NAME        nas-1
MAC         68:d7:9a:12:bb:c2
IP          192.168.1.10
CONNECTION  wired
SWITCH      Switch Living Room
SWITCH PORT 8
UPTIME      7d 0h 0m 0s
```

## Uptime Formatting

`FormatUptime(seconds int) string` helper added to the `output` package. Produces `"1d 2h 30m 5s"` style output, skipping **leading** zero segments only (e.g. `"2h 5m 3s"` not `"0d 2h 5m 3s"`), but always including seconds. Once a non-zero segment is encountered all subsequent segments are included even if zero (e.g. `"1d 0h 5m 3s"`). Used by both `device` and `client` detail renderers.

## `StubClient` (`stub.go`)

Function-field stub following the containers pattern — one `Func` field per interface method. `jsonResponse` helper (already in containers/stub.go) is duplicated into this package for self-contained tests.

## JSON Output

All commands pass `--output json` through by writing the raw response body directly, bypassing table formatting. Same pattern as containers.

## Error Handling

All non-2xx responses go through `apiclient.ParseError(resp)`. No per-status-code special casing.

## Testing (`network_test.go`)

- `TestDevicesCmd_tableOutput` — verify headers and ID/MAC/numClients in output
- `TestDeviceCmd_accessPoint` — verify all fields including CLIENTS and UPTIME
- `TestDeviceCmd_gateway` — verify CLIENTS row absent
- `TestClientsCmd_tableOutput` — verify headers and row values
- `TestClientCmd_wireless` — verify SSID/SIGNAL rows present, SWITCH rows absent
- `TestClientCmd_wired` — verify SWITCH/SWITCH PORT present, SSID/SIGNAL absent
- `TestDevicesCmd_apiError` — verify RFC 9457 error format (`"Title — detail"`)
- `TestClientsCmd_apiError` — same

Commands under test constructed as `newDevicesCmd(stub)` / `newClientCmd(stub)`, bypassing real HTTP. Stdout captured via `cmd.SetOut(buf)`.

## Discriminated Union Handling

`NetworkClientDetail` in the generated code is a union type backed by `json.RawMessage` with typed accessor methods:

- `detail.Discriminator()` — returns the `connectionType` string (`"wired"` or `"wireless"`)
- `detail.AsWiredNetworkClientDetail()` — unmarshals into `WiredNetworkClientDetail` (fields: `Id`, `Name`, `Mac`, `Ip`, `ConnectionType`, `SwitchName`, `SwitchPort`, `Uptime`)
- `detail.AsWirelessNetworkClientDetail()` — unmarshals into `WirelessNetworkClientDetail` (fields: `Id`, `Name`, `Mac`, `Ip`, `ConnectionType`, `Ssid`, `SignalStrength`, `Uptime`)

The renderer calls `Discriminator()` to branch, then calls the appropriate `As*` method to get the typed struct and build rows. No merged-struct approach needed — no empty placeholder rows.
