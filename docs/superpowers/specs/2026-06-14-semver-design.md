# Switch to SemVer with semantic-release

**Date:** 2026-06-14
**Status:** Approved

## Overview

Replace the current CalVer tagging scheme (`v<YYYYMMDD>.<short-sha>`) with Semantic Versioning (`v<major>.<minor>.<patch>`), automated via `semantic-release` on every push to `main`.

## Current state

The release workflow (`release.yml`) computes a tag with a shell one-liner:
```sh
TAG="v$(date +%Y%m%d).$(git rev-parse --short=4 HEAD)"
```
This tag is pushed to GitHub, then GoReleaser picks it up via `{{.Version}}` to build binaries, create a GitHub release, and update the Homebrew tap. GoReleaser runs with `--skip=validate` because CalVer tags like `v20260614.6da0` are not valid SemVer and GoReleaser's validation rejects them.

## Target state

`semantic-release` replaces the shell tagging step. It reads conventional commit messages since the last tag, computes the next SemVer tag, and pushes it. GoReleaser then runs conditionally, building and publishing only when a new version was actually produced.

Responsibility split:
- **semantic-release**: commit analysis, version computation, tag push
- **GoReleaser**: binary builds, GitHub release creation (with assets), Homebrew tap update

## Version bump rules

| Commit prefix | Bump |
|---|---|
| `fix:` | patch |
| `feat:` | minor |
| `BREAKING CHANGE:` footer | major |
| `refactor:`, `chore:`, `docs:`, etc. | none (no release) |

First release will be `v1.0.0` (no prior SemVer tag exists).

## Configuration

### `.releaserc.json` (new)

```json
{
  "branches": ["main"],
  "plugins": [
    "@semantic-release/commit-analyzer",
    "@semantic-release/release-notes-generator"
  ]
}
```

`@semantic-release/github` is intentionally excluded — GoReleaser owns GitHub release creation. Both `@semantic-release/commit-analyzer` and `@semantic-release/release-notes-generator` are bundled with `semantic-release`; no separate installation needed.

### `.github/workflows/release.yml` (updated)

Changes to the `release` job:

1. **Remove** the `Compute and push tag` step entirely.
2. **Add** a Semantic Release step:
   ```yaml
   - name: Semantic Release
     uses: cycjimmy/semantic-release-action@v6
     id: semantic
     env:
       GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
   ```
3. **Update** the GoReleaser step:
   - Add condition: `if: steps.semantic.outputs.new_release_published == 'true'`
   - Change args from `release --clean --skip=validate` to `release --clean`

`cycjimmy/semantic-release-action@v6` runs on Node.js 24 and bundles semantic-release — no `package.json` or `setup-node` step required for this purpose.

### No `package.json` needed

The action is self-contained. Adding a `package.json` to pin plugin versions would risk version conflicts with semantic-release's own bundled dependencies.

## What does not change

- GoReleaser config (`.goreleaser.yaml`) — unchanged
- Binary version injection via `-X main.version={{.Version}}` — unchanged
- Homebrew tap update — unchanged
- `ci` job — unchanged
- Commit message conventions already in use — unchanged
