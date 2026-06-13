# CLI Refactor Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate package-level mutable globals in `internal/cli/flags/`, introduce explicit dependency injection via a `*cmdutil.Factory` threaded from `main`, and reorganize generated code under `internal/api/<domain>/` so it no longer shadows command packages. Preserve `cmdutil.InjectClient` / `Client[C]` / `SetClient[C]` context-DI and the existing `StubClient` test pattern — those are Phase 2 concerns.

**Architecture:** Three structural moves followed by a signature sweep, six per-domain migrations, and a final cleanup. The codebase must build and pass tests at every committed checkpoint. While the migration is in flight, `flags.GetOutputFormat()` is passed as a temporary placeholder until the matching per-domain task swaps in `f.Output()`; the `flags` package is deleted only after every reference is gone.

**Tech Stack:** Go 1.26, Cobra v1.10, oapi-codegen, `make` for build/test.

**Reference spec:** `docs/superpowers/specs/2026-06-12-cli-refactor-design.md`

---

## File Structure

### New files (created)
- `internal/cli/cmdutil/iostreams.go` — `IOStreams` type + `SystemIOStreams()`.
- `internal/cli/cmdutil/factory.go` — `Factory` type + `NewFactory(version, *apiURL, *outputFmt) *Factory`.
- `internal/cli/cmdutil/testfactory.go` — exported `TestFactory(t *testing.T) *Factory` for tests in any package.

### Renamed paths
- `internal/apiclient/` → `internal/api/` (package rename `apiclient` → `api`, `NewHTTPClient` stays during migration then deleted in the final task).
- `internal/docker/`, `internal/system/`, `internal/storage/`, `internal/network/` → `internal/api/docker/`, `internal/api/system/`, `internal/api/storage/`, `internal/api/network/`.
- `oapi-codegen-docker.yaml`, `oapi-codegen-system.yaml`, `oapi-codegen-storage.yaml`, `oapi-codegen-network.yaml` → `codegen/docker.yaml`, `codegen/system.yaml`, `codegen/storage.yaml`, `codegen/network.yaml`.

### Modified files (substantial signature changes)
- `internal/cli/cmdutil/view.go` — `View.Render`, `View.RenderWith`, `PolymorphicView.Render` gain `outputFmt output.Format` as second argument.
- `internal/cli/cmdutil/view_test.go` — drop `flags.OutputFormat = "..."` mutations; pass `outputFmt` directly.
- `internal/cli/cmdutil/action.go` — `apiclient.ParseError` → `api.ParseError` import update only.
- `internal/cli/watch/watch.go` — `Wrap` gains `getOutputFmt func() output.Format` as first argument.
- `internal/cli/watch/watch_test.go` — pass an output-format getter rather than mutating `flags.OutputFormat`.
- `internal/cli/root.go` — replace package-level `rootCmd` + `init()` + `Execute(version)` with `NewRootCmd(f *cmdutil.Factory) *cobra.Command`.
- `cmd/hlctl/main.go` — construct `Factory`, call `cli.NewRootCmd(f)`, attach flag set.
- Per-domain `<domain>.go` files — `NewCmd()` → `NewCmd(f *cmdutil.Factory)`; closure passed to `cmdutil.InjectClient` captures `f.HTTPClient`.
- Per-domain leaf files (where they render via `cmdutil.View`) — leaf constructors take `f *cmdutil.Factory` and pass `f.Output()` into `view.Render` / `watch.Wrap`.
- Per-domain `*_test.go` files — construct leaves with `cmdutil.TestFactory(t)`; tests that need JSON output mode override `f.Output` directly.

### Deleted files / packages
- `internal/cli/flags/flags.go` (and the directory) — last task only.
- `internal/api/apiclient.go` (formerly `internal/apiclient/apiclient.go`) — last task only, when no leaves still call `api.NewHTTPClient`.

### Unchanged
- `internal/auth/`, `internal/config/`, `internal/output/` — no CLI knowledge.
- All `templates.go` files and embedded template trees.
- All `stub.go` files (typed-client `StubClient` definitions).
- `cmdutil.InjectClient` / `Client[C]` / `SetClient[C]` / `cmdutil.ActionCmd` / `cmdutil.DeviceFlag`.

---

## Task ordering and build invariant

Tasks land in this order so the codebase compiles + `make test` passes at every commit:

1. Move oapi-codegen YAMLs to `codegen/`
2. Move generated API code to `internal/api/<domain>/`
3. Rename `internal/apiclient` → `internal/api`
4. Add `cmdutil/iostreams.go`
5. Add `cmdutil/factory.go` + `cmdutil/testfactory.go`
6. Add `outputFmt` parameter to `View.Render` / `RenderWith` / `PolymorphicView.Render` (callers pass `flags.GetOutputFormat()` as a temporary bridge)
7. Add `getOutputFmt` parameter to `watch.Wrap` (callers pass `flags.GetOutputFormat`)
8. Migrate `auth` domain
9. Migrate `config` domain
10. Migrate `system` domain
11. Migrate `docker` domain
12. Migrate `network` domain
13. Migrate `storage` domain
14. Add `NewRootCmd(f)`, rewire `cmd/hlctl/main.go`, delete the old `init()` setup
15. Delete `internal/cli/flags/`
16. Delete `api.NewHTTPClient` and final cleanup

---

## Task 1: Move oapi-codegen YAMLs to `codegen/`

**Files:**
- Move: `oapi-codegen-docker.yaml` → `codegen/docker.yaml`
- Move: `oapi-codegen-system.yaml` → `codegen/system.yaml`
- Move: `oapi-codegen-storage.yaml` → `codegen/storage.yaml`
- Move: `oapi-codegen-network.yaml` → `codegen/network.yaml`
- Modify: `Makefile`

- [ ] **Step 1: Verify state**

Run: `ls oapi-codegen-*.yaml`
Expected: four YAMLs at repo root.

- [ ] **Step 2: Move YAMLs into `codegen/`**

```bash
mkdir -p codegen
git mv oapi-codegen-docker.yaml  codegen/docker.yaml
git mv oapi-codegen-system.yaml  codegen/system.yaml
git mv oapi-codegen-storage.yaml codegen/storage.yaml
git mv oapi-codegen-network.yaml codegen/network.yaml
```

- [ ] **Step 3: Update Makefile generate target**

The current `generate` target points at the old YAML paths. Replace those four lines so they read from `codegen/`:

```makefile
generate: bundle ## Generate client code from the bundled spec
	@mkdir -p internal/system internal/docker internal/storage internal/network
	$(OAPI_CODEGEN) --config codegen/system.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/docker.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/storage.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/network.yaml $(SPEC_FILE)
```

Note: the `mkdir -p` line still references the old generated locations because Task 2 owns that move.

- [ ] **Step 4: Regenerate to verify nothing else broke**

Run: `make generate`
Expected: regeneration succeeds; `git status` shows no diffs in generated files (the YAML `output:` paths are unchanged at this task).

- [ ] **Step 5: Verify the build and tests**

Run: `make build && make test`
Expected: both succeed.

- [ ] **Step 6: Commit**

```bash
git add codegen/ Makefile
git commit -m "refactor: move oapi-codegen configs to codegen/ subdirectory"
```

---

## Task 2: Move generated code to `internal/api/<domain>/`

**Files:**
- Modify: `codegen/docker.yaml`, `codegen/system.yaml`, `codegen/storage.yaml`, `codegen/network.yaml` (update `output:` paths)
- Modify: `Makefile` (update `mkdir -p` line)
- Delete: `internal/docker/api.gen.go`, `internal/system/api.gen.go`, `internal/storage/api.gen.go`, `internal/network/api.gen.go`
- Create: `internal/api/docker/api.gen.go`, `internal/api/system/api.gen.go`, `internal/api/storage/api.gen.go`, `internal/api/network/api.gen.go`
- Modify: every Go file that previously imported `internal/<domain>` under a `gen` alias (about 35 files)

- [ ] **Step 1: Update codegen YAML output paths**

For each of `codegen/docker.yaml`, `codegen/system.yaml`, `codegen/storage.yaml`, `codegen/network.yaml`, change the `output:` line.

`codegen/docker.yaml`:
```yaml
output: internal/api/docker/api.gen.go
```

`codegen/system.yaml`:
```yaml
output: internal/api/system/api.gen.go
```

`codegen/storage.yaml`:
```yaml
output: internal/api/storage/api.gen.go
```

`codegen/network.yaml`:
```yaml
output: internal/api/network/api.gen.go
```

- [ ] **Step 2: Update the Makefile `mkdir -p` line**

Edit `Makefile`. Replace the existing `mkdir -p` line in the `generate` target:

```makefile
generate: bundle ## Generate client code from the bundled spec
	@mkdir -p internal/api/system internal/api/docker internal/api/storage internal/api/network
	$(OAPI_CODEGEN) --config codegen/system.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/docker.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/storage.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/network.yaml $(SPEC_FILE)
```

- [ ] **Step 3: Delete the old generated files**

The generated files are gitignored, so this only affects the working tree:

```bash
rm -f internal/docker/api.gen.go
rm -f internal/system/api.gen.go
rm -f internal/storage/api.gen.go
rm -f internal/network/api.gen.go
rmdir internal/docker internal/system internal/storage internal/network 2>/dev/null || true
```

- [ ] **Step 4: Regenerate to the new location**

Run: `make generate`
Expected: four new `api.gen.go` files under `internal/api/<domain>/`.

- [ ] **Step 5: Rewrite imports across the codebase**

Every Go file that imports `"github.com/bwilczynski/hlctl/internal/<domain>"` (typically aliased as `gen`) needs the path updated and, where the file is *inside* a package whose own name matches the import path leaf, the alias renamed to `<domain>api`.

Run this in the repo root to verify the import change is mechanical (alias-aware):

```bash
gofmt -l ./internal/cli  # should report no diffs after edits
```

For each affected file, replace:

```go
// before
gen "github.com/bwilczynski/hlctl/internal/docker"
```

with:

```go
// after
dockerapi "github.com/bwilczynski/hlctl/internal/api/docker"
```

And rewrite the references inside the file: `gen.ListContainers...` → `dockerapi.ListContainers...`, etc.

Apply the same pattern per domain:
- `gen "github.com/bwilczynski/hlctl/internal/system"` → `systemapi "github.com/bwilczynski/hlctl/internal/api/system"`, references `gen.` → `systemapi.`
- `gen "github.com/bwilczynski/hlctl/internal/storage"` → `storageapi "github.com/bwilczynski/hlctl/internal/api/storage"`, references `gen.` → `storageapi.`
- `gen "github.com/bwilczynski/hlctl/internal/network"` → `networkapi "github.com/bwilczynski/hlctl/internal/api/network"`, references `gen.` → `networkapi.`

Files to touch (found via grep at planning time):
- `internal/cli/docker/client.go`, `containers.go`, `containers_test.go`, `images.go`, `images_test.go`, `networks.go`, `networks_test.go`, `stub.go`
- `internal/cli/system/client.go`, `info.go`, `info_test.go`, `health_test.go`, `updates.go`, `updates_test.go`, `utilization.go`, `utilization_test.go`, `stub.go`
- `internal/cli/storage/client.go`, `backups.go`, `backups_test.go`, `volumes.go`, `volumes_test.go`, `stub.go`
- `internal/cli/network/client.go`, `clients.go`, `clients_test.go`, `devices.go`, `devices_test.go`, `ssids_test.go`, `topology.go`, `topology_test.go`, `vlans.go`, `vlans_test.go`, `wans_test.go`, `stub.go`

- [ ] **Step 6: Verify the build and tests**

Run: `make build && make test`
Expected: both succeed.

- [ ] **Step 7: Commit**

```bash
git add Makefile codegen/ internal/
git commit -m "refactor: move generated API clients to internal/api/<domain>/"
```

---

## Task 3: Rename `internal/apiclient` → `internal/api`

**Files:**
- Move: `internal/apiclient/apiclient.go` → `internal/api/apiclient.go` (package rename)
- Move: `internal/apiclient/errors.go` → `internal/api/errors.go` (package rename)
- Move: `internal/apiclient/errors_test.go` → `internal/api/errors_test.go` (package rename)
- Modify: every Go file that imports `internal/apiclient` (about 7 files)

- [ ] **Step 1: Move the files**

```bash
git mv internal/apiclient/apiclient.go internal/api/apiclient.go
git mv internal/apiclient/errors.go    internal/api/errors.go
git mv internal/apiclient/errors_test.go internal/api/errors_test.go
rmdir internal/apiclient
```

- [ ] **Step 2: Rename the package declaration in each moved file**

In `internal/api/apiclient.go`, change the first line:

```go
package api
```

In `internal/api/errors.go`, change the first line:

```go
package api
```

In `internal/api/errors_test.go`, change the first line:

```go
package api_test
```

Also update the import in `errors_test.go` from `apiclient` to `api`:

```go
import (
    // ...
    "github.com/bwilczynski/hlctl/internal/api"
)
```

And replace every `apiclient.ParseError(...)` call with `api.ParseError(...)` in the test file body.

- [ ] **Step 3: Update import paths in consumers**

Every file that imports `"github.com/bwilczynski/hlctl/internal/apiclient"` becomes `"github.com/bwilczynski/hlctl/internal/api"`, and every `apiclient.NewHTTPClient` / `apiclient.ParseError` call becomes `api.NewHTTPClient` / `api.ParseError`.

Files to touch:
- `internal/cli/docker/docker.go` — `apiclient.NewHTTPClient()` → `api.NewHTTPClient()`
- `internal/cli/network/network.go` — same
- `internal/cli/storage/storage.go` — same
- `internal/cli/system/system.go` — same
- `internal/cli/cmdutil/action.go` — `apiclient.ParseError(code, body)` → `api.ParseError(code, body)`
- `internal/cli/cmdutil/view.go` — `apiclient.ParseError(statusCode, body)` → `api.ParseError(statusCode, body)`; update the docstring on line 46 to reference `api.ParseError`

Example for `internal/cli/cmdutil/view.go` (line 10 import + the call site + the docstring comment):

```go
import (
    // ...
    "github.com/bwilczynski/hlctl/internal/api"
    // ...
)
```

```go
// at line 36, in renderHead:
if statusCode != expected {
    return false, api.ParseError(statusCode, body)
}
```

- [ ] **Step 4: Verify the build and tests**

Run: `make build && make test`
Expected: both succeed; `internal/api/errors_test.go` runs in the renamed package.

- [ ] **Step 5: Commit**

```bash
git add internal/
git commit -m "refactor: rename internal/apiclient package to internal/api"
```

---

## Task 4: Add `cmdutil/iostreams.go`

**Files:**
- Create: `internal/cli/cmdutil/iostreams.go`

- [ ] **Step 1: Write the file**

```go
package cmdutil

import (
    "io"
    "os"
)

// IOStreams bundles a command's input/output writers so they can be substituted
// in tests. Production wiring (SystemIOStreams) points at the process stdio;
// tests construct an IOStreams with bytes.Buffer-backed writers.
type IOStreams struct {
    In     io.Reader
    Out    io.Writer
    ErrOut io.Writer
}

// SystemIOStreams returns an IOStreams bound to the process stdio.
func SystemIOStreams() *IOStreams {
    return &IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
}
```

- [ ] **Step 2: Verify the build (no consumers yet)**

Run: `go build ./internal/cli/cmdutil/...`
Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/cli/cmdutil/iostreams.go
git commit -m "feat(cmdutil): add IOStreams type and SystemIOStreams constructor"
```

---

## Task 5: Add `cmdutil/factory.go` + `cmdutil/testfactory.go`

**Files:**
- Create: `internal/cli/cmdutil/factory.go`
- Create: `internal/cli/cmdutil/testfactory.go`

- [ ] **Step 1: Write the failing test**

Create `internal/cli/cmdutil/factory_test.go`:

```go
package cmdutil_test

import (
    "testing"

    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/bwilczynski/hlctl/internal/output"
)

func TestNewFactory_outputFlagDefersToLatestValue(t *testing.T) {
    apiURL := ""
    outputFmt := "table"
    f := cmdutil.NewFactory("test", &apiURL, &outputFmt)

    if got := f.Output(); got != output.FormatTable {
        t.Errorf("expected table, got %q", got)
    }

    outputFmt = "json"
    if got := f.Output(); got != output.FormatJSON {
        t.Errorf("expected json after flag change, got %q", got)
    }
}

func TestNewFactory_apiURLFlagOverridesConfig(t *testing.T) {
    apiURL := "https://override.test"
    outputFmt := "table"
    f := cmdutil.NewFactory("test", &apiURL, &outputFmt)

    got, err := f.APIURL()
    if err != nil {
        t.Fatalf("APIURL: %v", err)
    }
    if got != "https://override.test" {
        t.Errorf("expected override URL, got %q", got)
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/cli/cmdutil/...`
Expected: FAIL — `cmdutil.NewFactory` undefined.

- [ ] **Step 3: Write `internal/cli/cmdutil/factory.go`**

```go
package cmdutil

import (
    "net/http"
    "sync"

    "github.com/bwilczynski/hlctl/internal/auth"
    "github.com/bwilczynski/hlctl/internal/config"
    "github.com/bwilczynski/hlctl/internal/output"
)

// Factory bundles the building blocks every command needs. Constructed once in
// main, threaded through every NewCmd. Function-valued fields defer expensive
// work (config load, token read, URL resolution) until a command actually runs.
type Factory struct {
    Version string

    IOStreams *IOStreams

    Config     func() (*config.Config, error)
    APIURL     func() (string, error)
    HTTPClient func() (*http.Client, string, error)
    Output     func() output.Format
}

// NewFactory builds the default Factory wired to real config/auth/http. The
// caller passes *string pointers to flag-backed storage (declared on the root
// command's PersistentFlags); the returned closures read the *latest* flag
// values each invocation, so resolution sees flag-parsing outcomes correctly.
func NewFactory(version string, apiURLFlag, outputFlag *string) *Factory {
    var (
        cfg     *config.Config
        cfgErr  error
        cfgOnce sync.Once
    )
    loadConfig := func() (*config.Config, error) {
        cfgOnce.Do(func() { cfg, cfgErr = config.Load() })
        return cfg, cfgErr
    }
    apiURLFn := func() (string, error) {
        if *apiURLFlag != "" {
            return *apiURLFlag, nil
        }
        c, err := loadConfig()
        if err != nil {
            return "", err
        }
        return c.ResolveAPIURL()
    }
    return &Factory{
        Version:   version,
        IOStreams: SystemIOStreams(),
        Config:    loadConfig,
        APIURL:    apiURLFn,
        HTTPClient: func() (*http.Client, string, error) {
            apiURL, err := apiURLFn()
            if err != nil {
                return nil, "", err
            }
            return &http.Client{Transport: auth.NewAuthenticatedTransport(nil)}, apiURL, nil
        },
        Output: func() output.Format { return output.Format(*outputFlag) },
    }
}
```

- [ ] **Step 4: Write `internal/cli/cmdutil/testfactory.go`**

```go
package cmdutil

import (
    "errors"
    "io"
    "net/http"
    "strings"
    "testing"

    "github.com/bwilczynski/hlctl/internal/config"
    "github.com/bwilczynski/hlctl/internal/output"
)

// TestFactory builds a Factory suitable for cobra leaf tests. The Config and
// HTTPClient closures return errors so any test that accidentally triggers
// real I/O fails loudly — tests that drive a stub via SetClient[C] never
// reach these closures.
//
// Tests that need JSON output mode override the Output field directly:
//
//	f := cmdutil.TestFactory(t)
//	f.Output = func() output.Format { return output.FormatJSON }
func TestFactory(t *testing.T) *Factory {
    t.Helper()
    return &Factory{
        Version:   "test",
        IOStreams: &IOStreams{In: strings.NewReader(""), Out: io.Discard, ErrOut: io.Discard},
        Config: func() (*config.Config, error) {
            return nil, errors.New("TestFactory: Config not configured")
        },
        APIURL: func() (string, error) {
            return "", errors.New("TestFactory: APIURL not configured")
        },
        HTTPClient: func() (*http.Client, string, error) {
            return nil, "", errors.New("TestFactory: HTTPClient not configured")
        },
        Output: func() output.Format { return output.FormatTable },
    }
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/cli/cmdutil/...`
Expected: PASS.

- [ ] **Step 6: Verify the full build and tests**

Run: `make build && make test`
Expected: both succeed.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/cmdutil/factory.go internal/cli/cmdutil/factory_test.go internal/cli/cmdutil/testfactory.go
git commit -m "feat(cmdutil): add Factory, NewFactory, and TestFactory helpers"
```

---

## Task 6: Add `outputFmt` parameter to View / RenderWith / PolymorphicView.Render

**Files:**
- Modify: `internal/cli/cmdutil/view.go`
- Modify: `internal/cli/cmdutil/view_test.go`
- Modify: every leaf file that calls `view.Render`, `view.RenderWith`, or `polymorphicView.Render` (about 12 files)

Callers across the domains will temporarily pass `flags.GetOutputFormat()` so the build stays green. Per-domain tasks 8–13 swap this for `f.Output()`.

- [ ] **Step 1: Change the View signatures in `internal/cli/cmdutil/view.go`**

Replace the imports block (remove `flags`):

```go
import (
    "fmt"
    "io"
    "io/fs"
    "net/http"

    "github.com/bwilczynski/hlctl/internal/api"
    "github.com/bwilczynski/hlctl/internal/output"
)
```

Replace `renderHead`:

```go
// renderHead handles the status check and JSON shortcut shared by every
// render path. Returns handled=true when the JSON body has been written and
// the caller should return nil; returns a non-nil error on status mismatch.
func renderHead(w io.Writer, outputFmt output.Format, expectedStatus, statusCode int, body []byte) (handled bool, err error) {
    expected := expectedStatus
    if expected == 0 {
        expected = http.StatusOK
    }
    if statusCode != expected {
        return false, api.ParseError(statusCode, body)
    }
    if outputFmt == output.FormatJSON {
        fmt.Fprint(w, string(body))
        return true, nil
    }
    return false, nil
}
```

Replace `View.Render`:

```go
// Render handles the standard response→output flow:
//   - status != v.Status (or 200 if unset) → api.ParseError
//   - outputFmt == FormatJSON → write raw body
//   - otherwise → render the bound template against data
func (v View) Render(w io.Writer, outputFmt output.Format, statusCode int, body []byte, data any) error {
    handled, err := renderHead(w, outputFmt, v.Status, statusCode, body)
    if handled || err != nil {
        return err
    }
    return output.RenderTemplate(w, v.Templates, v.Name, data)
}
```

Replace `View.RenderWith`:

```go
// RenderWith mirrors Render but defers data construction. fn is invoked only
// in table mode — JSON mode dumps the raw body without running fn.
func (v View) RenderWith(w io.Writer, outputFmt output.Format, statusCode int, body []byte, fn func() (any, error)) error {
    handled, err := renderHead(w, outputFmt, v.Status, statusCode, body)
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

Replace `PolymorphicView.Render`:

```go
// Render handles the status check + JSON shortcut, then dispatches on
// detail.Discriminator() to look up the variant template and resolved data.
func (v PolymorphicView[T]) Render(w io.Writer, outputFmt output.Format, statusCode int, body []byte, detail *T) error {
    handled, err := renderHead(w, outputFmt, v.Status, statusCode, body)
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

- [ ] **Step 2: Rewrite `internal/cli/cmdutil/view_test.go` to pass `outputFmt` directly**

Replace the imports — drop `flags`, add `output`:

```go
import (
    "bytes"
    "errors"
    "net/http"
    "strings"
    "testing"
    "testing/fstest"

    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/bwilczynski/hlctl/internal/output"
)
```

For every test that currently sets `flags.OutputFormat = "table"`, drop the `t.Cleanup` + assignment lines and pass `output.FormatTable` to `Render` / `RenderWith` instead. Same for `flags.OutputFormat = "json"` → pass `output.FormatJSON`.

Updated test (`TestView_Render_table`) as a worked example:

```go
func TestView_Render_table(t *testing.T) {
    v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
    var buf bytes.Buffer
    if err := v.Render(&buf, output.FormatTable, http.StatusOK, []byte(`{"name":"world"}`), greet{Name: "world"}); err != nil {
        t.Fatalf("Render: %v", err)
    }
    if got := buf.String(); got != "hello world\n" {
        t.Errorf("unexpected output: %q", got)
    }
}
```

Apply the same pattern to:
- `TestView_Render_json` — pass `output.FormatJSON`
- `TestView_Render_statusMismatch_returnsParseError` — pass `output.FormatTable` (doesn't actually reach the format check, but be consistent)
- `TestView_Render_customStatus` — pass `output.FormatTable`
- `TestView_RenderWith_tableInvokesFn` — pass `output.FormatTable`
- `TestView_RenderWith_jsonSkipsFn` — pass `output.FormatJSON`
- `TestView_RenderWith_statusMismatchSkipsFn` — pass `output.FormatTable`
- `TestView_RenderWith_fnErrorPropagates` — pass `output.FormatTable`
- `TestView_RenderWith_customStatus` — pass `output.FormatTable`
- `TestPolymorphicView_dispatchesToVariant` — pass `output.FormatTable`
- `TestPolymorphicView_jsonModeSkipsDiscriminator` — pass `output.FormatJSON`
- `TestPolymorphicView_statusMismatch` — pass `output.FormatTable`
- `TestPolymorphicView_customStatus` — pass `output.FormatTable`
- `TestPolymorphicView_unknownDiscriminator` — pass `output.FormatTable`
- `TestPolymorphicView_nilDetail` — pass `output.FormatTable`
- `TestPolymorphicView_resolveError` — pass `output.FormatTable`
- `TestPolymorphicView_discriminatorError` — pass `output.FormatTable`

All `t.Cleanup(func() { flags.OutputFormat = "" })` and `flags.OutputFormat = ...` lines must be removed.

- [ ] **Step 3: Update every leaf callsite to pass the format**

This is mechanical: every call to `view.Render(...)`, `view.RenderWith(...)`, or `polymorphicView.Render(...)` outside `cmdutil/view_test.go` gains `flags.GetOutputFormat()` as its second argument. Each caller will switch to `f.Output()` in its per-domain task (8–13).

Find them with:

```bash
grep -rn '\.Render(\|\.RenderWith(' --include='*.go' internal/cli/ | grep -v view_test.go | grep -v cmdutil/
```

For each call, change e.g.:

```go
return containersListView.Render(w, resp.StatusCode(), resp.Body, resp.JSON200)
```

to:

```go
return containersListView.Render(w, flags.GetOutputFormat(), resp.StatusCode(), resp.Body, resp.JSON200)
```

Each leaf file gains an import of `internal/cli/flags` if it doesn't already have one (most do).

- [ ] **Step 4: Verify the build and tests**

Run: `make build && make test`
Expected: both succeed.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit -m "refactor(cmdutil): pass outputFmt explicitly to View.Render"
```

---

## Task 7: Add `getOutputFmt` parameter to `watch.Wrap`

**Files:**
- Modify: `internal/cli/watch/watch.go`
- Modify: `internal/cli/watch/watch_test.go`
- Modify: every leaf file that calls `watch.Wrap(...)` (about 8 files)

The `watch` loop must know whether it's in JSON or table mode to choose between NDJSON output and ANSI-based screen redraws. It currently reads this via `flags.GetOutputFormat()`. After this task, callers pass `flags.GetOutputFormat` (the bare getter, not called) as a `func() output.Format`. Per-domain tasks 8–13 swap this for `f.Output`.

- [ ] **Step 1: Change the `watch.Wrap` signature**

In `internal/cli/watch/watch.go`, replace the imports (drop `flags`):

```go
import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "os/signal"
    "strings"
    "syscall"
    "time"

    "github.com/bwilczynski/hlctl/internal/output"
    "github.com/spf13/cobra"
    "golang.org/x/term"
)
```

Replace `Wrap` and the docstring note about JSON mode:

```go
// TickFunc is the per-tick body executed by the watch loop. It writes its
// rendered output to w and uses ctx for any cancellable work (e.g. HTTP calls).
//
// In JSON output mode (getOutputFmt() == output.FormatJSON), fn must write
// exactly one compact JSON document with no trailing newline; the loop
// appends the NDJSON newline separator. Do NOT call output.Print in JSON mode
// — it pretty-prints with indentation, which breaks NDJSON.
type TickFunc func(ctx context.Context, w io.Writer) error

// Wrap returns a cobra RunE. When --watch is false, it calls fn once with
// cmd.OutOrStdout(). When --watch is true, it runs fn on an interval until
// the context is cancelled by SIGINT/SIGTERM.
//
// getOutputFmt is a closure returning the current --output flag value, read
// per invocation so that flag parsing has already completed.
func Wrap(getOutputFmt func() output.Format, fn TickFunc) func(cmd *cobra.Command, args []string) error {
    return func(cmd *cobra.Command, args []string) error {
        watching, _ := cmd.Flags().GetBool("watch")
        if !watching {
            return fn(cmd.Context(), cmd.OutOrStdout())
        }
        interval, _ := cmd.Flags().GetDuration("watch-interval")
        if interval < minInterval {
            return fmt.Errorf("--watch-interval must be at least %s", minInterval)
        }
        return loop(cmd, interval, getOutputFmt, fn)
    }
}
```

Replace `loop` (signature gains `getOutputFmt`):

```go
func loop(cmd *cobra.Command, interval time.Duration, getOutputFmt func() output.Format, fn TickFunc) error {
    w := cmd.OutOrStdout()
    ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    tty := isTerminal(w)
    jsonMode := getOutputFmt() == output.FormatJSON

    // ... (rest of the function body unchanged from current implementation)
}
```

The body of `loop` is otherwise identical to today's — just replace the one `flags.GetOutputFormat()` call on line 74 with `getOutputFmt()` (which was already done above by declaring `jsonMode := getOutputFmt() == output.FormatJSON`).

- [ ] **Step 2: Rewrite `internal/cli/watch/watch_test.go` to pass an output-format getter**

Drop the `flags` import, add `output` if not present.

For each test that currently does:

```go
prev := flags.OutputFormat
flags.OutputFormat = "json"
t.Cleanup(func() { flags.OutputFormat = prev })
```

Replace with constructing a getter:

```go
getJSON := func() output.Format { return output.FormatJSON }
```

(Or `getTable` returning `output.FormatTable` where the test exercises table mode.)

Update each `watch.Wrap(fn)` call in the test file to `watch.Wrap(getJSON, fn)` (or `getTable`).

- [ ] **Step 3: Update every leaf `watch.Wrap(...)` call**

Find them with:

```bash
grep -rn 'watch\.Wrap(' --include='*.go' internal/cli/ | grep -v watch/
```

For each leaf call, change e.g.:

```go
cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error { ... })
```

to:

```go
cmd.RunE = watch.Wrap(flags.GetOutputFormat, func(ctx context.Context, w io.Writer) error { ... })
```

The closure `flags.GetOutputFormat` matches the `func() output.Format` signature. Each leaf file gains an import of `internal/cli/flags` if it didn't have one already.

Leaves that use `watch.Wrap` (from current codebase):
- `internal/cli/docker/containers.go` (list)
- `internal/cli/docker/images.go` (list)
- `internal/cli/docker/networks.go` (list)
- `internal/cli/network/clients.go` (list)
- `internal/cli/network/devices.go` (list)
- `internal/cli/network/vlans.go` (list)
- `internal/cli/network/wans.go` (list)
- `internal/cli/network/ssids.go` (list)
- `internal/cli/storage/volumes.go` (list)
- `internal/cli/storage/backups.go` (list)
- `internal/cli/system/health.go` (list)
- `internal/cli/system/utilization.go` (list)

Confirm the actual list with the grep above before editing.

- [ ] **Step 4: Verify the build and tests**

Run: `make build && make test`
Expected: both succeed.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit -m "refactor(watch): pass output-format getter explicitly to Wrap"
```

---

## Task 8: Migrate `auth` domain

**Files:**
- Modify: `internal/cli/auth/auth.go`
- Modify: `internal/cli/auth/auth_test.go`

`auth` doesn't render via `cmdutil.View` and doesn't use `watch.Wrap`. The only `flags` reference is the `flags.GetAPIURL()` call in `newLoginCmd`'s `RunE`.

- [ ] **Step 1: Modify `internal/cli/auth/auth.go`**

Replace the imports — drop `flags`, add `cmdutil`:

```go
import (
    "context"
    "fmt"

    authpkg "github.com/bwilczynski/hlctl/internal/auth"
    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/bwilczynski/hlctl/internal/config"
    "github.com/spf13/cobra"
)
```

Update `NewCmd` to take the Factory and thread it into `newLoginCmd`:

```go
func NewCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "auth",
        Short: "Authenticate with the Homelab API",
    }
    cmd.AddCommand(newLoginCmd(f))
    cmd.AddCommand(newLogoutCmd())
    return cmd
}
```

Update `newLoginCmd` to use `f.APIURL()` instead of `flags.GetAPIURL()`:

```go
func newLoginCmd(f *cmdutil.Factory) *cobra.Command {
    return &cobra.Command{
        Use:   "login",
        Short: "Log in via device authorization flow",
        RunE: func(cmd *cobra.Command, args []string) error {
            cfg, err := config.Load()
            if err != nil {
                return err
            }
            apiURL, err := f.APIURL()
            if err != nil {
                return err
            }

            info, err := authpkg.DiscoverHomelab(apiURL)
            if err != nil {
                return fmt.Errorf("discovery failed: %w", err)
            }
            if !info.Enabled {
                fmt.Fprintln(cmd.OutOrStdout(), "Server does not require authentication.")
                return nil
            }

            endpoints, err := authpkg.DiscoverOIDC(info.Issuer)
            if err != nil {
                return fmt.Errorf("OIDC discovery failed: %w", err)
            }

            creds, err := authpkg.Login(context.Background(), endpoints, cfg.ClientID(), cmd.OutOrStdout())
            if err != nil {
                return err
            }

            if err := authpkg.SaveCredentials(creds); err != nil {
                return fmt.Errorf("saving credentials: %w", err)
            }
            fmt.Fprintln(cmd.OutOrStdout(), "Login successful.")
            return nil
        },
    }
}
```

`newLogoutCmd()` is unchanged — it has no Factory needs.

- [ ] **Step 2: Update `internal/cli/auth/auth_test.go`**

Wherever the test constructs an auth command, pass `cmdutil.TestFactory(t)`. Existing tests that exercise logout don't need the factory threaded further; tests for login will need to override `f.APIURL` to return a fixture URL:

```go
f := cmdutil.TestFactory(t)
f.APIURL = func() (string, error) { return "https://fixture.test", nil }
cmd := newLoginCmd(f)
```

Open `auth_test.go` and update every constructor call accordingly. If existing tests stub out only the `logout` subcommand, they only need `cmdutil.TestFactory(t)` if they construct the root `NewCmd`; otherwise `newLogoutCmd()` continues to take no arguments and tests pass.

- [ ] **Step 3: Verify the build and tests**

Run: `make build && make test`
Expected: both succeed.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/auth/
git commit -m "refactor(auth): thread *cmdutil.Factory through NewCmd"
```

---

## Task 9: Migrate `config` domain

**Files:**
- Modify: `internal/cli/config/config.go`

`config` doesn't currently read any `flags` global. The change is purely the signature: `NewCmd()` → `NewCmd(f *cmdutil.Factory)`, so the root command can call it uniformly.

- [ ] **Step 1: Update `internal/cli/config/config.go`**

Add the import and update the signature:

```go
import (
    // ... existing imports ...
    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
    // ... existing body unchanged; ignore f for now ...
}
```

If `f` becomes "declared and not used" in this function, prefix with `_`:

```go
func NewCmd(_ *cmdutil.Factory) *cobra.Command {
    // ...
}
```

- [ ] **Step 2: Verify the build and tests**

Run: `make build && make test`
Expected: both succeed.

- [ ] **Step 3: Commit**

```bash
git add internal/cli/config/
git commit -m "refactor(config): accept *cmdutil.Factory in NewCmd"
```

---

## Task 10: Migrate `system` domain

**Files:**
- Modify: `internal/cli/system/system.go`
- Modify: `internal/cli/system/health.go`
- Modify: `internal/cli/system/info.go`
- Modify: `internal/cli/system/utilization.go`
- Modify: `internal/cli/system/updates.go`
- Modify: `internal/cli/system/health_test.go`
- Modify: `internal/cli/system/info_test.go`
- Modify: `internal/cli/system/utilization_test.go`
- Modify: `internal/cli/system/updates_test.go`

- [ ] **Step 1: Rewire `internal/cli/system/system.go`**

Drop the `internal/apiclient` use, take `f`, and use `f.HTTPClient()`:

```go
package system

import (
    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "system",
        Short: "System health and information",
    }
    cmdutil.InjectClient(cmd, func() (SystemClient, error) {
        httpClient, apiURL, err := f.HTTPClient()
        if err != nil {
            return nil, err
        }
        return NewSystemClient(httpClient, apiURL)
    })
    cmd.AddCommand(newHealthCmd(f), newInfoCmd(f), newUtilizationCmd(f), newUpdatesCmd(f))
    return cmd
}
```

Delete the now-unused `buildClient` helper if present.

- [ ] **Step 2: Update each leaf constructor in `internal/cli/system/`**

Every leaf that calls `cmdutil.View.Render` / `RenderWith` / `PolymorphicView.Render` (or `watch.Wrap`) gains `f *cmdutil.Factory` and swaps the temporary `flags.GetOutputFormat()` / `flags.GetOutputFormat` for `f.Output()` / `f.Output`.

Example for `internal/cli/system/info.go`:

```go
package system

import (
    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/bwilczynski/hlctl/internal/output"
    systemapi "github.com/bwilczynski/hlctl/internal/api/system"
    "github.com/spf13/cobra"
)

var infoView = cmdutil.View{Templates: systemTemplates, Name: "info.tmpl"}

type infoRow struct {
    Device   string
    Model    string
    Firmware string
    Ram      string
    Uptime   string
}

func newInfoCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "info",
        Short: "Show device information",
    }
    device := cmdutil.DeviceFlag(cmd)
    cmd.RunE = func(cmd *cobra.Command, args []string) error {
        params := &systemapi.ListSystemInfoParams{}
        if *device != "" {
            params.Device = device
        }

        resp, err := cmdutil.Client[SystemClient](cmd).ListSystemInfoWithResponse(cmd.Context(), params)
        if err != nil {
            return err
        }
        return infoView.RenderWith(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, func() (any, error) {
            items := make([]infoRow, 0, len(resp.JSON200.Items))
            for _, i := range resp.JSON200.Items {
                items = append(items, infoRow{
                    Device:   i.Device,
                    Model:    i.Model,
                    Firmware: i.Firmware,
                    Ram:      output.FormatBytes(int64(i.RamMb) * 1024 * 1024),
                    Uptime:   output.FormatUptime(int(i.UptimeSeconds)),
                })
            }
            return struct{ Items []infoRow }{items}, nil
        })
    }
    return cmd
}
```

The diff per leaf:
- Constructor signature gains `f *cmdutil.Factory`.
- Remove the import of `internal/cli/flags`.
- Replace the temporary `flags.GetOutputFormat()` in `Render`/`RenderWith` with `f.Output()`.
- Replace the temporary `flags.GetOutputFormat` in `watch.Wrap` (where applicable) with `f.Output`.

Apply the same shape to:
- `internal/cli/system/health.go` (`newHealthCmd(f)`)
- `internal/cli/system/utilization.go` (`newUtilizationCmd(f)`) — uses `watch.Wrap`; pass `f.Output` as first arg.
- `internal/cli/system/updates.go` (`newUpdatesCmd(f)`)

- [ ] **Step 3: Update each `*_test.go` in `internal/cli/system/`**

Every test that previously called `newXxxCmd()` now calls `newXxxCmd(cmdutil.TestFactory(t))`.

For tests that mutate `flags.OutputFormat = "json"` to exercise JSON mode (`internal/cli/system/updates_test.go`), replace the mutation with an override on the test factory:

```go
f := cmdutil.TestFactory(t)
f.Output = func() output.Format { return output.FormatJSON }
cmd := newUpdatesCmd(f)
cmdutil.SetClient[SystemClient](cmd, stub)
```

Drop the `internal/cli/flags` import from each test file once no references remain. Drop the `t.Cleanup(func() { flags.OutputFormat = "" })` lines.

- [ ] **Step 4: Verify the build and tests**

Run: `make build && make test`
Expected: both succeed.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/system/
git commit -m "refactor(system): thread *cmdutil.Factory through NewCmd and leaves"
```

---

## Task 11: Migrate `docker` domain

**Files:**
- Modify: `internal/cli/docker/docker.go`
- Modify: `internal/cli/docker/containers.go`
- Modify: `internal/cli/docker/images.go`
- Modify: `internal/cli/docker/networks.go`
- Modify: `internal/cli/docker/containers_test.go`
- Modify: `internal/cli/docker/images_test.go`
- Modify: `internal/cli/docker/networks_test.go`

`docker` has the group-parent shape: `docker.NewCmd` → `newContainersCmd` / `newImagesCmd` / `newNetworksCmd`, each of which has its own `InjectClient` call today and leaf children. After this task, the `InjectClient` lives on the `docker.NewCmd` (or each group parent — confirm with the current code; today each sub-parent calls `InjectClient` because they share one client).

- [ ] **Step 1: Rewire `internal/cli/docker/docker.go`**

If the existing code has `InjectClient` on the `docker` root, port that pattern:

```go
package docker

import (
    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{Use: "docker", Short: "Docker resources"}
    cmd.AddCommand(newContainersCmd(f), newNetworksCmd(f), newImagesCmd(f))
    return cmd
}
```

(Delete the existing `buildClient` helper if present.)

- [ ] **Step 2: Rewire each group parent in `internal/cli/docker/containers.go`, `images.go`, `networks.go`**

Per the CLAUDE.md convention, docker's `InjectClient` lives on each sub-parent. Each group parent gains `f` and builds its closure from `f.HTTPClient`:

```go
// in internal/cli/docker/containers.go
func newContainersCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{Use: "containers", Short: "Manage Docker containers"}
    cmdutil.InjectClient(cmd, func() (DockerClient, error) {
        httpClient, apiURL, err := f.HTTPClient()
        if err != nil {
            return nil, err
        }
        return NewClient(httpClient, apiURL)
    })
    cmd.AddCommand(
        newListContainersCmd(f),
        newGetContainerCmd(f),
        newStartContainerCmd(),
        newStopContainerCmd(),
        newRestartContainerCmd(),
    )
    return cmd
}
```

Apply the same shape to `newImagesCmd(f)` and `newNetworksCmd(f)`. (Note: action commands like `newStartContainerCmd` stay zero-argument — `cmdutil.ActionCmd` doesn't render via View and doesn't need Factory.)

- [ ] **Step 3: Update each view-rendering leaf**

Each leaf that calls `view.Render` / `view.RenderWith` (e.g. `newListContainersCmd`, `newGetContainerCmd`) gains `f *cmdutil.Factory` and passes `f.Output()` / `f.Output` to the view + watch APIs.

Example for `newListContainersCmd` in `internal/cli/docker/containers.go`:

```go
func newListContainersCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{Use: "list", Short: "List containers"}
    device := cmdutil.DeviceFlag(cmd)
    cmd.RunE = watch.Wrap(f.Output, func(ctx context.Context, w io.Writer) error {
        params := &dockerapi.ListContainersParams{}
        if *device != "" {
            params.Device = device
        }
        resp, err := cmdutil.Client[DockerClient](cmd).ListContainersWithResponse(ctx, params)
        if err != nil {
            return err
        }
        return containersListView.Render(w, f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
    })
    watch.RegisterFlags(cmd)
    return cmd
}
```

`newGetContainerCmd`:

```go
func newGetContainerCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{Use: "get <container-id>", Short: "Show container details", Args: cobra.ExactArgs(1)}
    cmd.RunE = func(cmd *cobra.Command, args []string) error {
        resp, err := cmdutil.Client[DockerClient](cmd).GetContainerWithResponse(cmd.Context(), args[0])
        if err != nil {
            return err
        }
        return containersGetView.Render(cmd.OutOrStdout(), f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
    }
    return cmd
}
```

Action commands stay zero-arg (no diff for `newStartContainerCmd`, `newStopContainerCmd`, `newRestartContainerCmd`).

Apply the same pattern to view-rendering leaves in `images.go` and `networks.go`. After this step, drop the `internal/cli/flags` import from every docker source file (it should no longer be referenced).

- [ ] **Step 4: Update each `*_test.go` in `internal/cli/docker/`**

Pass `cmdutil.TestFactory(t)` to view-rendering leaf constructors. Action-command tests (`TestStartContainerCmd` etc.) are unchanged — they call `newStartContainerCmd()` and `cmdutil.SetClient[DockerClient](cmd, stub)`.

For each view-rendering test, replace:

```go
cmd := newListContainersCmd()
cmdutil.SetClient[DockerClient](cmd, stub)
```

with:

```go
cmd := newListContainersCmd(cmdutil.TestFactory(t))
cmdutil.SetClient[DockerClient](cmd, stub)
```

If any docker test mutates `flags.OutputFormat` for JSON-mode assertions, replace with:

```go
f := cmdutil.TestFactory(t)
f.Output = func() output.Format { return output.FormatJSON }
cmd := newListContainersCmd(f)
cmdutil.SetClient[DockerClient](cmd, stub)
```

Drop the `internal/cli/flags` import from any docker test file once no references remain.

- [ ] **Step 5: Verify the build and tests**

Run: `make build && make test`
Expected: both succeed.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/docker/
git commit -m "refactor(docker): thread *cmdutil.Factory through NewCmd and leaves"
```

---

## Task 12: Migrate `network` domain

**Files:**
- Modify: `internal/cli/network/network.go`
- Modify: `internal/cli/network/clients.go`, `devices.go`, `ssids.go`, `topology.go`, `vlans.go`, `wans.go`
- Modify: `internal/cli/network/clients_test.go`, `devices_test.go`, `ssids_test.go`, `topology_test.go`, `vlans_test.go`, `wans_test.go`

- [ ] **Step 1: Rewire `internal/cli/network/network.go`**

```go
package network

import (
    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "network",
        Short: "Network resources",
    }
    cmdutil.InjectClient(cmd, func() (NetworkClient, error) {
        httpClient, apiURL, err := f.HTTPClient()
        if err != nil {
            return nil, err
        }
        return NewNetworkClient(httpClient, apiURL)
    })
    cmd.AddCommand(
        newClientsCmd(f),
        newDevicesCmd(f),
        newSSIDsCmd(f),
        newTopologyCmd(f),
        newVLANsCmd(f),
        newWANsCmd(f),
    )
    return cmd
}
```

(Delete the existing `buildClient` if present.)

- [ ] **Step 2: Update each view-rendering leaf constructor**

Same pattern as Task 11 step 3:
- Constructor gains `f *cmdutil.Factory`.
- `view.Render(...)` calls pass `f.Output()` in place of `flags.GetOutputFormat()`.
- `watch.Wrap(...)` calls pass `f.Output` as the first arg.
- Drop the `internal/cli/flags` import.

Leaves to update: `newClientsCmd`, `newDevicesCmd`, `newSSIDsCmd`, `newTopologyCmd`, `newVLANsCmd`, `newWANsCmd`. (`topology.go` uses a `PolymorphicView` — pass `f.Output()` to `polymorphicView.Render` as the second argument.)

- [ ] **Step 3: Update each `*_test.go` in `internal/cli/network/`**

Pass `cmdutil.TestFactory(t)` to leaf constructors. For `topology_test.go`, which today mutates `flags.OutputFormat = "json"`, replace with:

```go
f := cmdutil.TestFactory(t)
f.Output = func() output.Format { return output.FormatJSON }
cmd := newTopologyCmd(f)
cmdutil.SetClient[NetworkClient](cmd, stub)
```

Drop the `internal/cli/flags` import once no references remain.

- [ ] **Step 4: Verify the build and tests**

Run: `make build && make test`
Expected: both succeed.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/network/
git commit -m "refactor(network): thread *cmdutil.Factory through NewCmd and leaves"
```

---

## Task 13: Migrate `storage` domain

**Files:**
- Modify: `internal/cli/storage/storage.go`
- Modify: `internal/cli/storage/volumes.go`, `backups.go`
- Modify: `internal/cli/storage/volumes_test.go`, `backups_test.go`

- [ ] **Step 1: Rewire `internal/cli/storage/storage.go`**

Per CLAUDE.md, storage's `InjectClient` lives on each sub-group parent (volumes / backups) — confirm with the current code. If it's on the root, port that pattern.

```go
package storage

import (
    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{Use: "storage", Short: "Storage resources"}
    cmd.AddCommand(newVolumesCmd(f), newBackupsCmd(f))
    return cmd
}
```

(Delete the existing `buildClient` helper.)

- [ ] **Step 2: Rewire each sub-parent**

For each of `newVolumesCmd` / `newBackupsCmd`, add the `InjectClient` closure that captures `f.HTTPClient`:

```go
func newVolumesCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{Use: "volumes", Short: "Manage volumes"}
    cmdutil.InjectClient(cmd, func() (StorageClient, error) {
        httpClient, apiURL, err := f.HTTPClient()
        if err != nil {
            return nil, err
        }
        return NewClient(httpClient, apiURL)
    })
    cmd.AddCommand( /* leaves */ )
    return cmd
}
```

Apply the same shape to `newBackupsCmd`.

- [ ] **Step 3: Update each view-rendering leaf**

Same pattern as Tasks 11/12 step 3. Pass `f.Output()` / `f.Output` to view/watch APIs. Drop the `internal/cli/flags` import.

- [ ] **Step 4: Update each `*_test.go` in `internal/cli/storage/`**

Pass `cmdutil.TestFactory(t)` to leaf constructors. If any test mutates `flags.OutputFormat`, override `f.Output` instead.

- [ ] **Step 5: Verify the build and tests**

Run: `make build && make test`
Expected: both succeed.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/storage/
git commit -m "refactor(storage): thread *cmdutil.Factory through NewCmd and leaves"
```

---

## Task 14: Add `NewRootCmd(f)`, rewire `main`

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/root_test.go`
- Modify: `cmd/hlctl/main.go`

After this task, the only remaining `flags` references should be in tests that haven't been swept yet (verified in step 4 below).

- [ ] **Step 1: Rewrite `internal/cli/root.go`**

Replace the entire file:

```go
package cli

import (
    "github.com/bwilczynski/hlctl/internal/cli/auth"
    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/bwilczynski/hlctl/internal/cli/config"
    dockercli "github.com/bwilczynski/hlctl/internal/cli/docker"
    "github.com/bwilczynski/hlctl/internal/cli/network"
    "github.com/bwilczynski/hlctl/internal/cli/storage"
    "github.com/bwilczynski/hlctl/internal/cli/system"
    "github.com/spf13/cobra"
)

// NewRootCmd builds a fresh root command tree bound to f.
// No package-level state — safe to call multiple times (tests do).
func NewRootCmd(f *cmdutil.Factory) *cobra.Command {
    root := &cobra.Command{
        Use:          "hlctl",
        Short:        "CLI for controlling homelab services",
        Long:         "hlctl is a command-line interface for managing your homelab infrastructure via the Homelab API.",
        Version:      f.Version,
        SilenceUsage: true,
    }
    root.SetOut(f.IOStreams.Out)
    root.SetErr(f.IOStreams.ErrOut)
    root.SetIn(f.IOStreams.In)
    root.AddCommand(
        auth.NewCmd(f),
        config.NewCmd(f),
        dockercli.NewCmd(f),
        network.NewCmd(f),
        storage.NewCmd(f),
        system.NewCmd(f),
    )
    return root
}
```

The old `var rootCmd`, the `init()` function, and `func Execute(version string) error` are all deleted.

- [ ] **Step 2: Update `internal/cli/root_test.go`**

If `root_test.go` calls `cli.Execute(version)` or references the package-level `rootCmd`, replace with `cli.NewRootCmd(cmdutil.TestFactory(t))` and call `.Execute()` on the returned command. Read the existing file first; the rewrite is mechanical.

- [ ] **Step 3: Rewrite `cmd/hlctl/main.go`**

```go
package main

import (
    "os"

    "github.com/bwilczynski/hlctl/internal/cli"
    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/spf13/pflag"
)

var version = "dev"

func main() {
    var apiURL, outputFmt string
    pflag.StringVarP(&outputFmt, "output", "o", "table", "Output format: table or json")
    pflag.StringVar(&apiURL, "api-url", "", "Override API base URL")

    f := cmdutil.NewFactory(version, &apiURL, &outputFmt)
    root := cli.NewRootCmd(f)
    root.PersistentFlags().AddFlagSet(pflag.CommandLine)
    if err := root.Execute(); err != nil {
        os.Exit(1)
    }
}
```

- [ ] **Step 4: Verify the build, tests, and the help text**

Run: `make build && make test`
Expected: both succeed.

Run: `./bin/hlctl --help`
Expected: identical command tree to the pre-refactor binary; `--output` and `--api-url` listed under "Global Flags". Capture the pre-refactor output (e.g. from a previous binary) for diff if possible; otherwise verify by eye against `docs/superpowers/specs/2026-06-12-cli-refactor-design.md`'s compatibility checklist.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/root.go internal/cli/root_test.go cmd/hlctl/main.go
git commit -m "refactor(cli): replace init()-based root with NewRootCmd(*Factory)"
```

---

## Task 15: Delete `internal/cli/flags/`

**Files:**
- Delete: `internal/cli/flags/flags.go`
- Delete: `internal/cli/flags/` (directory)

- [ ] **Step 1: Verify no remaining references**

Run: `grep -rn '"github.com/bwilczynski/hlctl/internal/cli/flags"' --include='*.go'`
Expected: no matches.

If matches remain, return to the relevant per-domain task and swap them out before deleting the package.

- [ ] **Step 2: Delete the file and directory**

```bash
git rm internal/cli/flags/flags.go
rmdir internal/cli/flags
```

- [ ] **Step 3: Verify the build and tests**

Run: `make build && make test`
Expected: both succeed.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/flags/
git commit -m "refactor: remove internal/cli/flags package (replaced by Factory)"
```

---

## Task 16: Delete `api.NewHTTPClient` and final cleanup

**Files:**
- Modify: `internal/api/apiclient.go` (delete `NewHTTPClient`; possibly delete the whole file if nothing else lives there)

- [ ] **Step 1: Verify no remaining references**

Run: `grep -rn 'api\.NewHTTPClient' --include='*.go'`
Expected: no matches. (All domain `NewCmd` closures should now call `f.HTTPClient()` directly.)

If matches remain, fix the offending file first.

- [ ] **Step 2: Delete `NewHTTPClient` (and the file if otherwise empty)**

Read `internal/api/apiclient.go`. If `NewHTTPClient` is the only declaration, delete the file:

```bash
git rm internal/api/apiclient.go
```

Otherwise edit the file and remove the `NewHTTPClient` function (and any newly-unused imports like `internal/cli/flags` — already deleted in Task 15 — or `internal/config` if only used by the deleted function).

- [ ] **Step 3: Run the final verification gate**

Run: `make build && make test && make lint`
Expected: all three succeed.

Run: `./bin/hlctl --help`
Expected: identical command tree to the pre-refactor binary.

Run end-to-end smoke commands manually if convenient:
```
./bin/hlctl --help
./bin/hlctl version
./bin/hlctl system --help
./bin/hlctl docker containers --help
```
Expected: all produce the same output structure as before the refactor.

- [ ] **Step 4: Verify the compatibility checklist from the spec**

Open `docs/superpowers/specs/2026-06-12-cli-refactor-design.md` and walk the "Compatibility checklist (Phase 1)" section:

- [ ] `make build` produces a binary with character-identical `--help` output at every level
- [ ] All existing tests pass (mechanical edits only)
- [ ] `make lint` (currently `go vet ./...`) is clean
- [ ] `HOMELAB_API_URL`, `HOMELAB_TOKEN`, `--api-url`, `--output`, `--device`, and the watch flags behave identically
- [ ] Config file location and format are unchanged
- [ ] Exit codes and error message formats are unchanged
- [ ] No package-level `var` of mutable state under `internal/cli/` — verify with:
  ```bash
  grep -rn '^var ' --include='*.go' internal/cli/ | grep -v '_test.go'
  ```
  Expected: no `var ... string`, `var ... int`, etc. (only the view declarations like `var infoView = cmdutil.View{...}` which are immutable value declarations).

- [ ] **Step 5: Commit**

```bash
git add internal/api/
git commit -m "refactor: remove api.NewHTTPClient (Factory replaces it)"
```

---

## Self-review

**Spec coverage check:**

| Spec section | Covered by |
| --- | --- |
| Move codegen YAMLs to `codegen/` | Task 1 |
| Move generated code to `internal/api/<domain>/` | Task 2 |
| Rename `internal/apiclient` → `internal/api` | Task 3 |
| Add `IOStreams` type | Task 4 |
| Add `Factory` type + `NewFactory` + `TestFactory` | Task 5 |
| `View.Render` / `RenderWith` / `PolymorphicView.Render` gain `outputFmt` parameter | Task 6 |
| `watch.Wrap` gains output-format getter (spec lists this as a Phase 2 open question but Phase 1 needs it to delete the `flags` package) | Task 7 |
| Per-domain `NewCmd(f *cmdutil.Factory)` migration | Tasks 8–13 |
| Domain root uses `f.HTTPClient()` inside `cmdutil.InjectClient` closure | Tasks 10–13 |
| Leaves take `f` and pass `f.Output()` to `View.Render` | Tasks 10–13 |
| Action-style leaves stay zero-argument | Tasks 11, 13 (containers/volumes start/stop/restart pattern) |
| `auth.go` swaps `flags.GetAPIURL()` for `f.APIURL()` | Task 8 |
| `cmdutil.action.go` only needs the `apiclient.ParseError` → `api.ParseError` rename | Task 3 |
| `NewRootCmd(f)` replaces `init()`-based setup | Task 14 |
| `main.go` constructs Factory + attaches flagset | Task 14 |
| Delete `internal/cli/flags/` | Task 15 |
| Delete `api.NewHTTPClient` | Task 16 |
| Compatibility checklist | Task 16 step 4 |

**Spec amendments noted during planning:**
- The spec listed `watch.Wrap` signature change as a Phase 2 open question. Phase 1 actually requires it because `watch.go` reads `flags.GetOutputFormat()` directly and Phase 1 deletes the `flags` package. Task 7 handles it explicitly.
- The spec's `Factory` definition didn't include `APIURL func() (string, error)`, but `auth.go`'s `newLoginCmd` only needs the URL (not an http.Client). Adding `Factory.APIURL` avoids the wasted http.Client construction. Task 5 adds the field.

**Placeholder scan:** no "TBD", no "implement later", no "similar to Task N" — patterns are repeated in each per-domain task because tasks may be executed out of order.

**Type consistency:**
- `Factory` fields: `Version`, `IOStreams`, `Config`, `APIURL`, `HTTPClient`, `Output` — used identically across tasks 5, 7, 8, 10–14.
- `IOStreams` fields: `In`, `Out`, `ErrOut` — consistent across tasks 4 and 14.
- View signatures: `(w, outputFmt, statusCode, body, data)` and `(w, outputFmt, statusCode, body, fn)` — consistent across tasks 6 and 10–13.
- `watch.Wrap(getOutputFmt, fn)` — consistent across tasks 7 and 10–13.

**Open implementation choices the spec deferred and this plan resolves:**
- `testFactory` lives in `internal/cli/cmdutil/testfactory.go` as exported `cmdutil.TestFactory(t)` — single source of truth.
- Generated-code alias convention: `<domain>api` (e.g. `dockerapi`, `systemapi`) — replaces the generic `gen` alias used today.
