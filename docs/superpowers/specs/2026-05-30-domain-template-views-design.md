# Design: Extract Docker, Storage, and System Domain Views to Go Templates

**Date:** 2026-05-30
**Scope:** `internal/cli/docker/`, `internal/cli/storage/`, `internal/cli/system/`. Network domain unchanged.

## Problem

The docker, storage, and system command files hardcode view logic — building `headers []string` and `rows [][]string` in Go, then calling `output.Print`. This follows the same pattern the network domain had before it was migrated to templates in #13.

## Goal

Apply the same template migration to the remaining three domains: extract all table rendering into `text/template` files. The JSON fast-path and all HTTP/cobra plumbing stay exactly as-is. Existing tests must continue to pass without modification.

## Architecture

Same as the network domain:

```
cobra command (flags, args, HTTP call, JSON unmarshal)   ← unchanged
        │
        ├─ JSON path: fmt.Fprint(raw body)               ← unchanged
        │
        └─ table path: output.RenderTemplate(w, fs, "name.tmpl", data)   ← new
                │
                └─ text/template + tabwriter → stdout
```

Each domain gets its own embedded `templates/` directory and a `templates.go` file with the `//go:embed` declaration, identical to the network domain.

## New Func Map Entries

Two new entries added to `RenderTemplate`'s func map in `internal/output/output.go`:

| Function | Signature | Notes |
|---|---|---|
| `sortedPairs` | `(v any) [][2]string` | Accepts `map[string]string` or `*map[string]string`; returns key-value pairs sorted by key. Used for docker container labels. |
| `derefStrSlice` | `(v *[]string) []string` | Returns the underlying slice, or an empty slice if nil. Used for backup task folders. |

## Template Files

### Docker (6 files)

| File | Receives | Notes |
|---|---|---|
| `containers_list.tmpl` | `gen.ContainerList` | |
| `containers_get.tmpl` | `gen.ContainerDetail` | `flush` between each optional sub-section; labels via `sortedPairs`; entrypoint and cmd as plain indented lists |
| `networks_list.tmpl` | `gen.DockerNetworkList` | |
| `networks_get.tmpl` | `gen.DockerNetworkDetail` | Conditional subnet/gateway rows; optional connected-containers sub-table |
| `images_list.tmpl` | `gen.DockerImageList` | |
| `images_get.tmpl` | `gen.DockerImageDetail` | Conditional created row via `.Created.IsZero` |

### Storage (4 files)

| File | Receives | Notes |
|---|---|---|
| `volumes_list.tmpl` | `gen.StorageVolumeList` | |
| `volumes_get.tmpl` | `gen.VolumeDetail` | Disks sub-table separated by `flush` |
| `backups_list.tmpl` | `gen.BackupTaskList` | |
| `backups_get.tmpl` | `gen.BackupTaskDetail` | Optional rows for lastRunAt/nextRunAt/size; folders via `derefStrSlice` + `join` |

### System (5 files)

| File | Receives | Notes |
|---|---|---|
| `health.tmpl` | `gen.SystemHealth` | |
| `info.tmpl` | `infoRow` view slice | See view struct below; `RamMb` requires int64 conversion |
| `utilization.tmpl` | `utilizationRow` view slice | See view struct below |
| `updates_list.tmpl` | `gen.SystemUpdateList` | |
| `updates_get_container.tmpl` | `gen.ContainerSystemUpdateDetail` | Discriminator stays in Go |

## View Structs

**`infoRow`** — `RamMb` on the generated type is not `int64`, so `formatBytes(int64(info.RamMb) * 1024 * 1024)` cannot be expressed in the template without a conversion helper. Pre-format in Go instead:

```go
type infoRow struct {
    Device   string
    Model    string
    Firmware string
    Ram      string
    Uptime   string
}
```

The command builds `[]infoRow` and passes `struct{ Items []infoRow }` to `info.tmpl`.

**`utilizationRow`** — swap percent cannot be computed in a template without a division helper. A thin per-row struct is built in Go before calling `RenderTemplate`:

```go
type utilizationRow struct {
    Device string
    Cpu    string
    Memory string
    Swap   string
}
```

The command builds `[]utilizationRow`, wraps it in an anonymous struct `struct{ Items []utilizationRow }`, and passes that to `utilization.tmpl`. The template iterates `.Items`.

## Template Format Examples

### List (`containers_list.tmpl`)

```
ID	IMAGE	STATUS	CPU	MEMORY
{{ range .Items -}}
{{ .Id }}	{{ .Image }}	{{ string .Status }}	{{ printf "%.1f%%" .Resources.CpuPercent }}	{{ formatBytes .Resources.MemoryBytes }}
{{ end -}}
```

### Get with optional sub-tables (`containers_get.tmpl`)

```
FIELD	VALUE
ID	{{ .Id }}
NAME	{{ .Name }}
...
MEMORY LIMIT	{{- if gt .MemoryLimit 0 }} {{ formatBytes .MemoryLimit }}{{- else }} unlimited{{- end }}
{{- if .PortBindings }}
{{ flush }}
PORT BINDINGS
CONTAINER PORT	HOST PORT	PROTOCOL
{{ range .PortBindings -}}
{{ .ContainerPort }}	{{ .HostPort }}	{{ string .Protocol }}
{{ end -}}
{{- end }}
...
{{- if sortedPairs .Labels }}
{{ flush }}
LABELS
KEY	VALUE
{{ range sortedPairs .Labels -}}
{{ index . 0 }}	{{ index . 1 }}
{{ end -}}
{{- end }}
```

### Get with optional rows and `*[]string` (`backups_get.tmpl`)

```
FIELD	VALUE
ID	{{ .Id }}
...
{{- if .LastRunAt }}
LAST RUN	{{ formatTime (derefTime .LastRunAt) }}
{{- end }}
{{- if .Size }}
SIZE	{{ formatBytes (derefInt64 .Size) }}
{{- end }}
{{- if .Folders }}
FOLDERS	{{ join (derefStrSlice .Folders) ", " }}
{{- end }}
```

> Note: `LastRunAt` is `*time.Time` and `Size` is `*int64`. These require `derefTime` and `derefInt64` helpers (see below).

## Additional Func Map Entries

The backup template needs two more helpers not currently in the func map:

| Function | Notes |
|---|---|
| `derefTime` | `(*time.Time) time.Time` — nil-safe; returns zero time if nil. Used for `LastRunAt`/`NextRunAt`. |
| `derefInt64` | `(*int64) int64` — nil-safe. Used for backup `Size`. |

The existing `derefInt` covers `*int`, but `Size` is `*int64`. Rather than overloading, `derefInt64` is added alongside.

## Discriminated Union Handling

`system updates get` already resolves the discriminator in Go. The migration keeps this logic and replaces the inline `headers/rows/output.Print` with `RenderTemplate`:

```go
case "container":
    d, err := detail.AsContainerSystemUpdateDetail()
    if err != nil { return err }
    return output.RenderTemplate(cmd.OutOrStdout(), systemTemplates, "updates_get_container.tmpl", d)
```

## Error Handling

No change. `RenderTemplate` returns errors; commands propagate them as before.

## Testing

No test changes. All existing assertions use `strings.Contains`; rendered output is semantically identical to current output.

## Files Changed

| File | Change |
|---|---|
| `internal/output/output.go` | Add `sortedPairs`, `derefStrSlice`, `derefTime`, `derefInt64` to func map |
| `internal/cli/docker/templates.go` | New file: `//go:embed` declaration |
| `internal/cli/docker/templates/*.tmpl` | 6 new template files |
| `internal/cli/docker/docker.go` | Replace all table rendering with `RenderTemplate` |
| `internal/cli/storage/templates.go` | New file: `//go:embed` declaration |
| `internal/cli/storage/templates/*.tmpl` | 4 new template files |
| `internal/cli/storage/storage.go` | Replace all table rendering with `RenderTemplate` |
| `internal/cli/system/templates.go` | New file: `//go:embed` declaration |
| `internal/cli/system/templates/*.tmpl` | 5 new template files |
| `internal/cli/system/system.go` | Replace all table rendering with `RenderTemplate`; add `infoRow` and `utilizationRow` view structs |

## Out of Scope

- Migration of config, auth, login domains
- Code generator (`tools/hlctl-gen`)
- Reflection-based default renderer
