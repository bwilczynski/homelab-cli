# Unified View.Render for Polymorphic and Transform Responses — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate every inline `status check + JSON shortcut + RenderTemplate` trio in `internal/cli/<domain>/` by extending `cmdutil` with `View.RenderWith` (lazy data) and `PolymorphicView[T]` (discriminated-union dispatch).

**Architecture:** Two additions to `internal/cli/cmdutil/view.go`. A shared private helper `renderHead` handles the status + JSON behavior for `View.Render`, `View.RenderWith`, and `PolymorphicView.Render`. Six call sites are migrated to the new APIs; one helper (`buildSwitchPortViews`) is extracted from the device-get switch case.

**Tech Stack:** Go (generics with type-parameter constraint), `testing/fstest` for template fixtures, Cobra leaf commands, oapi-codegen union types.

**Spec:** `docs/superpowers/specs/2026-05-31-polymorphic-view-render-design.md`

---

## File map

- **Modify** `internal/cli/cmdutil/view.go` — add `renderHead` (private), `View.RenderWith`, `Discriminator` interface, `Variant[T]`, `PolymorphicView[T]`; refactor existing `View.Render` to call `renderHead`.
- **Modify** `internal/cli/cmdutil/view_test.go` — add tests for `RenderWith` and `PolymorphicView.Render`.
- **Modify** `internal/cli/network/network.go` — migrate `newGetClientCmd`, `newGetDeviceCmd` (+ extract `buildSwitchPortViews`), `newTopologyCmd`.
- **Modify** `internal/cli/system/system.go` — migrate `newGetUpdateCmd`, `newInfoCmd`, `newUtilizationCmd`.
- **Modify** `CLAUDE.md` — replace polymorphic carve-out with PolymorphicView guidance; document `RenderWith`.

---

## Task 1: Extract `renderHead` and add `View.RenderWith`

**Files:**
- Modify: `internal/cli/cmdutil/view.go`
- Modify: `internal/cli/cmdutil/view_test.go`

- [ ] **Step 1: Write failing tests for `View.RenderWith`**

Append to `internal/cli/cmdutil/view_test.go`:

```go
func TestView_RenderWith_tableInvokesFn(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	called := 0
	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	var buf bytes.Buffer
	err := v.RenderWith(&buf, http.StatusOK, []byte(`{"name":"world"}`), func() (any, error) {
		called++
		return greet{Name: "world"}, nil
	})
	if err != nil {
		t.Fatalf("RenderWith: %v", err)
	}
	if called != 1 {
		t.Errorf("expected fn called once, got %d", called)
	}
	if got := buf.String(); got != "hello world\n" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestView_RenderWith_jsonSkipsFn(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "json"

	called := 0
	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	var buf bytes.Buffer
	body := []byte(`{"name":"world"}`)
	err := v.RenderWith(&buf, http.StatusOK, body, func() (any, error) {
		called++
		return nil, nil
	})
	if err != nil {
		t.Fatalf("RenderWith: %v", err)
	}
	if called != 0 {
		t.Errorf("expected fn NOT called in json mode, got %d invocations", called)
	}
	if buf.String() != string(body) {
		t.Errorf("expected raw body, got %q", buf.String())
	}
}

func TestView_RenderWith_statusMismatchSkipsFn(t *testing.T) {
	called := 0
	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	err := v.RenderWith(&bytes.Buffer{}, http.StatusNotFound, []byte(`{"title":"Not Found"}`), func() (any, error) {
		called++
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
	if called != 0 {
		t.Errorf("expected fn NOT called on status mismatch, got %d invocations", called)
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected ParseError output, got %v", err)
	}
}

func TestView_RenderWith_fnErrorPropagates(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	wantErr := errors.New("boom")
	err := v.RenderWith(&bytes.Buffer{}, http.StatusOK, []byte(`{}`), func() (any, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected fn error to propagate, got %v", err)
	}
}
```

Add `"errors"` to the test file imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/cmdutil/ -run TestView_RenderWith -v`
Expected: FAIL — `v.RenderWith undefined`.

- [ ] **Step 3: Implement `renderHead` and `RenderWith` in `view.go`**

Replace the body of `internal/cli/cmdutil/view.go` with:

```go
package cmdutil

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
)

// View binds a template filesystem, a template name, and the expected success
// status code. Domains declare one View per renderable response and reuse it
// across the command(s) that produce it.
//
// Status is optional and defaults to http.StatusOK. Set it explicitly for
// endpoints that return 201, 202, etc. — the value pairs with the JSON field
// the caller passes (a 201 endpoint populates resp.JSON201, not resp.JSON200).
type View struct {
	Templates fs.FS
	Name      string
	Status    int
}

// renderHead handles the status check and JSON shortcut shared by every
// render path. Returns handled=true when the JSON body has been written and
// the caller should return nil; returns a non-nil error on status mismatch.
func renderHead(w io.Writer, expectedStatus, statusCode int, body []byte) (handled bool, err error) {
	expected := expectedStatus
	if expected == 0 {
		expected = http.StatusOK
	}
	if statusCode != expected {
		return false, apiclient.ParseError(statusCode, body)
	}
	if flags.GetOutputFormat() == output.FormatJSON {
		fmt.Fprint(w, string(body))
		return true, nil
	}
	return false, nil
}

// Render handles the standard response→output flow:
//   - status != v.Status (or 200 if unset) → apiclient.ParseError
//   - --output=json → write raw body
//   - otherwise → render the bound template against data
func (v View) Render(w io.Writer, statusCode int, body []byte, data any) error {
	handled, err := renderHead(w, v.Status, statusCode, body)
	if handled || err != nil {
		return err
	}
	return output.RenderTemplate(w, v.Templates, v.Name, data)
}

// RenderWith mirrors Render but defers data construction. fn is invoked only
// in table mode — JSON mode dumps the raw body without running fn. Use this
// when the template data needs to be derived from the response body and the
// derivation work would be wasted in JSON mode.
func (v View) RenderWith(w io.Writer, statusCode int, body []byte, fn func() (any, error)) error {
	handled, err := renderHead(w, v.Status, statusCode, body)
	if handled || err != nil {
		return err
	}
	data, err := fn()
	if err != nil {
		return err
	}
	return output.RenderTemplate(w, v.Templates, v.Name, data)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/cmdutil/ -v`
Expected: PASS — all existing `TestView_Render_*` plus the four new `TestView_RenderWith_*`.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/cmdutil/view.go internal/cli/cmdutil/view_test.go
git commit -m "feat(cmdutil): add View.RenderWith for lazy template data"
```

---

## Task 2: Add `PolymorphicView[T]` for discriminated-union dispatch

**Files:**
- Modify: `internal/cli/cmdutil/view.go`
- Modify: `internal/cli/cmdutil/view_test.go`

- [ ] **Step 1: Write failing tests for `PolymorphicView.Render`**

Append to `internal/cli/cmdutil/view_test.go`:

```go
type fakeUnion struct {
	Kind string
	Data string
}

func (u fakeUnion) Discriminator() (string, error) {
	if u.Kind == "" {
		return "", errors.New("missing discriminator")
	}
	return u.Kind, nil
}

func polyTemplates() fstest.MapFS {
	return fstest.MapFS{
		"a.tmpl": &fstest.MapFile{Data: []byte("A: {{.}}\n")},
		"b.tmpl": &fstest.MapFile{Data: []byte("B: {{.}}\n")},
	}
}

func newPolyView() cmdutil.PolymorphicView[fakeUnion] {
	return cmdutil.PolymorphicView[fakeUnion]{
		Templates: polyTemplates(),
		Variants: map[string]cmdutil.Variant[fakeUnion]{
			"a": {Template: "a.tmpl", Resolve: func(u fakeUnion) (any, error) { return u.Data, nil }},
			"b": {Template: "b.tmpl", Resolve: func(u fakeUnion) (any, error) { return u.Data, nil }},
		},
	}
}

func TestPolymorphicView_dispatchesToVariant(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := newPolyView()

	var bufA bytes.Buffer
	if err := v.Render(&bufA, http.StatusOK, []byte(`{"kind":"a"}`), &fakeUnion{Kind: "a", Data: "alpha"}); err != nil {
		t.Fatalf("Render a: %v", err)
	}
	if got := bufA.String(); got != "A: alpha\n" {
		t.Errorf("variant a output: %q", got)
	}

	var bufB bytes.Buffer
	if err := v.Render(&bufB, http.StatusOK, []byte(`{"kind":"b"}`), &fakeUnion{Kind: "b", Data: "beta"}); err != nil {
		t.Fatalf("Render b: %v", err)
	}
	if got := bufB.String(); got != "B: beta\n" {
		t.Errorf("variant b output: %q", got)
	}
}

func TestPolymorphicView_jsonModeSkipsDiscriminator(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "json"

	v := newPolyView()
	body := []byte(`{"kind":"a","data":"alpha"}`)
	var buf bytes.Buffer
	// Kind is empty — would error out of Discriminator() if called.
	if err := v.Render(&buf, http.StatusOK, body, &fakeUnion{}); err != nil {
		t.Fatalf("Render json: %v", err)
	}
	if buf.String() != string(body) {
		t.Errorf("expected raw body, got %q", buf.String())
	}
}

func TestPolymorphicView_statusMismatch(t *testing.T) {
	v := newPolyView()
	err := v.Render(&bytes.Buffer{}, http.StatusNotFound, []byte(`{"title":"Not Found"}`), &fakeUnion{Kind: "a"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected ParseError output, got %v", err)
	}
}

func TestPolymorphicView_customStatus(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := newPolyView()
	v.Status = http.StatusCreated

	var buf bytes.Buffer
	if err := v.Render(&buf, http.StatusCreated, []byte(`{}`), &fakeUnion{Kind: "a", Data: "alpha"}); err != nil {
		t.Fatalf("Render 201: %v", err)
	}
	if got := buf.String(); got != "A: alpha\n" {
		t.Errorf("custom status output: %q", got)
	}

	// 200 should now be treated as a mismatch.
	if err := v.Render(&bytes.Buffer{}, http.StatusOK, []byte(`{"title":"oops"}`), &fakeUnion{Kind: "a"}); err == nil {
		t.Fatal("expected mismatch error when status differs from configured Status")
	}
}

func TestPolymorphicView_unknownDiscriminator(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := newPolyView()
	err := v.Render(&bytes.Buffer{}, http.StatusOK, []byte(`{}`), &fakeUnion{Kind: "c"})
	if err == nil {
		t.Fatal("expected error for unknown discriminator")
	}
	if !strings.Contains(err.Error(), "fakeUnion") || !strings.Contains(err.Error(), `"c"`) {
		t.Errorf("expected error to mention type and discriminator value, got %v", err)
	}
}

func TestPolymorphicView_nilDetail(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := newPolyView()
	err := v.Render(&bytes.Buffer{}, http.StatusOK, []byte(`{}`), (*fakeUnion)(nil))
	if err == nil {
		t.Fatal("expected error for nil detail")
	}
}

func TestPolymorphicView_resolveError(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	wantErr := errors.New("resolve boom")
	v := cmdutil.PolymorphicView[fakeUnion]{
		Templates: polyTemplates(),
		Variants: map[string]cmdutil.Variant[fakeUnion]{
			"a": {Template: "a.tmpl", Resolve: func(u fakeUnion) (any, error) { return nil, wantErr }},
		},
	}
	err := v.Render(&bytes.Buffer{}, http.StatusOK, []byte(`{}`), &fakeUnion{Kind: "a"})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected resolve error to propagate, got %v", err)
	}
}

func TestPolymorphicView_discriminatorError(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := newPolyView()
	err := v.Render(&bytes.Buffer{}, http.StatusOK, []byte(`{}`), &fakeUnion{Kind: ""})
	if err == nil {
		t.Fatal("expected error for empty discriminator")
	}
	if !strings.Contains(err.Error(), "missing discriminator") {
		t.Errorf("expected fakeUnion discriminator error to propagate, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/cmdutil/ -run TestPolymorphicView -v`
Expected: FAIL — `cmdutil.PolymorphicView undefined`.

- [ ] **Step 3: Implement `Discriminator`, `Variant[T]`, `PolymorphicView[T]`**

Append to `internal/cli/cmdutil/view.go`:

```go
// Discriminator constrains polymorphic response bodies. Oapi-codegen union
// types satisfy this automatically — each generated *Detail struct has a
// Discriminator() (string, error) method.
type Discriminator interface {
	Discriminator() (string, error)
}

// Variant binds one discriminator branch to its template and a resolver that
// extracts the typed variant (and optionally transforms it) from the union.
type Variant[T Discriminator] struct {
	Template string
	Resolve  func(T) (any, error)
}

// PolymorphicView is the View equivalent for discriminated-union responses.
// Variants is keyed by the discriminator string returned by T.Discriminator().
// Status defaults to http.StatusOK.
type PolymorphicView[T Discriminator] struct {
	Templates fs.FS
	Status    int
	Variants  map[string]Variant[T]
}

// Render handles the status check + JSON shortcut, then dispatches on
// detail.Discriminator() to look up the variant template and resolved data.
func (v PolymorphicView[T]) Render(w io.Writer, statusCode int, body []byte, detail *T) error {
	handled, err := renderHead(w, v.Status, statusCode, body)
	if handled || err != nil {
		return err
	}
	if detail == nil {
		var zero T
		return fmt.Errorf("nil %T body", zero)
	}
	disc, err := (*detail).Discriminator()
	if err != nil {
		return err
	}
	variant, ok := v.Variants[disc]
	if !ok {
		return fmt.Errorf("unknown %T discriminator: %q", *detail, disc)
	}
	data, err := variant.Resolve(*detail)
	if err != nil {
		return err
	}
	return output.RenderTemplate(w, v.Templates, variant.Template, data)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/cmdutil/ -v`
Expected: PASS — all `TestView_*`, all `TestPolymorphicView_*`.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/cmdutil/view.go internal/cli/cmdutil/view_test.go
git commit -m "feat(cmdutil): add PolymorphicView for discriminated-union responses"
```

---

## Task 3: Migrate `network clients get` to `PolymorphicView`

**Files:**
- Modify: `internal/cli/network/network.go`

- [ ] **Step 1: Add the top-level `PolymorphicView` declaration**

In `internal/cli/network/network.go`, immediately after the existing `clientsListView` declaration (line 20), insert:

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
```

- [ ] **Step 2: Replace the `newGetClientCmd` body with the view call**

Replace lines 216–262 (`func newGetClientCmd() *cobra.Command { ... }`) with:

```go
func newGetClientCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <client-id>",
		Short: "Show network client details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[NetworkClient](cmd).GetNetworkClientWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return clientGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}
```

- [ ] **Step 3: Remove now-unused imports**

After this migration, check whether `fmt`, `net/http`, `apiclient`, `flags`, and `output` are still used in `network.go`. They are still used elsewhere in the file (other inline polymorphic and transform cases) — leave them. They'll be removed later as those sites migrate.

Run: `go vet ./...`
Expected: no errors.

- [ ] **Step 4: Run network tests**

Run: `go test ./internal/cli/network/ -v -run TestGetClientCmd`
Expected: PASS — `TestGetClientCmd_wired`, `TestGetClientCmd_wireless`, `TestGetClientCmd_offline_wired`, `TestGetClientCmd_offline_wireless`, `TestGetClientCmd_notFound`.

Run: `go test ./internal/cli/network/`
Expected: PASS overall.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/network/network.go
git commit -m "refactor(network): migrate clients get to PolymorphicView"
```

---

## Task 4: Migrate `system updates get` to `PolymorphicView`

**Files:**
- Modify: `internal/cli/system/system.go`

- [ ] **Step 1: Add the top-level `PolymorphicView` declaration**

In `internal/cli/system/system.go`, immediately after the existing `updatesListView` (line 20), insert:

```go
var updateGetView = cmdutil.PolymorphicView[gen.SystemUpdateDetail]{
	Templates: systemTemplates,
	Variants: map[string]cmdutil.Variant[gen.SystemUpdateDetail]{
		"container": {
			Template: "updates_get_container.tmpl",
			Resolve:  func(d gen.SystemUpdateDetail) (any, error) { return d.AsContainerSystemUpdateDetail() },
		},
	},
}
```

- [ ] **Step 2: Replace the `newGetUpdateCmd` body**

Replace lines 197–234 (`func newGetUpdateCmd() *cobra.Command { ... }`) with:

```go
func newGetUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <update-id>",
		Short: "Show update details for a tracked component",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := cmdutil.Client[SystemClient](cmd).GetSystemUpdateWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return updateGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/cli/system/ -v -run TestGetUpdateCmd`
Expected: PASS — `TestGetUpdateCmd_containerType`, `TestGetUpdateCmd_apiError`, `TestGetUpdateCmd_jsonOutput`.

Run: `go test ./internal/cli/system/`
Expected: PASS overall.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/system/system.go
git commit -m "refactor(system): migrate updates get to PolymorphicView"
```

---

## Task 5: Migrate `network devices get` to inline `PolymorphicView` + extract `buildSwitchPortViews`

**Files:**
- Modify: `internal/cli/network/network.go`

This task is the largest migration because of the per-call `--all-ports` flag and the switch-variant's port-resolution loop. The `PolymorphicView` is constructed inside `RunE` so the `switch` variant's `Resolve` closes over `allPorts`. Port resolution moves to a new helper `buildSwitchPortViews`.

- [ ] **Step 1: Add `buildSwitchPortViews` helper**

In `internal/cli/network/network.go`, add this helper function. A natural spot is immediately after the `switchPortView` type declaration (around line 34):

```go
// buildSwitchPortViews filters and decorates a switch's ports for display.
// When allPorts is false, only ports with state "up" are returned. Each
// remaining port's ConnectedTo field is resolved to a display name (device or
// client name, or "-" when nothing is connected).
func buildSwitchPortViews(ports []gen.SwitchPort, allPorts bool) ([]switchPortView, error) {
	var out []switchPortView
	for _, p := range ports {
		if !allPorts && p.State != gen.NetworkPortStateUp {
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
		out = append(out, switchPortView{SwitchPort: p, ConnectedToName: connectedTo})
	}
	return out, nil
}
```

- [ ] **Step 2: Replace `newGetDeviceCmd` body**

Replace lines 87–179 (`func newGetDeviceCmd() *cobra.Command { ... }`) with:

```go
func newGetDeviceCmd() *cobra.Command {
	var allPorts bool
	cmd := &cobra.Command{
		Use:   "get <device-id>",
		Short: "Show network device details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			view := cmdutil.PolymorphicView[gen.NetworkDeviceDetail]{
				Templates: networkTemplates,
				Variants: map[string]cmdutil.Variant[gen.NetworkDeviceDetail]{
					"switch": {
						Template: "devices_get_switch.tmpl",
						Resolve: func(d gen.NetworkDeviceDetail) (any, error) {
							sw, err := d.AsSwitchDetail()
							if err != nil {
								return nil, err
							}
							portViews, err := buildSwitchPortViews(sw.Ports, allPorts)
							if err != nil {
								return nil, err
							}
							return switchDetailView{SwitchDetail: sw, Ports: portViews}, nil
						},
					},
					"accessPoint": {
						Template: "devices_get_accesspoint.tmpl",
						Resolve:  func(d gen.NetworkDeviceDetail) (any, error) { return d.AsAccessPointDetail() },
					},
					"gateway": {
						Template: "devices_get_gateway.tmpl",
						Resolve:  func(d gen.NetworkDeviceDetail) (any, error) { return d.AsGatewayDetail() },
					},
					"unknown": {
						Template: "devices_get_unknown.tmpl",
						Resolve:  func(d gen.NetworkDeviceDetail) (any, error) { return d.AsUnknownDeviceDetail() },
					},
				},
			}

			resp, err := cmdutil.Client[NetworkClient](cmd).GetNetworkDeviceWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return view.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
		},
	}
	cmd.Flags().BoolVar(&allPorts, "all-ports", false, "Show all ports (default: active ports only)")
	return cmd
}
```

- [ ] **Step 3: Run device-get tests**

Run: `go test ./internal/cli/network/ -v -run TestGetDeviceCmd`
Expected: PASS — `TestGetDeviceCmd_gateway`, `TestGetDeviceCmd_unknownWithUplink`, `TestGetDeviceCmd_switch_activePorts`, `TestGetDeviceCmd_switch_allPorts`, `TestGetDeviceCmd_accessPoint`.

Run: `go test ./internal/cli/network/`
Expected: PASS overall.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/network/network.go
git commit -m "refactor(network): migrate devices get to PolymorphicView, extract buildSwitchPortViews"
```

---

## Task 6: Migrate `system info` to `View.RenderWith`

**Files:**
- Modify: `internal/cli/system/system.go`

- [ ] **Step 1: Add the top-level `View` declaration**

In `internal/cli/system/system.go`, after the existing `updatesListView` (and the `updateGetView` added in Task 4), insert:

```go
var infoView = cmdutil.View{Templates: systemTemplates, Name: "info.tmpl"}
```

- [ ] **Step 2: Replace `newInfoCmd` body**

Replace the current `newInfoCmd` (lines 81–120 in the original file — locate by searching for `func newInfoCmd`) with:

```go
func newInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show device information",
	}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		params := &gen.ListSystemInfoParams{}
		if *device != "" {
			params.Device = device
		}

		resp, err := cmdutil.Client[SystemClient](cmd).ListSystemInfoWithResponse(cmd.Context(), params)
		if err != nil {
			return err
		}
		return infoView.RenderWith(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, func() (any, error) {
			items := make([]infoRow, 0, len(resp.JSON200.Items))
			for _, info := range resp.JSON200.Items {
				items = append(items, infoRow{
					Device:   info.Device,
					Model:    info.Model,
					Firmware: info.Firmware,
					Ram:      output.FormatBytes(int64(info.RamMb) * 1024 * 1024),
					Uptime:   output.FormatUptime(int(info.UptimeSeconds)),
				})
			}
			return struct{ Items []infoRow }{items}, nil
		})
	}
	return cmd
}
```

- [ ] **Step 3: Run info tests**

Run: `go test ./internal/cli/system/ -v -run TestInfoCmd`
Expected: PASS — `TestInfoCmd_tableOutput`.

Run: `go test ./internal/cli/system/`
Expected: PASS overall.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/system/system.go
git commit -m "refactor(system): migrate info to View.RenderWith"
```

---

## Task 7: Migrate `system utilization` to `View.RenderWith`

**Files:**
- Modify: `internal/cli/system/system.go`

- [ ] **Step 1: Add the top-level `View` declaration**

In `internal/cli/system/system.go`, alongside the other view vars at the top, insert:

```go
var utilizationView = cmdutil.View{Templates: systemTemplates, Name: "utilization.tmpl"}
```

- [ ] **Step 2: Replace `newUtilizationCmd` body**

Replace the current `newUtilizationCmd` (locate by searching for `func newUtilizationCmd`) with:

```go
func newUtilizationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "utilization",
		Short: "Show live resource utilization",
	}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
		params := &gen.ListSystemUtilizationParams{}
		if *device != "" {
			params.Device = device
		}

		resp, err := cmdutil.Client[SystemClient](cmd).ListSystemUtilizationWithResponse(ctx, params)
		if err != nil {
			return err
		}
		return utilizationView.RenderWith(w, resp.StatusCode(), resp.Body, func() (any, error) {
			items := make([]utilizationRow, 0, len(resp.JSON200.Items))
			for _, u := range resp.JSON200.Items {
				swapPct := 0
				if u.Memory.SwapTotalBytes > 0 {
					swapPct = int(u.Memory.SwapUsedBytes * 100 / u.Memory.SwapTotalBytes)
				}
				items = append(items, utilizationRow{
					Device: u.Device,
					Cpu:    fmt.Sprintf("%d%%", u.Cpu.TotalPercent),
					Memory: fmt.Sprintf("%d%%", u.Memory.UsedPercent),
					Swap:   fmt.Sprintf("%d%%", swapPct),
				})
			}
			return struct{ Items []utilizationRow }{items}, nil
		})
	})
	watch.RegisterFlags(cmd)
	return cmd
}
```

Note: the writer used inside `watch.Wrap` is `w`, not `cmd.OutOrStdout()` — preserves existing behavior.

- [ ] **Step 3: Prune unused imports**

After this task, the system package no longer uses `apiclient` or `flags` directly. Confirm with:

```bash
go vet ./internal/cli/system/
```

If unused-import errors appear, remove them. Expected removals: `"github.com/bwilczynski/hlctl/internal/apiclient"` and `"github.com/bwilczynski/hlctl/internal/cli/flags"`. Keep `"github.com/bwilczynski/hlctl/internal/output"` (still used for `output.FormatBytes`, `output.FormatUptime`).

Also check `"net/http"` — no longer needed for inline `http.StatusOK` comparisons; remove if unused.

- [ ] **Step 4: Run utilization tests**

Run: `go test ./internal/cli/system/ -v -run TestUtilizationCmd`
Expected: PASS — `TestUtilizationCmd_tableOutput`.

Run: `go test ./internal/cli/system/`
Expected: PASS overall.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/system/system.go
git commit -m "refactor(system): migrate utilization to View.RenderWith"
```

---

## Task 8: Migrate `network topology` to `View.RenderWith`

**Files:**
- Modify: `internal/cli/network/network.go`

- [ ] **Step 1: Add the top-level `View` declaration**

In `internal/cli/network/network.go`, alongside the other view vars at the top, insert:

```go
var topologyView = cmdutil.View{Templates: networkTemplates, Name: "topology.tmpl"}
```

- [ ] **Step 2: Replace the topology `RunE` block**

Inside `newTopologyCmd`, replace the current `cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error { ... })` block (lines 284–309) with:

```go
cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
	params := &gen.GetNetworkTopologyParams{}
	if includeClients || includeWireless {
		t := true
		params.IncludeClients = &t
	}

	resp, err := cmdutil.Client[NetworkClient](cmd).GetNetworkTopologyWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return topologyView.RenderWith(w, resp.StatusCode(), resp.Body, func() (any, error) {
		return buildTopologyTree(*resp.JSON200, includeWireless)
	})
})
```

- [ ] **Step 3: Prune unused imports**

The network package no longer uses `apiclient` or `flags` directly. Confirm with:

```bash
go vet ./internal/cli/network/
```

If unused-import errors appear, remove them. Expected removals: `"github.com/bwilczynski/hlctl/internal/apiclient"`, `"github.com/bwilczynski/hlctl/internal/cli/flags"`, and possibly `"fmt"` and `"net/http"` (verify before removing — `fmt.Sprintf` calls remain inside `buildTopologyTree`, so `fmt` stays; `net/http` is likely removable). Keep `"github.com/bwilczynski/hlctl/internal/output"` (still used for `output.FormatLinkSpeed`).

- [ ] **Step 4: Run topology tests**

Run: `go test ./internal/cli/network/ -v -run TestTopologyCmd`
Expected: PASS — `TestTopologyCmd_devicesOnly`, `TestTopologyCmd_includeClientsWiredOnly`, `TestTopologyCmd_includeWireless`, `TestTopologyCmd_jsonOutput`, `TestTopologyCmd_apiError`.

Run: `go test ./internal/cli/network/`
Expected: PASS overall.

- [ ] **Step 5: Full-repo verification**

Run: `make lint && go test ./...`
Expected: no vet errors, all tests pass.

Also grep to confirm no `flags.GetOutputFormat` calls remain in domain code:

```bash
grep -rn "flags.GetOutputFormat" internal/cli/
```

Expected output: only `internal/cli/cmdutil/view.go` and `internal/cli/watch/watch.go`.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/network/network.go
git commit -m "refactor(network): migrate topology to View.RenderWith"
```

---

## Task 9: Update `CLAUDE.md`

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update the "Adding a New Domain Command" section**

Open `CLAUDE.md` and locate the "Adding a New Domain Command" section. Replace item 9 (the polymorphic carve-out beginning "For polymorphic responses…") with:

```markdown
9. For polymorphic responses (discriminated unions like `NetworkDeviceDetail`, `SystemUpdateDetail`), declare a `cmdutil.PolymorphicView[<UnionType>]` instead of a `View`. Its `Variants` map is keyed by the discriminator string returned by `T.Discriminator()`; each `Variant` binds a template name to a `Resolve func(T) (any, error)` that calls the appropriate `As<Variant>()` accessor (and optionally transforms the result). Render with `view.Render(w, resp.StatusCode(), resp.Body, resp.JSON200)` — same call shape as `View.Render`. When a variant resolver depends on per-call state (e.g. a flag), construct the `PolymorphicView` inside `RunE` so the resolver can close over it.
10. When template data must be derived from the response body (e.g. row structs with formatted bytes/uptime), use `view.RenderWith(w, resp.StatusCode(), resp.Body, fn)` instead of `view.Render`. `fn` is invoked only in table mode, so derivation work is skipped when `--output=json`.
```

- [ ] **Step 2: Verify the file builds cleanly**

There's nothing executable to test, but skim the section for numbering and grammar.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: document PolymorphicView and RenderWith in CLAUDE.md"
```

---

## Self-review checklist (run after implementing all tasks)

- [ ] `grep -rn "flags.GetOutputFormat" internal/cli/` shows only `cmdutil/view.go` and `watch/watch.go`.
- [ ] `grep -rn "Discriminator()" internal/cli/<domain>/` shows only intermediate-data uses (`connectionRefID`, `buildTopologyTree`, `buildSwitchPortViews`) — no inline render-dispatch switches.
- [ ] `make lint && go test ./...` is clean.
- [ ] `./bin/hlctl network devices get <switch-id>` and `./bin/hlctl network devices get <switch-id> --all-ports` produce the same output as before the change (spot check against a recorded golden if available).
