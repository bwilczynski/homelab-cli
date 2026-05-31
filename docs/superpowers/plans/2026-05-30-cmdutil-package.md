# `cmdutil` Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a small `internal/cli/cmdutil` package of four helpers and migrate the four CLI domains (`docker`, `storage`, `network`, `system`) to use it, removing ~250 lines of repeated boilerplate per domain.

**Architecture:** A new `cmdutil` package provides: (1) generic client injection via Cobra `PersistentPreRunE` on the domain parent + context-value lookup in leaves, (2) a `View` value type that binds `templates fs.FS + template name + expected status` and renders the standard "status check → JSON-or-template" branch, (3) an `ActionCmd[C]` factory for `<verb> <id>` commands that expect 204, and (4) a `DeviceFlag` helper standardizing the recurring `--device` flag.

**Tech Stack:** Go 1.22+ generics, Cobra, oapi-codegen `ClientWithResponses`, `testing/fstest` for template-FS test fixtures.

**Spec:** `docs/superpowers/specs/2026-05-30-cmdutil-package-design.md`

---

## File Map

| File | Change |
|------|--------|
| `internal/cli/cmdutil/flags.go` | New — `DeviceFlag` helper |
| `internal/cli/cmdutil/flags_test.go` | New — test flag registration |
| `internal/cli/cmdutil/view.go` | New — `View` type + `Render` method |
| `internal/cli/cmdutil/view_test.go` | New — test all three render branches |
| `internal/cli/cmdutil/client.go` | New — `InjectClient[C]`, `Client[C]`, `SetClient[C]` |
| `internal/cli/cmdutil/client_test.go` | New — test injection chain + test-helper seeding |
| `internal/cli/cmdutil/action.go` | New — `ActionCmd[C]` factory |
| `internal/cli/cmdutil/action_test.go` | New — test success and error paths |
| `internal/cli/docker/docker.go` | Migrate — use cmdutil helpers, drop boilerplate |
| `internal/cli/docker/docker_test.go` | Migrate — use `cmdutil.SetClient` instead of constructor arg |
| `internal/cli/storage/storage.go` | Migrate — same shape as docker |
| `internal/cli/storage/storage_test.go` | Migrate — same shape |
| `internal/cli/network/network.go` | Migrate — devices commands |
| `internal/cli/network/ssids.go` | Migrate |
| `internal/cli/network/vlans.go` | Migrate |
| `internal/cli/network/wans.go` | Migrate |
| `internal/cli/network/network_test.go` | Migrate — all stub seedings |
| `internal/cli/system/system.go` | Migrate |
| `internal/cli/system/system_test.go` | Migrate |
| `CLAUDE.md` | Update "Adding a New Domain Command" section |

---

## Task 1: Add `DeviceFlag` helper

**Files:**
- Create: `internal/cli/cmdutil/flags.go`
- Create: `internal/cli/cmdutil/flags_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/cli/cmdutil/flags_test.go`:

```go
package cmdutil_test

import (
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

func TestDeviceFlag_registersFlagAndReturnsPointer(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	device := cmdutil.DeviceFlag(cmd)

	if device == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *device != "" {
		t.Errorf("expected empty default, got %q", *device)
	}

	if err := cmd.Flags().Set("device", "nas-1"); err != nil {
		t.Fatalf("set device: %v", err)
	}
	if *device != "nas-1" {
		t.Errorf("expected pointer to track flag value, got %q", *device)
	}
}

func TestDeviceFlag_helpText(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmdutil.DeviceFlag(cmd)

	f := cmd.Flags().Lookup("device")
	if f == nil {
		t.Fatal("expected --device flag registered")
	}
	if f.Usage != "Filter by device ID" {
		t.Errorf("unexpected usage: %q", f.Usage)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/cmdutil/...`
Expected: build error (`cmdutil` package does not exist).

- [ ] **Step 3: Implement `flags.go`**

Create `internal/cli/cmdutil/flags.go`:

```go
package cmdutil

import "github.com/spf13/cobra"

// DeviceFlag registers --device on cmd and returns a pointer to the bound value.
func DeviceFlag(cmd *cobra.Command) *string {
	return cmd.Flags().String("device", "", "Filter by device ID")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/cmdutil/...`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/cli/cmdutil/flags.go internal/cli/cmdutil/flags_test.go
git commit -m "feat(cmdutil): add DeviceFlag helper"
```

---

## Task 2: Add `View` type with `Render` method

**Files:**
- Create: `internal/cli/cmdutil/view.go`
- Create: `internal/cli/cmdutil/view_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/cli/cmdutil/view_test.go`:

```go
package cmdutil_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
)

func fakeTemplates() fstest.MapFS {
	return fstest.MapFS{
		"greet.tmpl": &fstest.MapFile{Data: []byte("hello {{.Name}}\n")},
	}
}

type greet struct{ Name string }

func TestView_Render_table(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	var buf bytes.Buffer
	if err := v.Render(&buf, http.StatusOK, []byte(`{"name":"world"}`), greet{Name: "world"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "hello world\n" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestView_Render_json(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "json"

	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	var buf bytes.Buffer
	body := []byte(`{"name":"world"}`)
	if err := v.Render(&buf, http.StatusOK, body, greet{Name: "world"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if buf.String() != string(body) {
		t.Errorf("expected raw body in json mode, got %q", buf.String())
	}
}

func TestView_Render_statusMismatch_returnsParseError(t *testing.T) {
	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl"}
	body := []byte(`{"title":"Not Found","detail":"missing"}`)
	err := v.Render(&bytes.Buffer{}, http.StatusNotFound, body, nil)
	if err == nil {
		t.Fatal("expected error for non-OK status")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected ParseError output, got %v", err)
	}
}

func TestView_Render_customStatus(t *testing.T) {
	t.Cleanup(func() { flags.OutputFormat = "" })
	flags.OutputFormat = "table"

	v := cmdutil.View{Templates: fakeTemplates(), Name: "greet.tmpl", Status: http.StatusCreated}
	var buf bytes.Buffer
	if err := v.Render(&buf, http.StatusCreated, []byte(`{"name":"new"}`), greet{Name: "new"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "hello new\n" {
		t.Errorf("unexpected output: %q", got)
	}

	// 200 should now be treated as a mismatch.
	err := v.Render(&bytes.Buffer{}, http.StatusOK, []byte(`{"title":"oops"}`), nil)
	if err == nil {
		t.Fatal("expected error when status differs from configured Status")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/cmdutil/...`
Expected: build error (`View` undefined).

- [ ] **Step 3: Implement `view.go`**

Create `internal/cli/cmdutil/view.go`:

```go
package cmdutil

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
)

// View binds a template filesystem, a template name, and the expected success
// status code. Domains declare one View per renderable response and reuse it
// across the command(s) that produce it.
//
// Status is optional and defaults to http.StatusOK. Set it explicitly for
// endpoints that return 201, 202, etc. — the value pairs with the JSON field
// the caller passes (a 201 endpoint populates resp.JSON201, not resp.JSON200).
type View struct {
	Templates fs.FS
	Name      string
	Status    int
}

// Render handles the standard response→output flow:
//   - status != v.Status (or 200 if unset) → apiclient.ParseError
//   - --output=json → write raw body
//   - otherwise → render the bound template against data
func (v View) Render(w io.Writer, statusCode int, body []byte, data any) error {
	expected := v.Status
	if expected == 0 {
		expected = http.StatusOK
	}
	if statusCode != expected {
		return apiclient.ParseError(statusCode, body)
	}
	if flags.GetOutputFormat() == output.FormatJSON {
		fmt.Fprint(w, string(body))
		return nil
	}
	return output.RenderTemplate(w, v.Templates, v.Name, data)
}
```

- [ ] **Step 4: Run vet**

Run: `go vet ./internal/cli/cmdutil/...`
Expected: no output (success).

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/cli/cmdutil/...`
Expected: PASS (6 tests: 2 from Task 1, 4 from Task 2).

- [ ] **Step 6: Commit**

```bash
git add internal/cli/cmdutil/view.go internal/cli/cmdutil/view_test.go
git commit -m "feat(cmdutil): add View with template-bound response renderer"
```

---

## Task 3: Add `InjectClient`, `Client`, `SetClient`

**Files:**
- Create: `internal/cli/cmdutil/client.go`
- Create: `internal/cli/cmdutil/client_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/cli/cmdutil/client_test.go`:

```go
package cmdutil_test

import (
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

type fakeClient struct{ name string }

func TestInjectClient_setsContextValueOnLeafExecution(t *testing.T) {
	parent := &cobra.Command{Use: "parent"}
	cmdutil.InjectClient(parent, func() (*fakeClient, error) {
		return &fakeClient{name: "real"}, nil
	})

	var seen *fakeClient
	leaf := &cobra.Command{
		Use: "leaf",
		RunE: func(cmd *cobra.Command, _ []string) error {
			seen = cmdutil.Client[*fakeClient](cmd)
			return nil
		},
	}
	parent.AddCommand(leaf)

	parent.SetArgs([]string{"leaf"})
	if err := parent.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if seen == nil || seen.name != "real" {
		t.Errorf("expected injected client, got %+v", seen)
	}
}

func TestInjectClient_propagatesBuildError(t *testing.T) {
	parent := &cobra.Command{Use: "parent", SilenceUsage: true, SilenceErrors: true}
	cmdutil.InjectClient(parent, func() (*fakeClient, error) {
		return nil, errBoom
	})
	leaf := &cobra.Command{Use: "leaf", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	parent.AddCommand(leaf)

	parent.SetArgs([]string{"leaf"})
	err := parent.Execute()
	if err == nil || err.Error() != "boom" {
		t.Errorf("expected build error to propagate, got %v", err)
	}
}

func TestSetClient_seedsContextForLeafStandalone(t *testing.T) {
	var seen *fakeClient
	leaf := &cobra.Command{
		Use: "leaf",
		RunE: func(cmd *cobra.Command, _ []string) error {
			seen = cmdutil.Client[*fakeClient](cmd)
			return nil
		},
	}
	cmdutil.SetClient[*fakeClient](leaf, &fakeClient{name: "stub"})

	if err := leaf.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if seen == nil || seen.name != "stub" {
		t.Errorf("expected seeded stub, got %+v", seen)
	}
}

func TestSetClient_preservesExistingContextValues(t *testing.T) {
	type otherKey struct{}
	leaf := &cobra.Command{Use: "leaf"}
	ctx := context.WithValue(context.Background(), otherKey{}, "kept")
	leaf.SetContext(ctx)

	cmdutil.SetClient[*fakeClient](leaf, &fakeClient{name: "stub"})

	if got := leaf.Context().Value(otherKey{}); got != "kept" {
		t.Errorf("expected pre-existing context value preserved, got %v", got)
	}
	if cmdutil.Client[*fakeClient](leaf).name != "stub" {
		t.Error("expected client also seeded")
	}
}

var errBoom = stringError("boom")

type stringError string

func (e stringError) Error() string { return string(e) }
```

Add `"context"` to the import block of the test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/cmdutil/...`
Expected: build error (`InjectClient`, `Client`, `SetClient` undefined).

- [ ] **Step 3: Implement `client.go`**

Create `internal/cli/cmdutil/client.go`:

```go
package cmdutil

import (
	"context"

	"github.com/spf13/cobra"
)

type clientKey[C any] struct{}

// InjectClient registers a PersistentPreRunE on cmd that builds a client and
// stores it on the executing command's context. Leaf commands retrieve it via
// Client[C]. The PreRunE fires after flags are parsed and only when a real
// subcommand runs (not on --help/--version), so flag-dependent construction
// and disk I/O stay deferred until actually needed.
func InjectClient[C any](cmd *cobra.Command, build func() (C, error)) {
	cmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		c, err := build()
		if err != nil {
			return err
		}
		cmd.SetContext(context.WithValue(cmd.Context(), clientKey[C]{}, c))
		return nil
	}
}

// Client returns the client previously injected for type C. Panics if no
// client is set — callers should always run under an InjectClient ancestor
// or after a SetClient call.
func Client[C any](cmd *cobra.Command) C {
	return cmd.Context().Value(clientKey[C]{}).(C)
}

// SetClient layers a client value onto cmd's existing context. Intended for
// tests that exercise a leaf command directly (without its real parent's
// PersistentPreRunE chain). Preserves any pre-existing context values.
func SetClient[C any](cmd *cobra.Command, c C) {
	cmd.SetContext(context.WithValue(cmd.Context(), clientKey[C]{}, c))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/cmdutil/...`
Expected: PASS (10 tests total).

- [ ] **Step 5: Commit**

```bash
git add internal/cli/cmdutil/client.go internal/cli/cmdutil/client_test.go
git commit -m "feat(cmdutil): add generic client injection via cobra context"
```

---

## Task 4: Add `ActionCmd[C]` factory

**Files:**
- Create: `internal/cli/cmdutil/action.go`
- Create: `internal/cli/cmdutil/action_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/cli/cmdutil/action_test.go`:

```go
package cmdutil_test

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
)

func TestActionCmd_success_printsMessage(t *testing.T) {
	cmd := cmdutil.ActionCmd[*fakeClient]("start <id>", "Start it", "started",
		func(c *fakeClient, ctx context.Context, id string) (int, []byte, error) {
			if c == nil || ctx == nil || id == "" {
				t.Fatal("exec called with unexpected zero args")
			}
			return http.StatusNoContent, nil, nil
		})
	cmdutil.SetClient[*fakeClient](cmd, &fakeClient{name: "ok"})

	cmd.SetArgs([]string{"abc-1"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := "abc-1 started\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestActionCmd_nonNoContent_returnsParseError(t *testing.T) {
	cmd := cmdutil.ActionCmd[*fakeClient]("stop <id>", "Stop it", "stopped",
		func(*fakeClient, context.Context, string) (int, []byte, error) {
			return http.StatusInternalServerError, []byte(`{"title":"Boom"}`), nil
		})
	cmdutil.SetClient[*fakeClient](cmd, &fakeClient{})
	cmd.SetArgs([]string{"abc-1"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "Boom") {
		t.Errorf("expected ParseError with 'Boom', got %v", err)
	}
}

func TestActionCmd_execError_propagates(t *testing.T) {
	cmd := cmdutil.ActionCmd[*fakeClient]("restart <id>", "Restart it", "restarted",
		func(*fakeClient, context.Context, string) (int, []byte, error) {
			return 0, nil, errBoom
		})
	cmdutil.SetClient[*fakeClient](cmd, &fakeClient{})
	cmd.SetArgs([]string{"abc-1"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil || err.Error() != "boom" {
		t.Errorf("expected boom, got %v", err)
	}
}

func TestActionCmd_requiresExactlyOneArg(t *testing.T) {
	cmd := cmdutil.ActionCmd[*fakeClient]("start <id>", "Start it", "started",
		func(*fakeClient, context.Context, string) (int, []byte, error) { return 204, nil, nil })
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err == nil {
		t.Error("expected error when no arg given")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/cmdutil/...`
Expected: build error (`ActionCmd` undefined).

- [ ] **Step 3: Implement `action.go`**

Create `internal/cli/cmdutil/action.go`:

```go
package cmdutil

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/spf13/cobra"
)

// ActionCmd builds a Cobra command of the form `<verb> <id>` that calls exec
// with the resolved client and the positional id, asserts a 204 No Content
// response, and prints "<id> <pastTense>" on success.
func ActionCmd[C any](use, short, pastTense string, exec func(c C, ctx context.Context, id string) (int, []byte, error)) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code, body, err := exec(Client[C](cmd), cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if code != http.StatusNoContent {
				return apiclient.ParseError(code, body)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", args[0], pastTense)
			return nil
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/cmdutil/...`
Expected: PASS (14 tests total).

- [ ] **Step 5: Commit**

```bash
git add internal/cli/cmdutil/action.go internal/cli/cmdutil/action_test.go
git commit -m "feat(cmdutil): add ActionCmd factory for 204 no-body commands"
```

---

## Task 5: Migrate `docker` domain

**Files:**
- Modify: `internal/cli/docker/docker.go` (rewrite)
- Modify: `internal/cli/docker/docker_test.go` (update stub seeding)

This is a refactor — the existing test suite is the safety net. Run `go test ./internal/cli/docker/...` before starting to capture the baseline (all tests should pass).

- [ ] **Step 1: Capture baseline**

Run: `go test ./internal/cli/docker/...`
Expected: PASS.

- [ ] **Step 2: Rewrite `docker.go`**

Replace the entire contents of `internal/cli/docker/docker.go`:

```go
package docker

import (
	"context"
	"io"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	gen "github.com/bwilczynski/hlctl/internal/docker"
	"github.com/spf13/cobra"
)

var (
	containersListView = cmdutil.View{Templates: dockerTemplates, Name: "containers_list.tmpl"}
	containersGetView  = cmdutil.View{Templates: dockerTemplates, Name: "containers_get.tmpl"}
	networksListView   = cmdutil.View{Templates: dockerTemplates, Name: "networks_list.tmpl"}
	networksGetView    = cmdutil.View{Templates: dockerTemplates, Name: "networks_get.tmpl"}
	imagesListView     = cmdutil.View{Templates: dockerTemplates, Name: "images_list.tmpl"}
	imagesGetView      = cmdutil.View{Templates: dockerTemplates, Name: "images_get.tmpl"}
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "docker", Short: "Docker resources"}
	cmd.AddCommand(newContainersCmd(), newNetworksCmd(), newImagesCmd())
	return cmd
}

func buildClient() (DockerClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewDockerClient(httpClient, apiURL)
}

// --- containers ---

func newContainersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "containers", Short: "Manage Docker containers"}
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newListCmd(), newGetCmd(), newStartCmd(), newStopCmd(), newRestartCmd())
	return cmd
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List containers"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
		params := &gen.ListContainersParams{}
		if *device != "" {
			params.Device = device
		}
		resp, err := cmdutil.Client[DockerClient](cmd).ListContainersWithResponse(ctx, params)
		if err != nil {
			return err
		}
		return containersListView.Render(w, resp.StatusCode(), resp.Body, resp.JSON200)
	})
	watch.RegisterFlags(cmd)
	return cmd
}

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "get <container-id>", Short: "Show container details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[DockerClient](cmd).GetContainerWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return containersGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newStartCmd() *cobra.Command {
	return cmdutil.ActionCmd("start <container-id>", "Start a container", "started",
		func(c DockerClient, ctx context.Context, id string) (int, []byte, error) {
			r, err := c.StartContainerWithResponse(ctx, id, &gen.StartContainerParams{})
			if err != nil {
				return 0, nil, err
			}
			return r.StatusCode(), r.Body, nil
		})
}

func newStopCmd() *cobra.Command {
	return cmdutil.ActionCmd("stop <container-id>", "Stop a container", "stopped",
		func(c DockerClient, ctx context.Context, id string) (int, []byte, error) {
			r, err := c.StopContainerWithResponse(ctx, id, &gen.StopContainerParams{})
			if err != nil {
				return 0, nil, err
			}
			return r.StatusCode(), r.Body, nil
		})
}

func newRestartCmd() *cobra.Command {
	return cmdutil.ActionCmd("restart <container-id>", "Restart a container", "restarted",
		func(c DockerClient, ctx context.Context, id string) (int, []byte, error) {
			r, err := c.RestartContainerWithResponse(ctx, id, &gen.RestartContainerParams{})
			if err != nil {
				return 0, nil, err
			}
			return r.StatusCode(), r.Body, nil
		})
}

// --- networks ---

func newNetworksCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "networks", Short: "Docker networks"}
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newListNetworksCmd(), newGetNetworkCmd())
	return cmd
}

func newListNetworksCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List Docker networks"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		params := &gen.ListDockerNetworksParams{}
		if *device != "" {
			params.Device = device
		}
		resp, err := cmdutil.Client[DockerClient](cmd).ListDockerNetworksWithResponse(cmd.Context(), params)
		if err != nil {
			return err
		}
		return networksListView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newGetNetworkCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "get <network-id>", Short: "Show network details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[DockerClient](cmd).GetDockerNetworkWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return networksGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

// --- images ---

func newImagesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "images", Short: "Docker images"}
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newListImagesCmd(), newGetImageCmd())
	return cmd
}

func newListImagesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List Docker images"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		params := &gen.ListDockerImagesParams{}
		if *device != "" {
			params.Device = device
		}
		resp, err := cmdutil.Client[DockerClient](cmd).ListDockerImagesWithResponse(cmd.Context(), params)
		if err != nil {
			return err
		}
		return imagesListView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newGetImageCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "get <image-id>", Short: "Show image details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[DockerClient](cmd).GetDockerImageWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return imagesGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}
```

- [ ] **Step 3: Update `docker_test.go` to use `SetClient`**

In `internal/cli/docker/docker_test.go`, every test currently calls a constructor with the stub, e.g. `newListCmd(stub)`. Update every such call to:

```go
cmd := newListCmd()
cmdutil.SetClient[DockerClient](cmd, stub)
```

For action commands (`newStartCmd`, `newStopCmd`, `newRestartCmd`), same change. Add the import:

```go
"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
```

Apply the same two-line replacement to all 9 test cases in the file. No fixture changes; the `okFooResp` / `errFooResp` / `noContentFooResp` helpers stay identical.

- [ ] **Step 4: Build and test**

Run: `go build ./... && go vet ./... && go test ./internal/cli/docker/...`
Expected: PASS — all 9 docker tests still green.

- [ ] **Step 5: Sanity-check via the binary**

Run: `make build && ./bin/hlctl docker --help`
Expected: subcommands `containers`, `networks`, `images` listed without errors.

Run: `./bin/hlctl docker containers --help`
Expected: subcommands `list`, `get`, `start`, `stop`, `restart` listed.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/docker/docker.go internal/cli/docker/docker_test.go
git commit -m "refactor(docker): migrate to cmdutil helpers"
```

---

## Task 6: Migrate `storage` domain

**Files:**
- Modify: `internal/cli/storage/storage.go` (rewrite)
- Modify: `internal/cli/storage/storage_test.go` (update stub seeding)

- [ ] **Step 1: Capture baseline**

Run: `go test ./internal/cli/storage/...`
Expected: PASS.

- [ ] **Step 2: Rewrite `storage.go`**

Replace the entire contents of `internal/cli/storage/storage.go`:

```go
package storage

import (
	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	gen "github.com/bwilczynski/hlctl/internal/storage"
	"github.com/spf13/cobra"
)

var (
	volumesListView = cmdutil.View{Templates: storageTemplates, Name: "volumes_list.tmpl"}
	volumesGetView  = cmdutil.View{Templates: storageTemplates, Name: "volumes_get.tmpl"}
	backupsListView = cmdutil.View{Templates: storageTemplates, Name: "backups_list.tmpl"}
	backupsGetView  = cmdutil.View{Templates: storageTemplates, Name: "backups_get.tmpl"}
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "storage", Short: "NAS storage resources"}
	cmd.AddCommand(newVolumesCmd(), newBackupsCmd())
	return cmd
}

func buildClient() (StorageClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewStorageClient(httpClient, apiURL)
}

// --- volumes ---

func newVolumesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "volumes", Short: "Storage volumes"}
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newListVolumesCmd(), newGetVolumeCmd())
	return cmd
}

func newListVolumesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List storage volumes"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		params := &gen.ListStorageVolumesParams{}
		if *device != "" {
			params.Device = device
		}
		resp, err := cmdutil.Client[StorageClient](cmd).ListStorageVolumesWithResponse(cmd.Context(), params)
		if err != nil {
			return err
		}
		return volumesListView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newGetVolumeCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "get <volume-id>", Short: "Show volume details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[StorageClient](cmd).GetStorageVolumeWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return volumesGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

// --- backups ---

func newBackupsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "backups", Short: "Backup tasks and history"}
	cmdutil.InjectClient(cmd, buildClient)
	cmd.AddCommand(newListBackupsCmd(), newGetBackupCmd())
	return cmd
}

func newListBackupsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List backups"}
	device := cmdutil.DeviceFlag(cmd)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		params := &gen.ListBackupsParams{}
		if *device != "" {
			params.Device = device
		}
		resp, err := cmdutil.Client[StorageClient](cmd).ListBackupsWithResponse(cmd.Context(), params)
		if err != nil {
			return err
		}
		return backupsListView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}

func newGetBackupCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "get <backup-id>", Short: "Show backup details", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := cmdutil.Client[StorageClient](cmd).GetBackupWithResponse(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return backupsGetView.Render(cmd.OutOrStdout(), resp.StatusCode(), resp.Body, resp.JSON200)
	}
	return cmd
}
```

Note: if any existing list command in `storage.go` uses `watch.Wrap`, replicate the docker pattern (`cmd.RunE = watch.Wrap(...)` + `watch.RegisterFlags(cmd)`). Verify by reading the pre-migration file first; the snippet above assumes no watch usage in storage list commands. If it has watch wiring, mirror the docker `newListCmd` shape from Task 5.

- [ ] **Step 3: Update `storage_test.go` to use `SetClient`**

For every `newFooCmd(stub)` call in `internal/cli/storage/storage_test.go`, replace with:

```go
cmd := newFooCmd()
cmdutil.SetClient[StorageClient](cmd, stub)
```

Add the import `"github.com/bwilczynski/hlctl/internal/cli/cmdutil"`.

- [ ] **Step 4: Build and test**

Run: `go build ./... && go vet ./... && go test ./internal/cli/storage/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/storage/storage.go internal/cli/storage/storage_test.go
git commit -m "refactor(storage): migrate to cmdutil helpers"
```

---

## Task 7: Migrate `network` domain

**Files:**
- Modify: `internal/cli/network/network.go` (rewrite)
- Modify: `internal/cli/network/ssids.go` (rewrite)
- Modify: `internal/cli/network/vlans.go` (rewrite)
- Modify: `internal/cli/network/wans.go` (rewrite)
- Modify: `internal/cli/network/network_test.go` (update stub seedings)

This is the largest domain (4 files, 1249-line test file). The migration is mechanical — apply the same transformations as docker/storage.

- [ ] **Step 1: Capture baseline**

Run: `go test ./internal/cli/network/...`
Expected: PASS.

- [ ] **Step 2: Inventory the file**

Read each file and list:
- Each leaf command (name, current signature, body shape)
- Each parent (`newDevicesCmd`, `newSsidsCmd`, etc.) and which leaves it contains
- Which list commands use `watch.Wrap`
- Which leaves use `cmd.OutOrStdout()` vs a passed `w` (the watch-wrapped ones use `w`)
- All `<resource>_list.tmpl` / `<resource>_get.tmpl` template names per file

Use the docker migration in Task 5 as the canonical reference for each shape (list with watch, list without watch, get).

- [ ] **Step 3: Rewrite each file**

For each of `network.go`, `ssids.go`, `vlans.go`, `wans.go`:

1. Add the import `"github.com/bwilczynski/hlctl/internal/cli/cmdutil"`.
2. Declare a `View` for each `*.tmpl` used in the file. Place them in a `var (...)` block at the top of the file. Use names of the form `<resource><Action>View`, e.g. `vlansListView`, `vlansGetView`.
3. In each parent command, add `cmdutil.InjectClient(cmd, buildClient)` immediately after the parent is constructed and before `cmd.AddCommand(...)`.
4. Remove the `client NetworkClient` parameter from each leaf constructor; remove the `c := client; if c == nil { ... }` block from each `RunE`.
5. Replace each leaf's `client.<Method>` call site with `cmdutil.Client[NetworkClient](cmd).<Method>`.
6. Replace each leaf's 6-line status-check + JSON/template branch with `return <view>.Render(w, resp.StatusCode(), resp.Body, resp.JSON200)` (use `cmd.OutOrStdout()` instead of `w` when not under `watch.Wrap`).
7. Replace each `--device` flag setup with `device := cmdutil.DeviceFlag(cmd)` and use `*device` where the variable was used before.
8. Update the parent's `cmd.AddCommand(newFooCmd(nil), ...)` to `cmd.AddCommand(newFooCmd(), ...)`.

If `network.go` shares `buildClient()` across multiple files in the package (single function), keep it in `network.go` only and have ssids/vlans/wans rely on the package-level function.

- [ ] **Step 4: Update `network_test.go`**

For every `newFooCmd(stub)` call across all tests in the file, replace with:

```go
cmd := newFooCmd()
cmdutil.SetClient[NetworkClient](cmd, stub)
```

Add the import `"github.com/bwilczynski/hlctl/internal/cli/cmdutil"`.

- [ ] **Step 5: Build and test**

Run: `go build ./... && go vet ./... && go test ./internal/cli/network/...`
Expected: PASS.

- [ ] **Step 6: Sanity-check the binary**

Run: `./bin/hlctl network --help && ./bin/hlctl network devices --help && ./bin/hlctl network vlans --help && ./bin/hlctl network ssids --help && ./bin/hlctl network wans --help`
Expected: every subcommand listed, no errors.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/network/
git commit -m "refactor(network): migrate to cmdutil helpers"
```

---

## Task 8: Migrate `system` domain

**Files:**
- Modify: `internal/cli/system/system.go` (rewrite)
- Modify: `internal/cli/system/system_test.go` (update stub seedings)

- [ ] **Step 1: Capture baseline**

Run: `go test ./internal/cli/system/...`
Expected: PASS.

- [ ] **Step 2: Rewrite `system.go`**

Follow the exact same transformation pattern as Tasks 5–7:

1. Add `import "github.com/bwilczynski/hlctl/internal/cli/cmdutil"`.
2. Declare a `View` per template at the top of the file.
3. Add `cmdutil.InjectClient(cmd, buildClient)` to each parent command after construction.
4. Remove the `client SystemClient` parameter from every leaf and the nil-check block from every `RunE`.
5. Replace API calls with `cmdutil.Client[SystemClient](cmd).<Method>(cmd.Context(), ...)`.
6. Replace status-check + render branches with `<view>.Render(...)`.
7. Standardize `--device` setup via `cmdutil.DeviceFlag(cmd)` where the flag exists.

Mirror the docker `newListCmd`/`newGetCmd`/`newActionCmd` shapes from Task 5.

- [ ] **Step 3: Update `system_test.go`**

Same mechanical replacement as docker/storage/network. Add the `cmdutil` import.

- [ ] **Step 4: Build and test**

Run: `go build ./... && go vet ./... && go test ./internal/cli/system/...`
Expected: PASS.

- [ ] **Step 5: Run full suite**

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/system/
git commit -m "refactor(system): migrate to cmdutil helpers"
```

---

## Task 9: Update `CLAUDE.md`

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update "Adding a New Domain Command" section**

In `CLAUDE.md`, replace the existing numbered list under "Adding a New Domain Command" with:

```markdown
## Adding a New Domain Command

1. Create `internal/cli/<domain>/<domain>.go` with a `NewCmd() *cobra.Command` function.
2. Declare `View` values at the top of the file — one per template:
   ```go
   var fooListView = cmdutil.View{Templates: <domain>Templates, Name: "foo_list.tmpl"}
   ```
   Set `Status:` explicitly on the View only when the endpoint returns something other than 200.
3. Each parent command calls `cmdutil.InjectClient(cmd, buildClient)` after construction; leaf commands have no `client` parameter and call `cmdutil.Client[<Domain>Client](cmd).<Method>(...)` to retrieve it.
4. Leaf commands render with `<view>.Render(w, resp.StatusCode(), resp.Body, resp.JSON200)`.
5. List commands accepting a device filter use `device := cmdutil.DeviceFlag(cmd)`.
6. Start/stop/restart-style commands (204 No Content + success message) use `cmdutil.ActionCmd[<Domain>Client](use, short, pastTense, exec)`.
7. Register the new domain in `internal/cli/root.go` via `rootCmd.AddCommand(<domain>.NewCmd())`.
8. Tests construct leaves directly and seed the client via `cmdutil.SetClient[<Domain>Client](cmd, stub)`.
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md command-authoring guidance for cmdutil"
```

---

## Final verification

- [ ] **Run full test + vet**

Run: `go test ./... && go vet ./... && make build`
Expected: all green, binary built.

- [ ] **Smoke-test the binary**

Run:
```bash
./bin/hlctl --help
./bin/hlctl docker containers --help
./bin/hlctl storage volumes --help
./bin/hlctl network devices --help
./bin/hlctl system --help
```

Expected: every command tree resolves, help text renders.

- [ ] **Diff stats sanity check**

Run: `git diff --stat main...HEAD -- internal/cli/`

Expected: net negative LOC across the domain files; `internal/cli/docker/docker.go` shrinks from ~376 to ~130; comparable reductions in storage/network/system; offsetting ~100 LOC added in `internal/cli/cmdutil/`.
