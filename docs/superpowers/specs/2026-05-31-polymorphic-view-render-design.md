# Unified `View.Render` for Polymorphic and Transform Responses

## Problem

Six command leaves in `internal/cli/` open-code the same
`status check → JSON shortcut → render` pipeline that `cmdutil.View.Render`
provides for simple responses:

| File | Command | Shape |
|---|---|---|
| `internal/cli/network/network.go` | `network devices get` | polymorphic (4 variants, per-call `--all-ports`) |
| `internal/cli/network/network.go` | `network clients get` | polymorphic (2 variants) |
| `internal/cli/network/network.go` | `network topology` | transform-then-render |
| `internal/cli/system/system.go` | `system updates get` | polymorphic (1 variant) |
| `internal/cli/system/system.go` | `system info` | transform-then-render |
| `internal/cli/system/system.go` | `system utilization` | transform-then-render |

The carve-out in `CLAUDE.md` reads:

> For polymorphic responses (discriminated unions like `NetworkDeviceDetail`,
> `SystemUpdateDetail`), keep the status check + JSON branch inline (still using
> `flags.GetOutputFormat`) and call `output.RenderTemplate` directly with the
> resolved variant's template. `cmdutil.View.Render` cannot dispatch on a
> discriminator.

This leaks `flags.GetOutputFormat`, the status check, and the JSON-shortcut
write into every domain command that has a polymorphic body or that needs to
reshape data before rendering. The polymorphic blocks in particular run
~60 lines of nested switches per command.

## Goal

Eliminate every inline `flags.GetOutputFormat` / status-check / `RenderTemplate`
trio in `internal/cli/<domain>/` by extending the `cmdutil` rendering helpers
to cover the two missing shapes. After this work, `flags.GetOutputFormat` is
referenced only in `cmdutil/view.go` and `watch/watch.go`.

## Design

Two additions to `internal/cli/cmdutil/view.go`:

### 1. `View.RenderWith` — lazy data for transform-then-render

```go
// RenderWith mirrors Render but defers data construction. fn is invoked only
// in table mode — JSON mode dumps the raw body without running fn.
func (v View) RenderWith(w io.Writer, statusCode int, body []byte, fn func() (any, error)) error
```

The lazy callback exists so the table-mode transform (e.g. building
`[]infoRow` with `FormatBytes` / `FormatUptime`) is skipped in JSON mode. This
matches the implicit contract today — the inline code returns before doing any
transform when `--output=json` is set.

### 2. `PolymorphicView[T]` — discriminator dispatch

```go
// Discriminator constrains polymorphic response bodies. Oapi-codegen union
// types satisfy this automatically — each generated *Detail struct has a
// Discriminator() method.
type Discriminator interface {
    Discriminator() (string, error)
}

// Variant binds a discriminator branch to its template and a resolver that
// extracts the typed variant (and optionally transforms it) from the union.
type Variant[T Discriminator] struct {
    Template string
    Resolve  func(T) (any, error)
}

// PolymorphicView is the cmdutil.View equivalent for discriminated-union
// responses. Variants is keyed by the discriminator string. Status defaults
// to http.StatusOK.
type PolymorphicView[T Discriminator] struct {
    Templates fs.FS
    Status    int
    Variants  map[string]Variant[T]
}

// Render handles the status check + JSON shortcut, then dispatches on
// detail.Discriminator() to look up the variant template and resolved data.
func (v PolymorphicView[T]) Render(w io.Writer, statusCode int, body []byte, detail *T) error
```

Common pre-render logic (status check + JSON shortcut) is extracted to a
private helper so `View.Render`, `View.RenderWith`, and
`PolymorphicView.Render` share it.

### Call-site shapes after migration

**Polymorphic, top-level View** (`network clients get`, `system updates get`):

```go
var clientGetView = cmdutil.PolymorphicView[gen.NetworkClientDetail]{
    Templates: networkTemplates,
    Variants: map[string]cmdutil.Variant[gen.NetworkClientDetail]{
        "wired": {
            Template: "clients_get_wired.tmpl",
            Resolve:  func(d gen.NetworkClientDetail) (any, error) { return d.AsWiredNetworkClientDetail() },
        },
        "wireless": {
            Template: "clients_get_wireless.tmpl",
            Resolve:  func(d gen.NetworkClientDetail) (any, error) { return d.AsWirelessNetworkClientDetail() },
        },
    },
}
// In RunE:
return clientGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
```

**Polymorphic with per-call state** (`network devices get`): the
`PolymorphicView` is constructed inside `RunE` so the `switch` variant's
resolver can close over `allPorts`. The port-resolution loop is extracted to
`buildSwitchPortViews(ports []gen.SwitchPort, allPorts bool) ([]switchPortView, error)`.

**Transform-then-render** (`system info`, `system utilization`,
`network topology`): new top-level `View`, called via `RenderWith`:

```go
var infoView = cmdutil.View{Templates: systemTemplates, Name: "info.tmpl"}

return infoView.RenderWith(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, func() (any, error) {
    items := make([]infoRow, 0, len(resp.JSON200.Items))
    for _, info := range resp.JSON200.Items { /* ... */ }
    return struct{ Items []infoRow }{items}, nil
})
```

## Error handling

| Condition | `View.RenderWith` | `PolymorphicView.Render` |
|---|---|---|
| Status mismatch | `apiclient.ParseError(status, body)` | `apiclient.ParseError(status, body)` |
| JSON mode | Write raw body; do not invoke `fn` | Write raw body; do not consult discriminator |
| `detail == nil` | n/a | Return error (`nil <T> body`) — 200 with empty JSON would otherwise panic |
| `detail.Discriminator()` returns error | n/a | Propagate as-is |
| Discriminator not in `Variants` map | n/a | Return `fmt.Errorf("unknown %T discriminator: %q", *detail, disc)` |
| `Resolve` returns error | n/a | Propagate as-is |
| `fn` returns error | Propagate as-is | n/a |

Behavior matches the current inline code byte-for-byte, except the
unknown-discriminator error gains the union type name (current code says only
`"unknown device type: %s"`).

## Testing

### New unit tests in `internal/cli/cmdutil/view_test.go`

**`View.RenderWith`:**

- Table mode invokes `fn`, renders template against returned data.
- JSON mode dumps raw body and does **not** call `fn` (asserted via a counter
  on the closure).
- Status mismatch returns `ParseError`; `fn` not invoked.
- `fn`-returns-error propagates verbatim.

**`PolymorphicView.Render`:**

- Define a tiny fake union type in the test that implements `Discriminator`
  plus two `As<Variant>` style methods.
- Dispatches to the correct variant template when the discriminator matches
  (two variants exercised).
- JSON mode dumps raw body without consulting the discriminator.
- Status mismatch returns `ParseError`; custom `Status` field exercised in a
  separate case (e.g. 201).
- Unknown discriminator returns an error containing the type name and value.
- `detail == nil` returns an error.
- `Resolve` error propagates.

### Existing tests stay green

`internal/cli/network/network_test.go` and `internal/cli/system/system_test.go`
already cover the rendered output of every migrated command via stub clients.
They should pass unmodified — rendered bytes do not change. That is the
safety net for the migration itself.

### Refactor extractions

`buildSwitchPortViews(ports []gen.SwitchPort, allPorts bool) ([]switchPortView, error)`
is lifted out of the device-get switch case. The existing `get device <switch-id>`
test in `network_test.go` covers it end-to-end; no separate unit test added
unless gaps appear during migration.

## Migration scope

| Site | New construct |
|---|---|
| `network.go` `newGetClientCmd` | top-level `PolymorphicView[gen.NetworkClientDetail]` |
| `network.go` `newGetDeviceCmd` | inline `PolymorphicView[gen.NetworkDeviceDetail]` + extracted `buildSwitchPortViews` |
| `system.go` `newGetUpdateCmd` | top-level `PolymorphicView[gen.SystemUpdateDetail]` |
| `system.go` `newInfoCmd` | new top-level `View` + `RenderWith` |
| `system.go` `newUtilizationCmd` | new top-level `View` + `RenderWith` |
| `network.go` `newTopologyCmd` | new top-level `View` + `RenderWith` |

After migration, `flags.GetOutputFormat` is referenced only in
`cmdutil/view.go` and `watch/watch.go`. The inline `apiclient.ParseError` +
status check pattern disappears from `internal/cli/<domain>/`.

### Out of scope

- `network.go` `connectionRefID` and `buildTopologyTree` perform inline
  discriminator switches on `NetworkConnectionRef` and topology node/edge
  types but produce intermediate data, not rendered output. They are not
  render sites and stay as-is.
- The `watch` package's separate `flags.GetOutputFormat` reference is its own
  concern.

## Documentation

The "Adding a New Domain Command" section in `CLAUDE.md` updates:

- Remove the polymorphic carve-out (item 9 today).
- Add bullets describing `PolymorphicView` (when to use it, variant table
  shape) and `RenderWith` (when transform-before-render is needed).
