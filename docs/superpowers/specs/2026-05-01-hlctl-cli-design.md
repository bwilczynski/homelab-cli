# hlctl CLI Design Spec

## Overview

`hlctl` is a Go CLI application for controlling a homelab via the Homelab API. It provides a command-line interface to manage containers, system health, storage, backups, and network devices. The API spec lives in a separate repo (`bwilczynski/homelab-api-spec`) and client code is generated from it using oapi-codegen.

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Language | Go | Matches ecosystem (kubectl, Docker); single binary |
| CLI framework | Cobra | Standard for Go CLIs, modular command tree |
| Code generation | oapi-codegen | Go-native, idiomatic output, lightweight |
| Spec management | Git submodule | Mirrors homelab-api approach; single source of truth |
| Binary name | `hlctl` | Short, follows `*ctl` convention |
| Config location | `~/.config/homelab/` | Domain-based naming (like kubectl uses `~/.kube/`) |
| Env var prefix | `HOMELAB_` | Matches config location convention |
| Command structure | `hlctl <domain> <action>` | Two-level, scales well across domains |
| Auth flow | OAuth2 client-credentials | Matches API spec; env var override for CI |

## Project Structure

```
hlctl/
├── cmd/hlctl/main.go
├── spec/                              # git submodule → homelab-api-spec
├── oapi-codegen-containers.yaml       # per-domain codegen configs (client mode)
├── oapi-codegen-system.yaml
├── oapi-codegen-storage.yaml
├── oapi-codegen-backups.yaml
├── oapi-codegen-network.yaml
├── internal/
│   ├── cli/
│   │   ├── root.go                    # root command, global flags (--output, --api-url)
│   │   ├── config/                    # hlctl config set-url, hlctl config show
│   │   ├── login/                     # hlctl login
│   │   ├── containers/               # hlctl containers list|get|start|stop|restart
│   │   ├── system/                    # hlctl system health|info|utilization|updates|update|check-updates
│   │   ├── storage/                   # hlctl storage volumes|volume
│   │   ├── backups/                   # hlctl backups tasks|task
│   │   └── network/                   # hlctl network devices|device|clients|client
│   ├── containers/api.gen.go          # generated client per domain
│   ├── system/api.gen.go
│   ├── storage/api.gen.go
│   ├── backups/api.gen.go
│   ├── network/api.gen.go
│   ├── auth/                          # OAuth2 token flow + storage
│   ├── config/                        # config file read/write
│   └── output/                        # table/JSON output formatting
├── go.mod
├── go.sum
├── Makefile
├── CLAUDE.md
└── README.md
```

## Authentication & Config

### Config file

Location: `~/.config/homelab/config.yaml`

```yaml
api_url: https://homelab.local/api
```

### Credentials file

Location: `~/.config/homelab/credentials.json`

```json
{
  "access_token": "...",
  "token_type": "bearer",
  "expires_at": "2026-05-01T15:00:00Z"
}
```

### Auth flow

- `hlctl login` prompts for client ID + secret, calls the IdP token endpoint, stores the result.
- On each request, the `auth` package reads credentials, checks expiry, and attaches the `Authorization: Bearer` header.
- `HOMELAB_TOKEN` env var bypasses stored credentials entirely.
- `HOMELAB_API_URL` env var overrides the config file URL.

### Precedence

env var > config file > error ("run `hlctl config set-url` first")

## Command Tree

```
hlctl
├── login                              # OAuth2 client-credentials flow
├── config
│   ├── set-url <url>                  # set API base URL
│   └── show                           # display current config
├── containers
│   ├── list [--device <id>]           # list containers
│   ├── get <id>                       # container detail
│   ├── start <id>                     # start container
│   ├── stop <id>                      # stop container
│   └── restart <id>                   # restart container
├── system
│   ├── health                         # aggregate health
│   ├── info [--device <id>]           # device info
│   ├── utilization [--device <id>]    # live resource usage
│   ├── updates [--status] [--type]    # list updates
│   ├── update <id>                    # update detail
│   └── check-updates                  # force update check
├── storage
│   ├── volumes [--device <id>]        # list volumes
│   └── volume <id>                    # volume detail
├── backups
│   ├── tasks [--device <id>]          # list backup tasks
│   └── task <id>                      # task detail
└── network
    ├── devices                        # list network devices
    ├── device <id>                    # device detail
    ├── clients                        # list clients
    └── client <id>                    # client detail
```

### Global flags

- `--output, -o` — `table` (default) or `json`
- `--api-url` — override API URL for this invocation

## Code Generation

### Spec source

Git submodule at `spec/` pointing to `https://github.com/bwilczynski/homelab-api-spec`. The bundled spec lives at `spec/dist/openapi.bundled.yaml` (built via Redocly in the spec repo).

### oapi-codegen configs

One config per domain, generating client + models. Example (`oapi-codegen-containers.yaml`):

```yaml
package: containers
generate:
  client: true
  models: true
output: internal/containers/api.gen.go
output-options:
  include-tags:
    - containers
```

### Makefile targets

- `make generate` — runs oapi-codegen for all domains
- `make build` — builds the `hlctl` binary to `bin/hlctl`
- `make lint` — runs golangci-lint

Each domain package includes a `go generate` directive pointing to its config.

## Output Formatting

The `output` package provides a shared formatter:

- **Table mode** (default): human-readable table output. Each command defines its own column set.
- **JSON mode** (`-o json`): raw JSON from the API response.

## Phase 1 Scope

### Implemented (real code)

- Project structure, `go.mod`, `Makefile`
- Cobra command tree with all commands wired up and help text
- Config read/write (`~/.config/homelab/`)
- Output formatter (table + JSON)
- oapi-codegen configs + generated client code
- Git submodule for the spec
- `CLAUDE.md` with project conventions and patterns
- `README.md` with setup and usage

### Stubbed (pattern-setting, no real API calls)

- `hlctl login` — flag structure in place, prints "not yet implemented"
- All domain commands — wired to Cobra with correct flags/args, return placeholder output demonstrating table/JSON formatting
- Auth middleware — skeleton that reads token but does not validate

### Not included

- `.goreleaser.yaml`
- CI/CD
- Tests beyond compile verification
