# Testing & Distribution Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add CI that runs lint+tests on every push to `main`, publishes a versioned GitHub Release with macOS binaries, and pushes a Homebrew formula to `bwilczynski/homebrew-tap`.

**Architecture:** GitHub Actions orchestrates three sequential jobs (ci → release). GoReleaser builds `darwin/arm64` + `darwin/amd64` binaries with a CalVer+SHA tag auto-created by CI, creates the GitHub Release, and commits the updated Homebrew formula to the tap repo. Version is embedded at build time via ldflags and exposed via `hlctl --version`.

**Tech Stack:** Go 1.26, Cobra, GoReleaser v2, GitHub Actions, Homebrew

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `Makefile` | Modify | Add `test` target |
| `internal/cli/root_test.go` | Create | Test `--version` output |
| `internal/cli/root.go` | Modify | Accept version param, set `rootCmd.Version` |
| `cmd/hlctl/main.go` | Modify | Add `var version`, pass to `cli.Execute` |
| `.goreleaser.yaml` | Create | Build config, archive, checksum, Homebrew formula |
| `.github/workflows/release.yml` | Create | CI + release pipeline |

---

### Task 1: Add `make test` target

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add the test target**

Open `Makefile` and add after the `lint` target:

```makefile
test: ## Run tests
	go test ./...
```

The full targets section should look like:

```makefile
.PHONY: help build generate bundle lint test tidy
```

and:

```makefile
lint: ## Run go vet
	go vet ./...

test: ## Run tests
	go test ./...

tidy: ## Tidy go.mod
	go mod tidy
```

- [ ] **Step 2: Verify it works locally**

```bash
make test
```

Expected: all tests pass (same output as `go test ./...`)

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "build: add make test target"
```

---

### Task 2: Wire version into binary

**Files:**
- Create: `internal/cli/root_test.go`
- Modify: `internal/cli/root.go`
- Modify: `cmd/hlctl/main.go`

- [ ] **Step 1: Write the failing test**

Create `internal/cli/root_test.go`:

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestExecute_version(t *testing.T) {
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--version"})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.Version = ""
	})

	_ = Execute("v20260508.774a")

	if !strings.Contains(buf.String(), "v20260508.774a") {
		t.Errorf("expected version in output, got: %s", buf.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cli/ -run TestExecute_version -v
```

Expected: compile error — `Execute` takes no arguments.

- [ ] **Step 3: Update `internal/cli/root.go`**

Change the `Execute` function to accept and apply a version string:

```go
func Execute(version string) error {
	rootCmd.Version = version
	return rootCmd.Execute()
}
```

Full file after change:

```go
package cli

import (
	"github.com/bwilczynski/hlctl/internal/cli/auth"
	"github.com/bwilczynski/hlctl/internal/cli/config"
	dockercli "github.com/bwilczynski/hlctl/internal/cli/docker"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/cli/network"
	"github.com/bwilczynski/hlctl/internal/cli/storage"
	"github.com/bwilczynski/hlctl/internal/cli/system"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:          "hlctl",
	Short:        "CLI for controlling homelab services",
	Long:         "hlctl is a command-line interface for managing your homelab infrastructure via the Homelab API.",
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flags.OutputFormat, "output", "o", "table", "Output format: table or json")
	rootCmd.PersistentFlags().StringVar(&flags.APIURL, "api-url", "", "Override API base URL")
	rootCmd.AddCommand(auth.NewCmd())
	rootCmd.AddCommand(config.NewCmd())
	rootCmd.AddCommand(dockercli.NewCmd())
	rootCmd.AddCommand(network.NewCmd())
	rootCmd.AddCommand(storage.NewCmd())
	rootCmd.AddCommand(system.NewCmd())
}

func Execute(version string) error {
	rootCmd.Version = version
	return rootCmd.Execute()
}
```

- [ ] **Step 4: Update `cmd/hlctl/main.go`**

Add a `version` variable (set to `"dev"` by default, overridden by ldflags at build time) and pass it to `Execute`:

```go
package main

import (
	"os"

	"github.com/bwilczynski/hlctl/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./internal/cli/ -run TestExecute_version -v
```

Expected:
```
--- PASS: TestExecute_version (0.00s)
PASS
```

- [ ] **Step 6: Run all tests to check for regressions**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 7: Verify version flag works manually**

```bash
go run ./cmd/hlctl --version
```

Expected output: `hlctl version dev`

- [ ] **Step 8: Commit**

```bash
git add internal/cli/root.go internal/cli/root_test.go cmd/hlctl/main.go
git commit -m "feat: embed version via ldflags and expose via --version flag"
```

---

### Task 3: Add GoReleaser config

**Files:**
- Create: `.goreleaser.yaml`

- [ ] **Step 1: Create `.goreleaser.yaml`**

```yaml
version: 2

project_name: hlctl

before:
  hooks:
    - go mod tidy

builds:
  - binary: hlctl
    main: ./cmd/hlctl
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
    goarch:
      - arm64
      - amd64
    ldflags:
      - -s -w -X main.version={{.Version}}

archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: checksums.txt
  algorithm: sha256

brews:
  - name: hlctl
    repository:
      owner: bwilczynski
      name: homebrew-tap
      branch: master
      token: "{{ .Env.TAP_GITHUB_TOKEN }}"
    homepage: "https://github.com/bwilczynski/homelab-cli"
    description: "CLI for managing homelab infrastructure"
    test: |
      system "#{bin}/hlctl", "--version"
```

- [ ] **Step 2: Validate the config locally**

Install GoReleaser if not present: `brew install goreleaser`

```bash
goreleaser check
```

Expected: `• config is valid`

- [ ] **Step 3: Commit**

```bash
git add .goreleaser.yaml
git commit -m "build: add goreleaser config for darwin releases and homebrew tap"
```

---

### Task 4: Add GitHub Actions release workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create the workflows directory**

```bash
mkdir -p .github/workflows
```

- [ ] **Step 2: Create `.github/workflows/release.yml`**

```yaml
name: Release

on:
  push:
    branches:
      - main

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          submodules: recursive

      - uses: actions/setup-node@v4
        with:
          node-version: '20'

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Generate API client code
        run: make generate

      - name: Lint
        run: go vet ./...

      - name: Test
        run: go test ./...

  release:
    runs-on: ubuntu-latest
    needs: ci
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4
        with:
          submodules: recursive
          fetch-depth: 0

      - uses: actions/setup-node@v4
        with:
          node-version: '20'

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Generate API client code
        run: make generate

      - name: Compute and push tag
        run: |
          TAG="v$(date +%Y%m%d).$(git rev-parse --short=4 HEAD)"
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git tag "$TAG"
          git push origin "$TAG"

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          TAP_GITHUB_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add github actions release workflow with goreleaser"
```

---

### Task 5: Add TAP_GITHUB_TOKEN secret and verify

- [ ] **Step 1: Create a GitHub PAT for the tap repo**

Go to GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens.

Create a token with:
- **Resource owner:** `bwilczynski`
- **Repository access:** Only `bwilczynski/homebrew-tap`
- **Permissions:** Contents → Read and write

Copy the token value.

- [ ] **Step 2: Add the secret to the homelab-cli repo**

```bash
gh secret set TAP_GITHUB_TOKEN --repo bwilczynski/homelab-cli
```

Paste the token when prompted.

- [ ] **Step 3: Push to main and verify the workflow runs**

```bash
git push origin main
```

Then watch the Actions tab:

```bash
gh run watch --repo bwilczynski/homelab-cli
```

Expected: `ci` job passes, `release` job creates a GitHub Release (e.g. `v20260508.xxxx`) and pushes `Formula/hlctl.rb` to `bwilczynski/homebrew-tap`.

- [ ] **Step 4: Verify Homebrew install works**

```bash
brew tap bwilczynski/tap    # already done if tap is configured
brew install hlctl
hlctl --version
```

Expected: `hlctl version v20260508.xxxx`
