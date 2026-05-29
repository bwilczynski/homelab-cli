# Network Domain Template Views Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the table rendering path of all network domain commands into `text/template` files, leaving HTTP plumbing and JSON path unchanged, with all existing tests passing.

**Architecture:** Add `output.RenderTemplate(w, fs, name, data)` that wraps a tabwriter, parses templates from an embedded FS, and executes the named template. Template files in `internal/cli/network/templates/` replace hardcoded `headers+rows` blocks. Discriminated-union resolution stays in Go; the resolved typed struct is passed to the template. Topology preprocessing is extracted into `buildTopologyTree`, and recursive rendering moves to `topology.tmpl`.

**Tech Stack:** Go `text/template`, `tabwriter`, `io/fs.FS`, `embed.FS`, `testing/fstest.MapFS` (tests only).

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/output/output.go` | Modify | Add `RenderTemplate` + full func map |
| `internal/output/output_test.go` | Modify | Add `TestRenderTemplate_*` tests |
| `internal/cli/network/templates.go` | Create | `//go:embed` declaration + `networkTemplates fs.FS` |
| `internal/cli/network/templates/device_base.tmpl` | Create | Shared device base sub-template (used by all 4 device-get templates) |
| `internal/cli/network/templates/devices_list.tmpl` | Create | |
| `internal/cli/network/templates/devices_get_switch.tmpl` | Create | Calls `deviceBase`; ports sub-table |
| `internal/cli/network/templates/devices_get_accesspoint.tmpl` | Create | Calls `deviceBase`; clients sub-table |
| `internal/cli/network/templates/devices_get_gateway.tmpl` | Create | Calls `deviceBase` only |
| `internal/cli/network/templates/devices_get_unknown.tmpl` | Create | Calls `deviceBase` only |
| `internal/cli/network/templates/clients_list.tmpl` | Create | |
| `internal/cli/network/templates/clients_get_wired.tmpl` | Create | Optional port/linkspeed/uptime rows |
| `internal/cli/network/templates/clients_get_wireless.tmpl` | Create | Optional signal/uptime rows |
| `internal/cli/network/templates/vlans_list.tmpl` | Create | |
| `internal/cli/network/templates/vlans_get.tmpl` | Create | Conditional dhcp rows |
| `internal/cli/network/templates/ssids_list.tmpl` | Create | |
| `internal/cli/network/templates/ssids_get.tmpl` | Create | Clients + APs sub-tables |
| `internal/cli/network/templates/wans_list.tmpl` | Create | |
| `internal/cli/network/templates/wans_get.tmpl` | Create | |
| `internal/cli/network/templates/topology.tmpl` | Create | Recursive tree via named sub-template |
| `internal/cli/network/network.go` | Modify | Add `switchDetailView`, `switchPortView`, `topologyTree`, `topologyEdge`, `buildTopologyTree`; replace rendering in devices/clients/topology commands |
| `internal/cli/network/wans.go` | Modify | Replace rendering with `RenderTemplate` |
| `internal/cli/network/vlans.go` | Modify | Replace rendering with `RenderTemplate` |
| `internal/cli/network/ssids.go` | Modify | Replace rendering with `RenderTemplate`; remove `formatBands` |

> **Template tab notation:** Template files use literal tab characters (`0x09`) between columns. This plan shows them as `<TAB>`. In the Write tool calls below, actual tab characters are used.

---

## Task 1: Add `RenderTemplate` to the output package

**Files:**
- Modify: `internal/output/output.go`
- Modify: `internal/output/output_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/output/output_test.go`:

```go
import (
    "bytes"
    "strings"
    "testing"
    "testing/fstest"

    "github.com/bwilczynski/hlctl/internal/output"
)

func TestRenderTemplate_list(t *testing.T) {
    fsys := fstest.MapFS{
        "list.tmpl": &fstest.MapFile{Data: []byte("NAME\tCOUNT\n{{ range .Items }}{{ .Name }}\t{{ .Count }}\n{{ end }}")},
    }
    type row struct {
        Name  string
        Count int
    }
    type data struct{ Items []row }

    var buf bytes.Buffer
    err := output.RenderTemplate(&buf, fsys, "list.tmpl", data{Items: []row{{"foo", 1}, {"bar", 2}}})
    if err != nil {
        t.Fatal(err)
    }
    out := buf.String()
    for _, want := range []string{"NAME", "COUNT", "foo", "bar"} {
        if !strings.Contains(out, want) {
            t.Errorf("expected %q in output, got:\n%s", want, out)
        }
    }
}

func TestRenderTemplate_formatFuncs(t *testing.T) {
    fsys := fstest.MapFS{
        "t.tmpl": &fstest.MapFile{Data: []byte("{{ formatUptime .Uptime }}\n{{ formatBands .Bands }}\n{{ derefStr .Ptr }}")},
    }
    ptr := "hello"
    type data struct {
        Uptime int
        Bands  []string
        Ptr    *string
    }

    var buf bytes.Buffer
    err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{Uptime: 86400, Bands: []string{"band2g", "band5g"}, Ptr: &ptr})
    if err != nil {
        t.Fatal(err)
    }
    out := buf.String()
    for _, want := range []string{"1d", "2.4 GHz", "5 GHz", "hello"} {
        if !strings.Contains(out, want) {
            t.Errorf("expected %q in output, got:\n%s", want, out)
        }
    }
}

func TestRenderTemplate_unknownTemplate(t *testing.T) {
    fsys := fstest.MapFS{
        "a.tmpl": &fstest.MapFile{Data: []byte("hello")},
    }
    err := output.RenderTemplate(&bytes.Buffer{}, fsys, "missing.tmpl", nil)
    if err == nil {
        t.Fatal("expected error for missing template name")
    }
}

func TestRenderTemplate_flush(t *testing.T) {
    fsys := fstest.MapFS{
        "t.tmpl": &fstest.MapFile{Data: []byte("A\tB\nfoo\tbar\n{{ flush }}\nC\tD\nbaz\tqux\n")},
    }
    var buf bytes.Buffer
    if err := output.RenderTemplate(&buf, fsys, "t.tmpl", nil); err != nil {
        t.Fatal(err)
    }
    out := buf.String()
    for _, want := range []string{"A", "B", "foo", "bar", "C", "D", "baz", "qux"} {
        if !strings.Contains(out, want) {
            t.Errorf("expected %q in output, got:\n%s", want, out)
        }
    }
}
```

- [ ] **Step 2: Run the tests to confirm they fail**

```sh
cd /Users/bwilczynski/Projects/github/bwilczynski/homelab-cli
go test ./internal/output/... -run TestRenderTemplate -v
```

Expected: compilation error — `output.RenderTemplate` undefined.

- [ ] **Step 3: Implement `RenderTemplate` in `internal/output/output.go`**

Add these imports to the existing import block:

```go
import (
    "encoding/json"
    "fmt"
    "io"
    "io/fs"
    "reflect"
    "strings"
    "text/tabwriter"
    "text/template"
    "time"
)
```

Add the function at the end of `output.go`:

```go
// RenderTemplate executes the named template from fsys into w, with a tabwriter
// for column alignment. Call {{ flush }} in the template between independent
// table sections to reset column-width tracking.
func RenderTemplate(w io.Writer, fsys fs.FS, name string, data any) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	funcMap := template.FuncMap{
		"formatUptime":     FormatUptime,
		"formatBytes":      FormatBytes,
		"formatBytesPerSec": FormatBytesPerSec,
		"formatLinkSpeed":  FormatLinkSpeed,
		"formatTime":       FormatTime,
		"join":             strings.Join,
		"derefStr": func(v any) string {
			if v == nil {
				return ""
			}
			rv := reflect.ValueOf(v)
			if rv.Kind() == reflect.Ptr {
				if rv.IsNil() {
					return ""
				}
				return fmt.Sprintf("%s", rv.Elem().Interface())
			}
			return fmt.Sprintf("%s", v)
		},
		"derefInt": func(v any) int {
			if v == nil {
				return 0
			}
			rv := reflect.ValueOf(v)
			if rv.Kind() == reflect.Ptr {
				if rv.IsNil() {
					return 0
				}
				rv = rv.Elem()
			}
			switch rv.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return int(rv.Int())
			}
			return 0
		},
		"derefFloat": func(v any) float64 {
			if v == nil {
				return 0
			}
			rv := reflect.ValueOf(v)
			if rv.Kind() == reflect.Ptr {
				if rv.IsNil() {
					return 0
				}
				rv = rv.Elem()
			}
			switch rv.Kind() {
			case reflect.Float32, reflect.Float64:
				return rv.Float()
			}
			return 0
		},
		"string": func(v any) string {
			return fmt.Sprintf("%s", v)
		},
		"formatBands": func(bands any) string {
			rv := reflect.ValueOf(bands)
			if rv.Kind() != reflect.Slice {
				return ""
			}
			parts := make([]string, 0, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				switch fmt.Sprintf("%s", rv.Index(i).Interface()) {
				case "band2g":
					parts = append(parts, "2.4 GHz")
				case "band5g":
					parts = append(parts, "5 GHz")
				case "band6g":
					parts = append(parts, "6 GHz")
				default:
					parts = append(parts, fmt.Sprintf("%s", rv.Index(i).Interface()))
				}
			}
			return strings.Join(parts, ", ")
		},
		"dict": func(args ...any) (map[string]any, error) {
			if len(args)%2 != 0 {
				return nil, fmt.Errorf("dict requires an even number of arguments")
			}
			m := make(map[string]any, len(args)/2)
			for i := 0; i < len(args); i += 2 {
				k, ok := args[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				m[k] = args[i+1]
			}
			return m, nil
		},
		"isLast": func(i int, slice any) bool {
			rv := reflect.ValueOf(slice)
			if rv.Kind() != reflect.Slice {
				return false
			}
			return i == rv.Len()-1
		},
		"connector": func(isLast bool) string {
			if isLast {
				return "└── "
			}
			return "├── "
		},
		"childPrefix": func(prefix string, isLast bool) string {
			if isLast {
				return prefix + "    "
			}
			return prefix + "│   "
		},
		"flush": func() (string, error) {
			return "", tw.Flush()
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(fsys, "*.tmpl")
	if err != nil {
		return err
	}

	t := tmpl.Lookup(name)
	if t == nil {
		return fmt.Errorf("template %q not found", name)
	}

	if err := t.Execute(tw, data); err != nil {
		return err
	}
	return tw.Flush()
}
```

- [ ] **Step 4: Run the tests to confirm they pass**

```sh
go test ./internal/output/... -v
```

Expected: all tests pass including `TestRenderTemplate_*`.

- [ ] **Step 5: Commit**

```sh
git add internal/output/output.go internal/output/output_test.go
git commit -m "feat: add RenderTemplate to output package"
```

---

## Task 2: Embed declaration and wans templates

**Files:**
- Create: `internal/cli/network/templates.go`
- Create: `internal/cli/network/templates/wans_list.tmpl`
- Create: `internal/cli/network/templates/wans_get.tmpl`
- Modify: `internal/cli/network/wans.go`

- [ ] **Step 1: Create the embed declaration**

Create `internal/cli/network/templates.go`:

```go
package network

import (
	"embed"
	"io/fs"
)

//go:embed templates
var embeddedTemplates embed.FS

// networkTemplates is the root FS for network domain templates (no "templates/" prefix).
var networkTemplates, _ = fs.Sub(embeddedTemplates, "templates")
```

- [ ] **Step 2: Create `templates/wans_list.tmpl`**

File content (columns separated by literal tab characters):

```
ID	NAME	IP	UPTIME	STATUS
{{ range .Items -}}
{{ .Id }}	{{ .Name }}	{{ .IpAddress }}	{{ formatUptime .Uptime }}	{{ .Status }}
{{ end -}}
```

- [ ] **Step 3: Create `templates/wans_get.tmpl`**

```
FIELD	VALUE
ID	{{ .Id }}
NAME	{{ .Name }}
IP	{{ .IpAddress }}
UPTIME	{{ formatUptime .Uptime }}
STATUS	{{ .Status }}
DNS	{{ join .DnsServers ", " }}
```

- [ ] **Step 4: Wire up `wans.go`**

In `newListWansCmd`, replace the table-rendering block:

```go
// Before:
headers := []string{"ID", "NAME", "IP", "UPTIME", "STATUS"}
var rows [][]string
for _, w := range list.Items {
    rows = append(rows, []string{
        w.Id, w.Name, w.IpAddress,
        output.FormatUptime(w.Uptime),
        string(w.Status),
    })
}
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)

// After:
return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "wans_list.tmpl", list)
```

In `newGetWanCmd`, replace the table-rendering block:

```go
// Before:
headers := []string{"FIELD", "VALUE"}
rows := [][]string{
    {"ID", detail.Id},
    {"NAME", detail.Name},
    {"IP", detail.IpAddress},
    {"UPTIME", output.FormatUptime(detail.Uptime)},
    {"STATUS", string(detail.Status)},
    {"DNS", strings.Join(detail.DnsServers, ", ")},
}
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), nil, headers, rows)

// After:
return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "wans_get.tmpl", detail)
```

Remove the now-unused `strings` import from `wans.go` if it becomes unused (the `strings.Join` call is gone).

- [ ] **Step 5: Run existing network tests**

```sh
go test ./internal/cli/network/... -v -run "TestListWansCmd|TestGetWanCmd"
```

Expected: all 4 wans tests pass.

- [ ] **Step 6: Commit**

```sh
git add internal/cli/network/templates.go internal/cli/network/templates/wans_list.tmpl internal/cli/network/templates/wans_get.tmpl internal/cli/network/wans.go
git commit -m "feat: extract wans view to templates"
```

---

## Task 3: vlans templates

**Files:**
- Create: `internal/cli/network/templates/vlans_list.tmpl`
- Create: `internal/cli/network/templates/vlans_get.tmpl`
- Modify: `internal/cli/network/vlans.go`

- [ ] **Step 1: Create `templates/vlans_list.tmpl`**

```
ID	NAME	VLAN ID	SUBNET
{{ range .Items -}}
{{ .Id }}	{{ .Name }}	{{ .VlanId }}	{{ .Subnet }}
{{ end -}}
```

- [ ] **Step 2: Create `templates/vlans_get.tmpl`**

`DhcpMode` values are `"server"`, `"relay"`, `"disabled"`. `DhcpRange.Start`/`.End` are strings. `RelayServer` is `*string`.

```
FIELD	VALUE
ID	{{ .Id }}
NAME	{{ .Name }}
VLAN ID	{{ .VlanId }}
SUBNET	{{ .Subnet }}
GATEWAY IP	{{ .GatewayIp }}
BROADCAST	{{ .BroadcastIp }}
DHCP MODE	{{ .DhcpMode }}
{{- if and (eq (string .DhcpMode) "server") .DhcpRange }}
DHCP RANGE	{{ .DhcpRange.Start }} - {{ .DhcpRange.End }}
{{- end }}
{{- if and (eq (string .DhcpMode) "relay") .RelayServer }}
RELAY	{{ derefStr .RelayServer }}
{{- end }}
DNS	{{ join .DnsServers ", " }}
```

- [ ] **Step 3: Wire up `vlans.go`**

In `newListVlansCmd`, replace the table-rendering block:

```go
// Before:
headers := []string{"ID", "NAME", "VLAN ID", "SUBNET"}
var rows [][]string
for _, v := range list.Items {
    rows = append(rows, []string{
        v.Id, v.Name, fmt.Sprintf("%d", v.VlanId), v.Subnet,
    })
}
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)

// After:
return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "vlans_list.tmpl", list)
```

In `newGetVlanCmd`, replace the entire table-rendering block:

```go
// Before:
headers := []string{"FIELD", "VALUE"}
rows := [][]string{...}
if detail.DhcpMode == gen.DhcpModeServer && detail.DhcpRange != nil {
    rows = append(rows, ...)
}
if detail.DhcpMode == gen.DhcpModeRelay && detail.RelayServer != nil {
    rows = append(rows, ...)
}
rows = append(rows, []string{"DNS", strings.Join(detail.DnsServers, ", ")})
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), nil, headers, rows)

// After:
return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "vlans_get.tmpl", detail)
```

Remove unused imports (`fmt`, `strings`) from `vlans.go` if they become unused.

- [ ] **Step 4: Run tests**

```sh
go test ./internal/cli/network/... -v -run "TestListVlansCmd|TestGetVlanCmd"
```

Expected: all 6 vlans tests pass.

- [ ] **Step 5: Commit**

```sh
git add internal/cli/network/templates/vlans_list.tmpl internal/cli/network/templates/vlans_get.tmpl internal/cli/network/vlans.go
git commit -m "feat: extract vlans view to templates"
```

---

## Task 4: ssids templates

**Files:**
- Create: `internal/cli/network/templates/ssids_list.tmpl`
- Create: `internal/cli/network/templates/ssids_get.tmpl`
- Modify: `internal/cli/network/ssids.go`

- [ ] **Step 1: Create `templates/ssids_list.tmpl`**

`formatBands` takes the `[]WifiBand` slice and returns a comma-joined formatted string.

```
ID	NAME	VLAN ID	BANDS	CLIENTS
{{ range .Items -}}
{{ .Id }}	{{ .Name }}	{{ .VlanId }}	{{ formatBands .Bands }}	{{ .NumClients }}
{{ end -}}
```

- [ ] **Step 2: Create `templates/ssids_get.tmpl`**

The current code always prints both `--- CLIENTS ---` and `--- BROADCASTING APs ---` sections. The template matches this. `flush` resets tabwriter column tracking between sections.

```
FIELD	VALUE
ID	{{ .Id }}
NAME	{{ .Name }}
VLAN ID	{{ .VlanId }}
BANDS	{{ formatBands .Bands }}
CLIENTS	{{ .NumClients }}
SECURITY	{{ .SecurityProtocol }}
{{ flush }}
--- CLIENTS ---
NAME
{{ range .Clients -}}
{{ .Name }}
{{ end -}}
{{ flush }}
--- BROADCASTING APs ---
NAME
{{ range .BroadcastingAps -}}
{{ .Name }}
{{ end -}}
```

- [ ] **Step 3: Wire up `ssids.go`**

In `newListSsidsCmd`, replace the table-rendering block:

```go
// Before:
headers := []string{"ID", "NAME", "VLAN ID", "BANDS", "CLIENTS"}
var rows [][]string
for _, s := range list.Items {
    rows = append(rows, []string{
        s.Id, s.Name, fmt.Sprintf("%d", s.VlanId),
        formatBands(s.Bands),
        fmt.Sprintf("%d", s.NumClients),
    })
}
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)

// After:
return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "ssids_list.tmpl", list)
```

In `newGetSsidCmd`, replace from `headers := []string{"FIELD", "VALUE"}` through the final `output.Print` call:

```go
// After:
return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "ssids_get.tmpl", detail)
```

Delete the `formatBands` function at the bottom of `ssids.go` — it is now handled by the output func map. Remove unused imports (`fmt`, `strings`).

- [ ] **Step 4: Run tests**

```sh
go test ./internal/cli/network/... -v -run "TestListSsidsCmd|TestGetSsidCmd"
```

Expected: all 4 ssids tests pass.

- [ ] **Step 5: Commit**

```sh
git add internal/cli/network/templates/ssids_list.tmpl internal/cli/network/templates/ssids_get.tmpl internal/cli/network/ssids.go
git commit -m "feat: extract ssids view to templates"
```

---

## Task 5: devices templates

**Files:**
- Create: `internal/cli/network/templates/device_base.tmpl`
- Create: `internal/cli/network/templates/devices_list.tmpl`
- Create: `internal/cli/network/templates/devices_get_switch.tmpl`
- Create: `internal/cli/network/templates/devices_get_accesspoint.tmpl`
- Create: `internal/cli/network/templates/devices_get_gateway.tmpl`
- Create: `internal/cli/network/templates/devices_get_unknown.tmpl`
- Modify: `internal/cli/network/network.go`

- [ ] **Step 1: Create `templates/device_base.tmpl`**

Defines the `deviceBase` named sub-template. All four device-get templates call it. All device detail types have the same field names (`.Id`, `.Name`, `.Mac`, `.Ip`, `.Type`, `.Status`, `.Model`, `.FirmwareVersion`, `.Uptime`, `.Traffic`, `.Uplink`), so one template handles all. `Uplink` is `*NetworkConnection` with `.Device.Name`, `.Port *int`, `.LinkSpeed *NetworkLinkSpeed`.

```
{{- define "deviceBase" -}}
FIELD	VALUE
ID	{{ .Id }}
NAME	{{ .Name }}
MAC	{{ .Mac }}
IP	{{ .Ip }}
TYPE	{{ string .Type }}
STATUS	{{ string .Status }}
MODEL	{{ .Model }}
FIRMWARE	{{ .FirmwareVersion }}
UPTIME	{{ formatUptime .Uptime }}
TRAFFIC RX	{{ formatBytesPerSec .Traffic.RxBytesPerSec }} ({{ formatBytes .Traffic.RxBytesTotal }} total)
TRAFFIC TX	{{ formatBytesPerSec .Traffic.TxBytesPerSec }} ({{ formatBytes .Traffic.TxBytesTotal }} total)
{{- if .Uplink }}
{{- $ul := .Uplink }}
UPLINK	{{ $ul.Device.Name }}{{ if $ul.Port }} (port {{ derefInt $ul.Port }}{{ if $ul.LinkSpeed }}, {{ formatLinkSpeed (derefStr $ul.LinkSpeed) }}{{ end }}){{ end }}
{{- end }}
{{- end }}
```

- [ ] **Step 2: Create `templates/devices_list.tmpl`**

```
ID	NAME	MAC	IP	TYPE	STATUS
{{ range .Items -}}
{{ .Id }}	{{ .Name }}	{{ .Mac }}	{{ .Ip }}	{{ .Type }}	{{ .Status }}
{{ end -}}
```

- [ ] **Step 3: Add `switchDetailView` and `switchPortView` structs to `network.go`**

Add these types near the top of `network.go` (after the imports):

```go
// switchDetailView wraps gen.SwitchDetail with pre-resolved port data.
// The Ports field shadows gen.SwitchDetail.Ports so the template always
// sees []switchPortView, with ConnectedToName already resolved.
type switchDetailView struct {
	gen.SwitchDetail
	Ports []switchPortView
}

type switchPortView struct {
	gen.SwitchPort
	ConnectedToName string
}
```

- [ ] **Step 4: Create `templates/devices_get_switch.tmpl`**

Calls `deviceBase`, flushes, then renders ports. `switchPortView` has all `gen.SwitchPort` fields plus `.ConnectedToName`. `PoePowerWatts` is `*float32`; use `derefFloat`. `LinkSpeed` is `*NetworkLinkSpeed`; use `derefStr` then `formatLinkSpeed`.

```
{{ template "deviceBase" . }}
{{ flush }}
--- PORTS ---
PORT	STATE	SPEED	POE	POE WATTS	RX	TX	CONNECTED TO
{{ range .Ports -}}
{{ .Number }}	{{ .State }}	{{ if and (eq (string .State) "up") .LinkSpeed }}{{ formatLinkSpeed (derefStr .LinkSpeed) }}{{ else }}-{{ end }}	{{ .PoeMode }}	{{ if .PoePowerWatts }}{{ printf "%.1f W" (derefFloat .PoePowerWatts) }}{{ else }}-{{ end }}	{{ formatBytesPerSec .Traffic.RxBytesPerSec }}	{{ formatBytesPerSec .Traffic.TxBytesPerSec }}	{{ .ConnectedToName }}
{{ end -}}
```

- [ ] **Step 5: Create `templates/devices_get_accesspoint.tmpl`**

`AccessPointClient` has `.Client.Name` (NetworkClientRef), `.Ssid` (string), `.SignalStrength` (int).

```
{{ template "deviceBase" . }}
{{ flush }}
--- CLIENTS ---
CLIENT	SSID	SIGNAL
{{ range .ConnectedClients -}}
{{ .Client.Name }}	{{ .Ssid }}	{{ .SignalStrength }} dBm
{{ end -}}
```

- [ ] **Step 6: Create `templates/devices_get_gateway.tmpl`**

```
{{ template "deviceBase" . }}
```

- [ ] **Step 7: Create `templates/devices_get_unknown.tmpl`**

```
{{ template "deviceBase" . }}
```

- [ ] **Step 8: Wire up the devices commands in `network.go`**

In `newListDevicesCmd`, replace the table-rendering block:

```go
// Before:
headers := []string{"ID", "NAME", "MAC", "IP", "TYPE", "STATUS"}
var rows [][]string
for _, d := range list.Items {
    rows = append(rows, []string{
        d.Id, d.Name, d.Mac, d.Ip,
        string(d.Type), string(d.Status),
    })
}
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)

// After:
return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_list.tmpl", list)
```

In `newGetDeviceCmd`, replace each `case` body to use `RenderTemplate`. The discriminator switch stays. The `switch` case must also build the `switchDetailView` with pre-resolved ports and pre-applied `allPorts` filter.

Replace the `switch disc { ... }` block with:

```go
switch disc {
case "switch":
    d, err := detail.AsSwitchDetail()
    if err != nil {
        return err
    }
    var portViews []switchPortView
    for _, p := range d.Ports {
        if !allPorts && p.State != gen.NetworkPortStateUp {
            continue
        }
        connectedTo := "-"
        if p.ConnectedTo != nil {
            kind, err := p.ConnectedTo.Discriminator()
            if err != nil {
                return err
            }
            switch kind {
            case "device":
                ref, err := p.ConnectedTo.AsNetworkDeviceRef()
                if err != nil {
                    return err
                }
                connectedTo = ref.Name
            case "client":
                ref, err := p.ConnectedTo.AsNetworkClientRef()
                if err != nil {
                    return err
                }
                connectedTo = ref.Name
            }
        }
        portViews = append(portViews, switchPortView{SwitchPort: p, ConnectedToName: connectedTo})
    }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_get_switch.tmpl",
        switchDetailView{SwitchDetail: d, Ports: portViews})

case "accessPoint":
    d, err := detail.AsAccessPointDetail()
    if err != nil {
        return err
    }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_get_accesspoint.tmpl", d)

case "gateway":
    d, err := detail.AsGatewayDetail()
    if err != nil {
        return err
    }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_get_gateway.tmpl", d)

case "unknown":
    d, err := detail.AsUnknownDeviceDetail()
    if err != nil {
        return err
    }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_get_unknown.tmpl", d)

default:
    return fmt.Errorf("unknown device type: %s", disc)
}
```

Remove the `deviceBaseRows` helper function from `network.go` — it is no longer called. Remove unused imports (`fmt`, `strings`) if they become unused (check: `fmt` is still used by `newClientsCmd` and `newTopologyCmd`).

- [ ] **Step 9: Run tests**

```sh
go test ./internal/cli/network/... -v -run "TestListDevicesCmd|TestGetDeviceCmd"
```

Expected: all 7 device tests pass (`TestListDevicesCmd_tableOutput`, `TestListDevicesCmd_apiError`, `TestGetDeviceCmd_gateway`, `TestGetDeviceCmd_unknownWithUplink`, `TestGetDeviceCmd_switch_activePorts`, `TestGetDeviceCmd_switch_allPorts`, `TestGetDeviceCmd_accessPoint`).

- [ ] **Step 10: Commit**

```sh
git add internal/cli/network/templates/device_base.tmpl \
        internal/cli/network/templates/devices_list.tmpl \
        internal/cli/network/templates/devices_get_switch.tmpl \
        internal/cli/network/templates/devices_get_accesspoint.tmpl \
        internal/cli/network/templates/devices_get_gateway.tmpl \
        internal/cli/network/templates/devices_get_unknown.tmpl \
        internal/cli/network/network.go
git commit -m "feat: extract devices view to templates"
```

---

## Task 6: clients templates

**Files:**
- Create: `internal/cli/network/templates/clients_list.tmpl`
- Create: `internal/cli/network/templates/clients_get_wired.tmpl`
- Create: `internal/cli/network/templates/clients_get_wireless.tmpl`
- Modify: `internal/cli/network/network.go`

- [ ] **Step 1: Create `templates/clients_list.tmpl`**

`NetworkClient.Ip` is `*string`; use `derefStr`.

```
ID	NAME	MAC	IP	STATUS	CONNECTION
{{ range .Items -}}
{{ .Id }}	{{ .Name }}	{{ .Mac }}	{{ derefStr .Ip }}	{{ .Status }}	{{ .ConnectionType }}
{{ end -}}
```

- [ ] **Step 2: Create `templates/clients_get_wired.tmpl`**

`WiredNetworkClientDetail`: `.ConnectedTo NetworkConnection` (`.Device.Name`, `.Port *int`, `.LinkSpeed *NetworkLinkSpeed`), `.Uptime *int`, `.Ip *string`.

```
FIELD	VALUE
ID	{{ .Id }}
NAME	{{ .Name }}
MAC	{{ .Mac }}
IP	{{ derefStr .Ip }}
CONNECTION	{{ .ConnectionType }}
STATUS	{{ .Status }}
SWITCH	{{ .ConnectedTo.Device.Name }}
{{- if .ConnectedTo.Port }}
PORT	{{ derefInt .ConnectedTo.Port }}
{{- end }}
{{- if .ConnectedTo.LinkSpeed }}
LINK SPEED	{{ formatLinkSpeed (derefStr .ConnectedTo.LinkSpeed) }}
{{- end }}
{{- if .Uptime }}
UPTIME	{{ formatUptime (derefInt .Uptime) }}
{{- end }}
```

- [ ] **Step 3: Create `templates/clients_get_wireless.tmpl`**

`WirelessNetworkClientDetail`: `.ConnectedTo WirelessConnection` (`.Device.Name`, `.Ssid string`, `.SignalStrength *int`), `.Uptime *int`, `.Ip *string`.

```
FIELD	VALUE
ID	{{ .Id }}
NAME	{{ .Name }}
MAC	{{ .Mac }}
IP	{{ derefStr .Ip }}
CONNECTION	{{ .ConnectionType }}
STATUS	{{ .Status }}
AP	{{ .ConnectedTo.Device.Name }}
SSID	{{ .ConnectedTo.Ssid }}
{{- if .ConnectedTo.SignalStrength }}
SIGNAL	{{ derefInt .ConnectedTo.SignalStrength }} dBm
{{- end }}
{{- if .Uptime }}
UPTIME	{{ formatUptime (derefInt .Uptime) }}
{{- end }}
```

- [ ] **Step 4: Wire up clients commands in `network.go`**

In `newListClientsCmd`, replace the table-rendering block:

```go
// Before:
headers := []string{"ID", "NAME", "MAC", "IP", "STATUS", "CONNECTION"}
var rows [][]string
for _, cl := range list.Items {
    ip := ""
    if cl.Ip != nil {
        ip = *cl.Ip
    }
    rows = append(rows, []string{
        cl.Id, cl.Name, cl.Mac, ip,
        string(cl.Status),
        string(cl.ConnectionType),
    })
}
return output.Print(w, flags.GetOutputFormat(), list, headers, rows)

// After:
return output.RenderTemplate(w, networkTemplates, "clients_list.tmpl", list)
```

In `newGetClientCmd`, replace the discriminator switch body:

```go
switch disc {
case "wired":
    d, err := detail.AsWiredNetworkClientDetail()
    if err != nil {
        return err
    }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "clients_get_wired.tmpl", d)

case "wireless":
    d, err := detail.AsWirelessNetworkClientDetail()
    if err != nil {
        return err
    }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "clients_get_wireless.tmpl", d)

default:
    return fmt.Errorf("unknown connection type: %s", disc)
}
```

Remove the `rows` variable, `headers` variable, and the old `switch` body. Remove the final `output.Print` call. Remove unused imports (`fmt`) if no longer needed (check: `fmt` is still used elsewhere in the file).

- [ ] **Step 5: Run tests**

```sh
go test ./internal/cli/network/... -v -run "TestListClientsCmd|TestGetClientCmd"
```

Expected: all 8 client tests pass.

- [ ] **Step 6: Commit**

```sh
git add internal/cli/network/templates/clients_list.tmpl \
        internal/cli/network/templates/clients_get_wired.tmpl \
        internal/cli/network/templates/clients_get_wireless.tmpl \
        internal/cli/network/network.go
git commit -m "feat: extract clients view to templates"
```

---

## Task 7: topology bespoke template

**Files:**
- Create: `internal/cli/network/templates/topology.tmpl`
- Modify: `internal/cli/network/network.go`

- [ ] **Step 1: Add `topologyTree` and `topologyEdge` types to `network.go`**

Add these unexported types near the other view structs at the top of `network.go`:

```go
type topologyTree struct {
	GatewayID      string
	GatewayDisplay string
	Adjacency      map[string][]topologyEdge
}

type topologyEdge struct {
	ID       string
	Display  string
	EdgeDisp string
}
```

- [ ] **Step 2: Add `buildTopologyTree` to `network.go`**

This function contains the same logic as the current `printTopologyTree` but returns a struct instead of printing. Add it after the type declarations:

```go
func buildTopologyTree(topo gen.NetworkTopology, includeWireless bool) (topologyTree, error) {
	nodeDisp := make(map[string]string)
	var gatewayID string

	for _, n := range topo.Nodes {
		disc, err := n.Discriminator()
		if err != nil {
			return topologyTree{}, err
		}
		switch disc {
		case "device":
			d, err := n.AsTopologyDeviceNode()
			if err != nil {
				return topologyTree{}, err
			}
			disp := fmt.Sprintf("%s (%s)", d.Name, string(d.Type))
			if d.NumClients != nil && *d.NumClients > 0 {
				disp += fmt.Sprintf(" [%d clients]", *d.NumClients)
			}
			nodeDisp[d.Id] = disp
			if d.Type == gen.NetworkDeviceTypeGateway {
				gatewayID = d.Id
			}
		case "client":
			cl, err := n.AsTopologyClientNode()
			if err != nil {
				return topologyTree{}, err
			}
			nodeDisp[cl.Id] = fmt.Sprintf("%s (client, %s, %s)", cl.Name, string(cl.ConnectionType), string(cl.Status))
		}
	}

	if gatewayID == "" {
		return topologyTree{}, fmt.Errorf("no gateway node found in topology")
	}

	adjacency := make(map[string][]topologyEdge)

	for _, e := range topo.Edges {
		disc, err := e.Discriminator()
		if err != nil {
			return topologyTree{}, err
		}
		switch disc {
		case "wired":
			we, err := e.AsTopologyWiredEdge()
			if err != nil {
				return topologyTree{}, err
			}
			srcID, err := connectionRefID(we.Source)
			if err != nil {
				return topologyTree{}, err
			}
			edgeDisp := ""
			if we.Port != nil && we.LinkSpeed != nil {
				edgeDisp = fmt.Sprintf("[port %d, %s]", *we.Port, output.FormatLinkSpeed(string(*we.LinkSpeed)))
			} else if we.Port != nil {
				edgeDisp = fmt.Sprintf("[port %d]", *we.Port)
			}
			adjacency[we.Target.Id] = append(adjacency[we.Target.Id], topologyEdge{
				ID:       srcID,
				Display:  nodeDisp[srcID],
				EdgeDisp: edgeDisp,
			})
		case "wireless":
			if !includeWireless {
				continue
			}
			wire, err := e.AsTopologyWirelessEdge()
			if err != nil {
				return topologyTree{}, err
			}
			edgeDisp := fmt.Sprintf("[%s]", wire.Ssid)
			if wire.SignalStrength != nil {
				edgeDisp = fmt.Sprintf("[%s, %d dBm]", wire.Ssid, *wire.SignalStrength)
			}
			adjacency[wire.Target.Id] = append(adjacency[wire.Target.Id], topologyEdge{
				ID:       wire.Source.Id,
				Display:  nodeDisp[wire.Source.Id],
				EdgeDisp: edgeDisp,
			})
		}
	}

	return topologyTree{
		GatewayID:      gatewayID,
		GatewayDisplay: nodeDisp[gatewayID],
		Adjacency:      adjacency,
	}, nil
}
```

- [ ] **Step 3: Create `templates/topology.tmpl`**

The `subtree` define renders children recursively. `$` inside the define is the dict passed to each invocation. `$children` is a template variable (not `$`) so it remains accessible inside the range. `$.Prefix` and `$.Adjacency` access the current invocation's dict.

```
{{ .GatewayDisplay }}
{{- template "subtree" dict "ParentID" .GatewayID "Adjacency" .Adjacency "Prefix" "" }}
{{- define "subtree" }}
{{- $children := index .Adjacency .ParentID }}
{{- range $i, $child := $children }}
{{- $last := isLast $i $children }}
{{ $.Prefix }}{{ connector $last }}{{ $child.Display }}{{ if $child.EdgeDisp }} {{ $child.EdgeDisp }}{{ end }}
{{- template "subtree" dict "ParentID" $child.ID "Adjacency" $.Adjacency "Prefix" (childPrefix $.Prefix $last) }}
{{- end }}
{{- end }}
```

- [ ] **Step 4: Wire up the topology command in `network.go`**

In `newTopologyCmd`, replace the `printTopologyTree` call in the `RunE` body:

```go
// Before:
var topo gen.NetworkTopology
if err := json.Unmarshal(body, &topo); err != nil {
    return err
}
return printTopologyTree(w, topo, includeWireless)

// After:
var topo gen.NetworkTopology
if err := json.Unmarshal(body, &topo); err != nil {
    return err
}
tree, err := buildTopologyTree(topo, includeWireless)
if err != nil {
    return err
}
return output.RenderTemplate(w, networkTemplates, "topology.tmpl", tree)
```

Delete the `printTopologyTree`, `printTopologyNode`, `connectionRefID`, and `childEntry` declarations — they are replaced by `buildTopologyTree` + the template. `connectionRefID` is now called from `buildTopologyTree` so it should be kept. Remove only `printTopologyTree`, `printTopologyNode`, and `childEntry`.

- [ ] **Step 5: Run all network tests**

```sh
go test ./internal/cli/network/... -v
```

Expected: all tests pass, including all topology tests (`TestTopologyCmd_devicesOnly`, `TestTopologyCmd_includeClientsWiredOnly`, `TestTopologyCmd_includeWireless`, `TestTopologyCmd_jsonOutput`, `TestTopologyCmd_apiError`).

- [ ] **Step 6: Run the full test suite**

```sh
go test ./...
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```sh
git add internal/cli/network/templates/topology.tmpl internal/cli/network/network.go
git commit -m "feat: extract topology view to bespoke template"
```
