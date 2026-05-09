# Docker Networks & Images Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `hlctl docker networks` and `hlctl docker images` subcommands backed by the new `/docker/networks` and `/docker/images` API endpoints.

**Architecture:** Regenerate `internal/docker/api.gen.go` from the updated spec submodule (already at `1bd1b54`), then expand the `ContainersClient` interface (renaming it to `DockerClient`), update `stub.go` and `client.go` accordingly, and add the new command functions to `docker.go`.

**Tech Stack:** Go, Cobra, oapi-codegen, `internal/docker` (generated), `internal/output`, `internal/apiclient`

---

## File Map

| File | Change |
|---|---|
| `internal/docker/api.gen.go` | Regenerated — gains `ListDockerNetworks`, `GetDockerNetwork`, `ListDockerImages`, `GetDockerImage` methods and associated types |
| `internal/cli/docker/client.go` | Rename `ContainersClient` → `DockerClient`; add 4 new method signatures; rename constructor |
| `internal/cli/docker/stub.go` | Rename struct + update interface reference; add 4 new stub func fields + delegation methods |
| `internal/cli/docker/docker.go` | Update `buildClient()` return type; add `newNetworksCmd()`, `newImagesCmd()`; register both in `NewCmd()` |
| `internal/cli/docker/docker_test.go` | Update stub references; add 4 new test functions |

---

### Task 1: Regenerate API client

**Files:**
- Modify: `internal/docker/api.gen.go` (regenerated, do not edit manually)

- [ ] **Step 1: Run make generate**

```bash
make generate
```

Expected: exits 0, `internal/docker/api.gen.go` updated with new methods and types.

- [ ] **Step 2: Verify new types exist**

```bash
grep -E "DockerNetwork|DockerImage" internal/docker/api.gen.go | head -30
```

Expected: lines containing `DockerNetworkList`, `DockerNetwork`, `DockerNetworkDetail`, `DockerImageList`, `DockerImage`, `DockerImageDetail`.

- [ ] **Step 3: Verify new client methods exist**

```bash
grep -E "ListDockerNetworks|GetDockerNetwork|ListDockerImages|GetDockerImage" internal/docker/api.gen.go
```

Expected: 4 matching lines — the method signatures on the generated client interface.

- [ ] **Step 4: Verify build still passes**

```bash
make build
```

Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
git add internal/docker/api.gen.go
git commit -m "chore: regenerate docker API client with networks and images"
```

---

### Task 2: Expand and rename the client interface

**Files:**
- Modify: `internal/cli/docker/client.go`

- [ ] **Step 1: Write the failing test (compile error)**

The existing tests reference `ContainersClient` and `NewContainersClient`. After this task those names will still exist — this task only touches `client.go`. Confirm the current tests pass before making changes:

```bash
go test ./internal/cli/docker/... -run . -count=1
```

Expected: PASS.

- [ ] **Step 2: Replace client.go content**

Replace the entire file `internal/cli/docker/client.go` with:

```go
package docker

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/docker"
)

// DockerClient is the interface used by all docker subcommands.
type DockerClient interface {
	ListContainers(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetContainer(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StartContainer(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StopContainer(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	RestartContainer(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListDockerNetworks(ctx context.Context, params *gen.ListDockerNetworksParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetDockerNetwork(ctx context.Context, networkId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListDockerImages(ctx context.Context, params *gen.ListDockerImagesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetDockerImage(ctx context.Context, imageId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

// NewDockerClient constructs a DockerClient backed by the real API.
func NewDockerClient(httpClient *http.Client, apiURL string) (DockerClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
```

- [ ] **Step 3: Verify build fails with expected errors**

```bash
go build ./internal/cli/docker/...
```

Expected: compile errors referencing `ContainersClient` and `NewContainersClient` (used in `stub.go` and `docker.go`). This confirms the rename needs to propagate.

---

### Task 3: Update stub

**Files:**
- Modify: `internal/cli/docker/stub.go`

- [ ] **Step 1: Replace stub.go content**

Replace the entire file `internal/cli/docker/stub.go` with:

```go
package docker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gen "github.com/bwilczynski/hlctl/internal/docker"
)

// StubClient is a DockerClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListContainersFunc    func(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetContainerFunc      func(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StartContainerFunc    func(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StopContainerFunc     func(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	RestartContainerFunc  func(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListDockerNetworksFunc func(ctx context.Context, params *gen.ListDockerNetworksParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetDockerNetworkFunc   func(ctx context.Context, networkId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListDockerImagesFunc   func(ctx context.Context, params *gen.ListDockerImagesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetDockerImageFunc     func(ctx context.Context, imageId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func (s *StubClient) ListContainers(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListContainersFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetContainer(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetContainerFunc(ctx, containerId, reqEditors...)
}

func (s *StubClient) StartContainer(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.StartContainerFunc(ctx, containerId, params, reqEditors...)
}

func (s *StubClient) StopContainer(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.StopContainerFunc(ctx, containerId, params, reqEditors...)
}

func (s *StubClient) RestartContainer(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.RestartContainerFunc(ctx, containerId, params, reqEditors...)
}

func (s *StubClient) ListDockerNetworks(ctx context.Context, params *gen.ListDockerNetworksParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListDockerNetworksFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetDockerNetwork(ctx context.Context, networkId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetDockerNetworkFunc(ctx, networkId, reqEditors...)
}

func (s *StubClient) ListDockerImages(ctx context.Context, params *gen.ListDockerImagesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListDockerImagesFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetDockerImage(ctx context.Context, imageId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetDockerImageFunc(ctx, imageId, reqEditors...)
}

// jsonResponse builds an *http.Response with a JSON body and the given status code.
func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}
```

- [ ] **Step 2: Verify build still has only docker.go errors**

```bash
go build ./internal/cli/docker/...
```

Expected: compile error referencing `ContainersClient` and `NewContainersClient` only in `docker.go` (stub errors now resolved).

---

### Task 4: Update docker.go — rename interface references and add new commands

**Files:**
- Modify: `internal/cli/docker/docker.go`

- [ ] **Step 1: Update buildClient and newContainersCmd to use DockerClient**

In `docker.go`, replace the `buildClient` function and update `newContainersCmd` command functions to use `DockerClient`:

Replace:
```go
func buildClient() (ContainersClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewContainersClient(httpClient, apiURL)
}
```

With:
```go
func buildClient() (DockerClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewDockerClient(httpClient, apiURL)
}
```

Also update each command function signature that takes a `ContainersClient` parameter — replace all occurrences of `ContainersClient` with `DockerClient` in `docker.go`. There are 5 functions: `newListCmd`, `newGetCmd`, `newStartCmd`, `newStopCmd`, `newRestartCmd`.

- [ ] **Step 2: Register networks and images subcommands in NewCmd**

Replace:
```go
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Docker resources",
	}
	cmd.AddCommand(newContainersCmd())
	return cmd
}
```

With:
```go
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Docker resources",
	}
	cmd.AddCommand(newContainersCmd())
	cmd.AddCommand(newNetworksCmd())
	cmd.AddCommand(newImagesCmd())
	return cmd
}
```

- [ ] **Step 3: Add newNetworksCmd**

Append to `docker.go`:

```go
func newNetworksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "networks",
		Short: "Docker networks",
	}
	cmd.AddCommand(newListNetworksCmd(nil))
	cmd.AddCommand(newGetNetworkCmd(nil))
	return cmd
}

func newListNetworksCmd(client DockerClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Docker networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListDockerNetworksParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListDockerNetworks(context.Background(), params)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return apiclient.ParseError(resp)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var list gen.DockerNetworkList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "NAME", "DEVICE", "CONTAINERS"}
			var rows [][]string
			for _, n := range list.Items {
				rows = append(rows, []string{
					n.Id, n.Name, n.Device,
					fmt.Sprintf("%d", n.ConnectedContainers),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newGetNetworkCmd(client DockerClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <network-id>",
		Short: "Show network details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.GetDockerNetwork(context.Background(), args[0])
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return apiclient.ParseError(resp)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var detail gen.DockerNetworkDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			return printNetworkDetail(cmd, detail)
		},
	}
}

func printNetworkDetail(cmd *cobra.Command, d gen.DockerNetworkDetail) error {
	w := cmd.OutOrStdout()

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", d.Id},
		{"NAME", d.Name},
		{"DEVICE", d.Device},
		{"DRIVER", d.Driver},
		{"CONTAINERS", fmt.Sprintf("%d", d.ConnectedContainers)},
	}
	// subnet and gateway are optional (no IPAM on host/macvlan networks)
	if d.Subnet != nil {
		rows = append(rows, []string{"SUBNET", *d.Subnet})
	}
	if d.Gateway != nil {
		rows = append(rows, []string{"GATEWAY", *d.Gateway})
	}
	if err := output.Print(w, output.FormatTable, nil, headers, rows); err != nil {
		return err
	}

	if len(d.Containers) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "CONNECTED CONTAINERS")
		var rows [][]string
		for _, name := range d.Containers {
			rows = append(rows, []string{name})
		}
		if err := output.Print(w, output.FormatTable, nil, []string{"NAME"}, rows); err != nil {
			return err
		}
	}

	return nil
}
```

- [ ] **Step 4: Add newImagesCmd**

First add `"strings"` to the import block in `docker.go` (images commands use `strings.Join`):

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/docker"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)
```

Then append to `docker.go`:

```go
func newImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images",
		Short: "Docker images",
	}
	cmd.AddCommand(newListImagesCmd(nil))
	cmd.AddCommand(newGetImageCmd(nil))
	return cmd
}

func newListImagesCmd(client DockerClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Docker images",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListDockerImagesParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListDockerImages(context.Background(), params)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return apiclient.ParseError(resp)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var list gen.DockerImageList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "DEVICE", "REPOSITORY", "TAGS", "SIZE"}
			var rows [][]string
			for _, img := range list.Items {
				rows = append(rows, []string{
					img.Id,
					img.Device,
					img.Repository,
					strings.Join(img.Tags, ", "),
					output.FormatBytes(img.Size),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newGetImageCmd(client DockerClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <image-id>",
		Short: "Show image details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.GetDockerImage(context.Background(), args[0])
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return apiclient.ParseError(resp)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			var detail gen.DockerImageDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			return printImageDetail(cmd, detail)
		},
	}
}

func printImageDetail(cmd *cobra.Command, d gen.DockerImageDetail) error {
	w := cmd.OutOrStdout()

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", d.Id},
		{"DEVICE", d.Device},
		{"REPOSITORY", d.Repository},
		{"TAGS", strings.Join(d.Tags, ", ")},
		{"SIZE", output.FormatBytes(d.Size)},
		{"VIRTUAL SIZE", output.FormatBytes(d.VirtualSize)},
	}
	if !d.Created.IsZero() {
		rows = append(rows, []string{"CREATED", output.FormatTime(d.Created)})
	}
	return output.Print(w, output.FormatTable, nil, headers, rows)
}
```

- [ ] **Step 5: Verify build passes**

```bash
make build
```

Expected: exits 0.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/docker/client.go internal/cli/docker/stub.go internal/cli/docker/docker.go
git commit -m "feat: add docker networks and images subcommands"
```

---

### Task 5: Add tests

**Files:**
- Modify: `internal/cli/docker/docker_test.go`

- [ ] **Step 1: Write failing tests**

Append to `docker_test.go`:

```go
func TestListNetworksCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListDockerNetworksFunc: func(_ context.Context, _ *gen.ListDockerNetworksParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.DockerNetworkList{
				Items: []gen.DockerNetwork{
					{
						Id:                  "nas-1.immich_default",
						Name:                "immich_default",
						Device:              "nas-1",
						ConnectedContainers: 4,
					},
				},
			}), nil
		},
	}

	cmd := newListNetworksCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.immich_default", "immich_default", "nas-1", "4"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetNetworkCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		GetDockerNetworkFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.DockerNetworkDetail{
				Id:                  "nas-1.immich_default",
				Name:                "immich_default",
				Device:              "nas-1",
				Driver:              "bridge",
				Subnet:              "172.18.0.0/16",
				Gateway:             "172.18.0.1",
				ConnectedContainers: 4,
				Containers:          []string{"immich_server", "immich_redis"},
			}), nil
		},
	}

	cmd := newGetNetworkCmd(stub)
	cmd.SetArgs([]string{"nas-1.immich_default"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.immich_default", "bridge", "172.18.0.0/16", "172.18.0.1", "immich_server", "immich_redis"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListImagesCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListDockerImagesFunc: func(_ context.Context, _ *gen.ListDockerImagesParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.DockerImageList{
				Items: []gen.DockerImage{
					{
						Id:         "nas-1.925ff61909ae",
						Device:     "nas-1",
						Repository: "ghcr.io/immich-app/immich-server",
						Tags:       []string{"v1.120.0"},
						Size:       524288000,
					},
				},
			}), nil
		},
	}

	cmd := newListImagesCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.925ff61909ae", "ghcr.io/immich-app/immich-server", "v1.120.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetImageCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		GetDockerImageFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.DockerImageDetail{
				Id:          "nas-1.925ff61909ae",
				Device:      "nas-1",
				Repository:  "ghcr.io/immich-app/immich-server",
				Tags:        []string{"v1.120.0"},
				Size:        524288000,
				VirtualSize: 1073741824,
			}), nil
		},
	}

	cmd := newGetImageCmd(stub)
	cmd.SetArgs([]string{"nas-1.925ff61909ae"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.925ff61909ae", "ghcr.io/immich-app/immich-server", "v1.120.0", "500.0 MB", "1.0 GB"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/cli/docker/... -run "TestListNetworksCmd|TestGetNetworkCmd|TestListImagesCmd|TestGetImageCmd" -v
```

Expected: FAIL (functions not yet defined — or compile error if Task 4 not yet done).

- [ ] **Step 3: Run full test suite after Task 4 is complete**

```bash
go test ./internal/cli/docker/... -v
```

Expected: all tests PASS including new ones.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/docker/docker_test.go
git commit -m "test: add tests for docker networks and images commands"
```
