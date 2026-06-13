# CLAUDE.md

## Overview

`hlctl` is a Go CLI for managing homelab infrastructure via the Homelab API. It uses Cobra for command structure and oapi-codegen for API client generation.

## Build & Run

```sh
# First time: initialize the spec submodule
git submodule update --init

# Generate API client code from the OpenAPI spec
make generate

# Build the binary
make build

# Run
./bin/hlctl --help
```

## Key Commands

- `make generate` — regenerate API client code from the spec submodule
- `make build` — build `bin/hlctl`
- `make lint` — run `go vet`
- `make tidy` — run `go mod tidy`

## Project Structure

- `cmd/hlctl/` — entrypoint
- `internal/cli/` — Cobra command tree, root command
- `internal/cli/cmdutil/` — shared command-construction helpers (client injection, View renderer, ActionCmd factory, DeviceFlag, Factory, IOStreams, TestFactory)
- `internal/cli/<domain>/` — per-domain command packages (containers, system, storage, backups, network, config, login)
- `internal/api/<domain>/` — generated oapi-codegen client code (gitignored)
- `internal/config/` — config file read/write (`~/.config/homelab/`)
- `internal/auth/` — OAuth2 token storage and authenticated HTTP transport
- `internal/output/` — table/JSON output formatting
- `spec/` — git submodule pointing to `homelab-api-spec`
- `codegen/<domain>.yaml` — per-domain code generation configs

## Adding a New Domain Command

1. Create `internal/cli/<domain>/<domain>.go` with a `NewCmd(f *cmdutil.Factory) *cobra.Command` function.
2. Declare a `cmdutil.View` value at the top of the file for each template:
   ```go
   var fooListView = cmdutil.View{Templates: <domain>Templates, Name: "foo_list.tmpl"}
   ```
   Set `Status:` explicitly on the View only when the endpoint returns something other than 200.
3. Exactly one ancestor per leaf path calls `cmdutil.InjectClient(cmd, buildClient)` where `buildClient` closes over `f` and calls `f.HTTPClient()` — Cobra runs the closest `PersistentPreRunE` only, so additional calls on intermediate parents are dead. Put it on the domain root when all leaves share one client (`network`, `system`); put it on each sub-group parent when only some sub-trees need it (`docker`, `storage`). Leaf commands have no `client` parameter and call `cmdutil.Client[<Domain>Client](cmd).<Method>(...)` to retrieve it.
4. Leaf commands render with `<view>.Render(w, f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)` — use `cmd.OutOrStdout()` for the writer outside `watch.Wrap`, and the `w` argument passed by `watch.Wrap` inside it.
5. List commands accepting a device filter use `device := cmdutil.DeviceFlag(cmd)`; dereference with `*device`.
6. Start/stop/restart-style commands (204 No Content + success message) use `cmdutil.ActionCmd[<Domain>Client](use, short, pastTense, exec)`.
7. Register the new domain in `internal/cli/root.go` by adding `<domain>.NewCmd(f)` to the `root.AddCommand(...)` call inside `NewRootCmd(f)`.
8. Tests construct leaves directly using `cmdutil.TestFactory(t)` and seed the client via `cmdutil.SetClient[<Domain>Client](cmd, stub)`.
9. For polymorphic responses (discriminated unions like `NetworkDeviceDetail`, `SystemUpdateDetail`), declare a `cmdutil.PolymorphicView[<UnionType>]` instead of a `View`. Its `Variants` map is keyed by the discriminator string returned by `T.Discriminator()`; each `Variant` binds a template name to a `Resolve func(T) (any, error)` that calls the appropriate `As<Variant>()` accessor (and optionally transforms the result). Render with `view.Render(w, f.Output(), resp.StatusCode(), resp.Body, resp.JSON200)` — same call shape as `View.Render`. When a variant resolver depends on per-call state (e.g. a flag), construct the `PolymorphicView` inside `RunE` so the resolver can close over it.
10. When template data must be derived from the response body (e.g. row structs with formatted bytes/uptime), use `view.RenderWith(w, f.Output(), resp.StatusCode(), resp.Body, fn)` instead of `view.Render`. `fn` is invoked only in table mode, so derivation work is skipped when `--output=json`.

## Adding a New oapi-codegen Domain

1. Create `codegen/<domain>.yaml` with `generate: client: true, models: true`; set `output:` to `internal/api/<domain>/api.gen.go`
2. Add the generation line to the `Makefile` `generate` target
3. Run `make generate`

## Git

- Never add `Co-Authored-By` trailers to commit messages.

## Conventions

- Config location: `~/.config/homelab/`
- Env vars: `HOMELAB_API_URL`, `HOMELAB_TOKEN`
- Command structure: `hlctl <domain> <action> [args] [flags]`
- Global flags: `--output/-o` (table|json), `--api-url`
- Generated files (`api.gen.go`) are gitignored — run `make generate` after clone
