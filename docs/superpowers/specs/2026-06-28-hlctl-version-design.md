# hlctl version command — Design

## Overview

Add a `hlctl version` command (modelled after `kubectl version`) that prints four fields:
client binary version, client-embedded spec version, live server binary version, and live
server spec version. The existing `--version` flag keeps its current behaviour (client-only,
no network call).

## Context

The homelab API spec (`spec/`) was updated to v1.1.0, adding two public endpoints under the
`meta` tag:

- `GET /meta/version` — returns `{ "apiVersion": "...", "serverVersion": "..." }`
- `GET /meta/auth` — returns Dex auth-discovery config

Both are `security: []` (no auth required).

## Architecture

### New files

| Path | Purpose |
|---|---|
| `codegen/meta.yaml` | oapi-codegen config for the `meta` domain |
| `internal/api/meta/api.gen.go` | Generated client (gitignored, produced by `make generate`) |
| `internal/cli/version/version.go` | `hlctl version` command |

### Modified files

| Path | Change |
|---|---|
| `cmd/hlctl/main.go` | Add `var specVersion = "unknown"` ldflag var; pass to `NewFactory` |
| `internal/cli/cmdutil/factory.go` | Add `SpecVersion string` field |
| `internal/cli/root.go` | Register `version.NewCmd(f)` |
| `Makefile` | Extract spec version from bundled YAML; pass as ldflag in `build` target |
| `.goreleaser.yaml` | Add `specVersion` ldflag; add step to extract `SPEC_VERSION` env var before build |
| `.github/workflows/release.yml` | Extract `SPEC_VERSION` env var before goreleaser runs |

## Data Flow

```
hlctl version
  │
  ├─ print client fields immediately (no network)
  │    Client version:  <main.version ldflag>
  │    Client spec:     <main.specVersion ldflag>
  │
  └─ unless --client flag:
       GET <apiURL>/meta/version
         ↓ 200 OK
       { "apiVersion": "1.1.0", "serverVersion": "v2.1.0" }
         ↓
       Server version:  v2.1.0
       Server spec:     1.1.0
```

## Client Spec Version — Build-time Embedding

The Makefile `build` target extracts `info.version` from `spec/dist/openapi.bundled.yaml`:

```makefile
SPEC_VERSION := $(shell grep '^  version:' $(SPEC_FILE) | awk '{print $$2}')

build:
    go build -ldflags "-X main.specVersion=$(SPEC_VERSION)" -o $(BINARY) ./cmd/hlctl
```

GoReleaser uses the same value via an env var set in the release workflow:

```yaml
# release.yml (before goreleaser step)
- name: Extract spec version
  run: echo "SPEC_VERSION=$(grep '^  version:' spec/dist/openapi.bundled.yaml | awk '{print $2}')" >> $GITHUB_ENV
```

```yaml
# .goreleaser.yaml ldflags
- -s -w -X main.version={{.Version}} -X main.specVersion={{.Env.SPEC_VERSION}}
```

## Command Behaviour

```
$ hlctl version
Client version:  v1.2.0
Client spec:     1.1.0
Server version:  v2.1.0
Server spec:     1.1.0

$ hlctl version --client
Client version:  v1.2.0
Client spec:     1.1.0

$ hlctl version --output=json
{
  "clientVersion": "v1.2.0",
  "clientSpec": "1.1.0",
  "serverVersion": "v2.1.0",
  "serverSpec": "1.1.0"
}
```

## Flags

- `--client` — skip server fetch, print only client fields

## Error Handling

If the server is unreachable or returns non-200, the client fields still print normally.
Server fields print `(unavailable)` and a warning goes to stderr:

```
Client version:  v1.2.0
Client spec:     1.1.0
Server version:  (unavailable)
Server spec:     (unavailable)
warning: could not reach server: ...
```

## oapi-codegen Domain

`codegen/meta.yaml` follows the same pattern as other domains (`system`, `docker`, etc.):
`generate: client: true, models: true`, output to `internal/api/meta/api.gen.go`.

The `version` command retrieves the client via `cmdutil.Client[MetaClient](cmd)` and calls
`GetMetaVersion(ctx)`. `InjectClient` is called on the `version` command itself (no sibling
commands share this client yet).

## Testing

Tests follow the existing pattern: construct the leaf command directly with
`cmdutil.TestFactory(t)`, seed the client via `cmdutil.SetClient[MetaClient](cmd, stub)`,
capture stdout, assert output lines.

Two cases:
1. Server reachable — all four fields present.
2. `--client` flag — only two fields, no HTTP call made.
