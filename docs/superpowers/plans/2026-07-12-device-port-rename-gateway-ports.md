# DevicePort Rename & Gateway Ports Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update hlctl for homelab-api-spec v1.5.0 — rename `SwitchPort*` → `DevicePort*` throughout, rename the `--switch` flag to `--device`, and add ports + wans sections to the gateway detail view.

**Architecture:** The generated `internal/api/network/api.gen.go` drives all type names. After `make generate`, fix compile errors in `ports.go` and `devices.go`, update their tests, then add `gatewayDetailView` + template via TDD.

**Tech Stack:** Go, oapi-codegen, Cobra, Go `text/template`

## Global Constraints

- Never add `Co-Authored-By` trailers to commit messages.
- Run `go test ./internal/cli/network/...` (not `./...`) — other packages may be unaffected.
- `make generate` requires the spec submodule to already be up to date (already done — submodule points to v1.5.0).

---

### Task 1: Regenerate the API client

**Files:**
- Modify (generated): `internal/api/network/api.gen.go`

**Interfaces:**
- Produces: updated generated types — `DevicePort`, `DevicePortPoeMode` (pointer on `DevicePort`), `DevicePortVlanMode`, `DevicePortVlanConfig`, `DevicePortLagMembership`, `NetworkPort.Device`, `ListNetworkPortsParams.DeviceId`, `GatewayDetail.Ports []DevicePort`, `GatewayDetail.Wans []WanRef`, `WanRef{Id, Uri, Name string}`

- [ ] **Step 1: Run code generation**

```bash
make generate
```

Expected: regenerates `internal/api/network/api.gen.go`. No output errors.

- [ ] **Step 2: Verify compile errors are present (expected)**

```bash
go build ./internal/cli/network/...
```

Expected: multiple errors referencing `SwitchPort`, `SwitchPortVlanMode`, `SwitchId`, etc. These are fixed in Tasks 2–3.

- [ ] **Step 3: Commit the regenerated client**

```bash
git add internal/api/network/api.gen.go
git commit -m "chore: regenerate API client for homelab-api-spec v1.5.0"
```

---

### Task 2: Fix compile errors in `ports.go` + `devices.go` and update the ports template

**Files:**
- Modify: `internal/cli/network/ports.go`
- Modify: `internal/cli/network/devices.go`
- Modify: `internal/cli/network/templates/ports_list.tmpl`

**Interfaces:**
- Consumes: `networkapi.DevicePort`, `networkapi.DevicePortVlanMode`, `networkapi.DevicePortVlanConfig`, `networkapi.DevicePortVlanConfigTaggedVlansScope*` constants, `networkapi.ListNetworkPortsParams.DeviceId`, `networkapi.NetworkPort.Device` — all from Task 1
- Produces:
  - `listPortsOptions.Device string` (was `Switch`)
  - `portRow.DeviceName string` (was `SwitchName`)
  - `portInput{DeviceName string; DevicePort networkapi.DevicePort}` (was `SwitchName`/`SwitchPort`)
  - `decoratePort(portInput) (portRow, error)` — unchanged signature
  - `switchPortView` embeds `networkapi.DevicePort`, adds `PoeMode string` field
  - `buildDevicePortViews(ports []networkapi.DevicePort, allPorts bool) ([]switchPortView, error)` (was `buildSwitchPortViews`)

- [ ] **Step 1: Update `listPortsOptions` in `ports.go`**

In `internal/cli/network/ports.go`, change the `Switch` field to `Device`:

```go
type listPortsOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format

	Device   string
	Mode     string
	State    string
	VlanID   int
	Wide     bool
	AllPorts bool
}
```

- [ ] **Step 2: Update `portRow` and `portInput` in `ports.go`**

```go
type portRow struct {
	DeviceName      string
	Number          int
	Label           string
	State           string
	LinkSpeed       string
	VlanMode        string
	NativeVlan      string
	TaggedVlans     string
	PoeMode         string
	PoePowerWatts   string
	LagInfo         string
	SfpPresent      string
	LinkUptime      string
	RxPerSec        string
	TxPerSec        string
	ConnectedToName string
}

type portInput struct {
	DeviceName string
	DevicePort networkapi.DevicePort
}
```

- [ ] **Step 3: Update `decoratePort` in `ports.go`**

Replace the function body to use the renamed fields and handle `PoeMode` as a pointer:

```go
func decoratePort(in portInput) (portRow, error) {
	p := in.DevicePort

	connectedTo := "-"
	if p.ConnectedTo != nil {
		kind, err := p.ConnectedTo.Discriminator()
		if err != nil {
			return portRow{}, err
		}
		switch kind {
		case "device":
			ref, err := p.ConnectedTo.AsNetworkDeviceRef()
			if err != nil {
				return portRow{}, err
			}
			connectedTo = ref.Name
		case "client":
			ref, err := p.ConnectedTo.AsNetworkClientRef()
			if err != nil {
				return portRow{}, err
			}
			connectedTo = ref.Name
		}
	}

	label := "-"
	if p.Label != nil {
		label = *p.Label
	}

	linkUptime := "-"
	if p.LinkUptime != nil {
		linkUptime = output.FormatUptime(*p.LinkUptime)
	}

	linkSpeed := "-"
	if p.State == networkapi.NetworkPortStateUp && p.LinkSpeed != nil {
		linkSpeed = output.FormatLinkSpeed(string(*p.LinkSpeed))
	}

	lagInfo := "-"
	if p.LagMembership != nil {
		lagInfo = fmt.Sprintf("%s #%d", p.LagMembership.Role, p.LagMembership.Id)
	}

	sfpPresent := "-"
	if p.SfpModulePresent != nil {
		if *p.SfpModulePresent {
			sfpPresent = "yes"
		} else {
			sfpPresent = "no"
		}
	}

	poeMode := "-"
	if p.PoeMode != nil {
		poeMode = string(*p.PoeMode)
	}

	poePowerWatts := "-"
	if p.PoePowerWatts != nil {
		poePowerWatts = fmt.Sprintf("%.1f W", *p.PoePowerWatts)
	}

	vlanMode := "-"
	nativeVlan := "-"
	taggedVlans := "-"
	if p.VlanConfig != nil {
		vlanMode = string(p.VlanConfig.Mode)
		nativeVlan = fmt.Sprintf("%s (%d)", p.VlanConfig.NativeVlan.Name, p.VlanConfig.NativeVlan.VlanId)
		if p.VlanConfig.Mode == networkapi.DevicePortVlanModeTrunk && p.VlanConfig.TaggedVlans != nil {
			switch p.VlanConfig.TaggedVlans.Scope {
			case networkapi.DevicePortVlanConfigTaggedVlansScopeAll:
				taggedVlans = "all"
			case networkapi.DevicePortVlanConfigTaggedVlansScopeCustom:
				if p.VlanConfig.TaggedVlans.Items != nil && len(*p.VlanConfig.TaggedVlans.Items) > 0 {
					parts := make([]string, 0, len(*p.VlanConfig.TaggedVlans.Items))
					for _, v := range *p.VlanConfig.TaggedVlans.Items {
						parts = append(parts, fmt.Sprintf("%s (%d)", v.Name, v.VlanId))
					}
					taggedVlans = strings.Join(parts, ", ")
				}
			}
		}
	}

	return portRow{
		DeviceName:      in.DeviceName,
		Number:          p.Number,
		Label:           label,
		State:           string(p.State),
		LinkSpeed:       linkSpeed,
		VlanMode:        vlanMode,
		NativeVlan:      nativeVlan,
		TaggedVlans:     taggedVlans,
		PoeMode:         poeMode,
		PoePowerWatts:   poePowerWatts,
		LagInfo:         lagInfo,
		SfpPresent:      sfpPresent,
		LinkUptime:      linkUptime,
		RxPerSec:        output.FormatBytesPerSec(p.Traffic.RxBytesPerSec),
		TxPerSec:        output.FormatBytesPerSec(p.Traffic.TxBytesPerSec),
		ConnectedToName: connectedTo,
	}, nil
}
```

- [ ] **Step 4: Update `listPortsRun` in `ports.go`**

In `listPortsRun`, change the port mapping block to use `DevicePort` and `.Device.Name`:

```go
for _, p := range resp.JSON200.Items {
    row, err := decoratePort(portInput{
        DeviceName: p.Device.Name,
        DevicePort: networkapi.DevicePort{
            Number:           p.Number,
            Label:            p.Label,
            State:            p.State,
            LinkSpeed:        p.LinkSpeed,
            LinkUptime:       p.LinkUptime,
            SfpModulePresent: p.SfpModulePresent,
            PoeMode:          p.PoeMode,
            PoePowerWatts:    p.PoePowerWatts,
            LagMembership:    p.LagMembership,
            VlanConfig:       p.VlanConfig,
            Traffic:          p.Traffic,
            ConnectedTo:      p.ConnectedTo,
        },
    })
```

Also update the sort comparator:

```go
sort.SliceStable(rows, func(i, j int) bool {
    if rows[i].DeviceName != rows[j].DeviceName {
        return rows[i].DeviceName < rows[j].DeviceName
    }
    return rows[i].Number < rows[j].Number
})
```

- [ ] **Step 5: Update `buildListPortsParams` and the flag in `ports.go`**

In `newListPortsCmd`, change the flag registration:

```go
cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID (switch or gateway)")
```

In `buildListPortsParams`:

```go
func buildListPortsParams(opts *listPortsOptions) *networkapi.ListNetworkPortsParams {
	params := &networkapi.ListNetworkPortsParams{}
	if opts.Device != "" {
		s := opts.Device
		params.DeviceId = &s
	}
	if opts.Mode != "" {
		m := networkapi.DevicePortVlanMode(opts.Mode)
		params.Mode = &m
	}
	state := opts.State
	if !opts.AllPorts && state == "" {
		state = "up"
	}
	if !opts.AllPorts && state != "" {
		s := networkapi.NetworkPortState(state)
		params.State = &s
	}
	if opts.VlanID > 0 {
		v := opts.VlanID
		params.VlanId = &v
	}
	return params
}
```

- [ ] **Step 6: Update `switchPortView` and `buildDevicePortViews` in `devices.go`**

Replace the `switchPortView` struct (add `PoeMode string` to shadow the embedded pointer field, embed `DevicePort` instead of `SwitchPort`):

```go
type switchPortView struct {
	networkapi.DevicePort
	ConnectedToName string
	Label           string
	LinkUptime      string
	LagInfo         string
	SfpPresent      string
	PoeMode         string
	VlanMode        string
	NativeVlan      string
	TaggedVlans     string
}
```

Rename `buildSwitchPortViews` to `buildDevicePortViews` and update its signature:

```go
func buildDevicePortViews(ports []networkapi.DevicePort, allPorts bool) ([]switchPortView, error) {
	var out []switchPortView
	for _, p := range ports {
		if !allPorts && p.State != networkapi.NetworkPortStateUp {
			continue
		}
		row, err := decoratePort(portInput{DeviceName: "", DevicePort: p})
		if err != nil {
			return nil, err
		}
		out = append(out, switchPortView{
			DevicePort:      p,
			ConnectedToName: row.ConnectedToName,
			Label:           row.Label,
			LinkUptime:      row.LinkUptime,
			LagInfo:         row.LagInfo,
			SfpPresent:      row.SfpPresent,
			PoeMode:         row.PoeMode,
			VlanMode:        row.VlanMode,
			NativeVlan:      row.NativeVlan,
			TaggedVlans:     row.TaggedVlans,
		})
	}
	return out, nil
}
```

Update the call site in `getDeviceRun` (switch resolver):

```go
portViews, err := buildDevicePortViews(sw.Ports, opts.AllPorts)
```

- [ ] **Step 7: Update `ports_list.tmpl`**

Replace the full content of `internal/cli/network/templates/ports_list.tmpl`:

```
{{ if .Wide -}}
DEVICE	PORT	LABEL	STATE	LINK SPEED	MODE	NATIVE VLAN	TAGGED VLANS	POE	LAG	SFP	RX/S	TX/S	CONNECTED TO
{{ range .Rows -}}
{{ .DeviceName }}	{{ .Number }}	{{ .Label }}	{{ .State }}	{{ .LinkSpeed }}	{{ .VlanMode }}	{{ .NativeVlan }}	{{ .TaggedVlans }}	{{ .PoeMode }}	{{ .LagInfo }}	{{ .SfpPresent }}	{{ .RxPerSec }}	{{ .TxPerSec }}	{{ .ConnectedToName }}
{{ end -}}
{{ else -}}
DEVICE	PORT	LABEL	STATE	MODE	NATIVE VLAN	TAGGED VLANS	CONNECTED TO
{{ range .Rows -}}
{{ .DeviceName }}	{{ .Number }}	{{ .Label }}	{{ .State }}	{{ .VlanMode }}	{{ .NativeVlan }}	{{ .TaggedVlans }}	{{ .ConnectedToName }}
{{ end -}}
{{ end -}}
```

- [ ] **Step 8: Verify compilation**

```bash
go build ./internal/cli/network/...
```

Expected: no errors. (Tests still fail — that's fixed in Task 3.)

- [ ] **Step 9: Commit**

```bash
git add internal/cli/network/ports.go internal/cli/network/devices.go internal/cli/network/templates/ports_list.tmpl
git commit -m "refactor: rename SwitchPort* to DevicePort* and --switch flag to --device"
```

---

### Task 3: Update tests for the renames

**Files:**
- Modify: `internal/cli/network/ports_test.go`
- Modify: `internal/cli/network/devices_test.go`

**Interfaces:**
- Consumes: `listPortsOptions.Device`, `buildDevicePortViews`, `networkapi.DevicePort`, `networkapi.DevicePortVlanMode`, `networkapi.DevicePortVlanConfig`, `networkapi.DevicePortLagMembership` — all from Task 2

- [ ] **Step 1: Update `portsFixture` in `ports_test.go`**

Change the `"switch"` key to `"device"` in every item in `portsFixture`:

```go
"device": map[string]any{
    "kind": "device", "id": "unifi.switch-living-room",
    "uri": "/network/devices/unifi.switch-living-room",
    "name": "Switch Living Room",
},
```

Apply the same change to all three items (second item also references `"Switch Living Room"`, third references `"Switch Attic"`).

- [ ] **Step 2: Update flag test in `ports_test.go`**

In `TestNewListPortsCmd_flagsCaptured`, change `--switch` to `--device` and `captured.Switch` to `captured.Device`:

```go
cmd.SetArgs([]string{
    "--device", "unifi.switch-living-room",
    "--mode", "trunk",
    "--state", "up",
    "--vlan", "20",
    "--wide",
})
// ...
if captured.Device != "unifi.switch-living-room" {
    t.Errorf("Device: got %q", captured.Device)
}
```

- [ ] **Step 3: Update query param test in `ports_test.go`**

In `TestListPortsRun_allQueryParams`, change `Switch` field and `switchId` param:

```go
opts := &listPortsOptions{
    // ...
    Device: "unifi.switch-living-room",
    // ...
}
// ...
cases := map[string]string{
    "deviceId": "unifi.switch-living-room",
    "mode":     "trunk",
    "state":    "down",
    "vlanId":   "20",
}
```

- [ ] **Step 4: Update table output assertion in `ports_test.go`**

In `TestListPortsRun_tableDefault`, change `"SWITCH"` to `"DEVICE"` in the `want` slice:

```go
for _, want := range []string{
    "DEVICE", "PORT", "LABEL", "STATE", "MODE", "NATIVE VLAN", "TAGGED VLANS", "CONNECTED TO",
    // ...
```

Also update `TestListPortsRun_tableWide` if it checks for `"SWITCH"` (check and fix).

- [ ] **Step 5: Update JSON passthrough assertion in `ports_test.go`**

In `TestListPortsRun_jsonPassthrough`:

```go
if strings.Contains(got, "DEVICE") || strings.Contains(got, "NATIVE VLAN") {
    t.Errorf("expected JSON passthrough (no table headers), got:\n%s", got)
}
if !strings.Contains(got, `"device"`) || !strings.Contains(got, "unifi.switch-living-room") {
    t.Errorf("expected raw JSON body, got:\n%s", got)
}
```

- [ ] **Step 6: Update `TestBuildSwitchPortViews_newFields` in `devices_test.go`**

Rename the test to `TestBuildDevicePortViews_newFields`. Replace all `networkapi.SwitchPort*` types with `networkapi.DevicePort*`. Since `PoeMode` is now `*DevicePortPoeMode`, add a pointer variable:

```go
func TestBuildDevicePortViews_newFields(t *testing.T) {
	label := "Backhaul"
	uptime := 90061
	sfp := true
	poeOff := networkapi.DevicePortPoeMode("off")
	iotVlan := networkapi.NetworkVlanRef{Id: "unifi.iot", Uri: "/network/vlans/unifi.iot", Name: "IoT", VlanId: 20}
	guestVlan := networkapi.NetworkVlanRef{Id: "unifi.guest", Uri: "/network/vlans/unifi.guest", Name: "Guest", VlanId: 30}
	ports := []networkapi.DevicePort{
		{
			Number:  1,
			State:   networkapi.NetworkPortStateUp,
			PoeMode: &poeOff,
			Label:   &label,
			LinkUptime: &uptime,
			SfpModulePresent: &sfp,
			LagMembership: &networkapi.DevicePortLagMembership{
				Id:   3,
				Role: networkapi.DevicePortLagMembershipRoleMaster,
			},
			VlanConfig: &networkapi.DevicePortVlanConfig{
				Mode:       networkapi.DevicePortVlanModeTrunk,
				NativeVlan: networkapi.NetworkVlanRef{Id: "unifi.default", Uri: "/network/vlans/unifi.default", Name: "Default", VlanId: 1},
				TaggedVlans: &struct {
					Items *[]networkapi.NetworkVlanRef                         `json:"items,omitempty"`
					Scope networkapi.DevicePortVlanConfigTaggedVlansScope `json:"scope"`
				}{
					Scope: networkapi.DevicePortVlanConfigTaggedVlansScopeCustom,
					Items: &[]networkapi.NetworkVlanRef{iotVlan, guestVlan},
				},
			},
			Traffic: networkapi.NetworkTraffic{},
		},
		{
			Number:  2,
			State:   networkapi.NetworkPortStateUp,
			PoeMode: &poeOff,
			VlanConfig: &networkapi.DevicePortVlanConfig{
				Mode:       networkapi.DevicePortVlanModeTrunk,
				NativeVlan: networkapi.NetworkVlanRef{Id: "unifi.default", Uri: "/network/vlans/unifi.default", Name: "Default", VlanId: 1},
				TaggedVlans: &struct {
					Items *[]networkapi.NetworkVlanRef                         `json:"items,omitempty"`
					Scope networkapi.DevicePortVlanConfigTaggedVlansScope `json:"scope"`
				}{
					Scope: networkapi.DevicePortVlanConfigTaggedVlansScopeAll,
				},
			},
			Traffic: networkapi.NetworkTraffic{},
		},
		{
			Number:  3,
			State:   networkapi.NetworkPortStateUp,
			PoeMode: &poeOff,
			VlanConfig: &networkapi.DevicePortVlanConfig{
				Mode:       networkapi.DevicePortVlanModeAccess,
				NativeVlan: networkapi.NetworkVlanRef{Id: "unifi.default", Uri: "/network/vlans/unifi.default", Name: "Default", VlanId: 1},
			},
			Traffic: networkapi.NetworkTraffic{},
		},
		{
			Number:  4,
			State:   networkapi.NetworkPortStateUp,
			PoeMode: &poeOff,
			Traffic: networkapi.NetworkTraffic{},
		},
	}

	views, err := buildDevicePortViews(ports, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(views) != 4 {
		t.Fatalf("expected 4 views, got %d", len(views))
	}

	p1 := views[0]
	if p1.Label != "Backhaul" {
		t.Errorf("port 1 Label: got %q, want %q", p1.Label, "Backhaul")
	}
	if p1.LinkUptime != "1d 1h 1m 1s" {
		t.Errorf("port 1 LinkUptime: got %q, want %q", p1.LinkUptime, "1d 1h 1m 1s")
	}
	if p1.SfpPresent != "yes" {
		t.Errorf("port 1 SfpPresent: got %q, want %q", p1.SfpPresent, "yes")
	}
	if p1.LagInfo != "master #3" {
		t.Errorf("port 1 LagInfo: got %q, want %q", p1.LagInfo, "master #3")
	}
	if p1.VlanMode != "trunk" {
		t.Errorf("port 1 VlanMode: got %q, want %q", p1.VlanMode, "trunk")
	}
	if p1.NativeVlan != "Default (1)" {
		t.Errorf("port 1 NativeVlan: got %q, want %q", p1.NativeVlan, "Default (1)")
	}
	if p1.TaggedVlans != "IoT (20), Guest (30)" {
		t.Errorf("port 1 TaggedVlans: got %q, want %q", p1.TaggedVlans, "IoT (20), Guest (30)")
	}

	p2 := views[1]
	if p2.TaggedVlans != "all" {
		t.Errorf("port 2 TaggedVlans (scope=all): got %q, want %q", p2.TaggedVlans, "all")
	}

	p3 := views[2]
	if p3.TaggedVlans != "-" {
		t.Errorf("port 3 TaggedVlans (access): got %q, want %q", p3.TaggedVlans, "-")
	}

	p4 := views[3]
	if p4.VlanMode != "-" {
		t.Errorf("port 4 VlanMode (nil): got %q", p4.VlanMode)
	}
	if p4.NativeVlan != "-" {
		t.Errorf("port 4 NativeVlan (nil): got %q", p4.NativeVlan)
	}
	if p4.Label != "-" {
		t.Errorf("port 4 Label (nil): got %q", p4.Label)
	}
	if p4.LinkUptime != "-" {
		t.Errorf("port 4 LinkUptime (nil): got %q", p4.LinkUptime)
	}
	if p4.LagInfo != "-" {
		t.Errorf("port 4 LagInfo (nil): got %q", p4.LagInfo)
	}
	if p4.SfpPresent != "-" {
		t.Errorf("port 4 SfpPresent (nil): got %q", p4.SfpPresent)
	}
}
```

- [ ] **Step 7: Update `TestGetDeviceRun_gateway` in `devices_test.go`**

Add `"ports"` and `"wans"` (now required fields) to the gateway fixture, and remove `"PORTS"` from the `absent` assertion (the template will now always render the PORTS section):

```go
func TestGetDeviceRun_gateway(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.usg", "uri": "/network/devices/unifi.usg",
		"name": "USG", "mac": "aa:bb:cc:dd:00:01", "ip": "192.168.1.1",
		"type": "gateway", "status": "connected",
		"model": "USG-3P", "firmwareVersion": "4.4.57", "uptime": 86400,
		"traffic": map[string]any{
			"rxBytesTotal": int64(12884901888), "txBytesTotal": int64(4294967296),
			"rxBytesPerSec": int64(125000), "txBytesPerSec": int64(50000),
		},
		"ports": []any{},
		"wans":  []any{},
	}
	// ...assertions unchanged except remove "PORTS" from absent list...
	for _, absent := range []string{"CLIENTS", "UPLINK"} {
		if strings.Contains(out.String(), absent) {
			t.Errorf("expected %q absent for gateway, got:\n%s", absent, out.String())
		}
	}
```

- [ ] **Step 8: Update switch fixture tests that reference `SwitchPort` types**

Search `devices_test.go` for any remaining `SwitchPort` references and update them to `DevicePort`. Also check `TestGetDeviceRun_switch_activePorts` — its fixture uses JSON map literals (no type references), so no changes needed there.

- [ ] **Step 9: Run the tests**

```bash
go test ./internal/cli/network/...
```

Expected: all tests pass.

- [ ] **Step 10: Commit**

```bash
git add internal/cli/network/ports_test.go internal/cli/network/devices_test.go
git commit -m "test: update network tests for DevicePort rename and --device flag"
```

---

### Task 4: Add gateway detail view (TDD)

**Files:**
- Modify: `internal/cli/network/devices.go`
- Modify: `internal/cli/network/templates/devices_get_gateway.tmpl`
- Modify: `internal/cli/network/devices_test.go`

**Interfaces:**
- Consumes: `buildDevicePortViews`, `switchPortView`, `networkapi.GatewayDetail.Ports`, `networkapi.GatewayDetail.Wans`, `networkapi.WanRef{Id, Uri, Name string}` — from Tasks 1 & 2
- Produces:
  - `gatewayDetailView{networkapi.GatewayDetail; Ports []switchPortView}`
  - Updated `devices_get_gateway.tmpl` with PORTS and WANS sections

- [ ] **Step 1: Write failing test `TestGetDeviceRun_gateway_ports`**

Add to `internal/cli/network/devices_test.go`:

```go
func TestGetDeviceRun_gateway_ports(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.usg", "uri": "/network/devices/unifi.usg",
		"name": "USG", "mac": "aa:bb:cc:dd:00:01", "ip": "192.168.1.1",
		"type": "gateway", "status": "connected",
		"model": "USG-3P", "firmwareVersion": "4.4.57", "uptime": 86400,
		"traffic": map[string]any{
			"rxBytesTotal": int64(0), "txBytesTotal": int64(0),
			"rxBytesPerSec": int64(0), "txBytesPerSec": int64(0),
		},
		"ports": []any{
			map[string]any{
				"number": 1, "state": "up", "linkSpeed": "gbe1",
				"traffic": map[string]any{
					"rxBytesTotal": int64(0), "txBytesTotal": int64(0),
					"rxBytesPerSec": int64(5000), "txBytesPerSec": int64(1000),
				},
				"connectedTo": map[string]any{
					"kind": "device", "id": "unifi.switch-lr",
					"uri": "/network/devices/unifi.switch-lr", "name": "Switch LR",
				},
			},
			map[string]any{
				"number": 2, "state": "down",
				"traffic": map[string]any{
					"rxBytesTotal": int64(0), "txBytesTotal": int64(0),
					"rxBytesPerSec": int64(0), "txBytesPerSec": int64(0),
				},
			},
		},
		"wans": []any{
			map[string]any{
				"id": "unifi.wan1", "uri": "/network/wans/unifi.wan1", "name": "WAN 1",
			},
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/devices/*"), httpmock.JSONResponse(fixture))

	var out bytes.Buffer
	opts := &getDeviceOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.usg",
	}
	if err := getDeviceRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"PORTS", "Switch LR",
		"WANS", "unifi.wan1", "WAN 1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
	// down port hidden by default (--all-ports not set)
	if strings.Contains(got, "down") {
		t.Errorf("expected down port hidden by default, got:\n%s", got)
	}
	reg.Verify(t)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cli/network/... -run TestGetDeviceRun_gateway_ports -v
```

Expected: FAIL — template renders no PORTS or WANS sections.

- [ ] **Step 3: Add `gatewayDetailView` to `devices.go`**

Add after the `switchDetailView` struct:

```go
type gatewayDetailView struct {
	networkapi.GatewayDetail
	Ports []switchPortView
}
```

- [ ] **Step 4: Update the gateway resolver in `getDeviceRun` in `devices.go`**

Replace the `"gateway"` entry in the `view` declaration:

```go
"gateway": {
    Template: "devices_get_gateway.tmpl",
    Resolve: func(d networkapi.NetworkDeviceDetail) (any, error) {
        gw, err := d.AsGatewayDetail()
        if err != nil {
            return nil, err
        }
        portViews, err := buildDevicePortViews(gw.Ports, opts.AllPorts)
        if err != nil {
            return nil, err
        }
        return gatewayDetailView{GatewayDetail: gw, Ports: portViews}, nil
    },
},
```

- [ ] **Step 5: Update `devices_get_gateway.tmpl`**

Replace the full content of `internal/cli/network/templates/devices_get_gateway.tmpl`:

```
{{ template "deviceBase" . }}
{{ flush }}
--- PORTS ---
PORT	LABEL	STATE	SPEED	UPTIME	SFP	LAG	POE	POE WATTS	RX	TX	CONNECTED TO
{{ range .Ports -}}
{{ .Number }}	{{ .Label }}	{{ .State }}	{{ if and (eq (string .State) "up") .LinkSpeed }}{{ formatLinkSpeed (derefStr .LinkSpeed) }}{{ else }}-{{ end }}	{{ .LinkUptime }}	{{ .SfpPresent }}	{{ .LagInfo }}	{{ .PoeMode }}	{{ if .PoePowerWatts }}{{ printf "%.1f W" (derefFloat .PoePowerWatts) }}{{ else }}-{{ end }}	{{ formatBytesPerSec .Traffic.RxBytesPerSec }}	{{ formatBytesPerSec .Traffic.TxBytesPerSec }}	{{ .ConnectedToName }}
{{ end -}}
{{ flush }}
--- VLAN CONFIG ---
PORT	MODE	NATIVE VLAN	TAGGED VLANS
{{ range .Ports -}}
{{ .Number }}	{{ .VlanMode }}	{{ .NativeVlan }}	{{ .TaggedVlans }}
{{ end -}}
{{ flush }}
--- WANS ---
ID	NAME
{{ range .Wans -}}
{{ .Id }}	{{ .Name }}
{{ end -}}
```

- [ ] **Step 6: Run test to verify it passes**

```bash
go test ./internal/cli/network/... -run TestGetDeviceRun_gateway -v
```

Expected: both `TestGetDeviceRun_gateway` and `TestGetDeviceRun_gateway_ports` PASS.

- [ ] **Step 7: Run the full test suite**

```bash
go test ./internal/cli/network/...
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/network/devices.go internal/cli/network/templates/devices_get_gateway.tmpl internal/cli/network/devices_test.go
git commit -m "feat: show ports and wans in gateway device detail view"
```
