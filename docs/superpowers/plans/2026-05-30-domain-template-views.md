# Domain Template Views Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate table rendering in the docker, storage, and system CLI domains from inline `headers/rows/output.Print` to `text/template` files, matching the pattern already used by the network domain.

**Architecture:** Each domain gets a `templates/` directory with embedded `.tmpl` files and a `templates.go` embed declaration. Four new helpers (`sortedPairs`, `derefStrSlice`, `derefTime`, `derefInt64`) are added to `output.RenderTemplate`'s shared func map. Two thin view structs (`infoRow`, `utilizationRow`) pre-format fields that can't be expressed in templates without arithmetic helpers.

**Tech Stack:** Go `text/template`, `embed.FS`, `tabwriter`, `reflect`, `sort`

---

## File Map

| Action | Path | Purpose |
|--------|------|---------|
| Modify | `internal/output/output.go` | Add 4 new func map entries |
| Modify | `internal/output/output_test.go` | Unit tests for new helpers |
| Create | `internal/cli/docker/templates.go` | `//go:embed` declaration |
| Create | `internal/cli/docker/templates/containers_list.tmpl` | |
| Create | `internal/cli/docker/templates/containers_get.tmpl` | |
| Create | `internal/cli/docker/templates/networks_list.tmpl` | |
| Create | `internal/cli/docker/templates/networks_get.tmpl` | |
| Create | `internal/cli/docker/templates/images_list.tmpl` | |
| Create | `internal/cli/docker/templates/images_get.tmpl` | |
| Modify | `internal/cli/docker/docker.go` | Replace rendering with RenderTemplate |
| Create | `internal/cli/storage/templates.go` | `//go:embed` declaration |
| Create | `internal/cli/storage/templates/volumes_list.tmpl` | |
| Create | `internal/cli/storage/templates/volumes_get.tmpl` | |
| Create | `internal/cli/storage/templates/backups_list.tmpl` | |
| Create | `internal/cli/storage/templates/backups_get.tmpl` | |
| Modify | `internal/cli/storage/storage.go` | Replace rendering with RenderTemplate |
| Create | `internal/cli/system/templates.go` | `//go:embed` declaration |
| Create | `internal/cli/system/templates/health.tmpl` | |
| Create | `internal/cli/system/templates/info.tmpl` | |
| Create | `internal/cli/system/templates/utilization.tmpl` | |
| Create | `internal/cli/system/templates/updates_list.tmpl` | |
| Create | `internal/cli/system/templates/updates_get_container.tmpl` | |
| Modify | `internal/cli/system/system.go` | Replace rendering; add view structs |

---

## Task 1: Extend the shared func map with four new helpers

**Files:**
- Modify: `internal/output/output.go`
- Modify: `internal/output/output_test.go`

- [ ] **Step 1: Write failing tests for the new helpers**

Append to `internal/output/output_test.go`:

```go
func TestRenderTemplate_sortedPairs(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte(
			"{{ range sortedPairs .M }}{{ index . 0 }}\t{{ index . 1 }}\n{{ end }}",
		)},
	}
	type data struct{ M map[string]string }
	var buf bytes.Buffer
	err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{M: map[string]string{"b": "2", "a": "1", "c": "3"}})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"a", "1", "b", "2", "c", "3"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Index(out, "a") > strings.Index(out, "b") || strings.Index(out, "b") > strings.Index(out, "c") {
		t.Errorf("expected sorted order a < b < c, got:\n%s", out)
	}
}

func TestRenderTemplate_sortedPairs_nil(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte("{{ if sortedPairs .M }}HAS{{ else }}EMPTY{{ end }}")},
	}
	type data struct{ M *map[string]string }
	var buf bytes.Buffer
	if err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{M: nil}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "EMPTY") {
		t.Errorf("expected EMPTY for nil map, got: %s", buf.String())
	}
}

func TestRenderTemplate_derefStrSlice(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte(`{{ join (derefStrSlice .S) ", " }}`)},
	}
	s := []string{"x", "y", "z"}
	type data struct{ S *[]string }
	var buf bytes.Buffer
	if err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{S: &s}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "x, y, z") {
		t.Errorf("expected 'x, y, z', got: %s", buf.String())
	}
}

func TestRenderTemplate_derefStrSlice_nil(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte(`{{ if derefStrSlice .S }}HAS{{ else }}EMPTY{{ end }}`)},
	}
	type data struct{ S *[]string }
	var buf bytes.Buffer
	if err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{S: nil}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "EMPTY") {
		t.Errorf("expected EMPTY for nil slice, got: %s", buf.String())
	}
}

func TestRenderTemplate_derefTimeAndInt64(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte("{{ formatTime (derefTime .T) }}\n{{ derefInt64 .N }}")},
	}
	ts := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	n := int64(42)
	type data struct {
		T *time.Time
		N *int64
	}
	var buf bytes.Buffer
	if err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{T: &ts, N: &n}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"2026-01-02", "42"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestRenderTemplate_derefInt64_nil(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte("{{ derefInt64 .N }}")},
	}
	type data struct{ N *int64 }
	var buf bytes.Buffer
	if err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{N: nil}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "0" {
		t.Errorf("expected 0 for nil int64, got: %s", buf.String())
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/output/ -run 'TestRenderTemplate_sortedPairs|TestRenderTemplate_derefStrSlice|TestRenderTemplate_derefTimeAndInt64|TestRenderTemplate_derefInt64_nil' -v
```

Expected: FAIL — template function not found errors.

- [ ] **Step 3: Add `sort` to imports and four new entries to the func map in `internal/output/output.go`**

Add `"sort"` to the import block:

```go
import (
    "encoding/json"
    "fmt"
    "io"
    "io/fs"
    "reflect"
    "sort"
    "strings"
    "text/tabwriter"
    "text/template"
    "time"
)
```

Add the following four entries inside the `funcMap` literal in `RenderTemplate`, after the existing `"flush"` entry:

```go
"sortedPairs": func(v any) [][2]string {
    if v == nil {
        return nil
    }
    rv := reflect.ValueOf(v)
    if rv.Kind() == reflect.Pointer {
        if rv.IsNil() {
            return nil
        }
        rv = rv.Elem()
    }
    if rv.Kind() != reflect.Map {
        return nil
    }
    pairs := make([][2]string, 0, rv.Len())
    for _, k := range rv.MapKeys() {
        pairs = append(pairs, [2]string{k.String(), fmt.Sprintf("%v", rv.MapIndex(k).Interface())})
    }
    sort.Slice(pairs, func(i, j int) bool { return pairs[i][0] < pairs[j][0] })
    return pairs
},
"derefStrSlice": func(v *[]string) []string {
    if v == nil {
        return nil
    }
    return *v
},
"derefTime":  derefOrZero[time.Time],
"derefInt64": func(v any) int64 {
    if v == nil {
        return 0
    }
    rv := reflect.ValueOf(v)
    if rv.Kind() == reflect.Pointer {
        if rv.IsNil() {
            return 0
        }
        rv = rv.Elem()
    }
    switch rv.Kind() {
    case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
        return rv.Int()
    }
    return 0
},
```

- [ ] **Step 4: Run all output tests**

```bash
go test ./internal/output/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/output/output.go internal/output/output_test.go
git commit -m "feat(output): add sortedPairs, derefStrSlice, derefTime, derefInt64 to template func map"
```

---

## Task 2: Docker domain — templates and migration

**Files:**
- Create: `internal/cli/docker/templates.go`
- Create: `internal/cli/docker/templates/containers_list.tmpl`
- Create: `internal/cli/docker/templates/containers_get.tmpl`
- Create: `internal/cli/docker/templates/networks_list.tmpl`
- Create: `internal/cli/docker/templates/networks_get.tmpl`
- Create: `internal/cli/docker/templates/images_list.tmpl`
- Create: `internal/cli/docker/templates/images_get.tmpl`
- Modify: `internal/cli/docker/docker.go`

- [ ] **Step 1: Confirm existing docker tests pass (baseline)**

```bash
go test ./internal/cli/docker/ -v
```

Expected: all PASS.

- [ ] **Step 2: Create `internal/cli/docker/templates.go`**

```go
package docker

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed templates
var embeddedTemplates embed.FS

var dockerTemplates fs.FS

func init() {
	var err error
	dockerTemplates, err = fs.Sub(embeddedTemplates, "templates")
	if err != nil {
		panic(fmt.Sprintf("failed to create docker templates FS: %v", err))
	}
}
```

- [ ] **Step 3: Create `internal/cli/docker/templates/containers_list.tmpl`**

```
ID	IMAGE	STATUS	CPU	MEMORY
{{ range .Items -}}
{{ .Id }}	{{ .Image }}	{{ string .Status }}	{{ printf "%.1f%%" .Resources.CpuPercent }}	{{ formatBytes .Resources.MemoryBytes }}
{{ end -}}
```

- [ ] **Step 4: Create `internal/cli/docker/templates/containers_get.tmpl`**

```
FIELD	VALUE
ID	{{ .Id }}
NAME	{{ .Name }}
DEVICE	{{ .Device }}
STATUS	{{ string .Status }}
IMAGE	{{ .Image }}
RESTART COUNT	{{ .RestartCount }}
CPU	{{ printf "%.1f%%" .Resources.CpuPercent }}
MEMORY	{{ printf "%s (%.1f%%)" (formatBytes .Resources.MemoryBytes) .Resources.MemoryPercent }}
STARTED AT	{{ formatTime .StartedAt }}
EXIT CODE	{{ .ExitCode }}
OOM KILLED	{{ .OomKilled }}
RESTART POLICY	{{ string .RestartPolicy }}
PRIVILEGED	{{ .Privileged }}
MEMORY LIMIT	{{ if gt .MemoryLimit 0 }}{{ formatBytes .MemoryLimit }}{{ else }}unlimited{{ end }}
{{- if .PortBindings }}
{{ flush }}
PORT BINDINGS
CONTAINER PORT	HOST PORT	PROTOCOL
{{ range .PortBindings -}}
{{ .ContainerPort }}	{{ .HostPort }}	{{ string .Protocol }}
{{ end -}}
{{- end }}
{{- if .Networks }}
{{ flush }}
NETWORKS
NAME	DRIVER
{{ range .Networks -}}
{{ .Name }}	{{ .Driver }}
{{ end -}}
{{- end }}
{{- if .VolumeBindings }}
{{ flush }}
VOLUME BINDINGS
SOURCE	DESTINATION	MODE
{{ range .VolumeBindings -}}
{{ .Source }}	{{ .Destination }}	{{ string .Mode }}
{{ end -}}
{{- end }}
{{- if .EnvVariables }}
{{ flush }}
ENVIRONMENT VARIABLES
KEY	VALUE
{{ range .EnvVariables -}}
{{ .Key }}	{{ .Value }}
{{ end -}}
{{- end }}
{{- if .Entrypoint }}
{{ flush }}
ENTRYPOINT
{{ range .Entrypoint -}}
  {{ . }}
{{ end -}}
{{- end }}
{{- if .Cmd }}
{{ flush }}
COMMAND
{{ range .Cmd -}}
  {{ . }}
{{ end -}}
{{- end }}
{{- if sortedPairs .Labels }}
{{ flush }}
LABELS
KEY	VALUE
{{ range sortedPairs .Labels -}}
{{ index . 0 }}	{{ index . 1 }}
{{ end -}}
{{- end }}
```

- [ ] **Step 5: Create `internal/cli/docker/templates/networks_list.tmpl`**

```
ID	NAME	DEVICE	CONTAINERS
{{ range .Items -}}
{{ .Id }}	{{ .Name }}	{{ .Device }}	{{ .ConnectedContainers }}
{{ end -}}
```

- [ ] **Step 6: Create `internal/cli/docker/templates/networks_get.tmpl`**

```
FIELD	VALUE
ID	{{ .Id }}
NAME	{{ .Name }}
DEVICE	{{ .Device }}
DRIVER	{{ .Driver }}
CONTAINERS	{{ .ConnectedContainers }}
{{- if .Subnet }}
SUBNET	{{ derefStr .Subnet }}
{{- end }}
{{- if .Gateway }}
GATEWAY	{{ derefStr .Gateway }}
{{- end }}
{{- if .Containers }}
{{ flush }}
CONNECTED CONTAINERS
NAME
{{ range .Containers -}}
{{ . }}
{{ end -}}
{{- end }}
```

- [ ] **Step 7: Create `internal/cli/docker/templates/images_list.tmpl`**

```
ID	DEVICE	REPOSITORY	TAGS	SIZE
{{ range .Items -}}
{{ .Id }}	{{ .Device }}	{{ .Repository }}	{{ join .Tags ", " }}	{{ formatBytes .Size }}
{{ end -}}
```

- [ ] **Step 8: Create `internal/cli/docker/templates/images_get.tmpl`**

```
FIELD	VALUE
ID	{{ .Id }}
DEVICE	{{ .Device }}
REPOSITORY	{{ .Repository }}
TAGS	{{ join .Tags ", " }}
SIZE	{{ formatBytes .Size }}
VIRTUAL SIZE	{{ formatBytes .VirtualSize }}
{{- if not (.Created.IsZero) }}
CREATED	{{ formatTime .Created }}
{{- end }}
```

- [ ] **Step 9: Update `internal/cli/docker/docker.go` — replace all table rendering**

Replace the imports block (remove `"sort"` and `"strings"`, which are no longer used):

```go
import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	gen "github.com/bwilczynski/hlctl/internal/docker"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)
```

Replace the `newListCmd` render section (the block after the JSON fast-path check):

```go
// old:
list := resp.JSON200
headers := []string{"ID", "IMAGE", "STATUS", "CPU", "MEMORY"}
var rows [][]string
for _, c := range list.Items {
    rows = append(rows, []string{
        c.Id,
        c.Image,
        string(c.Status),
        fmt.Sprintf("%.1f%%", c.Resources.CpuPercent),
        output.FormatBytes(c.Resources.MemoryBytes),
    })
}
return output.Print(w, flags.GetOutputFormat(), list, headers, rows)

// new:
return output.RenderTemplate(w, dockerTemplates, "containers_list.tmpl", *resp.JSON200)
```

Replace the `newGetCmd` render call:

```go
// old:
return printContainerDetail(cmd, *resp.JSON200)

// new:
return output.RenderTemplate(cmd.OutOrStdout(), dockerTemplates, "containers_get.tmpl", *resp.JSON200)
```

Delete the entire `printContainerDetail` function (lines 139–252 in the original).

Replace the `newListNetworksCmd` render section:

```go
// old:
list := resp.JSON200
headers := []string{"ID", "NAME", "DEVICE", "CONTAINERS"}
var rows [][]string
for _, n := range list.Items {
    rows = append(rows, []string{
        n.Id, n.Name, n.Device,
        fmt.Sprintf("%d", n.ConnectedContainers),
    })
}
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)

// new:
return output.RenderTemplate(cmd.OutOrStdout(), dockerTemplates, "networks_list.tmpl", *resp.JSON200)
```

Replace the `newGetNetworkCmd` render call:

```go
// old:
return printNetworkDetail(cmd, *resp.JSON200)

// new:
return output.RenderTemplate(cmd.OutOrStdout(), dockerTemplates, "networks_get.tmpl", *resp.JSON200)
```

Delete the entire `printNetworkDetail` function (lines 429–464 in the original).

Replace the `newListImagesCmd` render section:

```go
// old:
list := resp.JSON200
headers := []string{"ID", "DEVICE", "REPOSITORY", "TAGS", "SIZE"}
var rows [][]string
for _, img := range list.Items {
    rows = append(rows, []string{
        img.Id,
        img.Device,
        img.Repository,
        strings.Join(img.Tags, ", "),
        output.FormatBytes(img.Size),
    })
}
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)

// new:
return output.RenderTemplate(cmd.OutOrStdout(), dockerTemplates, "images_list.tmpl", *resp.JSON200)
```

Replace the `newGetImageCmd` render call:

```go
// old:
return printImageDetail(cmd, *resp.JSON200)

// new:
return output.RenderTemplate(cmd.OutOrStdout(), dockerTemplates, "images_get.tmpl", *resp.JSON200)
```

Delete the entire `printImageDetail` function (lines 563–579 in the original).

- [ ] **Step 10: Run docker tests**

```bash
go test ./internal/cli/docker/ -v
```

Expected: all PASS.

- [ ] **Step 11: Commit**

```bash
git add internal/cli/docker/
git commit -m "feat(docker): extract table rendering to Go templates"
```

---

## Task 3: Storage domain — templates and migration

**Files:**
- Create: `internal/cli/storage/templates.go`
- Create: `internal/cli/storage/templates/volumes_list.tmpl`
- Create: `internal/cli/storage/templates/volumes_get.tmpl`
- Create: `internal/cli/storage/templates/backups_list.tmpl`
- Create: `internal/cli/storage/templates/backups_get.tmpl`
- Modify: `internal/cli/storage/storage.go`

- [ ] **Step 1: Confirm existing storage tests pass (baseline)**

```bash
go test ./internal/cli/storage/ -v
```

Expected: all PASS.

- [ ] **Step 2: Create `internal/cli/storage/templates.go`**

```go
package storage

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed templates
var embeddedTemplates embed.FS

var storageTemplates fs.FS

func init() {
	var err error
	storageTemplates, err = fs.Sub(embeddedTemplates, "templates")
	if err != nil {
		panic(fmt.Sprintf("failed to create storage templates FS: %v", err))
	}
}
```

- [ ] **Step 3: Create `internal/cli/storage/templates/volumes_list.tmpl`**

```
ID	NAME	DEVICE	RAID	STATUS	SIZE	USED
{{ range .Items -}}
{{ .Id }}	{{ .Name }}	{{ .Device }}	{{ .RaidType }}	{{ string .Status }}	{{ formatBytes .TotalBytes }}	{{ formatBytes .UsedBytes }}
{{ end -}}
```

- [ ] **Step 4: Create `internal/cli/storage/templates/volumes_get.tmpl`**

```
FIELD	VALUE
ID	{{ .Id }}
NAME	{{ .Name }}
DEVICE	{{ .Device }}
FILESYSTEM	{{ .FileSystem }}
RAID	{{ .RaidType }}
STATUS	{{ string .Status }}
POOL STATUS	{{ string .PoolStatus }}
MOUNT PATH	{{ .MountPath }}
SIZE	{{ formatBytes .TotalBytes }}
USED	{{ formatBytes .UsedBytes }}
{{- if .Disks }}
{{ flush }}
DISKS
ID	MODEL	STATUS	TEMP	SIZE
{{ range .Disks -}}
{{ .Id }}	{{ .Model }}	{{ string .Status }}	{{ .TemperatureCelsius }}°C	{{ formatBytes .TotalBytes }}
{{ end -}}
{{- end }}
```

- [ ] **Step 5: Create `internal/cli/storage/templates/backups_list.tmpl`**

```
ID	NAME	DEVICE	STATUS	LAST RESULT	TYPE
{{ range .Items -}}
{{ .Id }}	{{ .Name }}	{{ .Device }}	{{ string .Status }}	{{ string .LastResult }}	{{ .Type }}
{{ end -}}
```

- [ ] **Step 6: Create `internal/cli/storage/templates/backups_get.tmpl`**

```
FIELD	VALUE
ID	{{ .Id }}
NAME	{{ .Name }}
DEVICE	{{ .Device }}
STATUS	{{ string .Status }}
LAST RESULT	{{ string .LastResult }}
TYPE	{{ .Type }}
{{- if .LastRunAt }}
LAST RUN	{{ formatTime (derefTime .LastRunAt) }}
{{- end }}
{{- if .NextRunAt }}
NEXT RUN	{{ formatTime (derefTime .NextRunAt) }}
{{- end }}
{{- if .Size }}
SIZE	{{ formatBytes (derefInt64 .Size) }}
{{- end }}
{{- if .Folders }}
FOLDERS	{{ join (derefStrSlice .Folders) ", " }}
{{- end }}
```

- [ ] **Step 7: Update `internal/cli/storage/storage.go` — replace all table rendering**

Remove `"fmt"` import (all `fmt` usage was in the helper functions being deleted, except the JSON fast-path which uses `fmt.Fprint` — keep `fmt`).

Actually `fmt.Fprint` IS still used (JSON fast-paths), so `fmt` stays. No import changes needed.

Replace the `newListVolumesCmd` render section:

```go
// old:
list := resp.JSON200
headers := []string{"ID", "NAME", "DEVICE", "RAID", "STATUS", "SIZE", "USED"}
var rows [][]string
for _, v := range list.Items {
    rows = append(rows, []string{
        v.Id, v.Name, v.Device, v.RaidType,
        string(v.Status),
        output.FormatBytes(v.TotalBytes),
        output.FormatBytes(v.UsedBytes),
    })
}
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)

// new:
return output.RenderTemplate(cmd.OutOrStdout(), storageTemplates, "volumes_list.tmpl", *resp.JSON200)
```

Replace the `newGetVolumeCmd` render call:

```go
// old:
return printVolumeDetail(cmd, *resp.JSON200)

// new:
return output.RenderTemplate(cmd.OutOrStdout(), storageTemplates, "volumes_get.tmpl", *resp.JSON200)
```

Delete the entire `printVolumeDetail` function (lines 129–168 in the original).

Replace the `newListBackupsCmd` render section:

```go
// old:
list := resp.JSON200
headers := []string{"ID", "NAME", "DEVICE", "STATUS", "LAST RESULT", "TYPE"}
var rows [][]string
for _, t := range list.Items {
    rows = append(rows, []string{
        t.Id, t.Name, t.Device,
        string(t.Status), string(t.LastResult), t.Type,
    })
}
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)

// new:
return output.RenderTemplate(cmd.OutOrStdout(), storageTemplates, "backups_list.tmpl", *resp.JSON200)
```

Replace the entire render section in `newGetBackupCmd` (the block after the JSON fast-path check). The old code builds `headers`, `rows`, and conditionally appends `lastRunAt`, `nextRunAt`, `size`, and `folders` rows before calling `output.Print`. Replace all of that with:

```go
return output.RenderTemplate(cmd.OutOrStdout(), storageTemplates, "backups_get.tmpl", *resp.JSON200)
```

- [ ] **Step 8: Run storage tests**

```bash
go test ./internal/cli/storage/ -v
```

Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/cli/storage/
git commit -m "feat(storage): extract table rendering to Go templates"
```

---

## Task 4: System domain — templates and migration

**Files:**
- Create: `internal/cli/system/templates.go`
- Create: `internal/cli/system/templates/health.tmpl`
- Create: `internal/cli/system/templates/info.tmpl`
- Create: `internal/cli/system/templates/utilization.tmpl`
- Create: `internal/cli/system/templates/updates_list.tmpl`
- Create: `internal/cli/system/templates/updates_get_container.tmpl`
- Modify: `internal/cli/system/system.go`

- [ ] **Step 1: Confirm existing system tests pass (baseline)**

```bash
go test ./internal/cli/system/ -v
```

Expected: all PASS.

- [ ] **Step 2: Create `internal/cli/system/templates.go`**

```go
package system

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed templates
var embeddedTemplates embed.FS

var systemTemplates fs.FS

func init() {
	var err error
	systemTemplates, err = fs.Sub(embeddedTemplates, "templates")
	if err != nil {
		panic(fmt.Sprintf("failed to create system templates FS: %v", err))
	}
}
```

- [ ] **Step 3: Create `internal/cli/system/templates/health.tmpl`**

```
COMPONENT	STATUS
{{ range .Components -}}
{{ .Name }}	{{ string .Status }}
{{ end -}}
```

- [ ] **Step 4: Create `internal/cli/system/templates/info.tmpl`**

Receives `struct{ Items []infoRow }` (defined in system.go in step 8).

```
DEVICE	MODEL	FIRMWARE	RAM	UPTIME
{{ range .Items -}}
{{ .Device }}	{{ .Model }}	{{ .Firmware }}	{{ .Ram }}	{{ .Uptime }}
{{ end -}}
```

- [ ] **Step 5: Create `internal/cli/system/templates/utilization.tmpl`**

Receives `struct{ Items []utilizationRow }` (defined in system.go in step 8).

```
DEVICE	CPU	MEMORY	SWAP
{{ range .Items -}}
{{ .Device }}	{{ .Cpu }}	{{ .Memory }}	{{ .Swap }}
{{ end -}}
```

- [ ] **Step 6: Create `internal/cli/system/templates/updates_list.tmpl`**

```
ID	NAME	DEVICE	TYPE	STATUS	CURRENT	LATEST
{{ range .Items -}}
{{ .Id }}	{{ .Name }}	{{ .Device }}	{{ string .Type }}	{{ string .Status }}	{{ .CurrentVersion }}	{{ .LatestVersion }}
{{ end -}}
```

- [ ] **Step 7: Create `internal/cli/system/templates/updates_get_container.tmpl`**

```
FIELD	VALUE
ID	{{ .Id }}
NAME	{{ .Name }}
DEVICE	{{ .Device }}
TYPE	{{ string .Type }}
STATUS	{{ string .Status }}
CURRENT	{{ .CurrentVersion }}
LATEST	{{ .LatestVersion }}
CHECKED AT	{{ formatTime .CheckedAt }}
PUBLISHED AT	{{ formatTime .PublishedAt }}
IMAGE	{{ .Image }}
SOURCE	{{ .Source }}
RELEASE URL	{{ .ReleaseUrl }}
```

- [ ] **Step 8: Add view structs to `internal/cli/system/system.go`**

Add these two unexported structs at the package level (e.g., after the `buildClient` function):

```go
type infoRow struct {
	Device   string
	Model    string
	Firmware string
	Ram      string
	Uptime   string
}

type utilizationRow struct {
	Device string
	Cpu    string
	Memory string
	Swap   string
}
```

- [ ] **Step 9: Replace `newHealthCmd` render section**

```go
// old:
health := resp.JSON200
headers := []string{"COMPONENT", "STATUS"}
var rows [][]string
for _, comp := range health.Components {
    rows = append(rows, []string{comp.Name, string(comp.Status)})
}
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), health, headers, rows)

// new:
return output.RenderTemplate(cmd.OutOrStdout(), systemTemplates, "health.tmpl", *resp.JSON200)
```

- [ ] **Step 10: Replace `newInfoCmd` render section**

```go
// old:
list := resp.JSON200
headers := []string{"DEVICE", "MODEL", "FIRMWARE", "RAM", "UPTIME"}
var rows [][]string
for _, info := range list.Items {
    rows = append(rows, []string{
        info.Device,
        info.Model,
        info.Firmware,
        output.FormatBytes(int64(info.RamMb) * 1024 * 1024),
        output.FormatUptime(int(info.UptimeSeconds)),
    })
}
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)

// new:
list := resp.JSON200
var rows []infoRow
for _, info := range list.Items {
    rows = append(rows, infoRow{
        Device:   info.Device,
        Model:    info.Model,
        Firmware: info.Firmware,
        Ram:      output.FormatBytes(int64(info.RamMb) * 1024 * 1024),
        Uptime:   output.FormatUptime(int(info.UptimeSeconds)),
    })
}
return output.RenderTemplate(cmd.OutOrStdout(), systemTemplates, "info.tmpl", struct{ Items []infoRow }{rows})
```

- [ ] **Step 11: Replace `newUtilizationCmd` render section**

```go
// old:
list := resp.JSON200
headers := []string{"DEVICE", "CPU", "MEMORY", "SWAP"}
var rows [][]string
for _, u := range list.Items {
    swapPct := 0
    if u.Memory.SwapTotalBytes > 0 {
        swapPct = int(u.Memory.SwapUsedBytes * 100 / u.Memory.SwapTotalBytes)
    }
    rows = append(rows, []string{
        u.Device,
        fmt.Sprintf("%d%%", u.Cpu.TotalPercent),
        fmt.Sprintf("%d%%", u.Memory.UsedPercent),
        fmt.Sprintf("%d%%", swapPct),
    })
}
return output.Print(w, flags.GetOutputFormat(), list, headers, rows)

// new:
list := resp.JSON200
var rows []utilizationRow
for _, u := range list.Items {
    swapPct := 0
    if u.Memory.SwapTotalBytes > 0 {
        swapPct = int(u.Memory.SwapUsedBytes * 100 / u.Memory.SwapTotalBytes)
    }
    rows = append(rows, utilizationRow{
        Device: u.Device,
        Cpu:    fmt.Sprintf("%d%%", u.Cpu.TotalPercent),
        Memory: fmt.Sprintf("%d%%", u.Memory.UsedPercent),
        Swap:   fmt.Sprintf("%d%%", swapPct),
    })
}
return output.RenderTemplate(w, systemTemplates, "utilization.tmpl", struct{ Items []utilizationRow }{rows})
```

- [ ] **Step 12: Replace `printUpdateList` and its two call sites**

Delete the `printUpdateList` function entirely:

```go
// delete this entire function:
func printUpdateList(w io.Writer, list gen.SystemUpdateList) error {
    headers := []string{"ID", "NAME", "DEVICE", "TYPE", "STATUS", "CURRENT", "LATEST"}
    var rows [][]string
    for _, u := range list.Items {
        rows = append(rows, []string{...})
    }
    return output.Print(w, output.FormatTable, list, headers, rows)
}
```

In `newListUpdatesCmd`, replace the call:

```go
// old:
return printUpdateList(cmd.OutOrStdout(), *resp.JSON200)

// new:
return output.RenderTemplate(cmd.OutOrStdout(), systemTemplates, "updates_list.tmpl", *resp.JSON200)
```

In `newCheckUpdatesCmd`, replace the call:

```go
// old:
return printUpdateList(cmd.OutOrStdout(), *resp.JSON200)

// new:
return output.RenderTemplate(cmd.OutOrStdout(), systemTemplates, "updates_list.tmpl", *resp.JSON200)
```

- [ ] **Step 13: Replace `newGetUpdateCmd` container render section**

```go
// old:
headers := []string{"FIELD", "VALUE"}
rows := [][]string{
    {"ID", d.Id},
    {"NAME", d.Name},
    ... (all rows)
}
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)

// new:
return output.RenderTemplate(cmd.OutOrStdout(), systemTemplates, "updates_get_container.tmpl", d)
```

After this change, `io` is no longer used in system.go (the `printUpdateList` function that took `io.Writer` is deleted; `watch.Wrap` closure receives `io.Writer` from the watch package internally but system.go imports `io` for the type annotation in the closure). Check whether `io` is still referenced:

- `watch.Wrap(func(ctx context.Context, w io.Writer) error {` — yes, `io.Writer` is still referenced in the utilization command closure. Keep `io` import.

- [ ] **Step 14: Run system tests**

```bash
go test ./internal/cli/system/ -v
```

Expected: all PASS.

- [ ] **Step 15: Run all tests to confirm no regressions**

```bash
go test ./internal/...
```

Expected: all PASS.

- [ ] **Step 16: Commit**

```bash
git add internal/cli/system/
git commit -m "feat(system): extract table rendering to Go templates"
```
