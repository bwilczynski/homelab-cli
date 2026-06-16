# Conventional Commit CI Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject PRs whose title or commits do not follow Conventional Commits, using a single `commitlint` config that mirrors the ruleset semantic-release relies on.

**Architecture:** A new GitHub Actions workflow runs on `pull_request` events. One job installs `@commitlint/cli` + `@commitlint/config-conventional`, then validates the PR title via stdin and the PR commits via `--from/--to`. Both share a single `.commitlintrc.json` at the repo root.

**Tech Stack:** GitHub Actions, `@commitlint/cli`, `@commitlint/config-conventional`, Node.js (LTS, runner-managed).

**Spec:** `docs/superpowers/specs/2026-06-16-conventional-commit-ci-validation-design.md`

---

## Task 1: Create the feature branch

**Files:** none

- [ ] **Step 1: Confirm a clean working tree on `main`**

Run: `git status`
Expected: `nothing to commit, working tree clean`. If not clean, stash or resolve before continuing.

- [ ] **Step 2: Create and switch to the feature branch**

Run: `git checkout -b feat/pr-validation-workflow`
Expected: `Switched to a new branch 'feat/pr-validation-workflow'`

---

## Task 2: Add the commitlint config

**Files:**
- Create: `.commitlintrc.json`

- [ ] **Step 1: Write the config file**

Create `.commitlintrc.json` with this exact content:

```json
{ "extends": ["@commitlint/config-conventional"] }
```

- [ ] **Step 2: Verify the file is valid JSON**

Run: `python3 -m json.tool .commitlintrc.json`
Expected: the file's contents echoed back (no parse error). Any error means the JSON is malformed — fix and retry.

- [ ] **Step 3: Stage and commit**

```bash
git add .commitlintrc.json
git commit -m "build: add commitlint config extending conventional"
```

Expected: one file changed, 1 insertion(+).

---

## Task 3: Add the PR validation workflow

**Files:**
- Create: `.github/workflows/pr-validation.yml`

- [ ] **Step 1: Write the workflow file**

Create `.github/workflows/pr-validation.yml` with this exact content:

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
        run: echo "$PR_TITLE" | npx commitlint

      - name: Validate PR commits
        run: npx commitlint --from=${{ github.event.pull_request.base.sha }} --to=${{ github.event.pull_request.head.sha }}
```

- [ ] **Step 2: Verify the workflow file is valid YAML**

Run: `python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/pr-validation.yml'))" && echo OK`
Expected: `OK`. Any traceback means malformed YAML — fix and retry.

- [ ] **Step 3: Stage and commit**

```bash
git add .github/workflows/pr-validation.yml
git commit -m "ci: add PR conventional-commit validation workflow"
```

Expected: one file changed, ~30 insertions(+).

---

## Task 4: Push branch and open the PR

**Files:** none

- [ ] **Step 1: Push the branch**

Run: `git push -u origin feat/pr-validation-workflow`
Expected: branch published; URL to open a PR printed at the bottom.

- [ ] **Step 2: Open the PR with a conventional title**

Run:

```bash
gh pr create \
  --title "ci: validate conventional commits on PRs" \
  --body "$(cat <<'EOF'
## Summary
- Add `.commitlintrc.json` extending `@commitlint/config-conventional`.
- Add `.github/workflows/pr-validation.yml` that validates the PR title and commits on every `pull_request` event.

## Test plan
- [ ] This PR itself passes the new workflow (title + both commits are conventional).
- [ ] Editing the PR title to a non-conventional value fails the `Validate PR title` step.
- [ ] Restoring a conventional title passes again.
- [ ] Pushing a commit with a non-conventional message fails the `Validate PR commits` step.
EOF
)"
```

Expected: PR URL printed.

---

## Task 5: Verify the happy path (title + commits both conventional)

**Files:** none

- [ ] **Step 1: Wait for PR checks to finish**

Run: `gh pr checks --watch`
Expected: streams a table of checks; exits 0 once all checks (including the new `conventional-commits` job) complete successfully.

If `gh` reports "no checks reported" momentarily, GitHub has not yet registered the run — re-run the command.

- [ ] **Step 2: Confirm the `conventional-commits` check passed**

Run: `gh pr checks | grep conventional-commits`
Expected: a line containing `pass` (or a green check) for `conventional-commits`.

This validates spec acceptance criterion #3: a PR with a conventional title and conventional commits passes both steps.

---

## Task 6: Verify the title-failure path

**Files:** none

- [ ] **Step 1: Edit the PR title to a non-conventional value**

Run: `gh pr edit --title "broken title without a type"`
Expected: PR title updated.

- [ ] **Step 2: Wait for the re-triggered workflow to finish**

Run: `gh pr checks --watch || true`
Expected: streams checks until the run finishes; `gh pr checks --watch` exits non-zero because `conventional-commits` fails. The trailing `|| true` keeps the shell flow going so you can inspect the result.

If `gh` reports "no checks reported" momentarily, GitHub has not yet picked up the `edited` event — re-run.

- [ ] **Step 3: Confirm the failure was in the title step, not the commits step**

Run: `gh run view --log-failed $(gh run list --workflow=pr-validation.yml --branch=feat/pr-validation-workflow --limit=1 --json databaseId -q '.[0].databaseId') | grep -E "Validate PR (title|commits)" | head -5`
Expected: the failed step is `Validate PR title`. The commits step should not have failed (the PR's actual commits are still conventional).

This validates spec acceptance criterion #1, #4 (the `edited` trigger fires the workflow).

- [ ] **Step 4: Restore the conventional title**

Run: `gh pr edit --title "ci: validate conventional commits on PRs"`
Expected: title restored.

- [ ] **Step 5: Wait for the re-run and confirm green**

Run: `gh pr checks --watch`
Expected: exits 0; `conventional-commits` passes.

---

## Task 7: Verify the commits-failure path

**Files:** none

- [ ] **Step 1: Add a commit with a non-conventional message**

```bash
git commit --allow-empty -m "this message has no type"
git push
```

Expected: push succeeds.

- [ ] **Step 2: Wait for the workflow to re-run on the `synchronize` event**

Run: `gh pr checks --watch || true`
Expected: exits non-zero because `conventional-commits` fails on the new commit.

- [ ] **Step 3: Confirm the failure was in the commits step**

Run: `gh run view --log-failed $(gh run list --workflow=pr-validation.yml --branch=feat/pr-validation-workflow --limit=1 --json databaseId -q '.[0].databaseId') | grep -E "Validate PR (title|commits)" | head -5`
Expected: the failed step is `Validate PR commits`.

This validates spec acceptance criterion #2.

- [ ] **Step 4: Remove the bad commit and force-push**

```bash
git reset --hard HEAD~1
git push --force-with-lease
```

Expected: branch restored to the previous tip; remote updated.

- [ ] **Step 5: Wait for the re-run and confirm green**

Run: `gh pr checks --watch`
Expected: exits 0; `conventional-commits` passes.

---

## Task 8: Verify final state and hand off for merge

**Files:** none

- [ ] **Step 1: Confirm exactly the two intended files were added**

Run: `git diff --name-status main...HEAD`
Expected:

```
A	.commitlintrc.json
A	.github/workflows/pr-validation.yml
```

No other files. This validates spec acceptance criterion #5.

- [ ] **Step 2: Confirm the workflow only fires on `pull_request`**

Run: `grep -E '^on:|^  (push|pull_request|schedule):' .github/workflows/pr-validation.yml`
Expected: only `on:` and `  pull_request:` appear — no `push:`, `schedule:`, or other triggers. This validates spec acceptance criterion #4.

- [ ] **Step 3: Report PR ready for review/merge**

Print the PR URL (`gh pr view --json url -q .url`) and stop. Do not merge — leave that to the user.
