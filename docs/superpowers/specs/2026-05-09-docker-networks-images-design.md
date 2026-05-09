# Docker Networks & Images CLI Commands

**Date:** 2026-05-09

## Context

The `homelab-api-spec` submodule added two new resource groups under the `docker` tag:

- `GET /docker/networks` — list Docker networks across backends
- `GET /docker/networks/{networkId}` — get a single network by composite ID (`{device}.{name}`)
- `GET /docker/images` — list Docker images across backends
- `GET /docker/images/{imageId}` — get a single image by composite ID (`{device}.{shortId}`)

The submodule has been updated to commit `1bd1b54`.

## Approach

Add `networks` and `images` subcommands directly to `internal/cli/docker/docker.go`, following the existing containers pattern. No new packages, no new codegen configs.

The existing `oapi-codegen-docker.yaml` already captures all endpoints tagged `docker`, so `make generate` will emit the new client methods automatically.

## New Commands

```
hlctl docker networks list [--device <id>]
hlctl docker networks get <networkId>
hlctl docker images list [--device <id>]
hlctl docker images get <imageId>
```

## Table Output

**`networks list`:** `ID`, `NAME`, `DEVICE`, `CONTAINERS`

**`networks get`:** field/value — id, name, device, driver, subnet, gateway; followed by a `CONTAINERS` section listing connected container names

**`images list`:** `ID`, `DEVICE`, `REPOSITORY`, `TAGS`, `SIZE`

**`images get`:** field/value — id, device, repository, tags (comma-joined), size, created, virtual size

## Implementation Steps

1. Commit the updated submodule pointer (`spec` → `1bd1b54`)
2. Run `make generate` to regenerate `internal/docker/api.gen.go`
3. Add `newNetworksCmd()` and `newImagesCmd()` to `internal/cli/docker/docker.go`
4. Register both in `NewCmd()` via `cmd.AddCommand`
5. Add tests in `internal/cli/docker/docker_test.go` following the existing stub pattern
