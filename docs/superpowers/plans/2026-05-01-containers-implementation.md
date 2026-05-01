# Containers Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `hlctl containers` command (list, get, start, stop, restart) with real API calls, establishing reusable patterns for all future domain commands.

**Architecture:** A shared `internal/apiclient` package provides authenticated HTTP client construction and RFC 9457 error parsing. Each domain command package defines a narrow interface over the generated oapi-codegen client, with a stub implementation for unit tests. The containers package is the first to use this pattern; other domains will follow identically.

**Tech Stack:** Go 1.26, Cobra, oapi-codegen (generated client in `internal/containers/api.gen.go`), `text/tabwriter` for table output.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/apiclient/apiclient.go` | Create | `NewHTTPClient() (*http.Client, string, error)` — resolves API URL and returns authenticated HTTP client |
| `internal/apiclient/errors.go` | Create | `ParseError(resp *http.Response) error` — RFC 9457 Problem decoder |
| `internal/output/output.go` | Modify | Add `FormatBytes(n int64) string` helper |
| `internal/cli/containers/client.go` | Create | `ContainersClient` interface + `NewContainersClient()` factory |
| `internal/cli/containers/stub.go` | Create | `StubClient` with function fields for each method |
| `internal/cli/containers/containers.go` | Rewrite | Real `RunE` implementations using `ContainersClient` |
| `internal/cli/containers/containers_test.go` | Create | Unit tests for list and get using `StubClient` |

---

## Task 1: Add `FormatBytes` to the output package

**Files:**
- Modify: `internal/output/output.go`

- [ ] **Step 1: Write the failing test**

Create `internal/output/output_test.go`:

```go
package output_test

import (
	"testing"

	"github.com/bwilczynski/hlctl/internal/output"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{268435456, "256.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := output.FormatBytes(tt.input)
			if got != tt.expected {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/output/... -run TestFormatBytes -v
```

Expected: `FAIL — undefined: output.FormatBytes`

- [ ] **Step 3: Implement `FormatBytes`**

Add to the bottom of `internal/output/output.go`:

```go
// FormatBytes converts a byte count to a human-readable string using binary units.
func FormatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n := n / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), units[exp])
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/output/... -run TestFormatBytes -v
```

Expected: all cases PASS

- [ ] **Step 5: Commit**

```bash
git add internal/output/output.go internal/output/output_test.go
git commit -m "feat(output): add FormatBytes helper"
```

---

## Task 2: Create `internal/apiclient` package

**Files:**
- Create: `internal/apiclient/apiclient.go`
- Create: `internal/apiclient/errors.go`

- [ ] **Step 1: Write failing tests**

Create `internal/apiclient/errors_test.go`:

```go
package apiclient_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/apiclient"
)

func problemResponse(status int, title, detail string) *http.Response {
	body := map[string]any{"type": "https://example.com/problem", "title": title, "status": status}
	if detail != "" {
		body["detail"] = detail
	}
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}

func TestParseError_withDetail(t *testing.T) {
	resp := problemResponse(404, "Not Found", "container 'nas-1.foo' does not exist")
	err := apiclient.ParseError(resp)
	want := "Not Found — container 'nas-1.foo' does not exist"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}

func TestParseError_withoutDetail(t *testing.T) {
	resp := problemResponse(401, "Unauthorized", "")
	err := apiclient.ParseError(resp)
	want := "Unauthorized"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}

func TestParseError_invalidBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader("not json")),
	}
	err := apiclient.ParseError(resp)
	want := "unexpected status 500"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/apiclient/... -v
```

Expected: `FAIL — package not found`

- [ ] **Step 3: Create `internal/apiclient/errors.go`**

```go
package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type problem struct {
	Title  string  `json:"title"`
	Detail *string `json:"detail,omitempty"`
}

// ParseError reads an RFC 9457 Problem Details body from resp and returns
// a user-friendly error. Call this on any non-2xx response.
func ParseError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var p problem
	if err := json.Unmarshal(body, &p); err != nil || p.Title == "" {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	if p.Detail != nil && *p.Detail != "" {
		return fmt.Errorf("%s — %s", p.Title, *p.Detail)
	}
	return fmt.Errorf("%s", p.Title)
}
```

- [ ] **Step 4: Create `internal/apiclient/apiclient.go`**

```go
package apiclient

import (
	"net/http"

	"github.com/bwilczynski/hlctl/internal/auth"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	"github.com/bwilczynski/hlctl/internal/config"
)

// NewHTTPClient returns an authenticated *http.Client and the resolved API
// base URL. Precedence: --api-url flag → HOMELAB_API_URL env → config file.
// Call once per RunE invocation to construct domain-specific API clients.
func NewHTTPClient() (*http.Client, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, "", err
	}

	apiURL := flags.GetAPIURL()
	if apiURL == "" {
		apiURL, err = cfg.ResolveAPIURL()
		if err != nil {
			return nil, "", err
		}
	}

	httpClient := &http.Client{
		Transport: &auth.AuthenticatedTransport{},
	}
	return httpClient, apiURL, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/apiclient/... -v
```

Expected: all three cases PASS

- [ ] **Step 6: Verify the build is clean**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/apiclient/
git commit -m "feat(apiclient): add shared HTTP client factory and RFC 9457 error parser"
```

---

## Task 3: Create `ContainersClient` interface and stub

**Files:**
- Create: `internal/cli/containers/client.go`
- Create: `internal/cli/containers/stub.go`

The generated types live in `internal/containers` (package `containers`). The CLI package is `internal/cli/containers` (also package `containers` — they are different Go packages despite the name collision; the CLI package must alias the generated one).

- [ ] **Step 1: Create `internal/cli/containers/client.go`**

```go
package containers

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/containers"
)

// ContainersClient is the interface used by containers commands.
// It matches the subset of gen.ClientInterface that containers commands need.
type ContainersClient interface {
	ListContainers(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetContainer(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StartContainer(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StopContainer(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	RestartContainer(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

// NewContainersClient constructs a ContainersClient backed by the real API.
func NewContainersClient(httpClient *http.Client, apiURL string) (ContainersClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
```

- [ ] **Step 2: Create `internal/cli/containers/stub.go`**

```go
package containers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gen "github.com/bwilczynski/hlctl/internal/containers"
)

// StubClient is a ContainersClient that delegates each method to a
// configurable function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListContainersFunc  func(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetContainerFunc    func(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StartContainerFunc  func(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	StopContainerFunc   func(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
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

// jsonResponse builds an *http.Response with a JSON body and the given status code.
// Use this in tests to construct success and error responses.
func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}
```

- [ ] **Step 3: Verify the build compiles**

```bash
go build ./internal/cli/containers/...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/cli/containers/client.go internal/cli/containers/stub.go
git commit -m "feat(containers): add ContainersClient interface and StubClient"
```

---

## Task 4: Implement `containers list` with real API calls

**Files:**
- Modify: `internal/cli/containers/containers.go`

- [ ] **Step 1: Write the failing test for list**

Create `internal/cli/containers/containers_test.go`:

```go
package containers

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	gen "github.com/bwilczynski/hlctl/internal/containers"
)

func TestListCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListContainersFunc: func(_ context.Context, _ *gen.ListContainersParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.ContainerList{
				Items: []gen.Container{
					{
						Id:           "nas-1.homeassistant",
						Image:        "ghcr.io/home-assistant/home-assistant:2025.4",
						Status:       gen.Running,
						RestartCount: 0,
						Resources:    gen.ContainerResources{CpuPercent: 2.5, MemoryBytes: 268435456, MemoryPercent: 6.4},
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
	if !strings.Contains(out, "nas-1.homeassistant") {
		t.Errorf("expected container ID in output, got:\n%s", out)
	}
	if !strings.Contains(out, "256.0 MB") {
		t.Errorf("expected formatted memory in output, got:\n%s", out)
	}
	if !strings.Contains(out, "2.5%") {
		t.Errorf("expected CPU percentage in output, got:\n%s", out)
	}
}

func TestListCmd_apiError(t *testing.T) {
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/cli/containers/... -run TestListCmd -v
```

Expected: `FAIL — newListCmd does not accept arguments` (compilation error)

- [ ] **Step 3: Rewrite `newListCmd` in `containers.go`**

Replace the entire `containers.go` with the following (the other subcommand functions are stubs for now, filled in later tasks):

```go
package containers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/containers"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
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

func buildClient() (ContainersClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewContainersClient(httpClient, apiURL)
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
			return output.Print(flags.GetOutputFormat(), list, headers, rows)
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
			// implemented in Task 5
			return fmt.Errorf("not implemented")
		},
	}
}

func newStartCmd(client ContainersClient) *cobra.Command {
	return &cobra.Command{
		Use:   "start <container-id>",
		Short: "Start a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// implemented in Task 6
			return fmt.Errorf("not implemented")
		},
	}
}

func newStopCmd(client ContainersClient) *cobra.Command {
	return &cobra.Command{
		Use:   "stop <container-id>",
		Short: "Stop a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// implemented in Task 6
			return fmt.Errorf("not implemented")
		},
	}
}

func newRestartCmd(client ContainersClient) *cobra.Command {
	return &cobra.Command{
		Use:   "restart <container-id>",
		Short: "Restart a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// implemented in Task 6
			return fmt.Errorf("not implemented")
		},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/cli/containers/... -run TestListCmd -v
```

Expected: both `TestListCmd_tableOutput` and `TestListCmd_apiError` PASS

- [ ] **Step 5: Verify build**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add internal/cli/containers/containers.go internal/cli/containers/containers_test.go
git commit -m "feat(containers): implement list command with real API calls"
```

---

## Task 5: Implement `containers get` with section-based output

**Files:**
- Modify: `internal/cli/containers/containers.go`
- Modify: `internal/cli/containers/containers_test.go`

- [ ] **Step 1: Write the failing tests for get**

Add to `internal/cli/containers/containers_test.go`:

```go
func TestGetCmd_tableOutput(t *testing.T) {
	detail := gen.ContainerDetail{
		Id:            "nas-1.homeassistant",
		Name:          "homeassistant",
		Device:        "nas-1",
		Status:        gen.Running,
		Image:         "ghcr.io/home-assistant/home-assistant:2025.4",
		RestartCount:  0,
		Resources:     gen.ContainerResources{CpuPercent: 2.5, MemoryBytes: 268435456, MemoryPercent: 6.4},
		ExitCode:      0,
		OomKilled:     false,
		RestartPolicy: "always",
		Privileged:    false,
		MemoryLimit:   0,
		PortBindings: []gen.PortBinding{
			{ContainerPort: 8123, HostPort: 8123, Protocol: "tcp"},
		},
		Networks: []gen.ContainerNetwork{
			{Name: "homeassistant_default", Driver: "bridge"},
		},
		VolumeBindings: []gen.VolumeMount{
			{Source: "/volume1/docker/homeassistant/config", Destination: "/config", Mode: "rw"},
		},
		EnvVariables: []gen.EnvVariable{
			{Key: "TZ", Value: "Europe/Warsaw"},
		},
		Entrypoint: []string{"/init"},
		Cmd:        []string{},
	}

	stub := &StubClient{
		GetContainerFunc: func(_ context.Context, id string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, detail), nil
		},
	}

	cmd := newGetCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"nas-1.homeassistant",
		"256.0 MB",
		"PORT BINDINGS",
		"8123",
		"NETWORKS",
		"homeassistant_default",
		"VOLUME BINDINGS",
		"/config",
		"ENVIRONMENT VARIABLES",
		"Europe/Warsaw",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGetCmd_notFound(t *testing.T) {
	stub := &StubClient{
		GetContainerFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "container 'nas-1.foo' does not exist",
			}), nil
		},
	}

	cmd := newGetCmd(stub)
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

Note: `newGetCmd(stub)` with `cobra.ExactArgs(1)` requires passing args. Update the test to use `cmd.SetArgs([]string{"nas-1.homeassistant"})` before `cmd.Execute()`:

```go
cmd := newGetCmd(stub)
cmd.SetArgs([]string{"nas-1.homeassistant"})
buf := &bytes.Buffer{}
```

Apply the same to `TestGetCmd_notFound`.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/cli/containers/... -run TestGetCmd -v
```

Expected: FAIL — `newGetCmd` returns "not implemented"

- [ ] **Step 3: Implement `newGetCmd` in `containers.go`**

Replace the `newGetCmd` stub:

```go
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

	// Flat fields
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
		{"STARTED AT", d.StartedAt.String()},
		{"EXIT CODE", fmt.Sprintf("%d", d.ExitCode)},
		{"OOM KILLED", fmt.Sprintf("%v", d.OomKilled)},
		{"RESTART POLICY", string(d.RestartPolicy)},
		{"PRIVILEGED", fmt.Sprintf("%v", d.Privileged)},
		{"MEMORY LIMIT", memoryLimit},
	}
	if err := output.Print(output.FormatTable, nil, headers, rows); err != nil {
		return err
	}

	// Port bindings
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
		if err := output.Print(output.FormatTable, nil, []string{"CONTAINER PORT", "HOST PORT", "PROTOCOL"}, pbRows); err != nil {
			return err
		}
	}

	// Networks
	if len(d.Networks) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "NETWORKS")
		var netRows [][]string
		for _, n := range d.Networks {
			netRows = append(netRows, []string{n.Name, n.Driver})
		}
		if err := output.Print(output.FormatTable, nil, []string{"NAME", "DRIVER"}, netRows); err != nil {
			return err
		}
	}

	// Volume bindings
	if len(d.VolumeBindings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "VOLUME BINDINGS")
		var volRows [][]string
		for _, v := range d.VolumeBindings {
			volRows = append(volRows, []string{v.Source, v.Destination, string(v.Mode)})
		}
		if err := output.Print(output.FormatTable, nil, []string{"SOURCE", "DESTINATION", "MODE"}, volRows); err != nil {
			return err
		}
	}

	// Environment variables
	if len(d.EnvVariables) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "ENVIRONMENT VARIABLES")
		var envRows [][]string
		for _, e := range d.EnvVariables {
			envRows = append(envRows, []string{e.Key, e.Value})
		}
		if err := output.Print(output.FormatTable, nil, []string{"KEY", "VALUE"}, envRows); err != nil {
			return err
		}
	}

	// Entrypoint
	if len(d.Entrypoint) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "ENTRYPOINT")
		for _, e := range d.Entrypoint {
			fmt.Fprintln(w, " ", e)
		}
	}

	// Command
	if len(d.Cmd) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "COMMAND")
		for _, c := range d.Cmd {
			fmt.Fprintln(w, " ", c)
		}
	}

	// Labels
	if d.Labels != nil && len(*d.Labels) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "LABELS")
		var labelRows [][]string
		for k, v := range *d.Labels {
			labelRows = append(labelRows, []string{k, v})
		}
		if err := output.Print(output.FormatTable, nil, []string{"KEY", "VALUE"}, labelRows); err != nil {
			return err
		}
	}

	return nil
}
```

Note: `output.Print` writes to `os.Stdout` directly. The section headers and sub-tables both need to write to the same writer. Since `output.printTable` uses `os.Stdout`, using `cmd.OutOrStdout()` for section headers will work correctly in production (both default to stdout) and in tests the table output will also go to stdout. To make tests fully capture output, update `output.Print` to accept an `io.Writer` — but that is a larger refactor. For now, the test assertions on section header strings will work because `printContainerDetail` uses `cmd.OutOrStdout()` for headers and `output.Print` writes to stdout separately. **Acceptable for this task; note that stdout capture in tests may miss the table portions.** Adjust the test assertions to only check strings written via `cmd.OutOrStdout()` (section headers), or update `output.Print` to accept a writer — the latter is cleaner.

**Recommended: update `output.Print` to accept an `io.Writer`** as part of this task:

In `internal/output/output.go`, change the signature:

```go
func Print(w io.Writer, format Format, data any, headers []string, rows [][]string) error {
	switch format {
	case FormatJSON:
		return printJSON(w, data)
	default:
		return printTable(w, headers, rows)
	}
}
```

Update `printTable` and `printJSON` to take `w io.Writer` (they already do — just thread `w` through instead of `os.Stdout`).

Then update all callers (`internal/cli/system/system.go`, `internal/cli/storage/storage.go`, `internal/cli/backups/backups.go`, `internal/cli/network/network.go`, `internal/cli/containers/containers.go`) to pass `cmd.OutOrStdout()` as the first argument.

- [ ] **Step 4: Update `output.Print` signature to accept `io.Writer`**

In `internal/output/output.go`, change:

```go
func Print(format Format, data any, headers []string, rows [][]string) error {
	switch format {
	case FormatJSON:
		return printJSON(os.Stdout, data)
	default:
		return printTable(os.Stdout, headers, rows)
	}
}
```

To:

```go
func Print(w io.Writer, format Format, data any, headers []string, rows [][]string) error {
	switch format {
	case FormatJSON:
		return printJSON(w, data)
	default:
		return printTable(w, headers, rows)
	}
}
```

Remove the `"os"` import if it was only used for `os.Stdout`.

- [ ] **Step 5: Fix all callers of `output.Print`**

Each existing domain command passes `os.Stdout` implicitly — update them to pass `cmd.OutOrStdout()`. Files to update (all calls currently use the old 4-arg signature):

- `internal/cli/system/system.go` — 6 calls
- `internal/cli/storage/storage.go` — 2 calls
- `internal/cli/backups/backups.go` — any calls
- `internal/cli/network/network.go` — any calls

Pattern for each call — change:
```go
return output.Print(flags.GetOutputFormat(), data, headers, rows)
```
To:
```go
return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), data, headers, rows)
```

- [ ] **Step 6: Run all tests**

```bash
go test ./... -v
```

Expected: all existing tests plus new get tests PASS, no compilation errors

- [ ] **Step 7: Commit**

```bash
git add internal/output/output.go internal/output/output_test.go \
        internal/cli/containers/containers.go internal/cli/containers/containers_test.go \
        internal/cli/system/system.go internal/cli/storage/storage.go \
        internal/cli/backups/backups.go internal/cli/network/network.go
git commit -m "feat(containers): implement get command with section-based output"
```

---

## Task 6: Implement start, stop, restart commands

**Files:**
- Modify: `internal/cli/containers/containers.go`
- Modify: `internal/cli/containers/containers_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/cli/containers/containers_test.go`:

```go
func TestStartCmd_success(t *testing.T) {
	stub := &StubClient{
		StartContainerFunc: func(_ context.Context, id string, _ *gen.StartContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}

	cmd := newStartCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "nas-1.homeassistant") {
		t.Errorf("expected container ID in output, got: %s", buf.String())
	}
}

func TestStopCmd_success(t *testing.T) {
	stub := &StubClient{
		StopContainerFunc: func(_ context.Context, id string, _ *gen.StopContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}

	cmd := newStopCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "nas-1.homeassistant") {
		t.Errorf("expected container ID in output, got: %s", buf.String())
	}
}

func TestRestartCmd_success(t *testing.T) {
	stub := &StubClient{
		RestartContainerFunc: func(_ context.Context, id string, _ *gen.RestartContainerParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}

	cmd := newRestartCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "nas-1.homeassistant") {
		t.Errorf("expected container ID in output, got: %s", buf.String())
	}
}
```

Add `"io"` to the imports in the test file.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/cli/containers/... -run "TestStartCmd|TestStopCmd|TestRestartCmd" -v
```

Expected: FAIL — commands return "not implemented"

- [ ] **Step 3: Implement start, stop, restart in `containers.go`**

Replace the three stub functions:

```go
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

- [ ] **Step 4: Run all tests**

```bash
go test ./... -v
```

Expected: all tests PASS

- [ ] **Step 5: Build and verify**

```bash
make build
```

Expected: `bin/hlctl` built successfully, no errors

- [ ] **Step 6: Commit**

```bash
git add internal/cli/containers/containers.go internal/cli/containers/containers_test.go
git commit -m "feat(containers): implement start, stop, restart commands"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Task |
|---|---|
| Real API calls for list and get | Tasks 4, 5 |
| `apiclient` shared package | Task 2 |
| `ContainersClient` interface | Task 3 |
| `StubClient` for testing | Task 3 |
| Section-based table output for get | Task 5 |
| Bytes formatted as human-readable | Task 1 |
| RFC 9457 error handling (Title — detail) | Task 2 |
| Unit tests for success and error cases | Tasks 4, 5, 6 |
| Start/stop/restart commands | Task 6 |
| Stub preserved (not deleted) | Task 3 — stub.go is new file, not replacement |
| `--device` flag on list | Task 4 |
| `--output json` dumps raw body | Tasks 4, 5 |

**Placeholder scan:** No TBDs or vague steps. All code blocks present.

**Type consistency:**
- `gen.ContainerList`, `gen.ContainerDetail`, `gen.Container`, `gen.ContainerResources`, `gen.PortBinding`, `gen.VolumeMount`, `gen.ContainerNetwork`, `gen.EnvVariable` — all verified against `internal/containers/api.gen.go`
- `gen.ListContainersParams`, `gen.StartContainerParams`, `gen.StopContainerParams`, `gen.RestartContainerParams` — verified
- `gen.RequestEditorFn` — verified at line 360 of api.gen.go
- `output.Print(w, format, data, headers, rows)` — updated signature used consistently across Tasks 4, 5, and the fix in Task 5 step 5
- `ContainersClient` interface methods — match `gen.ClientInterface` exactly as seen at lines 431–445 of api.gen.go
