# Offline Client Support Design

**Date:** 2026-05-15
**Spec commit:** `4a073a5` — feat: add offline client support (status field, optional status filter)

## Overview

The homelab API now returns both online and offline network clients. This design covers the CLI changes needed to surface the new `status` field, support filtering by status, and handle optional session fields for offline clients.

## Spec Changes (from `4a073a5`)

- `NetworkClient` gains a required `status` field: enum `online` | `offline`
- `GET /network/clients` gains an optional `status` query parameter for filtering
- `WiredNetworkClientDetail`: `switchName`, `switchPort`, `uptime` are now optional (absent for offline clients)
- `WirelessNetworkClientDetail`: `ssid`, `signalStrength`, `uptime` are now optional (absent for offline clients)

## Submodule & Code Generation

Advance the `spec` submodule to `4a073a5` and run `make generate` to regenerate `internal/network/api.gen.go`. The generated code will include the `NetworkClientStatus` enum and an updated `ListNetworkClientsParams` struct with an optional `Status` field.

## CLI Changes

### `internal/cli/network/client.go`

Update the `NetworkClient` interface: `ListNetworkClients` gains a `params *gen.ListNetworkClientsParams` argument.

### `internal/cli/network/network.go`

**`newListClientsCmd`:**
- Add `--status` flag (string, optional). Accepts `online` or `offline`.
- When `--status` is set, populate `ListNetworkClientsParams.Status` and pass to the API call.
- Add `STATUS` column to table headers and rows (between `IP` and `CONNECTION`).

**`newGetClientCmd`:**
- Add `STATUS` row using the base client's `status` field.
- Optional fields are only appended to the row list when non-nil:
  - Wired: `SWITCH`, `SWITCH PORT`, `UPTIME`
  - Wireless: `SSID`, `SIGNAL`, `UPTIME`

### `internal/cli/network/stub.go`

Update `ListNetworkClients` signature to match the new interface (`params *gen.ListNetworkClientsParams`).

## Tests

| Test | Change |
|------|--------|
| `TestListClientsCmd_tableOutput` | Add `Status: gen.NetworkClientStatusOnline` to fixture; assert `STATUS` column present |
| `TestListClientsCmd_statusFilter` | New — pass `--status online`, assert params carry the filter |
| `TestGetClientCmd_wired` | Add `status: "online"` to JSON fixture; assert `STATUS` row present |
| `TestGetClientCmd_offline_wired` | New — wired client with no session fields; assert `SWITCH`, `SWITCH PORT`, `UPTIME` rows absent |
| `TestGetClientCmd_offline_wireless` | New — wireless client with no session fields; assert `SSID`, `SIGNAL`, `UPTIME` rows absent |

## Error Handling

No new error cases. The `--status` flag value is passed as-is to the API; invalid values will result in a standard API error response handled by `apiclient.ParseError`.
