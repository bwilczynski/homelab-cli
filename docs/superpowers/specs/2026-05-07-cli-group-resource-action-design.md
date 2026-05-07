# CLI Group/Resource/Action Reorganization

## Context

The API spec was reorganized into a group/resource hierarchy (commit `f481090`):
- `containers` tag renamed to `docker`; paths moved from `/containers` to `/docker/containers`
- `backups` tag removed; backups merged into `storage` tag at `/storage/backups`

This design reflects those changes in the CLI and applies a uniform `<group> <resource> <action>` command convention across all domains.

## Command Tree

```
hlctl docker containers list
hlctl docker containers get <id>
hlctl docker containers start <id>
hlctl docker containers stop <id>
hlctl docker containers restart <id>

hlctl storage volumes list
hlctl storage volumes get <id>
hlctl storage backups list
hlctl storage backups get <id>

hlctl network devices list
hlctl network devices get <id>
hlctl network clients list
hlctl network clients get <id>

hlctl system health
hlctl system info
hlctl system utilization
hlctl system updates list
hlctl system updates get <id>
hlctl system updates check
```

`system health`, `system info`, and `system utilization` remain flat singletons — they have no collection/id structure. `system updates` is a real collection and follows the pattern.

## Code Structure Changes

### oapi-codegen configs

| File | Change |
|------|--------|
| `oapi-codegen-containers.yaml` | Rename to `oapi-codegen-docker.yaml`; package `docker`; output `internal/docker/api.gen.go`; tag `docker` |
| `oapi-codegen-backups.yaml` | Delete — backups now covered by `storage` tag |
| `oapi-codegen-storage.yaml` | No change — tag `storage` already covers volumes + backups in updated spec |

### CLI packages

| Package | Change |
|---------|--------|
| `internal/cli/containers/` | Rename to `internal/cli/docker/`; wrap commands under a `containers` subcommand |
| `internal/cli/backups/` | Delete; move logic into `internal/cli/storage/` as a `backups` subgroup |
| `internal/cli/storage/` | Add `volumes` subgroup (`list`/`get`); add `backups` subgroup (`list`/`get`) |
| `internal/cli/network/` | Restructure `devices`/`device` → `devices list`/`devices get`; same for `clients` |
| `internal/cli/system/` | Add `updates` subgroup (`list`/`get`/`check`); keep `health`, `info`, `utilization` flat |

### Makefile

- `mkdir` target: replace `internal/containers internal/backups` with `internal/docker`
- Remove backups codegen line
- Rename containers codegen step to docker

### root.go

- Remove `backups` import and `AddCommand`
- Replace `containers` import/command with `docker`

## Conventions

- Each group (`docker`, `storage`, `network`, `system`) is a top-level Cobra command with no `RunE` of its own.
- Each resource (`containers`, `volumes`, `backups`, `devices`, `clients`, `updates`) is a subcommand of its group, also with no `RunE`.
- Actions (`list`, `get`, `start`, `stop`, `restart`, `check`) are leaf commands with `RunE`.
- Singleton system commands (`health`, `info`, `utilization`) are direct children of `system` with `RunE` — they are not resources.
- Client interfaces and stub files stay co-located with their CLI package.
