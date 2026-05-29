# Design: Migrate CLI domains to `WithResponse` API client methods

**Date:** 2026-05-29
**Scope:** All CLI domains — `network`, `system`, `storage`, `docker`

---

## Problem

Every command in every domain repeats the same 5-step HTTP plumbing:

1. Call the API method (returns `*http.Response`)
2. Check `resp.StatusCode != http.StatusOK` → call `apiclient.ParseError(resp)`
3. `io.ReadAll(resp.Body)`
4. `json.Unmarshal(body, &typed)`
5. Branch on output format: print raw body for JSON, build table for table

Steps 2–4 are already handled by the generated `ClientWithResponses` / `*WithResponse` methods, which return typed `*XxxResponse` structs with `JSON200`, `StatusCode()`, and raw `Body []byte` fields. We are not using them.

---

## Approach

**Option chosen: update domain client interfaces to declare `WithResponse` variants.**

Each domain's `client.go` interface changes method signatures from returning `(*http.Response, error)` to returning `(*gen.XxxResponse, error)`. The concrete `buildClient()` already returns `*gen.ClientWithResponses`, which implements all `WithResponse` methods, so the production wiring needs no changes beyond the interface declaration.

---

## Design

### 1. Interface changes (`internal/cli/<domain>/client.go`)

Rename each interface method to its `WithResponse` counterpart and update the return type:

```go
// Before
ListNetworkDevices(ctx context.Context, ...) (*http.Response, error)

// After
ListNetworkDevicesWithResponse(ctx context.Context, ...) (*gen.ListNetworkDevicesResponse, error)
```

Apply to all methods across `network`, `system`, `storage`, and `docker` client interfaces.

### 2. Command body simplification (`internal/cli/<domain>/*.go`)

Replace the manual read/unmarshal/error-check block with status code check + direct field access:

```go
// Before
resp, err := c.ListNetworkDevices(ctx)
if err != nil { return err }
defer resp.Body.Close()
if resp.StatusCode != http.StatusOK { return apiclient.ParseError(resp) }
body, err := io.ReadAll(resp.Body)
if err != nil { return err }
var list gen.NetworkDeviceList
if err := json.Unmarshal(body, &list); err != nil { return err }
if flags.GetOutputFormat() == output.FormatJSON {
    fmt.Fprint(w, string(body))
    return nil
}
// render table with list

// After
resp, err := c.ListNetworkDevicesWithResponse(ctx)
if err != nil { return err }
if resp.StatusCode() != http.StatusOK { return apiclient.ParseError(resp.StatusCode(), resp.Body) }
if flags.GetOutputFormat() == output.FormatJSON {
    fmt.Fprint(w, string(resp.Body))
    return nil
}
// render table with *resp.JSON200
```

Remove unused imports (`io`, `encoding/json`) from each file after the change.

### 3. Error parsing (`internal/apiclient/`)

Update `apiclient.ParseError` signature from `(r *http.Response) error` to `(statusCode int, body []byte) error`. After `WithResponse` parses the response, the HTTP body is already read and closed — re-reading it would silently return nothing. The raw bytes are available in `resp.Body`.

### 4. Test stubs (`internal/cli/<domain>/stub.go`)

Update `StubClient` struct field names and method signatures to match the new interface. Replace the `jsonResponse(*http.Response)` helper with direct construction of the typed response struct:

```go
// Success stub
ListNetworkDevicesWithResponseFunc: func(...) (*gen.ListNetworkDevicesResponse, error) {
    list := gen.NetworkDeviceList{Items: ...}
    b, _ := json.Marshal(list)
    return &gen.ListNetworkDevicesResponse{
        HTTPResponse: &http.Response{StatusCode: http.StatusOK},
        Body:         b,
        JSON200:      &list,
    }, nil
},

// Error stub
ListNetworkDevicesWithResponseFunc: func(...) (*gen.ListNetworkDevicesResponse, error) {
    b, _ := json.Marshal(map[string]any{"title": "Unauthorized", ...})
    return &gen.ListNetworkDevicesResponse{
        HTTPResponse: &http.Response{StatusCode: http.StatusUnauthorized},
        Body:         b,
    }, nil
},
```

Remove the `jsonResponse` helper from each domain's `stub.go`.

---

## Files affected

| File | Change |
|------|--------|
| `internal/apiclient/errors.go` | Update `ParseError` signature to `(statusCode int, body []byte)` |
| `internal/apiclient/errors_test.go` | Update tests to pass `statusCode int, body []byte` instead of `*http.Response` |
| `internal/cli/network/client.go` | Update interface to `WithResponse` methods |
| `internal/cli/network/network.go` | Remove manual HTTP plumbing, use typed fields |
| `internal/cli/network/clients.go` | Same |
| `internal/cli/network/vlans.go` | Same |
| `internal/cli/network/ssids.go` | Same |
| `internal/cli/network/wans.go` | Same |
| `internal/cli/network/topology.go` | Same |
| `internal/cli/network/stub.go` | Update StubClient, remove `jsonResponse` |
| `internal/cli/network/network_test.go` | Update stub initialization |
| `internal/cli/system/client.go` | Same pattern |
| `internal/cli/system/system.go` | Same pattern |
| `internal/cli/system/stub.go` | Same pattern |
| `internal/cli/system/*_test.go` | Same pattern |
| `internal/cli/storage/client.go` | Same pattern |
| `internal/cli/storage/storage.go` | Same pattern |
| `internal/cli/storage/stub.go` | Same pattern |
| `internal/cli/storage/*_test.go` | Same pattern |
| `internal/cli/docker/client.go` | Same pattern |
| `internal/cli/docker/docker.go` | Same pattern |
| `internal/cli/docker/stub.go` | Same pattern |
| `internal/cli/docker/*_test.go` | Same pattern |

---

## Out of scope

- Changes to output formatting logic (templates, `output.Print` calls, table row construction)
- Changes to command flags, args, or CLI structure
- The `auth` domain (uses a custom login flow, not a generated API client)
- The `config` domain (no API calls)
