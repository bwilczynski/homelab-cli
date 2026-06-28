# hlctl version Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `hlctl version` command that prints client version, client spec version, server version, and server spec version (live from `GET /meta/version`), modelled after `kubectl version`.

**Architecture:** A new `internal/cli/version/` package holds the command following the existing Options + runF pattern. The meta API client is generated via oapi-codegen from the `meta` tag in the bundled spec. Client spec version is embedded at build time via ldflag extracted from the bundled YAML.

**Tech Stack:** Go, Cobra, oapi-codegen, tabwriter, encoding/json, httpmock (tests)

---

### Task 1: Update CLAUDE.md — replace stale InjectClient references

The CLAUDE.md "Adding a New Domain Command" section documents `cmdutil.InjectClient`, `cmdutil.Client[T]`, and `cmdutil.SetClient` — none of which exist in the codebase. Replace with the Options + runF pattern that is actually used.

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Replace step 3**

Find and replace this block in `CLAUDE.md`:

```
3. Exactly one ancestor per leaf path calls `cmdutil.InjectClient(cmd, buildClient)` where `buildClient` closes over `f` and calls `f.HTTPClient()` — Cobra runs the closest `PersistentPreRunE` only, so additional calls on intermediate parents are dead. Put it on the domain root when all leaves share one client (`network`, `system`); put it on each sub-group parent when only some sub-trees need it (`docker`, `storage`). Leaf commands have no `client` parameter and call `cmdutil.Client[<Domain>Client>(cmd).<Method>(...)` to retrieve it.
```

Replace with:

```
3. Each leaf command uses the **Options + runF** pattern. Define an `<action>Options` struct with `HTTPClient func() (*http.Client, string, error)`, `IO *cmdutil.IOStreams`, `Output func() output.Format`, and any command-specific fields — all set from `f` in the constructor. The constructor signature is `newXxxCmd(f *cmdutil.Factory, runF func(*xxxOptions) error) *cobra.Command`; `RunE` calls `runF(opts)` when non-nil (test path) or the real run function otherwise. In the run function, call `opts.HTTPClient()` to get the `*http.Client` and base URL, construct the domain client with `New<Domain>Client(httpClient, apiURL)`, and make the API call.
```

- [ ] **Step 2: Replace step 8**

Find and replace this block in `CLAUDE.md`:

```
8. Tests construct leaves directly using `cmdutil.TestFactory(t)` and seed the client via `cmdutil.SetClient[<Domain>Client](cmd, stub)`.
```

Replace with:

```
8. Tests use two layers. Layer 1: pass a non-nil `runF` that sets a boolean and assert it was called — this verifies the Cobra wiring. Layer 2: construct the `opts` struct directly with `testHTTPClient(reg)` for the HTTP client and `httpmock.NewRegistry()` for mock responses, then call the run function directly. No `SetClient` or `InjectClient` — the `runF` hook and direct `opts` construction are the only test seams.
```

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: replace stale InjectClient refs with Options+runF pattern in CLAUDE.md"
```

---

### Task 2: Add oapi-codegen config for the meta domain

**Files:**
- Create: `codegen/meta.yaml`
- Modify: `Makefile`

- [ ] **Step 1: Create `codegen/meta.yaml`**

```yaml
package: meta
generate:
  client: true
  models: true
output: internal/api/meta/api.gen.go
output-options:
  include-tags:
    - meta
```

- [ ] **Step 2: Add meta generation line to Makefile**

In the `generate` target, after the existing `$(OAPI_CODEGEN)` lines, add:

```makefile
generate: bundle ## Generate client code from the bundled spec
	@mkdir -p internal/api/system internal/api/docker internal/api/storage internal/api/network internal/api/meta
	$(OAPI_CODEGEN) --config codegen/system.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/docker.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/storage.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/network.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/meta.yaml $(SPEC_FILE)
```

- [ ] **Step 3: Run generation and verify output**

```bash
make generate
```

Expected: `internal/api/meta/api.gen.go` created. Verify it contains `GetMetaVersionWithResponse` and a `Version` struct with `ApiVersion` and `ServerVersion` string fields.

```bash
grep -E "GetMetaVersionWithResponse|type Version struct|ApiVersion|ServerVersion" internal/api/meta/api.gen.go
```

- [ ] **Step 4: Commit**

```bash
git add codegen/meta.yaml Makefile
git commit -m "feat: add oapi-codegen config for meta domain"
```

---

### Task 3: Extend Factory with SpecVersion

**Files:**
- Modify: `internal/cli/cmdutil/factory.go`
- Modify: `internal/cli/cmdutil/testfactory.go`
- Modify: `internal/cli/cmdutil/factory_test.go`
- Modify: `cmd/hlctl/main.go`

- [ ] **Step 1: Add `SpecVersion` field to `Factory` and update `NewFactory` signature**

In `internal/cli/cmdutil/factory.go`, update the `Factory` struct and `NewFactory`:

```go
type Factory struct {
	Version     string
	SpecVersion string

	IOStreams *IOStreams

	Config     func() (*config.Config, error)
	APIURL     func() (string, error)
	HTTPClient func() (*http.Client, string, error)
	Output     func() output.Format
}

func NewFactory(version, specVersion string, apiURLFlag, outputFlag *string) *Factory {
	var (
		cfg     *config.Config
		cfgErr  error
		cfgOnce sync.Once
	)
	loadConfig := func() (*config.Config, error) {
		cfgOnce.Do(func() { cfg, cfgErr = config.Load() })
		return cfg, cfgErr
	}
	apiURLFn := func() (string, error) {
		if *apiURLFlag != "" {
			return *apiURLFlag, nil
		}
		c, err := loadConfig()
		if err != nil {
			return "", err
		}
		return c.ResolveAPIURL()
	}
	return &Factory{
		Version:     version,
		SpecVersion: specVersion,
		IOStreams:   SystemIOStreams(),
		Config:      loadConfig,
		APIURL:      apiURLFn,
		HTTPClient: func() (*http.Client, string, error) {
			apiURL, err := apiURLFn()
			if err != nil {
				return nil, "", err
			}
			return &http.Client{Transport: auth.NewAuthenticatedTransport(nil)}, apiURL, nil
		},
		Output: func() output.Format { return output.Format(*outputFlag) },
	}
}
```

- [ ] **Step 2: Update `TestFactory` in `testfactory.go`**

Add `SpecVersion: "test"` to the returned `Factory`:

```go
func TestFactory(t *testing.T) *Factory {
	t.Helper()
	return &Factory{
		Version:     "test",
		SpecVersion: "test",
		IOStreams:   &IOStreams{In: strings.NewReader(""), Out: io.Discard, ErrOut: io.Discard},
		Config: func() (*config.Config, error) {
			return nil, errors.New("TestFactory: Config not configured")
		},
		APIURL: func() (string, error) {
			return "", errors.New("TestFactory: APIURL not configured")
		},
		HTTPClient: func() (*http.Client, string, error) {
			return nil, "", errors.New("TestFactory: HTTPClient not configured")
		},
		Output: func() output.Format { return output.FormatTable },
	}
}
```

- [ ] **Step 3: Fix factory_test.go call sites**

`factory_test.go` calls `NewFactory("test", &apiURL, &outputFmt)` — add the `specVersion` argument:

```go
f := cmdutil.NewFactory("test", "0.0.0", &apiURL, &outputFmt)
```

Apply this change to both test functions (`TestNewFactory_outputFlagDefersToLatestValue` and `TestNewFactory_apiURLFlagOverridesConfig`).

- [ ] **Step 4: Update `cmd/hlctl/main.go`**

```go
package main

import (
	"os"

	"github.com/bwilczynski/hlctl/internal/cli"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/pflag"
)

var version = "dev"
var specVersion = "unknown"

func main() {
	var apiURL, outputFmt string
	pflag.StringVarP(&outputFmt, "output", "o", "table", "Output format: table or json")
	pflag.StringVar(&apiURL, "api-url", "", "Override API base URL")

	f := cmdutil.NewFactory(version, specVersion, &apiURL, &outputFmt)
	root := cli.NewRootCmd(f)
	root.PersistentFlags().AddFlagSet(pflag.CommandLine)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Run tests to verify nothing broke**

```bash
go test ./internal/cli/cmdutil/... ./cmd/...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/cmdutil/factory.go internal/cli/cmdutil/testfactory.go internal/cli/cmdutil/factory_test.go cmd/hlctl/main.go
git commit -m "feat: add SpecVersion field to Factory"
```

---

### Task 4: Implement the version command

**Files:**
- Create: `internal/cli/version/client.go`
- Create: `internal/cli/version/version.go`
- Create: `internal/cli/version/version_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/cli/version/version_test.go`:

```go
package version

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	metaapi "github.com/bwilczynski/hlctl/internal/api/meta"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

func testHTTPClient(reg *httpmock.Registry) func() (*http.Client, string, error) {
	return func() (*http.Client, string, error) {
		return &http.Client{Transport: reg}, "http://localhost", nil
	}
}

// Layer 1: runF hook fires

func TestNewVersionCmd_runFCalled(t *testing.T) {
	called := false
	cmd := NewCmd(cmdutil.TestFactory(t), func(o *versionOptions) error {
		called = true
		return nil
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected runF to be called")
	}
}

// Layer 2: business logic

func TestGetVersionRun_tableOutput_serverReachable(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/meta/version"), httpmock.JSONResponse(metaapi.Version{
		ApiVersion:    "1.1.0",
		ServerVersion: "v2.0.0",
	}))

	var out, errOut bytes.Buffer
	opts := &versionOptions{
		ClientVersion: "v1.0.0",
		ClientSpec:    "1.0.0",
		HTTPClient:    testHTTPClient(reg),
		IO:            &cmdutil.IOStreams{Out: &out, ErrOut: &errOut},
		Output:        func() output.Format { return output.FormatTable },
	}
	if err := getVersionRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"v1.0.0", "1.0.0", "v2.0.0", "1.1.0"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetVersionRun_tableOutput_clientOnly(t *testing.T) {
	var httpCalled bool
	var out, errOut bytes.Buffer
	opts := &versionOptions{
		ClientVersion: "v1.0.0",
		ClientSpec:    "1.0.0",
		ClientOnly:    true,
		HTTPClient: func() (*http.Client, string, error) {
			httpCalled = true
			return nil, "", nil
		},
		IO:     &cmdutil.IOStreams{Out: &out, ErrOut: &errOut},
		Output: func() output.Format { return output.FormatTable },
	}
	if err := getVersionRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpCalled {
		t.Error("expected HTTPClient not to be called with --client flag")
	}
	for _, want := range []string{"v1.0.0", "1.0.0"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	for _, absent := range []string{"Server version", "Server spec"} {
		if strings.Contains(out.String(), absent) {
			t.Errorf("expected %q absent from output, got:\n%s", absent, out.String())
		}
	}
}

func TestGetVersionRun_tableOutput_serverUnavailable(t *testing.T) {
	var out, errOut bytes.Buffer
	opts := &versionOptions{
		ClientVersion: "v1.0.0",
		ClientSpec:    "1.0.0",
		HTTPClient: func() (*http.Client, string, error) {
			return nil, "", fmt.Errorf("connection refused")
		},
		IO:     &cmdutil.IOStreams{Out: &out, ErrOut: &errOut},
		Output: func() output.Format { return output.FormatTable },
	}
	if err := getVersionRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("expected graceful degradation, got error: %v", err)
	}
	if !strings.Contains(out.String(), "(unavailable)") {
		t.Errorf("expected '(unavailable)' in output, got:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "warning") {
		t.Errorf("expected warning on stderr, got:\n%s", errOut.String())
	}
}

func TestGetVersionRun_jsonOutput_serverReachable(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/meta/version"), httpmock.JSONResponse(metaapi.Version{
		ApiVersion:    "1.1.0",
		ServerVersion: "v2.0.0",
	}))

	var out, errOut bytes.Buffer
	opts := &versionOptions{
		ClientVersion: "v1.0.0",
		ClientSpec:    "1.0.0",
		HTTPClient:    testHTTPClient(reg),
		IO:            &cmdutil.IOStreams{Out: &out, ErrOut: &errOut},
		Output:        func() output.Format { return output.FormatJSON },
	}
	if err := getVersionRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got versionOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out.String())
	}
	if got.ClientVersion != "v1.0.0" {
		t.Errorf("clientVersion: got %q, want %q", got.ClientVersion, "v1.0.0")
	}
	if got.ServerVersion == nil || *got.ServerVersion != "v2.0.0" {
		t.Errorf("serverVersion: got %v, want %q", got.ServerVersion, "v2.0.0")
	}
	reg.Verify(t)
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/cli/version/...
```

Expected: compile error — package does not exist yet.

- [ ] **Step 3: Create `internal/cli/version/client.go`**

```go
package version

import (
	"context"
	"net/http"

	metaapi "github.com/bwilczynski/hlctl/internal/api/meta"
)

// MetaClient is the interface used by the version command.
type MetaClient interface {
	GetMetaVersionWithResponse(ctx context.Context, reqEditors ...metaapi.RequestEditorFn) (*metaapi.GetMetaVersionResponse, error)
}

// NewMetaClient constructs a MetaClient backed by the real API.
func NewMetaClient(httpClient *http.Client, apiURL string) (MetaClient, error) {
	return metaapi.NewClientWithResponses(apiURL, metaapi.WithHTTPClient(httpClient))
}
```

- [ ] **Step 4: Create `internal/cli/version/version.go`**

```go
package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

type versionOptions struct {
	ClientVersion string
	ClientSpec    string
	ClientOnly    bool
	HTTPClient    func() (*http.Client, string, error)
	IO            *cmdutil.IOStreams
	Output        func() output.Format
}

type versionOutput struct {
	ClientVersion string  `json:"clientVersion"`
	ClientSpec    string  `json:"clientSpec"`
	ServerVersion *string `json:"serverVersion,omitempty"`
	ServerSpec    *string `json:"serverSpec,omitempty"`
}

// NewCmd returns the `hlctl version` command.
func NewCmd(f *cmdutil.Factory, runF func(*versionOptions) error) *cobra.Command {
	opts := &versionOptions{
		ClientVersion: f.Version,
		ClientSpec:    f.SpecVersion,
		HTTPClient:    f.HTTPClient,
		IO:            f.IOStreams,
		Output:        f.Output,
	}
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show client and server version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return getVersionRun(cmd.Context(), cmd.OutOrStdout(), opts)
		},
	}
	cmd.Flags().BoolVar(&opts.ClientOnly, "client", false, "Show client version only (no server request)")
	return cmd
}

func getVersionRun(ctx context.Context, w io.Writer, opts *versionOptions) error {
	var serverVersion, serverSpec string

	if !opts.ClientOnly {
		if httpClient, apiURL, err := opts.HTTPClient(); err != nil {
			fmt.Fprintf(opts.IO.ErrOut, "warning: could not reach server: %v\n", err)
		} else if c, err := NewMetaClient(httpClient, apiURL); err != nil {
			fmt.Fprintf(opts.IO.ErrOut, "warning: could not reach server: %v\n", err)
		} else if resp, err := c.GetMetaVersionWithResponse(ctx); err != nil {
			fmt.Fprintf(opts.IO.ErrOut, "warning: could not reach server: %v\n", err)
		} else if resp.JSON200 != nil {
			serverVersion = resp.JSON200.ServerVersion
			serverSpec = resp.JSON200.ApiVersion
		}
	}

	if opts.Output() == output.FormatJSON {
		out := versionOutput{
			ClientVersion: opts.ClientVersion,
			ClientSpec:    opts.ClientSpec,
		}
		if serverVersion != "" {
			out.ServerVersion = &serverVersion
			out.ServerSpec = &serverSpec
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Client version:\t%s\n", opts.ClientVersion)
	fmt.Fprintf(tw, "Client spec:\t%s\n", opts.ClientSpec)
	if !opts.ClientOnly {
		if serverVersion != "" {
			fmt.Fprintf(tw, "Server version:\t%s\n", serverVersion)
			fmt.Fprintf(tw, "Server spec:\t%s\n", serverSpec)
		} else {
			fmt.Fprintf(tw, "Server version:\t(unavailable)\n")
			fmt.Fprintf(tw, "Server spec:\t(unavailable)\n")
		}
	}
	return tw.Flush()
}
```

- [ ] **Step 5: Add missing `fmt` import to test file**

The test file references `fmt.Errorf` — add `"fmt"` to the imports in `version_test.go`.

- [ ] **Step 6: Run tests and verify they pass**

```bash
go test ./internal/cli/version/...
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/version/
git commit -m "feat: implement hlctl version command"
```

---

### Task 5: Wire the version command into root

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/root_test.go`

- [ ] **Step 1: Register the command in `root.go`**

```go
package cli

import (
	"github.com/bwilczynski/hlctl/internal/cli/auth"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/config"
	dockercli "github.com/bwilczynski/hlctl/internal/cli/docker"
	"github.com/bwilczynski/hlctl/internal/cli/network"
	"github.com/bwilczynski/hlctl/internal/cli/storage"
	"github.com/bwilczynski/hlctl/internal/cli/system"
	versioncli "github.com/bwilczynski/hlctl/internal/cli/version"
	"github.com/spf13/cobra"
)

func NewRootCmd(f *cmdutil.Factory) *cobra.Command {
	root := &cobra.Command{
		Use:          "hlctl",
		Short:        "CLI for controlling homelab services",
		Long:         "hlctl is a command-line interface for managing your homelab infrastructure via the Homelab API.",
		Version:      f.Version,
		SilenceUsage: true,
	}
	root.SetOut(f.IOStreams.Out)
	root.SetErr(f.IOStreams.ErrOut)
	root.SetIn(f.IOStreams.In)
	root.AddCommand(
		auth.NewCmd(f),
		config.NewCmd(f),
		dockercli.NewCmd(f),
		network.NewCmd(f),
		storage.NewCmd(f),
		system.NewCmd(f),
		versioncli.NewCmd(f, nil),
	)
	return root
}
```

- [ ] **Step 2: Add a smoke test to `root_test.go`**

Add this test after the existing `TestNewRootCmd_version` test:

```go
func TestNewRootCmd_versionSubcommand(t *testing.T) {
	var buf bytes.Buffer
	f := cmdutil.TestFactory(t)
	f.Version = "v1.0.0"
	f.SpecVersion = "1.1.0"
	f.IOStreams.Out = &buf

	root := NewRootCmd(f)
	root.SetArgs([]string{"version", "--client"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	for _, want := range []string{"v1.0.0", "1.1.0"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("expected %q in version output, got: %s", want, buf.String())
		}
	}
}
```

- [ ] **Step 3: Run all tests**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/root.go internal/cli/root_test.go
git commit -m "feat: register hlctl version command on root"
```

---

### Task 6: Embed spec version at build time

**Files:**
- Modify: `Makefile`
- Modify: `.goreleaser.yaml`
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Update `Makefile` build target**

Add the `SPEC_VERSION` variable and pass it as ldflag in `build`:

```makefile
SPEC_REPO    := spec
SPEC_FILE    := $(SPEC_REPO)/dist/openapi.bundled.yaml
BINARY       := bin/hlctl
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
SPEC_VERSION := $(shell grep '^  version:' $(SPEC_FILE) | awk '{print $$2}')

.PHONY: help build generate bundle lint test tidy

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the hlctl binary
	go build -ldflags "-X main.specVersion=$(SPEC_VERSION)" -o $(BINARY) ./cmd/hlctl

generate: bundle ## Generate client code from the bundled spec
	@mkdir -p internal/api/system internal/api/docker internal/api/storage internal/api/network internal/api/meta
	$(OAPI_CODEGEN) --config codegen/system.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/docker.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/storage.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/network.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config codegen/meta.yaml $(SPEC_FILE)

bundle: ## Bundle the OpenAPI spec from the submodule
	$(MAKE) -C $(SPEC_REPO) bundle

lint: ## Run go vet
	go vet ./...

test: ## Run tests
	go test ./...

tidy: ## Tidy go.mod
	go mod tidy
```

- [ ] **Step 2: Verify `make build` embeds the spec version**

```bash
make build && ./bin/hlctl version --client
```

Expected output contains `1.1.0` on the `Client spec` line.

- [ ] **Step 3: Update `.goreleaser.yaml` ldflags**

Change the `ldflags` line in `.goreleaser.yaml`:

```yaml
ldflags:
  - -s -w -X main.version={{.Version}} -X main.specVersion={{.Env.SPEC_VERSION}}
```

- [ ] **Step 4: Add `SPEC_VERSION` extraction step to `.github/workflows/release.yml`**

In the `build` job, add a step between "Generate API client code" and the goreleaser step:

```yaml
      - name: Generate API client code
        run: make generate

      - name: Extract spec version
        run: echo "SPEC_VERSION=$(grep '^  version:' spec/dist/openapi.bundled.yaml | awk '{print $2}')" >> $GITHUB_ENV

      - uses: goreleaser/goreleaser-action@v6
```

- [ ] **Step 5: Commit**

```bash
git add Makefile .goreleaser.yaml .github/workflows/release.yml
git commit -m "feat: embed spec version at build time via ldflag"
```

---

## Self-Review

**Spec coverage:**
- ✅ `hlctl version` command with four-field table output
- ✅ `--client` flag skips server fetch
- ✅ Server unavailable → graceful degradation with `(unavailable)` + warning
- ✅ `--output=json` support
- ✅ Client spec version embedded at build time (Makefile + goreleaser + release.yml)
- ✅ oapi-codegen meta domain
- ✅ CLAUDE.md InjectClient cleanup
- ✅ `SpecVersion` added to Factory and TestFactory

**Placeholder scan:** None found.

**Type consistency:**
- `versionOptions` defined in Task 4, referenced consistently in tests and `NewCmd`
- `versionOutput` defined and used only in Task 4
- `MetaClient` interface defined in `client.go`, used in `version.go` — both in Task 4
- `metaapi.Version.ApiVersion` / `metaapi.ServerVersion` — field names come from the generated code; verified against the spec schema (`apiVersion`, `serverVersion`) and oapi-codegen's camelCase convention (`ApiVersion`, `ServerVersion`)
- `NewCmd(f, nil)` called in Task 5 — matches signature `NewCmd(f *cmdutil.Factory, runF func(*versionOptions) error)` from Task 4
- `f.SpecVersion` in Task 5 root test — field added in Task 3
