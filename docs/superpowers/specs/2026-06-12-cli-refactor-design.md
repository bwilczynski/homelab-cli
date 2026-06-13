# CLI Refactor: Factory + idiomatic layout (Phase 1)

## Status

**Designed, implementation deferred.** This document captures the agreed end-state for **Phase 1** — the scope-reduced refactor that delivers the structural wins (no package-level globals, explicit DI at the root, idiomatic layout) without rewriting tests. The migration plan will be drafted in a separate session when work is scheduled.

A **Phase 2** that adopts the full gh-style per-command `Options` + `runF` + HTTP-transport mocking pattern is described at the end of this document. Phase 2 is independent of Phase 1 and can be deferred indefinitely or adopted opportunistically per-leaf.

## Goal (Phase 1)

Eliminate package-level mutable state, make dependency injection explicit at the root command, and reorganize generated code so it no longer shadows command packages. Leave the per-leaf command shape and test infrastructure substantially unchanged.

## Non-goals

- **No user-visible behavior changes.** Every command, flag, env var, exit code, and output format stays identical. A built binary before/after the refactor is behaviorally indistinguishable to users.
- **No new domains, no new commands, no new features.**
- **No template or rendering changes.** `cmdutil.View` / `cmdutil.PolymorphicView` and the embedded `*.tmpl` files are preserved.
- **No OpenAPI spec or generated-API contract changes.** Only the location and import path of the generated code move.
- **No test rewrites.** Existing `StubClient` + `cmdutil.SetClient[C]` pattern is preserved. The Phase 2 follow-up handles the test-layer overhaul if and when it's scheduled.

## Motivation

The current layout has accumulated three patterns at odds with idiomatic Go CLI conventions:

1. **Package-level mutable globals in `internal/cli/flags/`** (`OutputFormat`, `APIURL`). Cobra writes into them via `StringVarP`, and arbitrary code reads from them. This blocks parallel testing, hides dependencies, and couples `internal/apiclient` to a singleton flag namespace.
2. **Path collision between generated code (`internal/<domain>/`) and command code (`internal/cli/<domain>/`).** Every leaf imports the generated package with a generic `gen` alias to dodge the shadowing.
3. **Implicit, context-based DI via `cmdutil.InjectClient` / `Client[C]` / `SetClient[C]`.** Clients are stashed on `cmd.Context()` keyed by their type. **This pattern is preserved in Phase 1** — it remains the load-bearing simplification that lets leaves stay signature-compatible. The mechanism's invisibility is the price of avoiding a test-suite rewrite, and Phase 2 addresses it when the cost of rewriting tests is judged worth paying.

Phase 1 closes (1) and (2). Phase 2 closes (3).

## Final directory tree (Phase 1)

```
homelab-cli/
├── cmd/hlctl/
│   └── main.go                  # build Factory, call cli.NewRootCmd(f), Execute
├── codegen/                     # NEW: per-domain oapi-codegen configs, moved from repo root
│   ├── docker.yaml
│   ├── network.yaml
│   ├── storage.yaml
│   └── system.yaml
├── internal/
│   ├── api/                     # NEW: API surface (rename of internal/apiclient + internal/<domain>)
│   │   ├── errors.go            # package api — ParseError, moved from internal/apiclient/errors.go
│   │   ├── docker/              # package docker (generated, was internal/docker/)
│   │   ├── network/             # package network (generated, was internal/network/)
│   │   ├── storage/             # package storage (generated, was internal/storage/)
│   │   └── system/              # package system  (generated, was internal/system/)
│   ├── auth/                    # unchanged
│   ├── config/                  # unchanged
│   ├── output/                  # unchanged
│   └── cli/
│       ├── root.go              # NewRootCmd(f *cmdutil.Factory) *cobra.Command
│       ├── root_test.go
│       ├── cmdutil/
│       │   ├── factory.go       # NEW: Factory struct + NewFactory()
│       │   ├── iostreams.go     # NEW: IOStreams{In, Out, ErrOut}
│       │   ├── client.go        # PRESERVED: InjectClient, Client[C], SetClient[C]
│       │   ├── action.go        # PRESERVED: ActionCmd[C] (import path update only)
│       │   ├── view.go          # +1 parameter on Render/RenderWith for outputFmt
│       │   ├── flags.go         # unchanged (DeviceFlag)
│       │   └── *_test.go
│       ├── watch/               # unchanged
│       ├── auth/                # NewCmd(f *cmdutil.Factory)
│       ├── config/              # NewCmd(f *cmdutil.Factory)
│       ├── docker/
│       │   ├── docker.go        # NewCmd(f *cmdutil.Factory)
│       │   ├── client.go        # unchanged
│       │   ├── containers.go    # leaf signatures gain (f *cmdutil.Factory) only where they render via View
│       │   ├── networks.go
│       │   ├── images.go
│       │   ├── templates.go
│       │   ├── stub.go          # PRESERVED
│       │   └── *_test.go        # PRESERVED in structure (testFactory replaces nothing — just used in construction)
│       ├── network/             # same shape as docker/
│       ├── storage/             # same shape
│       └── system/              # same shape
└── Makefile, go.mod, README.md, .goreleaser.yaml, .github/  (unchanged at root)
```

### Removed paths

- `internal/cli/flags/` — package deleted. `OutputFormat` and `APIURL` globals replaced by Factory fields/methods.
- `internal/apiclient/` — package deleted. `NewHTTPClient` absorbed by Factory; `errors.go` moved to `internal/api/errors.go` (renamed package).
- `internal/<domain>/` (top-level generated dirs) — moved under `internal/api/`.
- `oapi-codegen-<domain>.yaml` files at repo root — moved into `codegen/`.

### Preserved paths (intentionally)

- `cmd/hlctl/main.go` — thin entrypoint pattern is already idiomatic.
- `internal/auth/`, `internal/config/`, `internal/output/` — no CLI knowledge.
- `internal/cli/watch/` — watch loop logic is independent of DI shape.
- `internal/cli/cmdutil/client.go` (`InjectClient`, `Client[C]`, `SetClient[C]`) — Phase 2 deletes these; Phase 1 keeps them so test code doesn't need rewriting.
- `internal/cli/cmdutil/action.go` (`ActionCmd[C]`) — kept; only the `apiclient.ParseError` → `api.ParseError` import changes.
- `internal/cli/cmdutil/flags.go` (`DeviceFlag`) — unchanged.
- Per-domain `stub.go` files (`StubClient` definitions) — unchanged.
- Per-domain `templates.go` and embedded template trees — unchanged.

## Factory

`Factory` bundles the building blocks every command needs. Constructed once in `main`, threaded into `NewRootCmd` and every domain's `NewCmd`. Function-valued fields defer expensive work (config load, token read, URL resolution) until a command actually runs.

```go
// internal/cli/cmdutil/factory.go
package cmdutil

import (
    "net/http"

    "github.com/bwilczynski/hlctl/internal/config"
    "github.com/bwilczynski/hlctl/internal/output"
)

type Factory struct {
    Version string

    IOStreams *IOStreams

    Config     func() (*config.Config, error)
    HTTPClient func() (*http.Client, string, error)  // returns client + resolved API URL
    Output     func() output.Format
}
```

### IOStreams

```go
// internal/cli/cmdutil/iostreams.go
package cmdutil

import (
    "io"
    "os"
)

type IOStreams struct {
    In     io.Reader
    Out    io.Writer
    ErrOut io.Writer
}

func SystemIOStreams() *IOStreams {
    return &IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
}
```

### Factory construction

```go
// internal/cli/cmdutil/factory.go (continued)
import "sync"

// NewFactory builds the default Factory wired to real config/auth/http.
// Flag-bound state (apiURL, outputFormat) is captured by closure over the
// *string the root command's PersistentFlags will write into.
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
    return &Factory{
        Version:   version,
        IOStreams: SystemIOStreams(),
        Config:    loadConfig,
        HTTPClient: func() (*http.Client, string, error) {
            c, err := loadConfig()
            if err != nil {
                return nil, "", err
            }
            apiURL := *apiURLFlag
            if apiURL == "" {
                apiURL, err = c.ResolveAPIURL()
                if err != nil {
                    return nil, "", err
                }
            }
            return &http.Client{Transport: auth.NewAuthenticatedTransport(nil)}, apiURL, nil
        },
        Output: func() output.Format { return output.Format(*outputFlag) },
    }
}
```

**Why function fields, not concrete values:** lazy evaluation lets `hlctl --help`, `hlctl version`, and any leaf that doesn't need the network skip config/auth entirely. Errors surface at the command that needs them, not at startup.

**Why `sync.Once` on config load:** preserves the current behavior (config loaded at most once per process) without relying on package-level state.

### Root wiring

```go
// internal/cli/root.go
package cli

import (
    "github.com/bwilczynski/hlctl/internal/cli/auth"
    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/bwilczynski/hlctl/internal/cli/config"
    "github.com/bwilczynski/hlctl/internal/cli/docker"
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
        docker.NewCmd(f),
        network.NewCmd(f),
        storage.NewCmd(f),
        system.NewCmd(f),
    )
    return root
}
```

Wiring `f.IOStreams` onto the root cobra command means every existing leaf that already calls `cmd.OutOrStdout()` continues working — the writer chain routes through IOStreams in production and through `cmd.SetOut(buf)` in tests.

### Entry point

```go
// cmd/hlctl/main.go
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

**Flag-binding seam:** `--output` and `--api-url` are declared on `pflag.CommandLine` in `main` so `NewFactory` can close over their backing `*string` pointers. `NewRootCmd` then attaches them as `PersistentFlags`. This keeps Factory independent of `*cobra.Command` without resorting to global flag state.

## Domain root and leaves (Phase 1)

The change is minimal: domain roots gain `f *cmdutil.Factory` and use it to build the client constructor that feeds `cmdutil.InjectClient`. Leaves that render via `cmdutil.View` gain `f` for `f.Output()`. Action-style leaves (built by `cmdutil.ActionCmd`) keep their existing zero-argument constructors.

### Domain root

```go
// internal/cli/docker/docker.go
package docker

import (
    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{Use: "docker", Short: "Docker resources"}
    cmd.AddCommand(
        newContainersCmd(f),
        newNetworksCmd(f),
        newImagesCmd(f),
    )
    return cmd
}
```

```go
// internal/cli/docker/containers.go (group parent)
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

The injected closure is the only difference from today's `buildClient` function — it now closes over `f.HTTPClient` instead of calling the deleted `apiclient.NewHTTPClient`.

### View-rendering leaves

```go
func newListContainersCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{Use: "list", Short: "List containers"}
    device := cmdutil.DeviceFlag(cmd)
    cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
        params := &docker.ListContainersParams{}
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

Two diffs from today:

1. Constructor gains `f *cmdutil.Factory`.
2. `view.Render(...)` call gains `f.Output()` as a second argument (see *cmdutil.View signature change* below).

The body otherwise stays identical: `cmdutil.Client[DockerClient](cmd)` continues to resolve via context-DI, just as it does today.

### Action-style leaves (start/stop/restart)

`cmdutil.ActionCmd[C]` does not render via View and does not need Factory — it writes a literal `<id> <pastTense>` string to `cmd.OutOrStdout()` and reads the client through `cmdutil.Client[C](cmd)`. Action-leaf constructors stay zero-argument:

```go
func newStartContainerCmd() *cobra.Command {
    return cmdutil.ActionCmd("start <container-id>", "Start a container", "started",
        func(c DockerClient, ctx context.Context, id string) (int, []byte, error) {
            r, err := c.StartContainerWithResponse(ctx, id, &docker.StartContainerParams{})
            if err != nil { return 0, nil, err }
            return r.StatusCode(), r.Body, nil
        })
}
```

Inside `cmdutil.ActionCmd`, the only edit is `apiclient.ParseError` → `api.ParseError`.

## `cmdutil.View` signature change

`View.Render` and `View.RenderWith` (and `PolymorphicView.Render`) gain an `outputFmt output.Format` parameter as their second argument. The internal `renderHead` helper drops the implicit read from `flags.GetOutputFormat()`.

```go
// Before
func (v View) Render(w io.Writer, statusCode int, body []byte, data any) error

// After
func (v View) Render(w io.Writer, outputFmt output.Format, statusCode int, body []byte, data any) error
```

Mechanical churn at every call site (the leaf passes `f.Output()`), but no semantic change. The same edit applies to `RenderWith` and `PolymorphicView.Render`.

## Testing (Phase 1)

The existing pattern is preserved end-to-end:

- `StubClient` per domain stays.
- `cmdutil.SetClient[C]` stays as the injection point.
- Per-leaf flag/output assertions stay table-driven and synchronous.

The only change is constructing leaves with a test Factory.

```go
// internal/cli/docker/containers_test.go (Phase 1)
func TestListContainersCmd_tableOutput(t *testing.T) {
    list := docker.ContainerList{Items: []docker.Container{ /* fixture */ }}
    stub := &StubClient{
        ListContainersWithResponseFunc: func(_ context.Context, _ *docker.ListContainersParams, _ ...docker.RequestEditorFn) (*docker.ListContainersResponse, error) {
            return okContainersResp(list), nil
        },
    }

    cmd := newListContainersCmd(testFactory(t))
    cmdutil.SetClient[DockerClient](cmd, stub)
    buf := &bytes.Buffer{}
    cmd.SetOut(buf)
    cmd.SetErr(buf)
    require.NoError(t, cmd.Execute())
    require.Contains(t, buf.String(), "nas-1.homeassistant")
}
```

### `testFactory` helper

A test-only helper builds a Factory with no-network defaults.

```go
// internal/cli/cmdutil/factory_test.go (or testfactory.go gated by _test.go)
func TestFactory(t *testing.T) *Factory {
    t.Helper()
    return &Factory{
        Version:   "test",
        IOStreams: &IOStreams{In: strings.NewReader(""), Out: io.Discard, ErrOut: io.Discard},
        Config:    func() (*config.Config, error) { return &config.Config{}, nil },
        HTTPClient: func() (*http.Client, string, error) {
            return nil, "", errors.New("TestFactory: HTTPClient not configured")
        },
        Output: func() output.Format { return output.FormatTable },
    }
}
```

The default `HTTPClient` returns an error so any test that accidentally triggers real HTTP construction fails loudly. Tests that drive a stub via `SetClient[C]` never hit `HTTPClient`.

Where the helper lives is an implementation detail — exported `cmdutil.TestFactory` for cross-package reuse, or a domain-local `testFactory(t)` helper duplicated per domain. Decide during implementation.

## Codegen reorganization

### Configs

Move `oapi-codegen-<domain>.yaml` from the repo root to a `codegen/` subdirectory:

```
codegen/
  docker.yaml
  network.yaml
  storage.yaml
  system.yaml
```

Each config keeps its current `generate:` shape; only the `output:` path changes to `internal/api/<domain>/api.gen.go`.

### Makefile

```makefile
generate: bundle ## Generate client code from the bundled spec
	@mkdir -p internal/api/system internal/api/docker internal/api/storage internal/api/network
	$(OAPI_CODEGEN) --config codegen/system.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/docker.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/storage.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/network.yaml $(SPEC_FILE)
```

### Import aliases

Inside `internal/cli/docker/`, importing `internal/api/docker` shadows the current package name. Files that need both must alias the API package:

```go
import (
    dockerapi "github.com/bwilczynski/hlctl/internal/api/docker"
)
```

`dockerapi` (typed, descriptive) replaces the current generic `gen` alias. Files that only reference one or the other can drop the alias.

## Compatibility checklist (Phase 1)

Before this refactor is considered complete, all of the following must hold:

- [ ] `make build` produces a binary whose `--help` output is character-identical to the pre-refactor binary at every level of the command tree.
- [ ] All existing tests pass (mechanical edits only — Factory construction, `view.Render` signature).
- [ ] `make lint` (currently `go vet ./...`) is clean.
- [ ] `HOMELAB_API_URL`, `HOMELAB_TOKEN`, `--api-url`, `--output`, `--device`, and the watch flags behave identically.
- [ ] Config file location and format are unchanged.
- [ ] Exit codes and error message formats are unchanged.
- [ ] No package-level `var` of mutable state under `internal/cli/` (verifies via grep).

## Open questions (defer to implementation phase)

1. **Where `testFactory` lives** — exported `cmdutil.TestFactory(t)` (convenient, grows cmdutil's test surface) vs. duplicated per domain (more boilerplate but each domain owns its own seed). Decide during implementation.
2. **Phasing within Phase 1** — whether to land the layout rename + codegen reorg as a separate PR before the Factory rewiring, or as one merge. The layout rename is mostly mechanical and could ship first to shrink the Factory PR's diff.
3. **Whether `pflag.CommandLine` in `main` causes Cobra's auto-generated `--help` text to differ** from today's `PersistentFlags`-only setup. Verify before considering Phase 1 complete.

---

# Follow-up: gh-style Options + runF + httpmock (Phase 2)

Phase 2 is a fully independent refactor, not part of Phase 1. It addresses the one anti-pattern Phase 1 deliberately leaves in place: implicit, context-keyed DI via `cmdutil.InjectClient` / `Client[C]` / `SetClient[C]`. It costs a full test-suite rewrite, which is why it's deferred.

Phase 2 can be adopted:

- **All at once**, once the team is ready to absorb a large test rewrite.
- **Opportunistically per-leaf**, accepting mixed styles during a migration window. Both patterns coexist cleanly because Phase 1 left the context-DI mechanism in place.

## What Phase 2 adds

Three layers, modeled on the GitHub CLI (`cli/cli`):

### 1. Per-leaf `Options` struct

Each leaf command defines a struct that names exactly the inputs it consumes. Function fields for lazy resources (`HTTPClient`, `Config`), value fields for everything else (`IO`, parsed flags).

```go
type listContainersOptions struct {
    HTTPClient func() (*http.Client, string, error)
    IO         *cmdutil.IOStreams
    Output     func() output.Format

    Device string         // --device flag
    Watch  watch.Options  // --watch flags
}
```

### 2. `NewCmdXxx(f, runF)` constructor with a test hook

```go
func newListContainersCmd(f *cmdutil.Factory, runF func(*listContainersOptions) error) *cobra.Command {
    opts := &listContainersOptions{
        HTTPClient: f.HTTPClient,
        IO:         f.IOStreams,
        Output:     f.Output,
    }
    cmd := &cobra.Command{
        Use:   "list",
        Short: "List containers",
        RunE: watch.Wrap(&opts.Watch, func(ctx context.Context, w io.Writer) error {
            if runF != nil {
                return runF(opts)
            }
            return listContainersRun(ctx, w, opts)
        }),
    }
    cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device")
    watch.RegisterFlags(cmd, &opts.Watch)
    return cmd
}

func listContainersRun(ctx context.Context, w io.Writer, opts *listContainersOptions) error {
    httpClient, apiURL, err := opts.HTTPClient()
    if err != nil { return err }
    c, err := NewClient(httpClient, apiURL)
    if err != nil { return err }
    params := &docker.ListContainersParams{}
    if opts.Device != "" { params.Device = &opts.Device }
    resp, err := c.ListContainersWithResponse(ctx, params)
    if err != nil { return err }
    return containersListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
```

Production callers pass `nil` for `runF`; tests pass a closure that intercepts the parsed `Options` without executing business logic.

### 3. HTTP-transport mocking via `cmdutil/httpmock`

A new test-support package modeled on gh's `pkg/httpmock`. Tests build a `Registry` that implements `http.RoundTripper`, register matchers + responders, and inject it as the Factory's `HTTPClient`.

```go
// Minimum surface area
type Registry struct { /* ... */ }
func NewRegistry() *Registry
func (r *Registry) Register(matcher Matcher, responder Responder)
func (r *Registry) RoundTrip(*http.Request) (*http.Response, error)
func (r *Registry) Verify(t *testing.T)
func (r *Registry) Count(spec string) int

func REST(method, pathPattern string) Matcher

func JSONResponse(body any) Responder
func StatusStringResponse(status int, body string) Responder
func StatusJSONResponse(status int, body any) Responder
```

## Phase 2 test pattern (two layers per leaf)

### Layer 1: flag parsing (`TestNewCmdXxx`)

Uses `runF` to intercept the parsed `Options`. Verifies flag and arg parsing without touching the network.

```go
func TestNewCmdListContainers_flags(t *testing.T) {
    var captured *listContainersOptions
    cmd := newListContainersCmd(testFactory(t), func(o *listContainersOptions) error {
        captured = o
        return nil
    })
    cmd.SetArgs([]string{"--device", "nas-1"})
    require.NoError(t, cmd.Execute())
    require.Equal(t, "nas-1", captured.Device)
}
```

### Layer 2: business logic (`TestXxxRun`)

Calls `runXxx` directly with a hand-constructed `Options`. The real generated client runs end-to-end against a fake transport.

```go
func TestListContainersRun(t *testing.T) {
    reg := httpmock.NewRegistry()
    reg.Register(
        httpmock.REST("GET", "/docker/containers"),
        httpmock.JSONResponse(docker.ContainerList{ /* fixture */ }),
    )

    var out bytes.Buffer
    opts := &listContainersOptions{
        IO: &cmdutil.IOStreams{Out: &out, ErrOut: &out},
        HTTPClient: func() (*http.Client, string, error) {
            return &http.Client{Transport: reg}, "https://homelab.test/api/v1", nil
        },
        Output: func() output.Format { return output.FormatTable },
    }
    require.NoError(t, listContainersRun(context.Background(), &out, opts))
    require.Contains(t, out.String(), "nas-1.homeassistant")
}
```

## What Phase 2 deletes

Once Phase 2 ships across every domain:

- `internal/cli/cmdutil/client.go` (`InjectClient`, `Client[C]`, `SetClient[C]`).
- `internal/cli/cmdutil/action.go` (`ActionCmd[C]`). Each action command becomes its own Options + NewCmdXxx + runXxx triple. The 24 lines `ActionCmd` saves don't justify keeping a generic shortcut once the Options pattern is standard.
- Per-domain `stub.go` files (typed-client stubs are replaced by `httpmock` fixtures).

## Phase 2 open questions (revisit when Phase 2 is scheduled)

1. **`watch.Wrap` signature.** Today `watch.Wrap` takes a closure. Phase 2 likely wants `watch.Wrap(*watch.Options, closure)` so flag state lives in Options.
2. **`view.Render` reshape.** Phase 1 already adds `outputFmt` as an argument. Phase 2 may want to bind it once into the View instead — decide based on how leaf code reads after the Options migration.
3. **httpmock surface.** Phase 1's spec defines a minimum; richer matchers (header asserts, body asserts) can be added on demand.
4. **Migration sequencing within Phase 2.** Per-domain rollout is the natural unit. Domains can be migrated in any order; mixed styles coexist via Phase 1's preserved cmdutil context-DI.

## Why split this way

Phase 1 closes the two anti-patterns whose harm is structural and cumulative — package globals, path collisions. Both are cheap to fix once and expensive to live with forever (every new domain pays the global-flag tax and the `gen`-alias tax).

Phase 2 closes the anti-pattern whose harm is local and predictable — context-keyed DI. Its cost is a test rewrite proportional to the number of leaves, and the test rewrite delivers value (two-layer testing, transport-level fixtures) that scales with the test suite's future growth. If `hlctl`'s test count stays modest, Phase 2 may never be worth doing; if test counts grow, the value catches up with the cost.

Either way, Phase 1 is the right floor and stands alone.
