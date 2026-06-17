# Conventional Commit CI Validation — Design

## Problem

The repository uses [semantic-release](https://semantic-release.gitbook.io) on `main` to derive versions and changelogs from [Conventional Commits](https://www.conventionalcommits.org). Today nothing checks the commit convention before merge: contributors discover non-conformance only when the release job either skips the change or attributes it incorrectly. PR titles, which become squash-merge commits on `main`, are also unvalidated.

## Goal

Reject PRs whose title or commits do not match Conventional Commits, using the same ruleset semantic-release relies on. Surface failures inline on the PR so authors can fix issues before merge.

## Non-goals

- Local hooks (commit-msg, pre-push). Out of scope; CI is sufficient.
- Adding lint/test runs on PRs. Existing `release.yml` runs them on `push: main`; broadening CI is a separate concern.
- Replacing or restructuring `release.yml`.
- Introducing a `package.json` for the repo.

## Approach

A new workflow `.github/workflows/pr-validation.yml` runs on `pull_request` events. A single job installs `@commitlint/cli` and `@commitlint/config-conventional` and runs two checks:

1. PR title — piped to `commitlint` via stdin.
2. PR commits — `commitlint --from=<base.sha> --to=<head.sha>`.

Both checks share a single config file at the repo root, so the ruleset is defined exactly once.

### Files

**`.commitlintrc.json`** (repo root):

```json
{ "extends": ["@commitlint/config-conventional"] }
```

**`.github/workflows/pr-validation.yml`**:

```yaml
name: PR Validation

on:
  pull_request:
    types: [opened, edited, synchronize, reopened]

permissions:
  contents: read
  pull-requests: read

jobs:
  conventional-commits:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-node@v4
        with:
          node-version: 'lts/*'

      - name: Install commitlint
        run: npm install --no-save @commitlint/cli @commitlint/config-conventional

      - name: Validate PR title
        env:
          PR_TITLE: ${{ github.event.pull_request.title }}
        run: printf '%s\n' "$PR_TITLE" | npx commitlint

      - name: Validate PR commits
        run: npx commitlint --from=${{ github.event.pull_request.base.sha }} --to=${{ github.event.pull_request.head.sha }}
```

## Design rationale

- **Single tool, single config.** Using `@commitlint/cli` for both checks means there is one place to extend, override, or audit the rules. Conventional Commits is what semantic-release's default analyzer expects, so the rulesets stay aligned without coordination.
- **Trigger types include `edited`.** A PR title is mutable; the check must re-run when the title changes, not only on commit push.
- **PR title via `env` + `printf '%s\n'`, not template interpolation or `echo`.** Substituting `${{ github.event.pull_request.title }}` directly into a shell command allows command injection via crafted titles (backticks, `$(…)`); passing through `env` and quoting `"$PR_TITLE"` blocks that. Bash's `echo` builtin also interprets leading `-e`, `-n`, `-E` as flags, so a title starting with `-e` would be expanded with backslash escapes before reaching commitlint — `printf '%s\n'` does not have that surface.
- **`fetch-depth: 0`.** `commitlint --from --to` requires history reachable from both endpoints; the default shallow checkout omits it.
- **Two steps, one job.** Sharing the `npm install` keeps the workflow fast and produces a single contiguous log for reviewers.
- **`npm install --no-save`.** The repo has no `package.json` and adding one purely for CI tooling would invite drift; `--no-save` keeps the install ephemeral.
- **`permissions:` declared minimal.** Read-only access to contents and PRs is all the action needs; explicit declaration follows least-privilege.

## Failure modes

| Scenario | Outcome |
| --- | --- |
| PR title not conventional | `Validate PR title` step fails; check appears red on the PR. |
| Any PR commit not conventional | `Validate PR commits` step fails. |
| Force-push to PR branch | `synchronize` event refires the workflow against the new head. |
| Title edited after open | `edited` event refires the workflow. |
| Empty PR (no commits beyond base) | `commitlint --from=X --to=X` exits 0; not a meaningful case. |

## Acceptance criteria

1. Opening a PR with a non-conventional title fails the workflow; fixing the title and saving re-runs it green.
2. Opening a PR containing a non-conventional commit (with a conventional title) fails the workflow.
3. A PR with a conventional title and conventional commits passes both steps.
4. The workflow runs only on `pull_request` events; pushes to `main` and other branches do not trigger it.
5. No new files appear in the repo besides `.commitlintrc.json` and `.github/workflows/pr-validation.yml`.

## Rollout

1. Land the workflow and config in a single PR. That PR's own title and commits must themselves be conventional — the workflow will validate itself once it exists on the branch.
2. No repo settings changes required initially. If the team wants the check to block merges, add it to the branch protection rule for `main` in repository settings after one successful run.
