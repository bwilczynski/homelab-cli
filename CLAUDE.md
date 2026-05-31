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
- `internal/cli/flags/` — shared global flags (output format, api-url)
- `internal/cli/cmdutil/` — shared command-construction helpers (client injection, View renderer, ActionCmd factory, DeviceFlag)
- `internal/cli/<domain>/` — per-domain command packages (containers, system, storage, backups, network, config, login)
- `internal/<domain>/` — generated oapi-codegen client code (gitignored)
- `internal/config/` — config file read/write (`~/.config/homelab/`)
- `internal/auth/` — OAuth2 token storage and authenticated HTTP transport
- `internal/output/` — table/JSON output formatting
- `spec/` — git submodule pointing to `homelab-api-spec`
- `oapi-codegen-*.yaml` — per-domain code generation configs

## Adding a New Domain Command

1. Create `internal/cli/<domain>/<domain>.go` with a `NewCmd() *cobra.Command` function.
2. Declare a `cmdutil.View` value at the top of the file for each template:
   ```go
   var fooListView = cmdutil.View{Templates: <domain>Templates, Name: "foo_list.tmpl"}
   ```
   Set `Status:` explicitly on the View only when the endpoint returns something other than 200.
3. Each parent command calls `cmdutil.InjectClient(cmd, buildClient)` after construction; leaf commands have no `client` parameter and call `cmdutil.Client[<Domain>Client](cmd).<Method>(...)` to retrieve it.
4. Leaf commands render with `<view>.Render(w, resp.StatusCode(), resp.Body, resp.JSON200)` — use `cmd.OutOrStdout()` for the writer outside `watch.Wrap`, and the `w` argument passed by `watch.Wrap` inside it.
5. List commands accepting a device filter use `device := cmdutil.DeviceFlag(cmd)`; dereference with `*device`.
6. Start/stop/restart-style commands (204 No Content + success message) use `cmdutil.ActionCmd[<Domain>Client](use, short, pastTense, exec)`.
7. Register the new domain in `internal/cli/root.go` via `rootCmd.AddCommand(<domain>.NewCmd())`.
8. Tests construct leaves directly and seed the client via `cmdutil.SetClient[<Domain>Client](cmd, stub)`.
9. For polymorphic responses (discriminated unions like `NetworkDeviceDetail`, `SystemUpdateDetail`), keep the status check + JSON branch inline (still using `flags.GetOutputFormat`) and call `output.RenderTemplate` directly with the resolved variant's template. `cmdutil.View.Render` cannot dispatch on a discriminator.

## Adding a New oapi-codegen Domain

1. Create `oapi-codegen-<domain>.yaml` with `generate: client: true, models: true`
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
