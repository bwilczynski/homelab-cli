# CLI Refactor: gh-style Factory + per-command Options

## Status

**Designed, implementation deferred.** This document captures the agreed end-state. The migration plan will be drafted in a separate session when work is scheduled.

## Goal

Restructure the `hlctl` source tree to match the patterns proven by the GitHub CLI (`cli/cli`). End state: explicit dependency injection via a Factory, per-command `Options` structs, two-layer testing (flag parsing + business logic), and no package-level mutable state.

## Non-goals

- **No user-visible behavior changes.** Every command, flag, env var, exit code, and output format stays identical. A built binary before/after the refactor is behaviorally indistinguishable to users.
- **No new domains, no new commands, no new features.**
- **No template or rendering changes.** `cmdutil.View` / `cmdutil.PolymorphicView` and the embedded `*.tmpl` files are preserved as-is.
- **No OpenAPI spec or generated-API contract changes.** Only the location and import path of the generated code move.

## Motivation

The current layout has accumulated three patterns that are at odds with idiomatic Go CLI conventions:

1. **Package-level mutable globals in `internal/cli/flags/`** (`OutputFormat`, `APIURL`). Cobra writes into them via `StringVarP`, and arbitrary code reads from them. This blocks parallel testing, hides dependencies, and couples `internal/apiclient` to a singleton flag namespace.
2. **Implicit, context-based DI via `cmdutil.InjectClient` / `Client[C]` / `SetClient[C]`.** Clients are stashed on `cmd.Context()` keyed by their type. The mechanism is invisible at the call site — readers can't tell where a leaf's client comes from without reading the cmdutil helpers.
3. **Path collision between generated code (`internal/<domain>/`) and command code (`internal/cli/<domain>/`).** Every leaf imports the generated package with a generic `gen` alias to dodge the shadowing.

The gh CLI solves all three with a single coherent pattern. Adopting it brings the codebase in line with the largest open-source Go CLI and unlocks two-layer testing (flag parsing isolated from network behavior).

## Final directory tree

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
│       │   ├── httpmock/        # NEW: http.RoundTripper-based test registry
│       │   ├── view.go          # unchanged in spirit (calls api.ParseError now)
│       │   ├── flags.go         # unchanged (DeviceFlag)
│       │   └── *_test.go
│       ├── watch/               # unchanged
│       ├── auth/
│       │   └── auth.go          # NewCmd(f *cmdutil.Factory) *cobra.Command
│       ├── config/
│       │   └── config.go        # NewCmd(f *cmdutil.Factory)
│       ├── docker/
│       │   ├── docker.go        # NewCmd(f *cmdutil.Factory)
│       │   ├── client.go        # DockerClient interface + NewClient(httpClient, apiURL)
│       │   ├── containers.go    # leaf: opts struct + NewCmdXxx + runXxx
│       │   ├── containers_test.go
│       │   ├── networks.go
│       │   ├── networks_test.go
│       │   ├── images.go
│       │   ├── images_test.go
│       │   └── templates.go
│       ├── network/             # same shape as docker/
│       ├── storage/             # same shape
│       └── system/              # same shape
└── Makefile, go.mod, README.md, .goreleaser.yaml, .github/  (unchanged at root)
```

### Removed paths

- `internal/cli/flags/` — package deleted. Replaced by Factory fields/methods.
- `internal/apiclient/` — package deleted. `NewHTTPClient` absorbed by Factory; `errors.go` moved to `internal/api/errors.go`.
- `internal/<domain>/` (top-level generated dirs) — moved under `internal/api/`.
- `internal/cli/cmdutil/client.go` — `InjectClient`, `Client[C]`, `SetClient[C]` deleted.
- `internal/cli/cmdutil/action.go` (`ActionCmd[C]`) — deleted. Each leaf action command becomes an explicit `Options`/`NewCmdXxx`/`runXxx` triple. The 24 lines `ActionCmd` saves don't justify keeping a generic shortcut once the Options pattern is standard.
- `internal/cli/<domain>/stub.go` files — generic typed-client stubs removed. Tests stub at the HTTP transport layer via `cmdutil/httpmock` instead.

### Preserved paths

- `cmd/hlctl/main.go` — thin entrypoint pattern is already idiomatic.
- `internal/auth/`, `internal/config/`, `internal/output/` — no CLI knowledge, no changes.
- `internal/cli/watch/` — watch loop logic is independent of DI shape.
- `internal/cli/cmdutil/view.go` — `View`, `PolymorphicView`, render helpers. Only their import of `apiclient.ParseError` becomes `api.ParseError`.
- `internal/cli/cmdutil/flags.go` — `DeviceFlag` helper unchanged.
- Per-domain `templates.go` and embedded template trees — unchanged.
- Per-domain `client.go` files exporting the typed interface + `NewClient(httpClient, apiURL)` constructor.

## Factory

`Factory` bundles the building blocks every command needs. Constructed once in `main`, threaded through every `NewCmd` and `NewCmdXxx`. Function-valued fields defer expensive work (config load, token read, URL resolution) until a command actually runs.

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

## Leaf command pattern (gh-style)

Every leaf command is three things in the same file: an `Options` struct, a `NewCmdXxx` constructor with a `runF` test hook, and a `runXxx` function that owns the business logic.

### Anatomy

```go
// internal/cli/docker/containers.go
package docker

import (
    "context"
    "io"
    "net/http"

    "github.com/bwilczynski/hlctl/internal/api/docker"
    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/bwilczynski/hlctl/internal/cli/watch"
    "github.com/bwilczynski/hlctl/internal/output"
    "github.com/spf13/cobra"
)

var containersListView = cmdutil.View{Templates: dockerTemplates, Name: "containers_list.tmpl"}

// listContainersOptions captures everything the list command consumes.
// Function fields are injected from the Factory in NewCmdListContainers
// and overridden by tests.
type listContainersOptions struct {
    HTTPClient func() (*http.Client, string, error)
    IO         *cmdutil.IOStreams
    Output     func() output.Format

    Device string         // --device flag
    Watch  watch.Options  // --watch flags
}

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
    if err != nil {
        return err
    }
    c, err := NewClient(httpClient, apiURL)
    if err != nil {
        return err
    }
    params := &docker.ListContainersParams{}
    if opts.Device != "" {
        params.Device = &opts.Device
    }
    resp, err := c.ListContainersWithResponse(ctx, params)
    if err != nil {
        return err
    }
    return containersListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
```

### Rules of the pattern

1. **One `xxxOptions` struct per leaf.** Names the exact fields the leaf consumes. Function fields for lazy resources (`HTTPClient`, `Config`), value fields for everything else (`IO`, parsed flags).
2. **`NewCmdXxx(f *cmdutil.Factory, runF func(*xxxOptions) error)` signature is uniform across all leaves.** `runF` is the test hook: production callers pass `nil`, tests pass a closure that captures `opts`.
3. **`runXxx(ctx, w, opts)` owns the business logic.** No `*cobra.Command`, no flag parsing. Takes everything it needs through `opts` and explicit parameters (`ctx`, `w` for the writer when the leaf uses `watch.Wrap`; otherwise `opts.IO.Out`).
4. **Constructors named `newXxxCmd`** (unexported) for leaves under a domain group; the domain root is exported `NewCmd(f)`.
5. **Each domain's `client.go` defines a `DockerClient` (etc.) interface and `NewClient(httpClient, apiURL) (DockerClient, error)`.** The interface stays for typing; the test seam shifts to the HTTP transport, not the interface.

### Domain root constructor

```go
// internal/cli/docker/docker.go
func NewCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{Use: "docker", Short: "Docker resources"}
    cmd.AddCommand(
        newContainersCmd(f),
        newNetworksCmd(f),
        newImagesCmd(f),
    )
    return cmd
}

// internal/cli/docker/containers.go (group parent for the containers sub-tree)
func newContainersCmd(f *cmdutil.Factory) *cobra.Command {
    cmd := &cobra.Command{Use: "containers", Short: "Manage Docker containers"}
    cmd.AddCommand(
        newListContainersCmd(f, nil),
        newGetContainerCmd(f, nil),
        newStartContainerCmd(f, nil),
        newStopContainerCmd(f, nil),
        newRestartContainerCmd(f, nil),
    )
    return cmd
}
```

Group parents never need `runF` since they don't have `RunE`.

### Action commands (start/stop/restart)

The current `cmdutil.ActionCmd[C]` shortcut is dropped. Each action becomes its own Options + NewCmdXxx + runXxx triple. The trio costs ~25 lines per command; with three action verbs per resource (start/stop/restart) the duplication is real but acceptable for the explicitness it buys. If duplication becomes a real cost, a per-domain helper (not a generic one) can be added later — but it's out of scope for the refactor.

```go
type startContainerOptions struct {
    HTTPClient func() (*http.Client, string, error)
    IO         *cmdutil.IOStreams
    ID         string
}

func newStartContainerCmd(f *cmdutil.Factory, runF func(*startContainerOptions) error) *cobra.Command {
    opts := &startContainerOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams}
    cmd := &cobra.Command{
        Use:   "start <container-id>",
        Short: "Start a container",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            opts.ID = args[0]
            if runF != nil {
                return runF(opts)
            }
            return startContainerRun(cmd.Context(), opts)
        },
    }
    return cmd
}

func startContainerRun(ctx context.Context, opts *startContainerOptions) error {
    httpClient, apiURL, err := opts.HTTPClient()
    if err != nil {
        return err
    }
    c, err := NewClient(httpClient, apiURL)
    if err != nil {
        return err
    }
    resp, err := c.StartContainerWithResponse(ctx, opts.ID, &docker.StartContainerParams{})
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusNoContent {
        return api.ParseError(resp.StatusCode(), resp.Body)
    }
    fmt.Fprintf(opts.IO.Out, "%s %s\n", opts.ID, "started")
    return nil
}
```

## Testing

Two test layers per leaf, each with one purpose.

### Layer 1: flag parsing (`TestNewCmdXxx`)

Uses the `runF` hook to intercept the parsed `Options` without executing business logic. Verifies that CLI flags and positional args map onto the correct `Options` fields with the correct types and defaults. Never touches the network or the file system.

```go
func TestNewCmdListContainers_flags(t *testing.T) {
    cases := []struct {
        name string
        args []string
        want listContainersOptions
    }{
        {"defaults", nil, listContainersOptions{}},
        {"device filter", []string{"--device", "nas-1"}, listContainersOptions{Device: "nas-1"}},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            var captured *listContainersOptions
            cmd := newListContainersCmd(testFactory(t), func(o *listContainersOptions) error {
                captured = o
                return nil
            })
            cmd.SetArgs(tc.args)
            require.NoError(t, cmd.Execute())
            require.Equal(t, tc.want.Device, captured.Device)
        })
    }
}
```

### Layer 2: business logic (`TestXxxRun`)

Calls `runXxx` directly with a hand-constructed `Options`. The HTTP client is backed by an `httpmock.Registry` (a `http.RoundTripper`) that matches outgoing requests against canned responses. The real generated client runs end-to-end against the fake transport, exercising URL building, request serialization, and response parsing.

```go
func TestListContainersRun(t *testing.T) {
    reg := httpmock.NewRegistry()
    reg.Register(
        httpmock.REST("GET", "/docker/containers"),
        httpmock.JSONResponse(docker.ContainerList{Items: []docker.Container{
            {Id: "nas-1.homeassistant", Image: "homeassistant/home-assistant:latest", Status: docker.Running},
        }}),
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
    require.Equal(t, 1, reg.Count("GET /docker/containers"))
}
```

### `cmdutil/httpmock`

A new test-support package modeled on gh's `pkg/httpmock`. The exact API is deferred to the implementation plan, but the minimum surface area:

```go
// internal/cli/cmdutil/httpmock/httpmock.go
type Registry struct { /* ... */ }
func NewRegistry() *Registry
func (r *Registry) Register(matcher Matcher, responder Responder)
func (r *Registry) RoundTrip(*http.Request) (*http.Response, error)   // implements http.RoundTripper
func (r *Registry) Verify(t *testing.T)                                // assert all matchers consumed
func (r *Registry) Count(spec string) int

// Matchers
func REST(method, pathPattern string) Matcher
func GraphQL(opNamePattern string) Matcher    // omitted for now; the API has no GraphQL

// Responders
func JSONResponse(body any) Responder
func StatusStringResponse(status int, body string) Responder
func StatusJSONResponse(status int, body any) Responder
```

The registry lives under `cmdutil/` because it's a test-construction helper, not an implementation detail of any one domain.

### `testFactory` helper

```go
// internal/cli/cmdutil/factory.go (or a _test.go in cmdutil/)
func TestFactory(t *testing.T) *Factory {
    t.Helper()
    return &Factory{
        Version:   "test",
        IOStreams: &IOStreams{In: strings.NewReader(""), Out: io.Discard, ErrOut: io.Discard},
        Config:    func() (*config.Config, error) { return &config.Config{}, nil },
        HTTPClient: func() (*http.Client, string, error) {
            return nil, "", errors.New("TestFactory: HTTPClient not configured — set opts.HTTPClient in your test")
        },
        Output: func() output.Format { return output.FormatTable },
    }
}
```

The default `HTTPClient` returns an error so any layer-1 test that accidentally executes business logic fails loudly.

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

Each config keeps its current `generate:` and `output:` shape; only the `output:` path changes to `internal/api/<domain>/api.gen.go`.

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

## `cmdutil.View` and `PolymorphicView`

Unchanged in spirit. Two micro-edits:

1. `apiclient.ParseError(statusCode, body)` becomes `api.ParseError(statusCode, body)` after the package rename.
2. `flags.GetOutputFormat()` becomes a call into the Factory: the View now receives `outputFmt output.Format` as a constructor parameter, or a `Render(...)`-time argument. Either form removes the implicit global read. Preferred: per-render argument, since it keeps `View` a value type with no hidden inputs.

```go
// Before
func (v View) Render(w io.Writer, statusCode int, body []byte, data any) error {
    // ... reads flags.GetOutputFormat() inside renderHead
}

// After
func (v View) Render(w io.Writer, outputFmt output.Format, statusCode int, body []byte, data any) error {
    // ... uses outputFmt
}
```

Every leaf already has `opts.Output()` available, so each `view.Render(...)` call becomes `view.Render(w, opts.Output(), ...)`. Mechanical churn but no semantic change.

## What every domain file looks like after the refactor

For one resource (`containers`) under one domain (`docker`):

- `internal/cli/docker/docker.go` — `NewCmd(f *cmdutil.Factory)` (group parent, 10 lines)
- `internal/cli/docker/client.go` — `DockerClient` interface + `NewClient(...)` (unchanged structure)
- `internal/cli/docker/containers.go` — group parent `newContainersCmd(f)` + the five leaf triples (Options + NewCmdXxx + runXxx) for list/get/start/stop/restart
- `internal/cli/docker/containers_test.go` — `TestNewCmd<Verb>Container_flags` (one per leaf, table-driven) + `Test<Verb>ContainerRun` (one per leaf) using `httpmock`
- `internal/cli/docker/templates.go` — unchanged (embedded templates)

No `stub.go`. No `cmdutil.SetClient` call. No package vars.

## Open questions (defer to implementation phase)

1. **`watch.Wrap` signature.** Today `watch.Wrap` takes a closure returning `(ctx, w)`. With Options, it likely should take `*watch.Options` directly (so flag state is captured in Options). Confirm during implementation.
2. **`view.Render` signature.** Pass `output.Format` as a `Render`-time argument vs. embed it in a `View.Bind(opts)` builder. Decide during implementation; the per-render argument is the proposed default.
3. **Where `testFactory` lives.** Exported `cmdutil.TestFactory(t)` is convenient but means cmdutil grows test-only API. Alternative: each domain defines its own helper. Decide during implementation.
4. **`httpmock` package surface.** Matchers and responders should be designed once and reused. The minimum surface listed above is sufficient; richer matchers (header asserts, body asserts) can be added on demand.
5. **Migration sequencing.** Will be defined when the implementation plan is drafted. Plausible order: introduce Factory + cmdutil scaffolding, then migrate one domain end-to-end as a proof, then the rest, then delete `internal/cli/flags/` and `internal/apiclient/`.

## Compatibility checklist

Before this refactor is considered complete, all of the following must hold:

- [ ] `make build` produces a binary whose `--help` output is character-identical to the pre-refactor binary at every level of the command tree.
- [ ] All existing tests pass, rewritten to the new pattern.
- [ ] `make lint` (currently `go vet ./...`) is clean.
- [ ] `HOMELAB_API_URL`, `HOMELAB_TOKEN`, `--api-url`, `--output`, `--device`, and the watch flags behave identically.
- [ ] Config file location and format are unchanged.
- [ ] Exit codes and error message formats are unchanged.
- [ ] No package-level `var` of mutable state introduced anywhere under `internal/cli/`.
