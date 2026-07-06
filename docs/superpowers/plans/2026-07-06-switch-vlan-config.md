# Switch Port VLAN Config & Detail Fields — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Surface five new `SwitchPort` fields (label, linkUptime, lagMembership, sfpModulePresent, vlanConfig) introduced in homelab-api-spec v1.3.0 in the `hlctl network devices get` switch view.

**Architecture:** Pre-resolve all new fields to strings in `buildSwitchPortViews` (same pattern as `ConnectedToName`), then render them in a single extended template — new columns in the existing summary table, and a new VLAN CONFIG block below it, both driven by the same `range .Ports` slice and both respecting `--all-ports`.

**Tech Stack:** Go, Cobra, Go `text/template`, oapi-codegen-generated client (`internal/api/network/api.gen.go`)

## Global Constraints

- API client is already regenerated from spec v1.3.0 (`make generate` was run). Do not re-run it.
- Never add `Co-Authored-By` trailers to commits.
- All new fields on `switchPortView` are pre-resolved strings; no logic in templates.
- `output.FormatUptime(seconds int) string` already exists in `internal/output/output.go` — use it, don't rewrite it.
- Build check: `make build` must pass after each task.
- Lint check: `make lint` must pass after each task.

---

## File Map

| File | Change |
|---|---|
| `internal/cli/network/devices.go` | Add 7 string fields to `switchPortView`; extend `buildSwitchPortViews` |
| `internal/cli/network/templates/devices_get_switch.tmpl` | Add 4 columns to summary table; append VLAN CONFIG block |
| `internal/cli/network/devices_test.go` | Add new switch test cases for new fields |

No new files.

---

### Task 1: Extend `switchPortView` and `buildSwitchPortViews`

**Files:**
- Modify: `internal/cli/network/devices.go`
- Test: `internal/cli/network/devices_test.go`

**Interfaces:**
- Produces: `switchPortView` with fields `Label`, `LinkUptime`, `LagInfo`, `SfpPresent`, `VlanMode`, `NativeVlan`, `TaggedVlans string` — all guaranteed non-empty (absent/nil → `"-"`)

- [ ] **Step 1: Write the failing unit test for `buildSwitchPortViews`**

Add this test to `internal/cli/network/devices_test.go` (after the existing layer-1 tests, before layer-2 tests):

```go
func TestBuildSwitchPortViews_newFields(t *testing.T) {
	label := "Backhaul"
	uptime := 90061 // 1d 1h 1m 1s
	sfp := true
	iotVlan := networkapi.NetworkVlanRef{Id: "unifi.iot", Uri: "/network/vlans/unifi.iot", Name: "IoT", VlanId: 20}
	guestVlan := networkapi.NetworkVlanRef{Id: "unifi.guest", Uri: "/network/vlans/unifi.guest", Name: "Guest", VlanId: 30}
	ports := []networkapi.SwitchPort{
		{
			Number: 1, State: networkapi.NetworkPortStateUp,
			PoeMode: "off",
			Label:   &label,
			LinkUptime: &uptime,
			SfpModulePresent: &sfp,
			LagMembership: &networkapi.SwitchPortLagMembership{Id: 3, Role: networkapi.SwitchPortLagMembershipRoleMaster},
			VlanConfig: &networkapi.SwitchPortVlanConfig{
				Mode:       networkapi.SwitchPortVlanConfigModeTrunk,
				NativeVlan: networkapi.NetworkVlanRef{Id: "unifi.default", Uri: "/network/vlans/unifi.default", Name: "Default", VlanId: 1},
				TaggedVlans: &struct {
					Items *[]networkapi.NetworkVlanRef                        `json:"items,omitempty"`
					Scope networkapi.SwitchPortVlanConfigTaggedVlansScope `json:"scope"`
				}{
					Scope: networkapi.SwitchPortVlanConfigTaggedVlansScopeCustom,
					Items: &[]networkapi.NetworkVlanRef{iotVlan, guestVlan},
				},
			},
			Traffic: networkapi.NetworkTraffic{},
		},
		{
			Number: 2, State: networkapi.NetworkPortStateUp,
			PoeMode: "off",
			VlanConfig: &networkapi.SwitchPortVlanConfig{
				Mode:       networkapi.SwitchPortVlanConfigModeTrunk,
				NativeVlan: networkapi.NetworkVlanRef{Id: "unifi.default", Uri: "/network/vlans/unifi.default", Name: "Default", VlanId: 1},
				TaggedVlans: &struct {
					Items *[]networkapi.NetworkVlanRef                        `json:"items,omitempty"`
					Scope networkapi.SwitchPortVlanConfigTaggedVlansScope `json:"scope"`
				}{
					Scope: networkapi.SwitchPortVlanConfigTaggedVlansScopeAll,
				},
			},
			Traffic: networkapi.NetworkTraffic{},
		},
		{
			Number: 3, State: networkapi.NetworkPortStateUp,
			PoeMode: "off",
			VlanConfig: &networkapi.SwitchPortVlanConfig{
				Mode:       networkapi.SwitchPortVlanConfigModeAccess,
				NativeVlan: networkapi.NetworkVlanRef{Id: "unifi.default", Uri: "/network/vlans/unifi.default", Name: "Default", VlanId: 1},
			},
			Traffic: networkapi.NetworkTraffic{},
		},
		{
			Number: 4, State: networkapi.NetworkPortStateUp,
			PoeMode: "off",
			Traffic: networkapi.NetworkTraffic{},
		},
	}

	views, err := buildSwitchPortViews(ports, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(views) != 4 {
		t.Fatalf("expected 4 views, got %d", len(views))
	}

	// Port 1: all new fields set
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

	// Port 2: scope=all
	p2 := views[1]
	if p2.TaggedVlans != "all" {
		t.Errorf("port 2 TaggedVlans (scope=all): got %q, want %q", p2.TaggedVlans, "all")
	}

	// Port 3: access mode → tagged vlans = "-"
	p3 := views[2]
	if p3.TaggedVlans != "-" {
		t.Errorf("port 3 TaggedVlans (access): got %q, want %q", p3.TaggedVlans, "-")
	}

	// Port 4: no vlanConfig → all VLAN fields "-"
	p4 := views[3]
	if p4.VlanMode != "-" {
		t.Errorf("port 4 VlanMode (nil): got %q, want %q", p4.VlanMode, "-")
	}
	if p4.NativeVlan != "-" {
		t.Errorf("port 4 NativeVlan (nil): got %q, want %q", p4.NativeVlan, "-")
	}
	if p4.Label != "-" {
		t.Errorf("port 4 Label (nil): got %q, want %q", p4.Label, "-")
	}
	if p4.LinkUptime != "-" {
		t.Errorf("port 4 LinkUptime (nil): got %q, want %q", p4.LinkUptime, "-")
	}
	if p4.LagInfo != "-" {
		t.Errorf("port 4 LagInfo (nil): got %q, want %q", p4.LagInfo, "-")
	}
	if p4.SfpPresent != "-" {
		t.Errorf("port 4 SfpPresent (nil): got %q, want %q", p4.SfpPresent, "-")
	}
}
```

The test also needs this import added to the import block in `devices_test.go`:
```go
networkapi "github.com/bwilczynski/hlctl/internal/api/network"
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./internal/cli/network/... -run TestBuildSwitchPortViews_newFields -v
```

Expected: FAIL — `switchPortView` has no fields `Label`, `LinkUptime`, etc.

- [ ] **Step 3: Add new fields to `switchPortView`**

In `internal/cli/network/devices.go`, replace the `switchPortView` struct:

```go
type switchPortView struct {
	networkapi.SwitchPort
	ConnectedToName string
	Label           string
	LinkUptime      string
	LagInfo         string
	SfpPresent      string
	VlanMode        string
	NativeVlan      string
	TaggedVlans     string
}
```

- [ ] **Step 4: Add `fmt` and `strings` imports to `devices.go`**

The import block in `internal/cli/network/devices.go` becomes:

```go
import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)
```

- [ ] **Step 5: Extend `buildSwitchPortViews` to populate the new fields**

Replace the body of `buildSwitchPortViews` in `internal/cli/network/devices.go` with:

```go
func buildSwitchPortViews(ports []networkapi.SwitchPort, allPorts bool) ([]switchPortView, error) {
	var out []switchPortView
	for _, p := range ports {
		if !allPorts && p.State != networkapi.NetworkPortStateUp {
			continue
		}

		connectedTo := "-"
		if p.ConnectedTo != nil {
			kind, err := p.ConnectedTo.Discriminator()
			if err != nil {
				return nil, err
			}
			switch kind {
			case "device":
				ref, err := p.ConnectedTo.AsNetworkDeviceRef()
				if err != nil {
					return nil, err
				}
				connectedTo = ref.Name
			case "client":
				ref, err := p.ConnectedTo.AsNetworkClientRef()
				if err != nil {
					return nil, err
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

		vlanMode := "-"
		nativeVlan := "-"
		taggedVlans := "-"
		if p.VlanConfig != nil {
			vlanMode = string(p.VlanConfig.Mode)
			nativeVlan = fmt.Sprintf("%s (%d)", p.VlanConfig.NativeVlan.Name, p.VlanConfig.NativeVlan.VlanId)
			if p.VlanConfig.Mode == networkapi.SwitchPortVlanConfigModeTrunk && p.VlanConfig.TaggedVlans != nil {
				switch p.VlanConfig.TaggedVlans.Scope {
				case networkapi.SwitchPortVlanConfigTaggedVlansScopeAll:
					taggedVlans = "all"
				case networkapi.SwitchPortVlanConfigTaggedVlansScopeCustom:
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

		out = append(out, switchPortView{
			SwitchPort:      p,
			ConnectedToName: connectedTo,
			Label:           label,
			LinkUptime:      linkUptime,
			LagInfo:         lagInfo,
			SfpPresent:      sfpPresent,
			VlanMode:        vlanMode,
			NativeVlan:      nativeVlan,
			TaggedVlans:     taggedVlans,
		})
	}
	return out, nil
}
```

- [ ] **Step 6: Run the unit test to confirm it passes**

```bash
go test ./internal/cli/network/... -run TestBuildSwitchPortViews_newFields -v
```

Expected: PASS

- [ ] **Step 7: Build and lint**

```bash
make build && make lint
```

Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/network/devices.go internal/cli/network/devices_test.go
git commit -m "feat: extend switchPortView with VLAN config and detail fields"
```

---

### Task 2: Update template and add integration tests

**Files:**
- Modify: `internal/cli/network/templates/devices_get_switch.tmpl`
- Modify: `internal/cli/network/devices_test.go`

**Interfaces:**
- Consumes: `switchPortView.Label`, `.LinkUptime`, `.LagInfo`, `.SfpPresent`, `.VlanMode`, `.NativeVlan`, `.TaggedVlans` from Task 1

- [ ] **Step 1: Write failing integration test for VLAN config rendering**

Add this test to `internal/cli/network/devices_test.go`:

```go
func TestGetDeviceRun_switch_vlanConfig(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr",
		"name": "Switch LR", "mac": "aa:bb:cc:dd:00:10", "ip": "192.168.1.10",
		"type": "switch", "status": "connected",
		"model": "USW-24-PoE", "firmwareVersion": "6.2.14", "uptime": 86400,
		"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
		"ports": []map[string]any{
			{
				"number": 1, "state": "up", "poeMode": "auto",
				"label": "Uplink",
				"linkUptime": 3661,
				"lagMembership": map[string]any{"id": 3, "role": "master"},
				"sfpModulePresent": true,
				"vlanConfig": map[string]any{
					"mode": "trunk",
					"nativeVlan": map[string]any{"id": "unifi.default", "uri": "/network/vlans/unifi.default", "name": "Default", "vlanId": 1},
					"taggedVlans": map[string]any{
						"scope": "custom",
						"items": []map[string]any{
							{"id": "unifi.iot", "uri": "/network/vlans/unifi.iot", "name": "IoT", "vlanId": 20},
							{"id": "unifi.guest", "uri": "/network/vlans/unifi.guest", "name": "Guest", "vlanId": 30},
						},
					},
				},
				"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(1000), "txBytesPerSec": int64(500)},
			},
			{
				"number": 2, "state": "up", "poeMode": "off",
				"vlanConfig": map[string]any{
					"mode": "trunk",
					"nativeVlan": map[string]any{"id": "unifi.default", "uri": "/network/vlans/unifi.default", "name": "Default", "vlanId": 1},
					"taggedVlans": map[string]any{"scope": "all"},
				},
				"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
			},
			{
				"number": 3, "state": "up", "poeMode": "off",
				"vlanConfig": map[string]any{
					"mode": "access",
					"nativeVlan": map[string]any{"id": "unifi.iot", "uri": "/network/vlans/unifi.iot", "name": "IoT", "vlanId": 20},
				},
				"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
			},
			{
				"number": 4, "state": "down", "poeMode": "off",
				"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
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
		ID:         "unifi.switch-lr",
	}
	if err := getDeviceRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()

	// Summary table columns
	for _, want := range []string{"LABEL", "UPTIME", "SFP", "LAG", "Uplink", "1h 1m 1s", "yes", "master #3"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}

	// VLAN CONFIG section
	for _, want := range []string{
		"VLAN CONFIG",
		"trunk", "access",
		"Default (1)", "IoT (20)",
		"IoT (20), Guest (30)",
		"all",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in VLAN CONFIG output, got:\n%s", want, got)
		}
	}

	// Port 4 (down) should be hidden by default
	if strings.Contains(got, "\n4\t") {
		t.Errorf("expected down port 4 hidden by default, got:\n%s", got)
	}

	reg.Verify(t)
}

func TestGetDeviceRun_switch_vlanConfig_allPorts(t *testing.T) {
	fixture := map[string]any{
		"id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr",
		"name": "Switch LR", "mac": "aa:bb:cc:dd:00:10", "ip": "192.168.1.10",
		"type": "switch", "status": "connected",
		"model": "USW-24-PoE", "firmwareVersion": "6.2.14", "uptime": 3600,
		"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
		"ports": []map[string]any{
			{
				"number": 1, "state": "up", "poeMode": "off",
				"vlanConfig": map[string]any{
					"mode":       "access",
					"nativeVlan": map[string]any{"id": "unifi.default", "uri": "/network/vlans/unifi.default", "name": "Default", "vlanId": 1},
				},
				"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
			},
			{
				"number": 2, "state": "down", "poeMode": "off",
				"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
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
		ID:         "unifi.switch-lr",
		AllPorts:   true,
	}
	if err := getDeviceRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()

	// Both ports appear in VLAN CONFIG section when --all-ports
	if !strings.Contains(got, "down") {
		t.Errorf("expected down port visible with --all-ports, got:\n%s", got)
	}
	// Port 2 has no vlanConfig → "-" in VLAN columns
	if !strings.Contains(got, "VLAN CONFIG") {
		t.Errorf("expected VLAN CONFIG section, got:\n%s", got)
	}
	reg.Verify(t)
}
```

- [ ] **Step 2: Run new tests to confirm they fail**

```bash
go test ./internal/cli/network/... -run "TestGetDeviceRun_switch_vlanConfig" -v
```

Expected: FAIL — output doesn't contain `LABEL`, `UPTIME`, `VLAN CONFIG`, etc.

- [ ] **Step 3: Update `devices_get_switch.tmpl`**

Replace the entire content of `internal/cli/network/templates/devices_get_switch.tmpl`:

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
```

- [ ] **Step 4: Run all tests**

```bash
go test ./internal/cli/network/... -v
```

Expected: all tests pass, including the two new ones and all pre-existing switch tests.

- [ ] **Step 5: Build and lint**

```bash
make build && make lint
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/network/templates/devices_get_switch.tmpl internal/cli/network/devices_test.go
git commit -m "feat: add VLAN config section and detail columns to switch port view"
```
