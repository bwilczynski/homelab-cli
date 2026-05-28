# Design: Extract Network Domain Views to Go Templates

**Date:** 2026-05-28
**Branch:** feat/generator
**Scope:** Network domain only (`internal/cli/network/`). Other domains unchanged.

## Problem

Every command file in `internal/cli/network/` hardcodes view logic — building `headers []string` and `rows [][]string` in Go, then calling `output.Print`. This makes rendering impossible to customise without editing command files, and couples presentation to controller logic.

## Goal

Extract the table rendering path of all network domain commands into `text/template` files. The JSON fast-path and all HTTP/cobra plumbing stay exactly as-is. Existing tests must continue to pass without modification.

This is a pilot for a future code generator that will generate controller code from the OpenAPI spec; validating the template approach on a real domain first.

## Architecture

### Layers after this change

```
cobra command (flags, args, HTTP call, JSON unmarshal)   ← unchanged
        │
        ├─ JSON path: fmt.Fprint(raw body)               ← unchanged
        │
        └─ table path: output.RenderTemplate(w, fs, "name.tmpl", data)   ← new
                │
                └─ text/template + tabwriter → stdout
```

### Components

**`internal/output` — new function**

```go
func RenderTemplate(w io.Writer, fs embed.FS, name string, data any) error
```

- Parses all `.tmpl` files from `fs` on each call (or caches — implementation detail).
- Wraps `w` in `tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)` before executing the template.
- Executes the named template with `data`.
- Flushes the tabwriter before returning.
- Registers a shared func map (see below).

**Func map** — exposes existing output formatters to templates:

| Template function | Go source |
|---|---|
| `formatUptime` | `output.FormatUptime` |
| `formatBytes` | `output.FormatBytes` |
| `formatBytesPerSec` | `output.FormatBytesPerSec` |
| `formatLinkSpeed` | `output.FormatLinkSpeed` |
| `formatTime` | `output.FormatTime` |
| `join` | `strings.Join` |
| `derefStr` | dereference `*string`, return `""` if nil |
| `derefInt` | dereference `*int`, return `0` if nil |
| `string` | convert a typed string (enum) to `string` |
| `dict` | construct `map[string]any` for passing multiple values to sub-templates |
| `formatBand` | convert `WifiBand` enum to human string (`band2g` → `2.4 GHz`, etc.) |

**`internal/cli/network/templates/` — template files**

Embedded in the `network` package:

```go
//go:embed templates/*.tmpl
var networkTemplates embed.FS
```

Templates emit tab-separated columns (`\t`) and newline-terminated rows (`\n`). The wrapping tabwriter handles column alignment. Whitespace trim markers (`{{-` / `-}}`) are used where needed to suppress blank lines from conditional blocks.

## Template Files

| File | Receives | Notes |
|---|---|---|
| `wans_list.tmpl` | `gen.WanList` | |
| `wans_get.tmpl` | `gen.WanDetail` | |
| `vlans_list.tmpl` | `gen.VlanList` | |
| `vlans_get.tmpl` | `gen.VlanDetail` | Conditional rows for `dhcpMode` (server/relay/disabled) |
| `ssids_list.tmpl` | `gen.SsidList` | Bands formatted via `formatBand` helper added to func map |
| `ssids_get.tmpl` | `gen.SsidDetail` | Sub-tables for clients and broadcasting APs |
| `devices_list.tmpl` | `gen.NetworkDeviceList` | |
| `devices_get_switch.tmpl` | `switchDetailView` | Sub-table for ports; `--all-ports` filtering applied before render (see below) |
| `devices_get_accesspoint.tmpl` | `gen.AccessPointDetail` | Sub-table for connected clients |
| `devices_get_gateway.tmpl` | `gen.GatewayDetail` | |
| `devices_get_unknown.tmpl` | `gen.UnknownDeviceDetail` | |
| `clients_list.tmpl` | `gen.NetworkClientList` | `ip` is `*string`; use `derefStr` |
| `clients_get_wired.tmpl` | `gen.WiredNetworkClientDetail` | Optional port, link speed, uptime rows |
| `clients_get_wireless.tmpl` | `gen.WirelessNetworkClientDetail` | Optional signal, uptime rows |
| `topology.tmpl` | `TopologyTree` | Recursive sub-template; see below |

### Template format — list example (`wans_list.tmpl`)

```
ID	NAME	IP	UPTIME	STATUS
{{ range .Items }}{{ .Id }}	{{ .Name }}	{{ .IpAddress }}	{{ formatUptime .Uptime }}	{{ .Status }}
{{ end }}
```

### Template format — get example (`wans_get.tmpl`)

```
FIELD	VALUE
ID	{{ .Id }}
NAME	{{ .Name }}
IP	{{ .IpAddress }}
UPTIME	{{ formatUptime .Uptime }}
STATUS	{{ .Status }}
DNS	{{ join .DnsServers ", " }}
```

### Template format — conditional rows (`vlans_get.tmpl`)

```
...
DHCP MODE	{{ .DhcpMode }}
{{- if eq (string .DhcpMode) "server" }}
DHCP RANGE	{{ .DhcpRange.Start }} - {{ .DhcpRange.End }}
{{- end }}
{{- if eq (string .DhcpMode) "relay" }}
RELAY	{{ derefStr .RelayServer }}
{{- end }}
...
```

### Template format — sub-tables (`ssids_get.tmpl`)

Blank line then a new header row separates each sub-table, matching current output:

```
...
{{ if .Clients }}
CLIENT
{{ range .Clients }}{{ .Name }}
{{ end }}{{- end }}
```

## Discriminated Union Handling

For `devices get` and `clients get`, the discriminator resolution stays in Go. Each branch calls `RenderTemplate` with the resolved typed struct:

```go
switch disc {
case "switch":
    d, err := detail.AsSwitchDetail()
    if err != nil { return err }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_get_switch.tmpl", d)
case "accessPoint":
    ...
}
```

The `--all-ports` flag for switches cannot be handled by mutating `gen.SwitchDetail` (generated type). Instead a thin view struct is built in Go before calling `RenderTemplate`:

```go
type switchDetailView struct {
    gen.SwitchDetail
    Ports []gen.NetworkPort  // filtered slice replaces the embedded field
}
```

The Go code filters `d.Ports` based on `allPorts`, sets `view.Ports`, and passes `view` to the template. The template accesses `.Ports` and sees only the relevant ports.

## Topology — Bespoke Template

The topology command is the canonical bespoke case: a tree render that fits neither list nor get.

**Data preprocessing stays in Go.** A new unexported `topologyTree` struct holds the pre-processed tree:

```go
type topologyTree struct {
    Root      topologyNode
    Adjacency map[string][]topologyNode
}

type topologyNode struct {
    ID       string
    Display  string
    EdgeDisp string
}
```

`buildTopologyTree(topo gen.NetworkTopology, includeWireless bool) (topologyTree, error)` replaces the current inline preprocessing. The existing `printTopologyTree` / `printTopologyNode` functions are removed.

**`topology.tmpl`** renders the tree recursively using a named sub-template and the `dict` helper:

```
{{ .Root.Display }}
{{- range $i, $child := index .Adjacency .Root.ID }}
{{ template "node" dict "Entry" $child "Adjacency" $.Adjacency "Prefix" "" "IsLast" (isLast $i (index $.Adjacency $.Root.ID)) }}
{{- end }}

{{- define "node" }}
{{ .Prefix }}{{ connector .IsLast }}{{ .Entry.Display }}{{ if .Entry.EdgeDisp }} {{ .Entry.EdgeDisp }}{{ end }}
{{- range $i, $child := index .Adjacency .Entry.ID }}
{{ template "node" dict "Entry" $child "Adjacency" $.Adjacency "Prefix" (childPrefix $.Prefix $.IsLast) "IsLast" (isLast $i (index $.Adjacency $.Entry.ID)) }}
{{- end }}
{{- end }}
```

Additional func map entries for topology: `connector` (returns `├── ` or `└── `), `childPrefix` (returns `│   ` or `    `), `isLast`.

## Error Handling

`RenderTemplate` returns the first error from template parsing or execution. Commands propagate it as before (`return err`). Template errors surface as command errors.

## Testing

No test changes. All existing assertions use `strings.Contains` — rendered output is semantically identical to current output. Whitespace trim markers in templates ensure no extra blank lines are introduced.

The `output.RenderTemplate` function itself warrants a small unit test: one list template and one get template exercising the tabwriter flush and func map.

## Files Changed

| File | Change |
|---|---|
| `internal/output/output.go` | Add `RenderTemplate` function and func map |
| `internal/cli/network/templates.go` | New file: `//go:embed` declaration |
| `internal/cli/network/templates/*.tmpl` | 15 new template files |
| `internal/cli/network/network.go` | Replace table rendering in devices/clients commands; extract `buildTopologyTree` |
| `internal/cli/network/wans.go` | Replace table rendering |
| `internal/cli/network/vlans.go` | Replace table rendering |
| `internal/cli/network/ssids.go` | Replace table rendering |

## Out of Scope

- Reflection-based default renderer (deferred to generator phase)
- Migration of docker, storage, system domains
- Code generator (`tools/hlctl-gen`)
- Any changes to `oapi-codegen` pipeline
