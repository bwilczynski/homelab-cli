# CLI Group/Resource/Action Reorganization — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure hlctl to use a uniform `<group> <resource> <action>` convention, reflecting the API spec reorganization (docker group, storage backups, consistent list/get subcommands across all domains).

**Architecture:** Update oapi-codegen configs to match new spec tags, regenerate client code, then restructure each CLI package to introduce resource-level subcommand groups. No business logic changes — only command routing and package naming.

**Tech Stack:** Go 1.21+, Cobra, oapi-codegen v2

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `oapi-codegen-docker.yaml` | Create | Codegen config for docker tag (replaces containers) |
| `oapi-codegen-backups.yaml` | Delete | Backups now covered by storage tag |
| `Makefile` | Modify | Update generate target |
| `internal/docker/api.gen.go` | Generated | Docker client (replaces internal/containers/) |
| `internal/storage/api.gen.go` | Regenerated | Now includes ListBackups/GetBackup |
| `internal/cli/docker/client.go` | Create | ContainersClient interface (imports internal/docker) |
| `internal/cli/docker/stub.go` | Create | Test stub |
| `internal/cli/docker/docker_test.go` | Create | Tests for docker containers commands |
| `internal/cli/docker/docker.go` | Create | docker → containers → list/get/start/stop/restart |
| `internal/cli/containers/` | Delete | Replaced by internal/cli/docker/ |
| `internal/cli/storage/client.go` | Modify | Add ListBackups/GetBackup to interface |
| `internal/cli/storage/stub.go` | Modify | Add ListBackupsFunc/GetBackupFunc |
| `internal/cli/storage/storage_test.go` | Modify | Update volumes tests; add backups tests |
| `internal/cli/storage/storage.go` | Modify | volumes subgroup + backups subgroup |
| `internal/cli/backups/` | Delete | Replaced by storage backups subgroup |
| `internal/cli/network/network_test.go` | Modify | Update to call newListDevicesCmd/newGetDeviceCmd etc. |
| `internal/cli/network/network.go` | Modify | devices/clients become subgroups with list/get |
| `internal/cli/system/system_test.go` | Modify | Update updates tests |
| `internal/cli/system/system.go` | Modify | updates becomes a subgroup with list/get/check |
| `internal/cli/root.go` | Modify | Replace containers with docker; remove backups |

---

### Task 1: Update codegen configs and Makefile

**Files:**
- Create: `oapi-codegen-docker.yaml`
- Delete: `oapi-codegen-backups.yaml`
- Modify: `Makefile`

- [ ] **Step 1: Create `oapi-codegen-docker.yaml`**

```yaml
package: docker
generate:
  client: true
  models: true
output: internal/docker/api.gen.go
output-options:
  include-tags:
    - docker
```

- [ ] **Step 2: Update the `generate` target in `Makefile`** — replace `internal/containers internal/backups` with `internal/docker`, rename the containers codegen line to docker, remove the backups line:

```makefile
generate: bundle ## Generate client code from the bundled spec
	@mkdir -p internal/system internal/docker internal/storage internal/network
	$(OAPI_CODEGEN) --config oapi-codegen-system.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config oapi-codegen-docker.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config oapi-codegen-storage.yaml $(SPEC_FILE)
	$(OAPI_CODEGEN) --config oapi-codegen-network.yaml $(SPEC_FILE)
```

- [ ] **Step 3: Run `make generate`**

Expected output: no errors; `internal/docker/api.gen.go` is created; `internal/storage/api.gen.go` is regenerated.

Verify the new generated files have the expected methods:

```bash
grep -n "func.*ListContainers\|func.*GetContainer\|func.*StartContainer\|func.*StopContainer\|func.*RestartContainer" internal/docker/api.gen.go
grep -n "func.*ListBackups\|func.*GetBackup\|func.*ListStorageVolumes\|func.*GetStorageVolume" internal/storage/api.gen.go
```

Expected: all five container methods in docker, plus `ListBackups` and `GetBackup` in storage alongside the volume methods.

- [ ] **Step 4: Delete `oapi-codegen-backups.yaml` and commit**

```bash
git rm oapi-codegen-backups.yaml
git add oapi-codegen-docker.yaml Makefile spec
git commit -m "chore: update codegen configs for docker group and storage backups"
```

---

### Task 2: Create internal/cli/docker package

Move containers CLI from `internal/cli/containers/` to `internal/cli/docker/`. Commands gain a `containers` subcommand level: `hlctl docker containers list`.

**Files:**
- Create: `internal/cli/docker/client.go`
- Create: `internal/cli/docker/stub.go`
- Create: `internal/cli/docker/docker_test.go`
- Create: `internal/cli/docker/docker.go`
- Modify: `internal/cli/root.go`
- Delete: `internal/cli/containers/` (all 4 files)

- [ ] **Step 1: Write failing tests in `internal/cli/docker/docker_test.go`**

```go
package docker

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	gen "github.com/bwilczynski/hlctl/internal/docker"
)

func TestListContainersCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListContainersFunc: func(_ context.Context, _ *gen.ListContainersParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.ContainerList{
				Items: []gen.Container{
					{
						Id:     "nas-1.homeassistant",
						Image:  "homeassistant/home-assistant:latest",
						Status: gen.Running,
						Resources: gen.ContainerResources{
							CpuPercent:  1.5,
							MemoryBytes: 104857600,
						},
					},
				},
			}), nil
		},
	}

	cmd := newListCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.homeassistant", "homeassistant/home-assistant:latest", "running", "1.5%"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListContainersCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListContainersFunc: func(_ context.Context, _ *gen.ListContainersParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
}

func TestGetContainerCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		GetContainerFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.ContainerDetail{
				Id:            "nas-1.homeassistant",
				Name:          "homeassistant",
				Device:        "nas-1",
				Status:        gen.Running,
				Image:         "homeassistant/home-assistant:latest",
				RestartPolicy: gen.Always,
				Resources: gen.ContainerResources{
					CpuPercent:    1.5,
					MemoryBytes:   104857600,
					MemoryPercent: 5.0,
				},
			}), nil
		},
	}

	cmd := newGetCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.homeassistant", "homeassistant", "running", "always"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestStartContainerCmd(t *testing.T) {
	stub := &StubClient{
		StartContainerFunc: func(_ context.Context, _ string, _ *gen.StartContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
		},
	}
	cmd := newStartCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "started") {
		t.Errorf("expected 'started' in output, got: %s", buf.String())
	}
}

func TestStopContainerCmd(t *testing.T) {
	stub := &StubClient{
		StopContainerFunc: func(_ context.Context, _ string, _ *gen.StopContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
		},
	}
	cmd := newStopCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "stopped") {
		t.Errorf("expected 'stopped' in output, got: %s", buf.String())
	}
}

func TestRestartContainerCmd(t *testing.T) {
	stub := &StubClient{
		RestartContainerFunc: func(_ context.Context, _ string, _ *gen.RestartContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
		},
	}
	cmd := newRestartCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "restarted") {
		t.Errorf("expected 'restarted' in output, got: %s", buf.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/cli/docker/...
```

Expected: FAIL — package `docker` does not exist yet.

- [ ] **Step 3: Create `internal/cli/docker/client.go`**

```go
package docker

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/docker"
)

type ContainersClient interface {
	ListContainers(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetContainer(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StartContainer(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StopContainer(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	RestartContainer(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func NewContainersClient(httpClient *http.Client, apiURL string) (ContainersClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
```

- [ ] **Step 4: Create `internal/cli/docker/stub.go`**

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

type StubClient struct {
	ListContainersFunc   func(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetContainerFunc     func(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StartContainerFunc   func(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StopContainerFunc    func(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	RestartContainerFunc func(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
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

func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}
```

- [ ] **Step 5: Create `internal/cli/docker/docker.go`**

Port the logic verbatim from `internal/cli/containers/containers.go`, updating only the package name, import path, and command tree structure:

```go
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/docker"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Docker resources",
	}
	cmd.AddCommand(newContainersCmd())
	return cmd
}

func buildClient() (ContainersClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewContainersClient(httpClient, apiURL)
}

func newContainersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "containers",
		Short: "Manage Docker containers",
	}
	cmd.AddCommand(newListCmd(nil))
	cmd.AddCommand(newGetCmd(nil))
	cmd.AddCommand(newStartCmd(nil))
	cmd.AddCommand(newStopCmd(nil))
	cmd.AddCommand(newRestartCmd(nil))
	return cmd
}

func newListCmd(client ContainersClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListContainersParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListContainers(context.Background(), params)
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
			var list gen.ContainerList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "IMAGE", "STATUS", "CPU", "MEMORY"}
			var rows [][]string
			for _, c := range list.Items {
				rows = append(rows, []string{
					c.Id,
					c.Image,
					string(c.Status),
					fmt.Sprintf("%.1f%%", c.Resources.CpuPercent),
					output.FormatBytes(c.Resources.MemoryBytes),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newGetCmd(client ContainersClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <container-id>",
		Short: "Show container details",
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

			resp, err := c.GetContainer(context.Background(), args[0])
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
			var detail gen.ContainerDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			return printContainerDetail(cmd, detail)
		},
	}
}

func printContainerDetail(cmd *cobra.Command, d gen.ContainerDetail) error {
	w := cmd.OutOrStdout()

	memoryLimit := "unlimited"
	if d.MemoryLimit > 0 {
		memoryLimit = output.FormatBytes(d.MemoryLimit)
	}

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", d.Id},
		{"NAME", d.Name},
		{"DEVICE", d.Device},
		{"STATUS", string(d.Status)},
		{"IMAGE", d.Image},
		{"RESTART COUNT", fmt.Sprintf("%d", d.RestartCount)},
		{"CPU", fmt.Sprintf("%.1f%%", d.Resources.CpuPercent)},
		{"MEMORY", fmt.Sprintf("%s (%.1f%%)", output.FormatBytes(d.Resources.MemoryBytes), d.Resources.MemoryPercent)},
		{"STARTED AT", output.FormatTime(d.StartedAt)},
		{"EXIT CODE", fmt.Sprintf("%d", d.ExitCode)},
		{"OOM KILLED", fmt.Sprintf("%v", d.OomKilled)},
		{"RESTART POLICY", string(d.RestartPolicy)},
		{"PRIVILEGED", fmt.Sprintf("%v", d.Privileged)},
		{"MEMORY LIMIT", memoryLimit},
	}
	if err := output.Print(w, output.FormatTable, nil, headers, rows); err != nil {
		return err
	}

	if len(d.PortBindings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "PORT BINDINGS")
		var pbRows [][]string
		for _, pb := range d.PortBindings {
			pbRows = append(pbRows, []string{
				fmt.Sprintf("%d", pb.ContainerPort),
				fmt.Sprintf("%d", pb.HostPort),
				string(pb.Protocol),
			})
		}
		if err := output.Print(w, output.FormatTable, nil, []string{"CONTAINER PORT", "HOST PORT", "PROTOCOL"}, pbRows); err != nil {
			return err
		}
	}

	if len(d.Networks) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "NETWORKS")
		var netRows [][]string
		for _, n := range d.Networks {
			netRows = append(netRows, []string{n.Name, n.Driver})
		}
		if err := output.Print(w, output.FormatTable, nil, []string{"NAME", "DRIVER"}, netRows); err != nil {
			return err
		}
	}

	if len(d.VolumeBindings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "VOLUME BINDINGS")
		var volRows [][]string
		for _, v := range d.VolumeBindings {
			volRows = append(volRows, []string{v.Source, v.Destination, string(v.Mode)})
		}
		if err := output.Print(w, output.FormatTable, nil, []string{"SOURCE", "DESTINATION", "MODE"}, volRows); err != nil {
			return err
		}
	}

	if len(d.EnvVariables) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "ENVIRONMENT VARIABLES")
		var envRows [][]string
		for _, e := range d.EnvVariables {
			envRows = append(envRows, []string{e.Key, e.Value})
		}
		if err := output.Print(w, output.FormatTable, nil, []string{"KEY", "VALUE"}, envRows); err != nil {
			return err
		}
	}

	if len(d.Entrypoint) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "ENTRYPOINT")
		for _, e := range d.Entrypoint {
			fmt.Fprintln(w, " ", e)
		}
	}

	if len(d.Cmd) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "COMMAND")
		for _, c := range d.Cmd {
			fmt.Fprintln(w, " ", c)
		}
	}

	if d.Labels != nil && len(*d.Labels) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "LABELS")
		var labelRows [][]string
		for k, v := range *d.Labels {
			labelRows = append(labelRows, []string{k, v})
		}
		sort.Slice(labelRows, func(i, j int) bool {
			return labelRows[i][0] < labelRows[j][0]
		})
		if err := output.Print(w, output.FormatTable, nil, []string{"KEY", "VALUE"}, labelRows); err != nil {
			return err
		}
	}

	return nil
}

func newStartCmd(client ContainersClient) *cobra.Command {
	return &cobra.Command{
		Use:   "start <container-id>",
		Short: "Start a container",
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
			resp, err := c.StartContainer(context.Background(), args[0], &gen.StartContainerParams{})
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				return apiclient.ParseError(resp)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Container %s started\n", args[0])
			return nil
		},
	}
}

func newStopCmd(client ContainersClient) *cobra.Command {
	return &cobra.Command{
		Use:   "stop <container-id>",
		Short: "Stop a container",
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
			resp, err := c.StopContainer(context.Background(), args[0], &gen.StopContainerParams{})
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				return apiclient.ParseError(resp)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Container %s stopped\n", args[0])
			return nil
		},
	}
}

func newRestartCmd(client ContainersClient) *cobra.Command {
	return &cobra.Command{
		Use:   "restart <container-id>",
		Short: "Restart a container",
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
			resp, err := c.RestartContainer(context.Background(), args[0], &gen.RestartContainerParams{})
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				return apiclient.ParseError(resp)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Container %s restarted\n", args[0])
			return nil
		},
	}
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/cli/docker/...
```

Expected: PASS

- [ ] **Step 7: Update `internal/cli/root.go`** — replace `containers` import/command with `docker`, remove `backups`:

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

func Execute() error {
	return rootCmd.Execute()
}
```

- [ ] **Step 8: Delete `internal/cli/containers/` and verify build**

```bash
rm -rf internal/cli/containers
go build ./...
```

Expected: no errors.

- [ ] **Step 9: Commit**

```bash
git add internal/cli/docker/ internal/cli/root.go
git rm -r internal/cli/containers/
git commit -m "feat(cli): rename containers to docker group with containers subcommand"
```

---

### Task 3: Restructure storage — volumes subgroup + backups subgroup

Add a `volumes` resource subgroup and a `backups` resource subgroup under `storage`. Delete `internal/cli/backups/`.

**Files:**
- Modify: `internal/cli/storage/client.go`
- Modify: `internal/cli/storage/stub.go`
- Modify: `internal/cli/storage/storage_test.go`
- Modify: `internal/cli/storage/storage.go`
- Delete: `internal/cli/backups/` (all 4 files)

- [ ] **Step 1: Write failing tests in `internal/cli/storage/storage_test.go`**

Replace the existing test file with this content (preserves old volume tests updated for new command names; adds backup tests):

```go
package storage

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	gen "github.com/bwilczynski/hlctl/internal/storage"
)

// --- volumes ---

func TestListVolumesCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListStorageVolumesFunc: func(_ context.Context, _ *gen.ListStorageVolumesParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.VolumeList{
				Items: []gen.Volume{
					{
						Id:         "nas-1.volume1",
						Name:       "volume1",
						Device:     "nas-1",
						RaidType:   "SHR-2",
						Status:     gen.VolumeStatusNormal,
						TotalBytes: 15_981_977_067_520,
						UsedBytes:  10_132_536_762_777,
						FileSystem: "ext4",
					},
				},
			}), nil
		},
	}

	cmd := newListVolumesCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.volume1", "nas-1", "SHR-2", "normal"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListVolumesCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListStorageVolumesFunc: func(_ context.Context, _ *gen.ListStorageVolumesParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListVolumesCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
}

func TestGetVolumeCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		GetStorageVolumeFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.VolumeDetail{
				Id:         "nas-1.volume1",
				Name:       "volume1",
				Device:     "nas-1",
				RaidType:   "SHR-2",
				Status:     gen.VolumeStatusNormal,
				PoolStatus: gen.PoolStatusNormal,
				MountPath:  "/volume1",
				FileSystem: "ext4",
				TotalBytes: 15_981_977_067_520,
				UsedBytes:  10_132_536_762_777,
			}), nil
		},
	}

	cmd := newGetVolumeCmd(stub)
	cmd.SetArgs([]string{"nas-1.volume1"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.volume1", "volume1", "SHR-2", "/volume1"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

// --- backups ---

func TestListBackupsCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListBackupsFunc: func(_ context.Context, _ *gen.ListBackupsParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.BackupTaskList{
				Items: []gen.BackupTask{
					{
						Id:         "nas-1.daily-backup",
						Name:       "Daily Backup",
						Device:     "nas-1",
						Status:     gen.Idle,
						LastResult: gen.Success,
						Type:       "hyperBackup",
					},
				},
			}), nil
		},
	}

	cmd := newListBackupsCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.daily-backup", "Daily Backup", "nas-1", "idle", "success", "hyperBackup"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListBackupsCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListBackupsFunc: func(_ context.Context, _ *gen.ListBackupsParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListBackupsCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
}

func TestGetBackupCmd_withDates(t *testing.T) {
	lastRun := time.Date(2026, 4, 30, 3, 0, 0, 0, time.UTC)
	nextRun := time.Date(2026, 5, 1, 3, 0, 0, 0, time.UTC)
	stub := &StubClient{
		GetBackupFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.BackupTaskDetail{
				Id:         "nas-1.daily-backup",
				Name:       "Daily Backup",
				Device:     "nas-1",
				Status:     gen.Idle,
				LastResult: gen.Success,
				Type:       "hyperBackup",
				LastRunAt:  &lastRun,
				NextRunAt:  &nextRun,
			}), nil
		},
	}

	cmd := newGetBackupCmd(stub)
	cmd.SetArgs([]string{"nas-1.daily-backup"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.daily-backup", "hyperBackup", "LAST RUN", "NEXT RUN", "2026-04-30", "2026-05-01"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetBackupCmd_withSizeAndFolders(t *testing.T) {
	size := gen.Bytes(10737418240)
	folders := []string{"/volume1/photos", "/volume1/documents"}
	stub := &StubClient{
		GetBackupFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.BackupTaskDetail{
				Id:         "nas-1.daily-backup",
				Name:       "Daily Backup",
				Device:     "nas-1",
				Status:     gen.Idle,
				LastResult: gen.Success,
				Type:       "hyperBackup",
				Size:       &size,
				Folders:    &folders,
			}), nil
		},
	}

	cmd := newGetBackupCmd(stub)
	cmd.SetArgs([]string{"nas-1.daily-backup"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"SIZE", "10.0 GB", "FOLDERS", "/volume1/photos", "/volume1/documents"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetBackupCmd_apiError(t *testing.T) {
	stub := &StubClient{
		GetBackupFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "backup 'nas-1.foo' not found",
			}), nil
		},
	}
	cmd := newGetBackupCmd(stub)
	cmd.SetArgs([]string{"nas-1.foo"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/cli/storage/...
```

Expected: FAIL — `newListVolumesCmd`, `newGetVolumeCmd`, `newListBackupsCmd`, `newGetBackupCmd`, `ListBackupsFunc`, `GetBackupFunc` do not exist yet.

- [ ] **Step 3: Update `internal/cli/storage/client.go`** — add `ListBackups` and `GetBackup` to the interface:

```go
package storage

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/storage"
)

type StorageClient interface {
	ListStorageVolumes(ctx context.Context, params *gen.ListStorageVolumesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetStorageVolume(ctx context.Context, volumeId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListBackups(ctx context.Context, params *gen.ListBackupsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetBackup(ctx context.Context, backupId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func NewStorageClient(httpClient *http.Client, apiURL string) (StorageClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
```

- [ ] **Step 4: Update `internal/cli/storage/stub.go`** — add `ListBackupsFunc` and `GetBackupFunc`:

```go
package storage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gen "github.com/bwilczynski/hlctl/internal/storage"
)

type StubClient struct {
	ListStorageVolumesFunc func(ctx context.Context, params *gen.ListStorageVolumesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetStorageVolumeFunc   func(ctx context.Context, volumeId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListBackupsFunc        func(ctx context.Context, params *gen.ListBackupsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetBackupFunc          func(ctx context.Context, backupId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func (s *StubClient) ListStorageVolumes(ctx context.Context, params *gen.ListStorageVolumesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListStorageVolumesFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetStorageVolume(ctx context.Context, volumeId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetStorageVolumeFunc(ctx, volumeId, reqEditors...)
}

func (s *StubClient) ListBackups(ctx context.Context, params *gen.ListBackupsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListBackupsFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetBackup(ctx context.Context, backupId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetBackupFunc(ctx, backupId, reqEditors...)
}

func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}
```

- [ ] **Step 5: Rewrite `internal/cli/storage/storage.go`** — restructure into volumes and backups subgroups:

```go
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
	gen "github.com/bwilczynski/hlctl/internal/storage"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "NAS storage resources",
	}
	cmd.AddCommand(newVolumesCmd())
	cmd.AddCommand(newBackupsCmd())
	return cmd
}

func buildClient() (StorageClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewStorageClient(httpClient, apiURL)
}

func newVolumesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volumes",
		Short: "Storage volumes",
	}
	cmd.AddCommand(newListVolumesCmd(nil))
	cmd.AddCommand(newGetVolumeCmd(nil))
	return cmd
}

func newListVolumesCmd(client StorageClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List storage volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListStorageVolumesParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListStorageVolumes(context.Background(), params)
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
			var list gen.VolumeList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "NAME", "DEVICE", "RAID", "STATUS", "SIZE", "USED"}
			var rows [][]string
			for _, v := range list.Items {
				rows = append(rows, []string{
					v.Id, v.Name, v.Device, v.RaidType,
					string(v.Status),
					output.FormatBytes(v.TotalBytes),
					output.FormatBytes(v.UsedBytes),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newGetVolumeCmd(client StorageClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <volume-id>",
		Short: "Show volume details",
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

			resp, err := c.GetStorageVolume(context.Background(), args[0])
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
			var detail gen.VolumeDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			return printVolumeDetail(cmd, detail)
		},
	}
}

func printVolumeDetail(cmd *cobra.Command, d gen.VolumeDetail) error {
	w := cmd.OutOrStdout()

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", d.Id},
		{"NAME", d.Name},
		{"DEVICE", d.Device},
		{"FILESYSTEM", d.FileSystem},
		{"RAID", d.RaidType},
		{"STATUS", string(d.Status)},
		{"POOL STATUS", string(d.PoolStatus)},
		{"MOUNT PATH", d.MountPath},
		{"SIZE", output.FormatBytes(d.TotalBytes)},
		{"USED", output.FormatBytes(d.UsedBytes)},
	}
	if err := output.Print(w, output.FormatTable, nil, headers, rows); err != nil {
		return err
	}

	if len(d.Disks) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "DISKS")
		var diskRows [][]string
		for _, disk := range d.Disks {
			diskRows = append(diskRows, []string{
				disk.Id,
				disk.Model,
				string(disk.Status),
				fmt.Sprintf("%d°C", disk.TemperatureCelsius),
				output.FormatBytes(disk.TotalBytes),
			})
		}
		if err := output.Print(w, output.FormatTable, nil, []string{"ID", "MODEL", "STATUS", "TEMP", "SIZE"}, diskRows); err != nil {
			return err
		}
	}

	return nil
}

func newBackupsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backups",
		Short: "Backup tasks and history",
	}
	cmd.AddCommand(newListBackupsCmd(nil))
	cmd.AddCommand(newGetBackupCmd(nil))
	return cmd
}

func newListBackupsCmd(client StorageClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListBackupsParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListBackups(context.Background(), params)
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
			var list gen.BackupTaskList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "NAME", "DEVICE", "STATUS", "LAST RESULT", "TYPE"}
			var rows [][]string
			for _, t := range list.Items {
				rows = append(rows, []string{
					t.Id, t.Name, t.Device,
					string(t.Status), string(t.LastResult), t.Type,
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newGetBackupCmd(client StorageClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <backup-id>",
		Short: "Show backup details",
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

			resp, err := c.GetBackup(context.Background(), args[0])
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
			var detail gen.BackupTaskDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", detail.Id},
				{"NAME", detail.Name},
				{"DEVICE", detail.Device},
				{"STATUS", string(detail.Status)},
				{"LAST RESULT", string(detail.LastResult)},
				{"TYPE", detail.Type},
			}
			if detail.LastRunAt != nil {
				rows = append(rows, []string{"LAST RUN", output.FormatTime(*detail.LastRunAt)})
			}
			if detail.NextRunAt != nil {
				rows = append(rows, []string{"NEXT RUN", output.FormatTime(*detail.NextRunAt)})
			}
			if detail.Size != nil {
				rows = append(rows, []string{"SIZE", output.FormatBytes(*detail.Size)})
			}
			if detail.Folders != nil && len(*detail.Folders) > 0 {
				for i, folder := range *detail.Folders {
					label := "FOLDERS"
					if i > 0 {
						label = ""
					}
					rows = append(rows, []string{label, folder})
				}
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)
		},
	}
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/cli/storage/...
```

Expected: PASS

- [ ] **Step 7: Verify build passes, then delete `internal/cli/backups/` and commit**

```bash
go build ./...
git add internal/cli/storage/
git rm -r internal/cli/backups/
git commit -m "feat(cli): restructure storage with volumes/backups subgroups; replace backups package"
```

---

### Task 4: Restructure network — devices and clients subgroups

Convert flat `devices`/`device` and `clients`/`client` commands into `devices list`/`devices get` and `clients list`/`clients get`.

**Files:**
- Modify: `internal/cli/network/network_test.go`
- Modify: `internal/cli/network/network.go`

- [ ] **Step 1: Write failing tests in `internal/cli/network/network_test.go`**

Replace the existing test file with this content (same assertions, updated to call the new constructor names):

```go
package network

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	gen "github.com/bwilczynski/hlctl/internal/network"
)

func TestListDevicesCmd_tableOutput(t *testing.T) {
	numClients := 5
	stub := &StubClient{
		ListNetworkDevicesFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkDeviceList{
				Items: []gen.NetworkDevice{
					{
						Id:     "unifi.usg",
						Name:   "USG",
						Mac:    "aa:bb:cc:dd:00:01",
						Ip:     "192.168.1.1",
						Type:   gen.Gateway,
						Status: gen.Connected,
					},
					{
						Id:         "unifi.ap-living-room",
						Name:       "AP Living Room",
						Mac:        "aa:bb:cc:dd:00:03",
						Ip:         "192.168.1.3",
						Type:       gen.AccessPoint,
						Status:     gen.Connected,
						NumClients: &numClients,
					},
				},
			}), nil
		},
	}

	cmd := newListDevicesCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"unifi.usg", "unifi.ap-living-room", "gateway", "accessPoint", "5"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListDevicesCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListNetworkDevicesFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListDevicesCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
}

func TestGetDeviceCmd_tableOutput(t *testing.T) {
	numClients := 3
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkDeviceDetail{
				Id:              "unifi.usg",
				Name:            "USG",
				Mac:             "aa:bb:cc:dd:00:01",
				Ip:              "192.168.1.1",
				Type:            gen.Gateway,
				Status:          gen.Connected,
				NumClients:      &numClients,
				Model:           "USG-3P",
				FirmwareVersion: "4.4.57",
				Uptime:          86400,
			}), nil
		},
	}

	cmd := newGetDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.usg"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"unifi.usg", "USG-3P", "4.4.57", "gateway"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestListClientsCmd_tableOutput(t *testing.T) {
	ip := "192.168.1.50"
	stub := &StubClient{
		ListNetworkClientsFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkClientList{
				Items: []gen.NetworkClient{
					{
						Id:             "unifi.aa:bb:cc:dd:ee:01",
						Name:           "laptop",
						Mac:            "aa:bb:cc:dd:ee:01",
						Ip:             &ip,
						ConnectionType: gen.Wired,
					},
				},
			}), nil
		},
	}

	cmd := newListClientsCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"unifi.aa:bb:cc:dd:ee:01", "laptop", "192.168.1.50", "wired"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetClientCmd_wired(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:01",
				"name":           "laptop",
				"mac":            "aa:bb:cc:dd:ee:01",
				"ip":             "192.168.1.50",
				"connectionType": "wired",
				"switchName":     "switch-1",
				"switchPort":     3,
				"uptime":         3600,
			}), nil
		},
	}

	cmd := newGetClientCmd(stub)
	cmd.SetArgs([]string{"unifi.aa:bb:cc:dd:ee:01"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"laptop", "switch-1", fmt.Sprintf("%d", 3)} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/cli/network/...
```

Expected: FAIL — `newListDevicesCmd`, `newGetDeviceCmd`, `newListClientsCmd`, `newGetClientCmd` do not exist yet.

- [ ] **Step 3: Rewrite `internal/cli/network/network.go`** — convert `newDevicesCmd`/`newDeviceCmd` into a `devices` subgroup with `list`/`get`, same for `clients`:

```go
package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/network"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Network devices and clients",
	}
	cmd.AddCommand(newDevicesCmd())
	cmd.AddCommand(newClientsCmd())
	return cmd
}

func buildClient() (NetworkClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewNetworkClient(httpClient, apiURL)
}

func newDevicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Network devices",
	}
	cmd.AddCommand(newListDevicesCmd(nil))
	cmd.AddCommand(newGetDeviceCmd(nil))
	return cmd
}

func newListDevicesCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List network devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.ListNetworkDevices(context.Background())
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
			var list gen.NetworkDeviceList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "NAME", "MAC", "IP", "TYPE", "STATUS", "CLIENTS"}
			var rows [][]string
			for _, d := range list.Items {
				clients := ""
				if d.NumClients != nil {
					clients = fmt.Sprintf("%d", *d.NumClients)
				}
				rows = append(rows, []string{
					d.Id, d.Name, d.Mac, d.Ip,
					string(d.Type), string(d.Status),
					clients,
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}
}

func newGetDeviceCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <device-id>",
		Short: "Show network device details",
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

			resp, err := c.GetNetworkDevice(context.Background(), args[0])
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
			var detail gen.NetworkDeviceDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", detail.Id},
				{"NAME", detail.Name},
				{"MAC", detail.Mac},
				{"IP", detail.Ip},
				{"TYPE", string(detail.Type)},
				{"STATUS", string(detail.Status)},
			}
			if detail.NumClients != nil {
				rows = append(rows, []string{"CLIENTS", fmt.Sprintf("%d", *detail.NumClients)})
			}
			rows = append(rows,
				[]string{"MODEL", detail.Model},
				[]string{"FIRMWARE", detail.FirmwareVersion},
				[]string{"UPTIME", output.FormatUptime(detail.Uptime)},
			)
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)
		},
	}
}

func newClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Connected network clients",
	}
	cmd.AddCommand(newListClientsCmd(nil))
	cmd.AddCommand(newGetClientCmd(nil))
	return cmd
}

func newListClientsCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List connected network clients",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.ListNetworkClients(context.Background())
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
			var list gen.NetworkClientList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"ID", "NAME", "MAC", "IP", "CONNECTION"}
			var rows [][]string
			for _, cl := range list.Items {
				ip := ""
				if cl.Ip != nil {
					ip = *cl.Ip
				}
				rows = append(rows, []string{
					cl.Id, cl.Name, cl.Mac, ip,
					string(cl.ConnectionType),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}
}

func newGetClientCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <client-id>",
		Short: "Show network client details",
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

			resp, err := c.GetNetworkClient(context.Background(), args[0])
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
			var detail gen.NetworkClientDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			disc, err := detail.Discriminator()
			if err != nil {
				return err
			}

			headers := []string{"FIELD", "VALUE"}
			var rows [][]string

			switch disc {
			case "wired":
				d, err := detail.AsWiredNetworkClientDetail()
				if err != nil {
					return err
				}
				ip := ""
				if d.Ip != nil {
					ip = *d.Ip
				}
				rows = [][]string{
					{"ID", d.Id},
					{"NAME", d.Name},
					{"MAC", d.Mac},
					{"IP", ip},
					{"CONNECTION", string(d.ConnectionType)},
					{"SWITCH", d.SwitchName},
					{"SWITCH PORT", fmt.Sprintf("%d", d.SwitchPort)},
					{"UPTIME", output.FormatUptime(d.Uptime)},
				}
			case "wireless":
				d, err := detail.AsWirelessNetworkClientDetail()
				if err != nil {
					return err
				}
				ip := ""
				if d.Ip != nil {
					ip = *d.Ip
				}
				rows = [][]string{
					{"ID", d.Id},
					{"NAME", d.Name},
					{"MAC", d.Mac},
					{"IP", ip},
					{"CONNECTION", string(d.ConnectionType)},
					{"SSID", d.Ssid},
					{"SIGNAL", fmt.Sprintf("%d dBm", d.SignalStrength)},
					{"UPTIME", output.FormatUptime(d.Uptime)},
				}
			default:
				return fmt.Errorf("unknown connection type: %s", disc)
			}

			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)
		},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/cli/network/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/network/
git commit -m "feat(cli): restructure network with devices/clients subgroups"
```

---

### Task 5: Restructure system — updates subgroup

Move `updates`, `update <id>`, and `check-updates` under a `updates` subgroup as `updates list`, `updates get <id>`, `updates check`. Keep `health`, `info`, `utilization` as direct children of `system`.

**Files:**
- Modify: `internal/cli/system/system_test.go`
- Modify: `internal/cli/system/system.go`

- [ ] **Step 1: Write failing tests** — add to `internal/cli/system/system_test.go`

The existing tests for `health`, `info`, `utilization` call `newHealthCmd`, `newInfoCmd`, `newUtilizationCmd` — those names are unchanged, so existing tests keep passing. Add the following tests for the restructured updates commands, replacing references to `newUpdatesCmd`/`newUpdateCmd`/`newCheckUpdatesCmd` with the new names:

Locate the `TestUpdatesCmd_*`, `TestUpdateCmd_*`, and `TestCheckUpdatesCmd_*` tests in the existing file and rename the constructors called:

- `newUpdatesCmd(stub)` → `newListUpdatesCmd(stub)`
- `newUpdateCmd(stub)` → `newGetUpdateCmd(stub)`
- `newCheckUpdatesCmd(stub)` → `newCheckUpdatesCmd(stub)` (Use changes from `check-updates` to `check`; constructor name stays the same)

Run a search to confirm the existing test names:

```bash
grep -n "func Test" internal/cli/system/system_test.go
```

Then update each updates-related test call site. For example, if the file has:

```go
cmd := newUpdatesCmd(stub)
```

Change to:

```go
cmd := newListUpdatesCmd(stub)
```

And if it has:

```go
cmd := newUpdateCmd(stub)
```

Change to:

```go
cmd := newGetUpdateCmd(stub)
```

- [ ] **Step 2: Run tests to verify the updates tests fail**

```bash
go test ./internal/cli/system/...
```

Expected: FAIL on the updates-related tests; health/info/utilization tests still pass.

- [ ] **Step 3: Rewrite `internal/cli/system/system.go`** — wrap updates commands under an `updates` subgroup:

In `NewCmd()`, replace the three direct `AddCommand` calls for updates with a single `newUpdatesCmd()`:

```go
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System health and information",
	}
	cmd.AddCommand(newHealthCmd(nil))
	cmd.AddCommand(newInfoCmd(nil))
	cmd.AddCommand(newUtilizationCmd(nil))
	cmd.AddCommand(newUpdatesCmd())
	return cmd
}
```

Add the `newUpdatesCmd()` subgroup constructor:

```go
func newUpdatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "updates",
		Short: "Software update tracking",
	}
	cmd.AddCommand(newListUpdatesCmd(nil))
	cmd.AddCommand(newGetUpdateCmd(nil))
	cmd.AddCommand(newCheckUpdatesCmd(nil))
	return cmd
}
```

Rename the existing leaf constructors (no logic changes, only name changes):
- `newUpdatesCmd(client SystemClient)` → `newListUpdatesCmd(client SystemClient)` with `Use: "list"`
- `newUpdateCmd(client SystemClient)` → `newGetUpdateCmd(client SystemClient)` with `Use: "get <update-id>"`
- `newCheckUpdatesCmd(client SystemClient)` — keep name, change `Use: "check"` (was `"check-updates"`)

The full updated file:

```go
package system

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/output"
	gen "github.com/bwilczynski/hlctl/internal/system"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System health and information",
	}
	cmd.AddCommand(newHealthCmd(nil))
	cmd.AddCommand(newInfoCmd(nil))
	cmd.AddCommand(newUtilizationCmd(nil))
	cmd.AddCommand(newUpdatesCmd())
	return cmd
}

func buildClient() (SystemClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewSystemClient(httpClient, apiURL)
}

func newHealthCmd(client SystemClient) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Show aggregate system health",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.GetSystemHealth(context.Background())
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
			var health gen.Health
			if err := json.Unmarshal(body, &health); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"COMPONENT", "STATUS"}
			var rows [][]string
			for _, comp := range health.Components {
				rows = append(rows, []string{comp.Name, string(comp.Status)})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), health, headers, rows)
		},
	}
}

func newInfoCmd(client SystemClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show device information",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListSystemInfoParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListSystemInfo(context.Background(), params)
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
			var list gen.SystemInfoList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"DEVICE", "MODEL", "FIRMWARE", "RAM", "UPTIME"}
			var rows [][]string
			for _, info := range list.Items {
				rows = append(rows, []string{
					info.Device,
					info.Model,
					info.Firmware,
					output.FormatBytes(int64(info.RamMb) * 1024 * 1024),
					output.FormatUptime(int(info.UptimeSeconds)),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newUtilizationCmd(client SystemClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "utilization",
		Short: "Show live resource utilization",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListSystemUtilizationParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListSystemUtilization(context.Background(), params)
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
			var list gen.SystemUtilizationList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			headers := []string{"DEVICE", "CPU", "MEMORY", "SWAP"}
			var rows [][]string
			for _, u := range list.Items {
				swapPct := 0
				if u.Memory.SwapTotalBytes > 0 {
					swapPct = int(u.Memory.SwapUsedBytes * 100 / u.Memory.SwapTotalBytes)
				}
				rows = append(rows, []string{
					u.Device,
					fmt.Sprintf("%d%%", u.Cpu.TotalPercent),
					fmt.Sprintf("%d%%", u.Memory.UsedPercent),
					fmt.Sprintf("%d%%", swapPct),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}

	cmd.Flags().StringVar(&device, "device", "", "Filter by device ID")
	return cmd
}

func newUpdatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "updates",
		Short: "Software update tracking",
	}
	cmd.AddCommand(newListUpdatesCmd(nil))
	cmd.AddCommand(newGetUpdateCmd(nil))
	cmd.AddCommand(newCheckUpdatesCmd(nil))
	return cmd
}

func newListUpdatesCmd(client SystemClient) *cobra.Command {
	var status, updateType string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tracked software updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListSystemUpdatesParams{}
			if status != "" {
				s := gen.UpdateStatusFilter(status)
				params.Status = &s
			}
			if updateType != "" {
				ut := gen.UpdateTypeFilter(updateType)
				params.Type = &ut
			}

			resp, err := c.ListSystemUpdates(context.Background(), params)
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
			var list gen.SystemUpdateList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			return printUpdateList(cmd.OutOrStdout(), list)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by update status (unknown, upToDate, updateAvailable)")
	cmd.Flags().StringVar(&updateType, "type", "", "Filter by component type (container)")
	return cmd
}

func printUpdateList(w io.Writer, list gen.SystemUpdateList) error {
	headers := []string{"ID", "NAME", "DEVICE", "TYPE", "STATUS", "CURRENT", "LATEST"}
	var rows [][]string
	for _, u := range list.Items {
		rows = append(rows, []string{
			u.Id, u.Name, u.Device,
			string(u.Type), string(u.Status),
			u.CurrentVersion, u.LatestVersion,
		})
	}
	return output.Print(w, output.FormatTable, list, headers, rows)
}

func newGetUpdateCmd(client SystemClient) *cobra.Command {
	return &cobra.Command{
		Use:   "get <update-id>",
		Short: "Show update details for a tracked component",
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

			resp, err := c.GetSystemUpdate(context.Background(), args[0])
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
			var detail gen.SystemUpdateDetail
			if err := json.Unmarshal(body, &detail); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			disc, err := detail.Discriminator()
			if err != nil {
				return err
			}

			switch disc {
			case "container":
				d, err := detail.AsContainerSystemUpdateDetail()
				if err != nil {
					return err
				}
				headers := []string{"FIELD", "VALUE"}
				rows := [][]string{
					{"ID", d.Id},
					{"NAME", d.Name},
					{"DEVICE", d.Device},
					{"TYPE", string(d.Type)},
					{"STATUS", string(d.Status)},
					{"CURRENT", d.CurrentVersion},
					{"LATEST", d.LatestVersion},
					{"CHECKED AT", output.FormatTime(d.CheckedAt)},
					{"PUBLISHED AT", output.FormatTime(d.PublishedAt)},
					{"IMAGE", d.Image},
					{"SOURCE", d.Source},
					{"RELEASE URL", d.ReleaseUrl},
				}
				return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)
			default:
				return fmt.Errorf("unknown update type: %s", disc)
			}
		},
	}
}

func newCheckUpdatesCmd(client SystemClient) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Force check for upstream updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			resp, err := c.CheckSystemUpdates(context.Background(), &gen.CheckSystemUpdatesParams{})
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
			var list gen.SystemUpdateList
			if err := json.Unmarshal(body, &list); err != nil {
				return err
			}

			if flags.GetOutputFormat() == output.FormatJSON {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			return printUpdateList(cmd.OutOrStdout(), list)
		},
	}
}
```

- [ ] **Step 4: Run all tests**

```bash
go test ./internal/cli/system/...
```

Expected: PASS

- [ ] **Step 5: Verify full build passes**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/system/
git commit -m "feat(cli): restructure system with updates subgroup (list/get/check)"
```

---

### Task 6: Final verification

- [ ] **Step 1: Run full test suite**

```bash
go test ./...
```

Expected: all tests pass, no compilation errors.

- [ ] **Step 2: Build and smoke-test help output**

```bash
make build
./bin/hlctl --help
./bin/hlctl docker --help
./bin/hlctl docker containers --help
./bin/hlctl storage --help
./bin/hlctl storage backups --help
./bin/hlctl network devices --help
./bin/hlctl system updates --help
```

Expected: each command shows its subcommands correctly with no `containers`, `backups` at top level.

- [ ] **Step 3: Commit spec submodule pointer if not already done**

```bash
git status
# if spec shows as modified:
git add spec
git commit -m "chore: update spec submodule to group/resource hierarchy"
```
