# Containers Command Implementation Design

**Date:** 2026-05-01  

## Overview

Implement the `hlctl containers` subcommand (list, get, start, stop, restart) with real API calls to the Homelab API. This implementation establishes reusable patterns — shared API client initialization, error handling, interface-based testability — that all future domain command implementations will follow.

## Package Structure

```
internal/
  apiclient/
    apiclient.go       ← NewHTTPClient() — shared HTTP client factory
    errors.go          ← ParseError(resp *http.Response) error — RFC 9457 Problem parser

  cli/containers/
    containers.go      ← NewCmd(); newListCmd/newGetCmd accept a ContainersClient
    client.go          ← ContainersClient interface + NewContainersClient() factory
    stub.go            ← StubClient implementing ContainersClient (hardcoded data)
    containers_test.go ← unit tests using StubClient
```

### `internal/apiclient`

Domain-agnostic package used by all CLI domain packages.

**`apiclient.go`** — `NewHTTPClient() (*http.Client, error)`

Resolves the API URL (precedence: `--api-url` flag → `HOMELAB_API_URL` env → config file) and returns an `*http.Client` wrapping `auth.AuthenticatedTransport`. Each domain package calls this once inside `RunE` to construct its oapi-codegen client.

**`errors.go`** — `ParseError(resp *http.Response) error`

Called on any non-2xx response. Reads and decodes the RFC 9457 Problem body:

- Returns `"Title — detail"` when `detail` is present
- Returns `"Title"` when `detail` is absent
- Falls back to `"unexpected status <N>"` if the body cannot be decoded

### `internal/cli/containers`

**`client.go`** — declares `ContainersClient` interface with only the methods used by containers commands:

```go
type ContainersClient interface {
    ListContainers(ctx context.Context, params *containers.ListContainersParams,
        reqEditors ...RequestEditorFn) (*http.Response, error)
    GetContainer(ctx context.Context, containerId string,
        reqEditors ...RequestEditorFn) (*http.Response, error)
}
```

Signatures match oapi-codegen output exactly — no adapter needed. Also provides `NewContainersClient(httpClient *http.Client, apiURL string) ContainersClient` that wraps the generated client.

**`stub.go`** — `StubClient` with function fields for each method:

```go
type StubClient struct {
    ListContainersFunc func(ctx context.Context, params *containers.ListContainersParams,
        reqEditors ...RequestEditorFn) (*http.Response, error)
    GetContainerFunc   func(ctx context.Context, containerId string,
        reqEditors ...RequestEditorFn) (*http.Response, error)
}
```

Used in tests and preserved for future integration test scaffolding.

## Command Output

### `containers list`

Flat table, one container per row:

```
ID                    IMAGE                                           STATUS   CPU    MEMORY
nas-1.homeassistant   ghcr.io/home-assistant/home-assistant:2025.4   running  2.5%   256 MB
nas-1.immich-server   ghcr.io/immich-app/immich-server:v1.120.0      running  0.8%   512 MB
```

- `CPU` formatted as `"2.5%"` (one decimal place)
- `MEMORY` formatted as human-readable bytes (e.g. `268435456` → `256 MB`)
- Accepts `--device` flag to filter by device ID

### `containers get <id>`

Two-pass table output: flat scalar fields first, then one sub-section per nested array. Empty arrays are omitted entirely.

```
FIELD           VALUE
ID              nas-1.homeassistant
NAME            homeassistant
DEVICE          nas-1
STATUS          running
IMAGE           ghcr.io/home-assistant/home-assistant:2025.4
RESTART COUNT   0
CPU             2.5%
MEMORY          256 MB (6.4%)
STARTED AT      2026-04-08T19:46:34Z
EXIT CODE       0
OOM KILLED      false
RESTART POLICY  always
PRIVILEGED      false
MEMORY LIMIT    unlimited

PORT BINDINGS
CONTAINER PORT  HOST PORT  PROTOCOL
8123            8123       tcp

NETWORKS
NAME                    DRIVER
homeassistant_default   bridge

VOLUME BINDINGS
SOURCE                                DESTINATION  MODE
/volume1/docker/homeassistant/config  /config      rw

ENVIRONMENT VARIABLES
KEY  VALUE
TZ   Europe/Warsaw

ENTRYPOINT
/init

COMMAND
(empty)

LABELS
KEY                              VALUE
com.docker.compose.project       homeassistant
com.docker.compose.service       homeassistant
```

`MEMORY LIMIT` shows `"unlimited"` when the value is 0; otherwise formats as human-readable bytes.

### Bytes Formatting

A shared `FormatBytes(n int64) string` helper in the `output` package converts raw byte counts to human-readable strings using binary units (KB, MB, GB, TB). Used in both list and get table renderers.

### `--output json`

Bypasses all table formatting and dumps the raw API response body as-is.

## Error Handling

All non-2xx API responses go through `apiclient.ParseError(resp)`:

```go
resp, err := client.ListContainers(ctx, params)
if err != nil {
    return err  // network/transport error
}
defer resp.Body.Close()
if resp.StatusCode != http.StatusOK {
    return apiclient.ParseError(resp)
}
```

Auth errors (missing or expired token) surface before the request is sent, from `auth.AuthenticatedTransport.RoundTrip`, as plain Go errors with user-friendly messages (`"not logged in (run 'hlctl login')"`, `"token expired (run 'hlctl login')"`).

No per-status-code special casing. The Problem `detail` field carries actionable context from the server.

## Testing

**`containers_test.go`** uses `StubClient` with function fields so each test can inject specific behavior:

- Successful list → verify table headers and row values
- Successful get → verify flat fields and all sub-section headers appear
- API error → verify error message format (`"Title — detail"`)
- Network error → verify error propagates unchanged

Commands under test are constructed as `newListCmd(stub)` / `newGetCmd(stub)`, bypassing real HTTP entirely. Stdout is captured via `cmd.SetOut(buf)`.

## Patterns for Future Domains

Each new domain command package will:

1. Call `apiclient.NewHTTPClient()` inside `RunE` to get an authenticated `*http.Client`
2. Construct the oapi-codegen client with that HTTP client and the resolved API URL
3. Define a narrow `<Domain>Client` interface matching only the methods the commands use
4. Provide a `Stub<Domain>Client` in `stub.go` for unit tests
5. Call `apiclient.ParseError(resp)` on any non-2xx response

Start, stop, and restart commands (`containers start/stop/restart`) print a confirmation message on success and propagate errors — no data to format.
