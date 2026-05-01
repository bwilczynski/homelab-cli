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
- `internal/cli/<domain>/` — per-domain command packages (containers, system, storage, backups, network, config, login)
- `internal/<domain>/` — generated oapi-codegen client code (gitignored)
- `internal/config/` — config file read/write (`~/.config/homelab/`)
- `internal/auth/` — OAuth2 token storage and authenticated HTTP transport
- `internal/output/` — table/JSON output formatting
- `spec/` — git submodule pointing to `homelab-api-spec`
- `oapi-codegen-*.yaml` — per-domain code generation configs

## Adding a New Domain Command

1. Create `internal/cli/<domain>/<domain>.go` with a `NewCmd() *cobra.Command` function
2. Register it in `internal/cli/root.go` via `rootCmd.AddCommand(<domain>.NewCmd())`
3. Each command uses `output.Print(flags.GetOutputFormat(), data, headers, rows)` for output
4. Import `flags` from `github.com/bwilczynski/hlctl/internal/cli/flags` (not `cli` directly — avoids import cycle)
5. List commands accept `--device` flag where the API supports filtering
6. Detail commands use `cobra.ExactArgs(1)` for the resource ID

## Adding a New oapi-codegen Domain

1. Create `oapi-codegen-<domain>.yaml` with `generate: client: true, models: true`
2. Add the generation line to the `Makefile` `generate` target
3. Run `make generate`

## Conventions

- Config location: `~/.config/homelab/`
- Env vars: `HOMELAB_API_URL`, `HOMELAB_TOKEN`
- Command structure: `hlctl <domain> <action> [args] [flags]`
- Global flags: `--output/-o` (table|json), `--api-url`
- Generated files (`api.gen.go`) are gitignored — run `make generate` after clone
