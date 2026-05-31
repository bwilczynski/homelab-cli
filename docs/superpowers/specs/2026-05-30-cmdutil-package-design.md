# Design: `cmdutil` package — reduce CLI command boilerplate

**Date:** 2026-05-30
**Scope:** New `internal/cli/cmdutil/` package + migration of `docker`, `storage`, `network`, `system` domains

---

## Problem

Every leaf command across the four CLI domains repeats the same shapes:

1. **Client construction (~6 lines/command).** Each leaf takes a `client DomainClient` parameter that may be `nil`; if nil, the command calls a domain-local `buildClient()`. This exists to make tests injectable but bloats every `RunE`.
2. **Response handling (~6 lines/command).** Status-code check → `apiclient.ParseError` on failure → branch on `--output` to print raw JSON or render a template. Identical at every callsite that returns a `200`.
3. **`--device` flag (~2 lines/list command).** Same flag, same description, same conditional `if device != ""` block populating the params struct.
4. **Action commands (~24 lines × 3).** `start`/`stop`/`restart` differ only in the API method called, the success status code (always 204), and the verb in the "Container X started" message.

`internal/cli/docker/docker.go` is 376 lines; roughly 250 of those are mechanical boilerplate.

---

## Approach

Add a small `cmdutil` package with four focused helpers. No DSL, no codegen, no command-builder framework — just functions and one struct, each replacing a specific repeated shape. Domains remain Cobra-native; only the boilerplate disappears.

---

## Design

### 1. Package layout

```
internal/cli/cmdutil/
  client.go    // InjectClient[C], Client[C], SetClient[C]
  view.go      // View, View.Render
  action.go    // ActionCmd[C]
  flags.go     // DeviceFlag
```

Target size: ~100 lines total across the package.

### 2. Client injection (`client.go`)

Client construction moves from each leaf's `RunE` to the domain's parent command via `PersistentPreRunE`. Cobra runs `PersistentPreRunE` after flags are parsed and only when an actual subcommand executes (not on `--help`/`--version`), so flag-dependent construction works and idle invocations stay free of disk I/O.

```go
package cmdutil

import (
    "context"
    "github.com/spf13/cobra"
)

type clientKey[C any] struct{}

// InjectClient registers a PersistentPreRunE on cmd that builds a client and
// stores it on the command's context. Leaf commands retrieve it via Client[C].
func InjectClient[C any](cmd *cobra.Command, build func() (C, error)) {
    cmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
        c, err := build()
        if err != nil {
            return err
        }
        cmd.SetContext(context.WithValue(cmd.Context(), clientKey[C]{}, c))
        return nil
    }
}

// Client returns the client previously injected for type C.
// Panics if no client is set — callers should always run under InjectClient.
func Client[C any](cmd *cobra.Command) C {
    return cmd.Context().Value(clientKey[C]{}).(C)
}

// SetClient seeds a client on cmd's context. Intended for tests.
// Preserves whatever context is already on cmd (cobra's default is context.Background).
func SetClient[C any](cmd *cobra.Command, c C) {
    cmd.SetContext(context.WithValue(cmd.Context(), clientKey[C]{}, c))
}
```

The generic `clientKey[C]` makes one helper work for every domain: `clientKey[DockerClient]` and `clientKey[StorageClient]` are distinct types in Go's type system, so values stored under them never collide.

Each domain keeps its own `buildClient()` (it alone knows which concrete client to construct); `InjectClient` just wires it in.

### 3. View + Render (`view.go`)

The repeating response-handling shape is replaced by named `View` values declared once at the top of each domain. Each `View` binds the templates `fs.FS` and the template name, so callsites pass only the response triplet.

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

// Render handles the standard response→output flow:
//   - status != v.Status (or 200 if unset) → apiclient.ParseError
//   - --output=json → write raw body
//   - otherwise → render the bound template against data
func (v View) Render(w io.Writer, statusCode int, body []byte, data any) error {
    expected := v.Status
    if expected == 0 {
        expected = http.StatusOK
    }
    if statusCode != expected {
        return apiclient.ParseError(statusCode, body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(w, string(body))
        return nil
    }
    return output.RenderTemplate(w, v.Templates, v.Name, data)
}
```

Note on argument count: the three `resp.*` arguments (`StatusCode()`, `Body`, `JSON200`) cannot be collapsed into one. oapi-codegen exposes `StatusCode()` as a method but `Body` and `JSON200` as fields, and Go interfaces cannot express field access. Reflection or per-call wrappers would reduce the count further but at a worse cost-to-clarity ratio than four explicit arguments.

### 4. ActionCmd (`action.go`)

Container `start`/`stop`/`restart` collapse to a single factory. The `exec` callback receives the typed client (via `Client[C]`) and returns the status code + body so the helper can decide between "success message" and `ParseError`.

```go
package cmdutil

import (
    "context"
    "fmt"
    "net/http"

    "github.com/bwilczynski/hlctl/internal/apiclient"
    "github.com/spf13/cobra"
)

// ActionCmd builds a Cobra command of the form `<verb> <id>` that calls exec
// with the resolved client, asserts a 204 No Content response, and prints
// "<id> <pastTense>" on success.
func ActionCmd[C any](use, short, pastTense string, exec func(c C, ctx context.Context, id string) (int, []byte, error)) *cobra.Command {
    return &cobra.Command{
        Use:   use,
        Short: short,
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            code, body, err := exec(Client[C](cmd), cmd.Context(), args[0])
            if err != nil {
                return err
            }
            if code != http.StatusNoContent {
                return apiclient.ParseError(code, body)
            }
            fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", args[0], pastTense)
            return nil
        },
    }
}
```

### 5. DeviceFlag (`flags.go`)

```go
package cmdutil

import "github.com/spf13/cobra"

// DeviceFlag registers --device on cmd and returns a pointer to the bound value.
func DeviceFlag(cmd *cobra.Command) *string {
    return cmd.Flags().String("device", "", "Filter by device ID")
}
```

---

## What a domain looks like after

```go
// internal/cli/docker/docker.go

package docker

import (
    "context"
    "io"

    "github.com/bwilczynski/hlctl/internal/apiclient"
    "github.com/bwilczynski/hlctl/internal/cli/cmdutil"
    "github.com/bwilczynski/hlctl/internal/cli/watch"
    gen "github.com/bwilczynski/hlctl/internal/docker"
    "github.com/spf13/cobra"
)

var (
    containersListView = cmdutil.View{Templates: dockerTemplates, Name: "containers_list.tmpl"}
    containersGetView  = cmdutil.View{Templates: dockerTemplates, Name: "containers_get.tmpl"}
    networksListView   = cmdutil.View{Templates: dockerTemplates, Name: "networks_list.tmpl"}
    networksGetView    = cmdutil.View{Templates: dockerTemplates, Name: "networks_get.tmpl"}
    imagesListView     = cmdutil.View{Templates: dockerTemplates, Name: "images_list.tmpl"}
    imagesGetView      = cmdutil.View{Templates: dockerTemplates, Name: "images_get.tmpl"}
)

func NewCmd() *cobra.Command {
    cmd := &cobra.Command{Use: "docker", Short: "Docker resources"}
    cmd.AddCommand(newContainersCmd(), newNetworksCmd(), newImagesCmd())
    return cmd
}

func buildClient() (DockerClient, error) {
    httpClient, apiURL, err := apiclient.NewHTTPClient()
    if err != nil {
        return nil, err
    }
    return NewDockerClient(httpClient, apiURL)
}

func newContainersCmd() *cobra.Command {
    cmd := &cobra.Command{Use: "containers", Short: "Manage Docker containers"}
    cmdutil.InjectClient(cmd, buildClient)
    cmd.AddCommand(newListCmd(), newGetCmd(), newStartCmd(), newStopCmd(), newRestartCmd())
    return cmd
}

func newStartCmd() *cobra.Command {
    return cmdutil.ActionCmd("start <container-id>", "Start a container", "started",
        func(c DockerClient, ctx context.Context, id string) (int, []byte, error) {
            r, err := c.StartContainerWithResponse(ctx, id, &gen.StartContainerParams{})
            if err != nil {
                return 0, nil, err
            }
            return r.StatusCode(), r.Body, nil
        })
}

// newStopCmd and newRestartCmd follow the same shape, swapping the API call and verb.

func newListCmd() *cobra.Command {
    cmd := &cobra.Command{Use: "list", Short: "List containers"}
    device := cmdutil.DeviceFlag(cmd)
    cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
        params := &gen.ListContainersParams{}
        if *device != "" {
            params.Device = device
        }
        resp, err := cmdutil.Client[DockerClient](cmd).ListContainersWithResponse(ctx, params)
        if err != nil {
            return err
        }
        return containersListView.Render(w, resp.StatusCode(), resp.Body, resp.JSON200)
    })
    watch.RegisterFlags(cmd)
    return cmd
}
```

Estimated `docker.go` size after migration: ~130 lines (down from 376).

---

## Tests

Leaf constructors no longer take a `client` parameter. Tests seed the client on the command's context with `cmdutil.SetClient` before calling `Execute`:

```go
// Before
cmd := newListCmd(stub)

// After
cmd := newListCmd()
cmdutil.SetClient[DockerClient](cmd, stub)
```

The `StubClient` types and `okFooResp` / `errFooResp` fixtures in each `*_test.go` are unchanged. Only the cmd construction call needs adjustment.

`SetClient` layers the client value onto whatever context cmd already has (Cobra defaults to `context.Background()` when none is set), mirroring `InjectClient`'s preserve-and-extend behavior. Tests that need a custom context can `cmd.SetContext(ctx)` before calling `SetClient`.

---

## Migration order

Each step is a self-contained change that compiles and tests green.

1. **Add `cmdutil` package** with the four helpers and unit tests for each.
2. **Migrate `docker`** (largest, most variety — validates all four helpers including `ActionCmd`).
3. **Migrate `storage`** (list + get only — validates the simpler path).
4. **Migrate `network`** (multiple parent commands: devices, vlans, ssids, wans).
5. **Migrate `system`** (smallest).
6. **Update `CLAUDE.md`** "Adding a New Domain Command" section to reference the new helpers.

---

## What is not changing

- Generated oapi-codegen client code (`internal/<domain>/`).
- `apiclient` package (`NewHTTPClient`, `ParseError`).
- `output` package (`RenderTemplate`, formats).
- `flags` package (global `--output`, `--api-url`).
- `watch` package (continues to wrap `RunE` of list commands).
- Per-domain `buildClient()` functions (unchanged; just called from `InjectClient` instead of inline).
- Per-domain template `embed.FS` variables.
- Test stub clients and response fixtures.

---

## Out of scope

- Eliminating the writer (`w io.Writer`) argument from `View.Render` — would require coordinating with `watch.Wrap` to redirect `cmd.OutOrStdout()`. Separate concern.
- Generic helpers for list parameters beyond `--device`. The `--device` flag is the only one repeated 5+ times today; broader flag abstractions await more evidence of duplication.
- Consolidating per-domain `buildClient()` into a shared factory. Each domain currently constructs a different generated client type; sharing requires either generics over the constructor or a registry, neither warranted by current evidence.
