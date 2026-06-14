# CLI Refactor Phase 2: Options + runF + httpmock

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace implicit context-keyed DI (`InjectClient`/`Client[C]`/`SetClient[C]`) with explicit per-leaf `Options` structs and a `runF` test hook, backed by HTTP-transport mocking instead of typed client stubs.

**Architecture:** Each leaf command defines an `Options` struct (function fields for lazy resources, value fields for flags/args), a `NewCmdXxx(f, runF)` constructor that binds flags into `opts` and calls either `runF(opts)` or `xyzRun(ctx, w, opts)`, and a package-private `xyzRun` that owns all business logic. Tests split into Layer 1 (flag parsing via `runF` interception) and Layer 2 (business logic via direct `xyzRun` call + `httpmock.Registry`). Domain roots drop `InjectClient` entirely.

**Tech Stack:** Go stdlib `net/http`, `regexp`, `sync`; `github.com/spf13/cobra`; existing `internal/output`, `internal/api`, `internal/cli/cmdutil` packages.

---

## File map

**New:**
- `internal/cli/cmdutil/httpmock/httpmock.go` — Registry, Matcher, Responder, REST, JSONResponse, StatusStringResponse, StatusJSONResponse
- `internal/cli/cmdutil/httpmock/httpmock_test.go` — Registry unit tests

**Modified (domain roots — remove InjectClient):**
- `internal/cli/docker/docker.go` — `newContainersCmd`, `newNetworksCmd`, `newImagesCmd` drop InjectClient, pass `nil` runF to leaves
- `internal/cli/storage/storage.go` — `newVolumesCmd`, `newBackupsCmd` drop InjectClient
- `internal/cli/system/system.go` — `NewCmd` drops InjectClient
- `internal/cli/network/network.go` — `NewCmd` drops InjectClient

**Modified (leaves — Options+runF rewrite):**
- `internal/cli/docker/containers.go` + `containers_test.go`
- `internal/cli/docker/networks.go` + `networks_test.go`
- `internal/cli/docker/images.go` + `images_test.go`
- `internal/cli/storage/volumes.go` + `volumes_test.go`
- `internal/cli/storage/backups.go` + `backups_test.go`
- `internal/cli/system/health.go` + `health_test.go`
- `internal/cli/system/info.go` + `info_test.go`
- `internal/cli/system/utilization.go` + `utilization_test.go`
- `internal/cli/system/updates.go` + `updates_test.go`
- `internal/cli/network/clients.go` + `clients_test.go`
- `internal/cli/network/devices.go` + `devices_test.go`
- `internal/cli/network/topology.go` + `topology_test.go`
- `internal/cli/network/ssids.go` + `ssids_test.go`
- `internal/cli/network/vlans.go` + `vlans_test.go`
- `internal/cli/network/wans.go` + `wans_test.go`

**Deleted:**
- `internal/cli/docker/stub.go`
- `internal/cli/storage/stub.go`
- `internal/cli/system/stub.go`
- `internal/cli/network/stub.go`
- `internal/cli/cmdutil/client.go`
- `internal/cli/cmdutil/client_test.go`
- `internal/cli/cmdutil/action.go`
- `internal/cli/cmdutil/action_test.go`

---

## API path reference

The oapi-codegen client appends spec paths to the server URL. When tests set `apiURL = "http://localhost"`, the `req.URL.Path` seen by httpmock is the raw spec path. Verify these against the generated client after `make generate`:

| Method | Spec path |
|---|---|
| ListContainersWithResponse | GET /docker/containers |
| GetContainerWithResponse | GET /docker/containers/{id} |
| StartContainerWithResponse | POST /docker/containers/{id}/start |
| StopContainerWithResponse | POST /docker/containers/{id}/stop |
| RestartContainerWithResponse | POST /docker/containers/{id}/restart |
| ListDockerNetworksWithResponse | GET /docker/networks |
| GetDockerNetworkWithResponse | GET /docker/networks/{id} |
| ListDockerImagesWithResponse | GET /docker/images |
| GetDockerImageWithResponse | GET /docker/images/{id} |
| ListStorageVolumesWithResponse | GET /storage/volumes |
| GetStorageVolumeWithResponse | GET /storage/volumes/{id} |
| ListBackupsWithResponse | GET /storage/backups |
| GetBackupWithResponse | GET /storage/backups/{id} |
| GetSystemHealthWithResponse | GET /system/health |
| ListSystemInfoWithResponse | GET /system/info |
| ListSystemUtilizationWithResponse | GET /system/utilization |
| ListSystemUpdatesWithResponse | GET /system/updates |
| GetSystemUpdateWithResponse | GET /system/updates/{id} |
| CheckSystemUpdatesWithResponse | POST /system/updates/check |
| ListNetworkDevicesWithResponse | GET /network/devices |
| GetNetworkDeviceWithResponse | GET /network/devices/{id} |
| ListNetworkClientsWithResponse | GET /network/clients |
| GetNetworkClientWithResponse | GET /network/clients/{id} |
| GetNetworkTopologyWithResponse | GET /network/topology |
| ListVlansWithResponse | GET /network/vlans |
| GetVlanWithResponse | GET /network/vlans/{id} |
| ListSsidsWithResponse | GET /network/ssids |
| GetSsidWithResponse | GET /network/ssids/{id} |
| ListWansWithResponse | GET /network/wans |
| GetWanWithResponse | GET /network/wans/{id} |

---

## Task 1: httpmock package

**Files:**
- Create: `internal/cli/cmdutil/httpmock/httpmock.go`
- Create: `internal/cli/cmdutil/httpmock/httpmock_test.go`

- [ ] **Step 1: Write `httpmock.go`**

```go
// Package httpmock provides a fake http.RoundTripper for testing commands
// that build real HTTP clients via their Options.HTTPClient field.
package httpmock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// Matcher returns true when a request should be handled by its paired Responder.
type Matcher func(*http.Request) bool

// Responder produces an HTTP response for a matched request.
type Responder func(*http.Request) (*http.Response, error)

type registered struct {
	matcher   Matcher
	responder Responder
	count     int
}

// Registry is an http.RoundTripper that matches requests against registered
// matchers and calls the associated responder. Unmatched requests return an error.
type Registry struct {
	mu   sync.Mutex
	regs []*registered
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{} }

// Register adds a matcher+responder pair. Matchers are checked in registration order.
func (r *Registry) Register(matcher Matcher, responder Responder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.regs = append(r.regs, &registered{matcher: matcher, responder: responder})
}

// RoundTrip implements http.RoundTripper. Returns an error if no matcher matches.
func (r *Registry) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, reg := range r.regs {
		if reg.matcher(req) {
			reg.count++
			return reg.responder(req)
		}
	}
	return nil, fmt.Errorf("httpmock: no match for %s %s", req.Method, req.URL.Path)
}

// Verify fails the test if any registered matcher was never matched.
func (r *Registry) Verify(t *testing.T) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, reg := range r.regs {
		if reg.count == 0 {
			t.Errorf("httpmock: registered handler was never called")
		}
	}
}

// REST returns a Matcher that checks method (exact) and path (glob: * matches [^/]+).
func REST(method, pathPattern string) Matcher {
	regexStr := "^" + strings.ReplaceAll(regexp.QuoteMeta(pathPattern), `\*`, `[^/]+`) + "$"
	pathRe := regexp.MustCompile(regexStr)
	return func(req *http.Request) bool {
		return req.Method == method && pathRe.MatchString(req.URL.Path)
	}
}

// JSONResponse returns a 200 OK with body marshalled as JSON.
func JSONResponse(body any) Responder {
	return func(_ *http.Request) (*http.Response, error) {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(b)),
		}, nil
	}
}

// StatusStringResponse returns a response with the given status code and plain text body.
func StatusStringResponse(status int, body string) Responder {
	return func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}
}

// StatusJSONResponse returns a response with the given status code and body marshalled as JSON.
func StatusJSONResponse(status int, body any) Responder {
	return func(_ *http.Request) (*http.Response, error) {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(b)),
		}, nil
	}
}
```

- [ ] **Step 2: Write `httpmock_test.go`**

```go
package httpmock_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
)

func TestRegistry_matchesAndCounts(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/docker/containers"), httpmock.JSONResponse(map[string]any{"items": []any{}}))

	client := &http.Client{Transport: reg}
	resp, err := client.Get("http://localhost/docker/containers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	reg.Verify(t) // should not fail
}

func TestRegistry_noMatch(t *testing.T) {
	reg := httpmock.NewRegistry()
	client := &http.Client{Transport: reg}
	_, err := client.Get("http://localhost/not/registered")
	if err == nil {
		t.Fatal("expected error for unmatched request")
	}
}

func TestREST_glob(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(
		httpmock.REST("POST", "/docker/containers/*/start"),
		httpmock.StatusStringResponse(http.StatusNoContent, ""),
	)
	client := &http.Client{Transport: reg}
	resp, err := client.Post("http://localhost/docker/containers/nas-1.homeassistant/start", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

func TestStatusJSONResponse(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/x"), httpmock.StatusJSONResponse(404, map[string]any{"title": "Not Found"}))
	client := &http.Client{Transport: reg}
	resp, err := client.Get("http://localhost/x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != `{"title":"Not Found"}` {
		t.Errorf("unexpected body: %s", b)
	}
}
```

- [ ] **Step 3: Run tests**

```
cd internal/cli/cmdutil/httpmock && go test ./...
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/cli/cmdutil/httpmock/
git commit -m "feat: add cmdutil/httpmock test transport package"
```

---

## Task 2: Docker domain

**Files:**
- Modify: `internal/cli/docker/containers.go`
- Modify: `internal/cli/docker/containers_test.go`
- Modify: `internal/cli/docker/networks.go`
- Modify: `internal/cli/docker/networks_test.go`
- Modify: `internal/cli/docker/images.go`
- Modify: `internal/cli/docker/images_test.go`
- Modify: `internal/cli/docker/docker.go`
- Delete: `internal/cli/docker/stub.go`

- [ ] **Step 1: Rewrite `containers.go`**

```go
package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"

	dockerapi "github.com/bwilczynski/hlctl/internal/api"
	dockerapiclient "github.com/bwilczynski/hlctl/internal/api/docker"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	containersListView = cmdutil.View{Templates: dockerTemplates, Name: "containers_list.tmpl"}
	containersGetView  = cmdutil.View{Templates: dockerTemplates, Name: "containers_get.tmpl"}
)

// --- list ---

type listContainersOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newListContainersCmd(f *cmdutil.Factory, runF func(*listContainersOptions) error) *cobra.Command {
	opts := &listContainersOptions{
		HTTPClient: f.HTTPClient,
		IO:         f.IOStreams,
		Output:     f.Output,
	}
	cmd := &cobra.Command{Use: "list", Short: "List containers"}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	cmd.RunE = watch.Wrap(f.Output, func(ctx context.Context, w io.Writer) error {
		if runF != nil {
			return runF(opts)
		}
		return listContainersRun(ctx, w, opts)
	})
	watch.RegisterFlags(cmd)
	return cmd
}

func listContainersRun(ctx context.Context, w io.Writer, opts *listContainersOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &dockerapiclient.ListContainersParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListContainersWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return containersListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

// --- get ---

type getContainerOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newGetContainerCmd(f *cmdutil.Factory, runF func(*getContainerOptions) error) *cobra.Command {
	opts := &getContainerOptions{
		HTTPClient: f.HTTPClient,
		IO:         f.IOStreams,
		Output:     f.Output,
	}
	return &cobra.Command{
		Use:   "get <container-id>",
		Short: "Show container details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getContainerRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getContainerRun(ctx context.Context, w io.Writer, opts *getContainerOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetContainerWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return containersGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

// --- start ---

type startContainerOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	ID         string
}

func newStartContainerCmd(f *cmdutil.Factory, runF func(*startContainerOptions) error) *cobra.Command {
	opts := &startContainerOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams}
	return &cobra.Command{
		Use:   "start <container-id>",
		Short: "Start a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return startContainerRun(cmd.Context(), opts)
		},
	}
}

func startContainerRun(ctx context.Context, opts *startContainerOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	r, err := c.StartContainerWithResponse(ctx, opts.ID, &dockerapiclient.StartContainerParams{})
	if err != nil {
		return err
	}
	if r.StatusCode() != http.StatusNoContent {
		return dockerapi.ParseError(r.StatusCode(), r.Body)
	}
	fmt.Fprintf(opts.IO.Out, "%s started\n", opts.ID)
	return nil
}

// --- stop ---

type stopContainerOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	ID         string
}

func newStopContainerCmd(f *cmdutil.Factory, runF func(*stopContainerOptions) error) *cobra.Command {
	opts := &stopContainerOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams}
	return &cobra.Command{
		Use:   "stop <container-id>",
		Short: "Stop a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return stopContainerRun(cmd.Context(), opts)
		},
	}
}

func stopContainerRun(ctx context.Context, opts *stopContainerOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	r, err := c.StopContainerWithResponse(ctx, opts.ID, &dockerapiclient.StopContainerParams{})
	if err != nil {
		return err
	}
	if r.StatusCode() != http.StatusNoContent {
		return dockerapi.ParseError(r.StatusCode(), r.Body)
	}
	fmt.Fprintf(opts.IO.Out, "%s stopped\n", opts.ID)
	return nil
}

// --- restart ---

type restartContainerOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	ID         string
}

func newRestartContainerCmd(f *cmdutil.Factory, runF func(*restartContainerOptions) error) *cobra.Command {
	opts := &restartContainerOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams}
	return &cobra.Command{
		Use:   "restart <container-id>",
		Short: "Restart a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return restartContainerRun(cmd.Context(), opts)
		},
	}
}

func restartContainerRun(ctx context.Context, opts *restartContainerOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	r, err := c.RestartContainerWithResponse(ctx, opts.ID, &dockerapiclient.RestartContainerParams{})
	if err != nil {
		return err
	}
	if r.StatusCode() != http.StatusNoContent {
		return dockerapi.ParseError(r.StatusCode(), r.Body)
	}
	fmt.Fprintf(opts.IO.Out, "%s restarted\n", opts.ID)
	return nil
}
```

Note: `internal/api` is the package containing `ParseError`. Verify the import path — `dockerapi "github.com/bwilczynski/hlctl/internal/api"` for `dockerapi.ParseError`.

- [ ] **Step 2: Rewrite `containers_test.go`**

```go
package docker

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	dockerapiclient "github.com/bwilczynski/hlctl/internal/api/docker"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

// --- Layer 1: flag parsing ---

func TestNewListContainersCmd_deviceFlag(t *testing.T) {
	var captured *listContainersOptions
	cmd := newListContainersCmd(cmdutil.TestFactory(t), func(o *listContainersOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"--device", "nas-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Device != "nas-1" {
		t.Errorf("expected Device=nas-1, got %q", captured.Device)
	}
}

func TestNewGetContainerCmd_argParsed(t *testing.T) {
	var captured *getContainerOptions
	cmd := newGetContainerCmd(cmdutil.TestFactory(t), func(o *getContainerOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.ID != "nas-1.homeassistant" {
		t.Errorf("expected ID=nas-1.homeassistant, got %q", captured.ID)
	}
}

func TestNewStartContainerCmd_argParsed(t *testing.T) {
	var captured *startContainerOptions
	cmd := newStartContainerCmd(cmdutil.TestFactory(t), func(o *startContainerOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.ID != "nas-1.homeassistant" {
		t.Errorf("expected ID=nas-1.homeassistant, got %q", captured.ID)
	}
}

// --- Layer 2: business logic ---

func testHTTPClient(reg *httpmock.Registry) func() (*http.Client, string, error) {
	return func() (*http.Client, string, error) {
		return &http.Client{Transport: reg}, "http://localhost", nil
	}
}

func TestListContainersRun_tableOutput(t *testing.T) {
	list := dockerapiclient.ContainerList{
		Items: []dockerapiclient.Container{{
			Id:        "nas-1.homeassistant",
			Image:     "homeassistant/home-assistant:latest",
			Status:    dockerapiclient.Running,
			Resources: dockerapiclient.ContainerResources{CpuPercent: 1.5, MemoryBytes: 104857600},
		}},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/docker/containers"), httpmock.JSONResponse(list))

	var out bytes.Buffer
	opts := &listContainersOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listContainersRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.homeassistant", "homeassistant/home-assistant:latest", "running", "1.5%"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestListContainersRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(
		httpmock.REST("GET", "/docker/containers"),
		httpmock.StatusJSONResponse(http.StatusUnauthorized, map[string]any{
			"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized", "status": 401, "detail": "Bearer token missing",
		}),
	)
	var out bytes.Buffer
	opts := &listContainersOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	err := listContainersRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
	reg.Verify(t)
}

func TestGetContainerRun_tableOutput(t *testing.T) {
	detail := dockerapiclient.ContainerDetail{
		Id: "nas-1.homeassistant", Name: "homeassistant", Device: "nas-1",
		Status: dockerapiclient.Running, Image: "homeassistant/home-assistant:latest",
		RestartPolicy: dockerapiclient.Always,
		Resources:     dockerapiclient.ContainerResources{CpuPercent: 1.5, MemoryBytes: 104857600, MemoryPercent: 5.0},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/docker/containers/*"), httpmock.JSONResponse(detail))

	var out bytes.Buffer
	opts := &getContainerOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.homeassistant",
	}
	if err := getContainerRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.homeassistant", "homeassistant", "running", "always"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestStartContainerRun(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("POST", "/docker/containers/*/start"), httpmock.StatusStringResponse(http.StatusNoContent, ""))

	var out bytes.Buffer
	opts := &startContainerOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		ID:         "nas-1.homeassistant",
	}
	if err := startContainerRun(context.Background(), opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "nas-1.homeassistant started") {
		t.Errorf("expected 'started' in output, got: %s", out.String())
	}
	reg.Verify(t)
}

func TestStopContainerRun(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("POST", "/docker/containers/*/stop"), httpmock.StatusStringResponse(http.StatusNoContent, ""))

	var out bytes.Buffer
	opts := &stopContainerOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		ID:         "nas-1.homeassistant",
	}
	if err := stopContainerRun(context.Background(), opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "nas-1.homeassistant stopped") {
		t.Errorf("expected 'stopped' in output, got: %s", out.String())
	}
	reg.Verify(t)
}

func TestRestartContainerRun(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("POST", "/docker/containers/*/restart"), httpmock.StatusStringResponse(http.StatusNoContent, ""))

	var out bytes.Buffer
	opts := &restartContainerOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		ID:         "nas-1.homeassistant",
	}
	if err := restartContainerRun(context.Background(), opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "nas-1.homeassistant restarted") {
		t.Errorf("expected 'restarted' in output, got: %s", out.String())
	}
	reg.Verify(t)
}

// unused import guard
var _ = time.Now
```

Note: Remove the `time` import if no test uses it. Add it only if a fixture needs it.

- [ ] **Step 3: Rewrite `networks.go`**

```go
package docker

import (
	"context"
	"io"
	"net/http"

	dockerapiclient "github.com/bwilczynski/hlctl/internal/api/docker"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	networksListView = cmdutil.View{Templates: dockerTemplates, Name: "networks_list.tmpl"}
	networksGetView  = cmdutil.View{Templates: dockerTemplates, Name: "networks_get.tmpl"}
)

type listNetworksOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newNetworksCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "networks", Short: "Docker networks"}
	cmd.AddCommand(newListNetworksCmd(f, nil), newGetNetworkCmd(f, nil))
	return cmd
}

func newListNetworksCmd(f *cmdutil.Factory, runF func(*listNetworksOptions) error) *cobra.Command {
	opts := &listNetworksOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Docker networks",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listNetworksRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	return cmd
}

func listNetworksRun(ctx context.Context, w io.Writer, opts *listNetworksOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &dockerapiclient.ListDockerNetworksParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListDockerNetworksWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return networksListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

type getNetworkOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newGetNetworkCmd(f *cmdutil.Factory, runF func(*getNetworkOptions) error) *cobra.Command {
	opts := &getNetworkOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <network-id>",
		Short: "Show network details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getNetworkRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getNetworkRun(ctx context.Context, w io.Writer, opts *getNetworkOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetDockerNetworkWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return networksGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
```

- [ ] **Step 4: Rewrite `networks_test.go`** (keep same fixture data as existing test, adapt to httpmock)

Look at existing `networks_test.go` for fixture values, then write:

```go
package docker

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

func TestNewListNetworksCmd_deviceFlag(t *testing.T) {
	var captured *listNetworksOptions
	cmd := newListNetworksCmd(cmdutil.TestFactory(t), func(o *listNetworksOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"--device", "nas-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Device != "nas-1" {
		t.Errorf("expected Device=nas-1, got %q", captured.Device)
	}
}

func TestListNetworksRun_tableOutput(t *testing.T) {
	// Use the same fixture data as the existing networks_test.go
	// Check that file for the exact struct fields used.
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/docker/networks"), httpmock.JSONResponse(map[string]any{
		"items": []map[string]any{
			{"id": "nas-1.bridge", "name": "bridge", "driver": "bridge", "device": "nas-1"},
		},
	}))
	var out bytes.Buffer
	opts := &listNetworksOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listNetworksRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "bridge") {
		t.Errorf("expected 'bridge' in output, got:\n%s", out.String())
	}
	reg.Verify(t)
}

func TestGetNetworkRun_tableOutput(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/docker/networks/*"), httpmock.JSONResponse(map[string]any{
		"id": "nas-1.bridge", "name": "bridge", "driver": "bridge", "device": "nas-1",
	}))
	var out bytes.Buffer
	opts := &getNetworkOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.bridge",
	}
	if err := getNetworkRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "bridge") {
		t.Errorf("expected 'bridge' in output, got:\n%s", out.String())
	}
	reg.Verify(t)
}

// suppress unused import
var _ = http.StatusOK
```

Note: Look at the existing `networks_test.go` for the full set of fixture fields to use in the JSON maps. Fill in any fields that the template renders so `strings.Contains` assertions match.

- [ ] **Step 5: Rewrite `images.go`**

```go
package docker

import (
	"context"
	"io"
	"net/http"

	dockerapiclient "github.com/bwilczynski/hlctl/internal/api/docker"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	imagesListView = cmdutil.View{Templates: dockerTemplates, Name: "images_list.tmpl"}
	imagesGetView  = cmdutil.View{Templates: dockerTemplates, Name: "images_get.tmpl"}
)

type listImagesOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newImagesCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "images", Short: "Docker images"}
	cmd.AddCommand(newListImagesCmd(f, nil), newGetImageCmd(f, nil))
	return cmd
}

func newListImagesCmd(f *cmdutil.Factory, runF func(*listImagesOptions) error) *cobra.Command {
	opts := &listImagesOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Docker images",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listImagesRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	return cmd
}

func listImagesRun(ctx context.Context, w io.Writer, opts *listImagesOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &dockerapiclient.ListDockerImagesParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListDockerImagesWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return imagesListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

type getImageOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newGetImageCmd(f *cmdutil.Factory, runF func(*getImageOptions) error) *cobra.Command {
	opts := &getImageOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <image-id>",
		Short: "Show image details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getImageRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getImageRun(ctx context.Context, w io.Writer, opts *getImageOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewDockerClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetDockerImageWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return imagesGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
```

- [ ] **Step 6: Rewrite `images_test.go`** following the same two-layer pattern as `networks_test.go`; use `httpmock.REST("GET", "/docker/images")` and `httpmock.REST("GET", "/docker/images/*")`. Fixture JSON fields should match whatever the template renders (check existing `images_test.go` for field names).

- [ ] **Step 7: Update `docker.go`** — `newContainersCmd` drops InjectClient, passes `nil` runF to leaves:

```go
package docker

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "docker", Short: "Docker resources"}
	cmd.AddCommand(
		newContainersCmd(f),
		newNetworksCmd(f),
		newImagesCmd(f),
	)
	return cmd
}

func newContainersCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "containers", Short: "Manage Docker containers"}
	cmd.AddCommand(
		newListContainersCmd(f, nil),
		newGetContainerCmd(f, nil),
		newStartContainerCmd(f, nil),
		newStopContainerCmd(f, nil),
		newRestartContainerCmd(f, nil),
	)
	return cmd
}
```

- [ ] **Step 8: Delete `stub.go`**

```bash
rm internal/cli/docker/stub.go
```

- [ ] **Step 9: Build and test**

```bash
make build && go test ./internal/cli/docker/...
```

Expected: build succeeds, all tests pass.

- [ ] **Step 10: Commit**

```bash
git add internal/cli/docker/
git commit -m "refactor(docker): Options+runF+httpmock, delete stub"
```

---

## Task 3: Storage domain

**Files:**
- Modify: `internal/cli/storage/volumes.go` + `volumes_test.go`
- Modify: `internal/cli/storage/backups.go` + `backups_test.go`
- Modify: `internal/cli/storage/storage.go`
- Delete: `internal/cli/storage/stub.go`

- [ ] **Step 1: Rewrite `volumes.go`**

```go
package storage

import (
	"context"
	"io"
	"net/http"

	storageapi "github.com/bwilczynski/hlctl/internal/api/storage"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	volumesListView = cmdutil.View{Templates: storageTemplates, Name: "volumes_list.tmpl"}
	volumesGetView  = cmdutil.View{Templates: storageTemplates, Name: "volumes_get.tmpl"}
)

type listVolumesOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newVolumesCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "volumes", Short: "Storage volumes"}
	cmd.AddCommand(newListVolumesCmd(f, nil), newGetVolumeCmd(f, nil))
	return cmd
}

func newListVolumesCmd(f *cmdutil.Factory, runF func(*listVolumesOptions) error) *cobra.Command {
	opts := &listVolumesOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List storage volumes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listVolumesRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	return cmd
}

func listVolumesRun(ctx context.Context, w io.Writer, opts *listVolumesOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewStorageClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &storageapi.ListStorageVolumesParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListStorageVolumesWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return volumesListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

type getVolumeOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newGetVolumeCmd(f *cmdutil.Factory, runF func(*getVolumeOptions) error) *cobra.Command {
	opts := &getVolumeOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <volume-id>",
		Short: "Show volume details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getVolumeRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getVolumeRun(ctx context.Context, w io.Writer, opts *getVolumeOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewStorageClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetStorageVolumeWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return volumesGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
```

- [ ] **Step 2: Rewrite `volumes_test.go`** — two-layer tests using `httpmock.REST("GET", "/storage/volumes")` and `httpmock.REST("GET", "/storage/volumes/*")`. Copy the fixture data from the existing `volumes_test.go`.

```go
package storage

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

func testHTTPClient(reg *httpmock.Registry) func() (*http.Client, string, error) {
	return func() (*http.Client, string, error) {
		return &http.Client{Transport: reg}, "http://localhost", nil
	}
}

func TestNewListVolumesCmd_deviceFlag(t *testing.T) {
	var captured *listVolumesOptions
	cmd := newListVolumesCmd(cmdutil.TestFactory(t), func(o *listVolumesOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"--device", "nas-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Device != "nas-1" {
		t.Errorf("expected Device=nas-1, got %q", captured.Device)
	}
}

func TestListVolumesRun_tableOutput(t *testing.T) {
	// Use the same fixture fields as the existing volumes_test.go
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/storage/volumes"), httpmock.JSONResponse(map[string]any{
		"items": []map[string]any{
			{"id": "nas-1.data", "name": "data", "device": "nas-1", "mountpoint": "/mnt/data"},
		},
	}))
	var out bytes.Buffer
	opts := &listVolumesOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listVolumesRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "nas-1.data") {
		t.Errorf("expected 'nas-1.data' in output, got:\n%s", out.String())
	}
	reg.Verify(t)
}

func TestGetVolumeRun_tableOutput(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/storage/volumes/*"), httpmock.JSONResponse(map[string]any{
		"id": "nas-1.data", "name": "data", "device": "nas-1", "mountpoint": "/mnt/data",
	}))
	var out bytes.Buffer
	opts := &getVolumeOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.data",
	}
	if err := getVolumeRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "nas-1.data") {
		t.Errorf("expected 'nas-1.data' in output, got:\n%s", out.String())
	}
	reg.Verify(t)
}
```

Note: Fill in the JSON fixture fields to match what the volume templates actually render. Check `internal/cli/storage/templates.go` and the `.tmpl` files.

- [ ] **Step 3: Rewrite `backups.go`**

```go
package storage

import (
	"context"
	"io"
	"net/http"

	storageapi "github.com/bwilczynski/hlctl/internal/api/storage"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	backupsListView = cmdutil.View{Templates: storageTemplates, Name: "backups_list.tmpl"}
	backupsGetView  = cmdutil.View{Templates: storageTemplates, Name: "backups_get.tmpl"}
)

type listBackupsOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newBackupsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "backups", Short: "Backup tasks and history"}
	cmd.AddCommand(newListBackupsCmd(f, nil), newGetBackupCmd(f, nil))
	return cmd
}

func newListBackupsCmd(f *cmdutil.Factory, runF func(*listBackupsOptions) error) *cobra.Command {
	opts := &listBackupsOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List backups",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listBackupsRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	return cmd
}

func listBackupsRun(ctx context.Context, w io.Writer, opts *listBackupsOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewStorageClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &storageapi.ListBackupsParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListBackupsWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return backupsListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

type getBackupOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newGetBackupCmd(f *cmdutil.Factory, runF func(*getBackupOptions) error) *cobra.Command {
	opts := &getBackupOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <backup-id>",
		Short: "Show backup details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getBackupRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getBackupRun(ctx context.Context, w io.Writer, opts *getBackupOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewStorageClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetBackupWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return backupsGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
```

- [ ] **Step 4: Rewrite `backups_test.go`** — same two-layer pattern; use `httpmock.REST("GET", "/storage/backups")` / `"/storage/backups/*"`. Fixture JSON should match the existing `backups_test.go` fixture data.

- [ ] **Step 5: Update `storage.go`** — remove InjectClient from `newVolumesCmd` and `newBackupsCmd` (both sub-groups now build their own client in each `run` func):

```go
package storage

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "storage", Short: "Storage volumes and backups"}
	cmd.AddCommand(newVolumesCmd(f), newBackupsCmd(f))
	return cmd
}
```

- [ ] **Step 6: Delete `stub.go`**

```bash
rm internal/cli/storage/stub.go
```

- [ ] **Step 7: Build and test**

```bash
make build && go test ./internal/cli/storage/...
```

- [ ] **Step 8: Commit**

```bash
git add internal/cli/storage/
git commit -m "refactor(storage): Options+runF+httpmock, delete stub"
```

---

## Task 4: System domain

**Files:**
- Modify: `internal/cli/system/health.go` + `health_test.go`
- Modify: `internal/cli/system/info.go` + `info_test.go`
- Modify: `internal/cli/system/utilization.go` + `utilization_test.go`
- Modify: `internal/cli/system/updates.go` + `updates_test.go`
- Modify: `internal/cli/system/system.go`
- Delete: `internal/cli/system/stub.go`

- [ ] **Step 1: Rewrite `health.go`**

```go
package system

import (
	"context"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var healthView = cmdutil.View{Templates: systemTemplates, Name: "health.tmpl"}

type healthOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
}

func newHealthCmd(f *cmdutil.Factory, runF func(*healthOptions) error) *cobra.Command {
	opts := &healthOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "health",
		Short: "Show aggregate system health",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return healthRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func healthRun(ctx context.Context, w io.Writer, opts *healthOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewSystemClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetSystemHealthWithResponse(ctx)
	if err != nil {
		return err
	}
	return healthView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
```

- [ ] **Step 2: Rewrite `health_test.go`**

```go
package system

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

func testHTTPClient(reg *httpmock.Registry) func() (*http.Client, string, error) {
	return func() (*http.Client, string, error) {
		return &http.Client{Transport: reg}, "http://localhost", nil
	}
}

func TestNewHealthCmd_noFlags(t *testing.T) {
	called := false
	cmd := newHealthCmd(cmdutil.TestFactory(t), func(o *healthOptions) error {
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

func TestHealthRun_tableOutput(t *testing.T) {
	// Use fixture data from existing health_test.go
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/system/health"), httpmock.JSONResponse(map[string]any{
		"status": "healthy",
		"devices": []map[string]any{
			{"id": "nas-1", "name": "nas-1", "status": "healthy"},
		},
	}))
	var out bytes.Buffer
	opts := &healthOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := healthRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "healthy") {
		t.Errorf("expected 'healthy' in output, got:\n%s", out.String())
	}
	reg.Verify(t)
}
```

Note: Check the existing `health_test.go` for the exact fixture JSON structure and assertions to port.

- [ ] **Step 3: Rewrite `info.go`**

```go
package system

import (
	"context"
	"io"
	"net/http"

	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var infoView = cmdutil.View{Templates: systemTemplates, Name: "info.tmpl"}

type infoRow struct {
	Device   string
	Model    string
	Firmware string
	Ram      string
	Uptime   string
}

type infoOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newInfoCmd(f *cmdutil.Factory, runF func(*infoOptions) error) *cobra.Command {
	opts := &infoOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show device information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return infoRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	return cmd
}

func infoRun(ctx context.Context, w io.Writer, opts *infoOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewSystemClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &systemapi.ListSystemInfoParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListSystemInfoWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return infoView.RenderWith(w, opts.Output(), resp.StatusCode(), resp.Body, func() (any, error) {
		items := make([]infoRow, 0, len(resp.JSON200.Items))
		for _, info := range resp.JSON200.Items {
			items = append(items, infoRow{
				Device:   info.Device,
				Model:    info.Model,
				Firmware: info.Firmware,
				Ram:      output.FormatBytes(int64(info.RamMb) * 1024 * 1024),
				Uptime:   output.FormatUptime(int(info.UptimeSeconds)),
			})
		}
		return struct{ Items []infoRow }{items}, nil
	})
}
```

- [ ] **Step 4: Rewrite `info_test.go`** — two-layer tests using `httpmock.REST("GET", "/system/info")`. Port fixture data from existing `info_test.go`. The JSON fixture must include `items` array with `device`, `model`, `firmware`, `ramMb`, `uptimeSeconds` fields.

- [ ] **Step 5: Rewrite `utilization.go`**

```go
package system

import (
	"context"
	"fmt"
	"io"
	"net/http"

	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var utilizationView = cmdutil.View{Templates: systemTemplates, Name: "utilization.tmpl"}

type utilizationRow struct {
	Device string
	Cpu    string
	Memory string
	Swap   string
}

type utilizationOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Device     string
}

func newUtilizationCmd(f *cmdutil.Factory, runF func(*utilizationOptions) error) *cobra.Command {
	opts := &utilizationOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "utilization",
		Short: "Show live resource utilization",
	}
	cmd.Flags().StringVar(&opts.Device, "device", "", "Filter by device ID")
	cmd.RunE = watch.Wrap(f.Output, func(ctx context.Context, w io.Writer) error {
		if runF != nil {
			return runF(opts)
		}
		return utilizationRun(ctx, w, opts)
	})
	watch.RegisterFlags(cmd)
	return cmd
}

func utilizationRun(ctx context.Context, w io.Writer, opts *utilizationOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewSystemClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &systemapi.ListSystemUtilizationParams{}
	if opts.Device != "" {
		params.Device = &opts.Device
	}
	resp, err := c.ListSystemUtilizationWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return utilizationView.RenderWith(w, opts.Output(), resp.StatusCode(), resp.Body, func() (any, error) {
		items := make([]utilizationRow, 0, len(resp.JSON200.Items))
		for _, u := range resp.JSON200.Items {
			swapPct := 0
			if u.Memory.SwapTotalBytes > 0 {
				swapPct = int(u.Memory.SwapUsedBytes * 100 / u.Memory.SwapTotalBytes)
			}
			items = append(items, utilizationRow{
				Device: u.Device,
				Cpu:    fmt.Sprintf("%d%%", u.Cpu.TotalPercent),
				Memory: fmt.Sprintf("%d%%", u.Memory.UsedPercent),
				Swap:   fmt.Sprintf("%d%%", swapPct),
			})
		}
		return struct{ Items []utilizationRow }{items}, nil
	})
}
```

- [ ] **Step 6: Rewrite `utilization_test.go`** — two-layer tests using `httpmock.REST("GET", "/system/utilization")`. Port fixture from existing file. JSON needs `items` with `device`, `cpu.totalPercent`, `memory.usedPercent`, `memory.swapUsedBytes`, `memory.swapTotalBytes`.

- [ ] **Step 7: Rewrite `updates.go`**

```go
package system

import (
	"context"
	"io"
	"net/http"

	systemapi "github.com/bwilczynski/hlctl/internal/api/system"
	apipkg "github.com/bwilczynski/hlctl/internal/api"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	updatesListView = cmdutil.View{Templates: systemTemplates, Name: "updates_list.tmpl"}
	updateGetView   = cmdutil.PolymorphicView[systemapi.SystemUpdateDetail]{
		Templates: systemTemplates,
		Variants: map[string]cmdutil.Variant[systemapi.SystemUpdateDetail]{
			"container": {
				Template: "updates_get_container.tmpl",
				Resolve:  func(d systemapi.SystemUpdateDetail) (any, error) { return d.AsContainerSystemUpdateDetail() },
			},
		},
	}
)

func newUpdatesCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "updates", Short: "Software update tracking"}
	cmd.AddCommand(newListUpdatesCmd(f, nil), newGetUpdateCmd(f, nil), newCheckUpdatesCmd(f, nil))
	return cmd
}

// --- list ---

type listUpdatesOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Status     string
	Type       string
}

func newListUpdatesCmd(f *cmdutil.Factory, runF func(*listUpdatesOptions) error) *cobra.Command {
	opts := &listUpdatesOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tracked software updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listUpdatesRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
	cmd.Flags().StringVar(&opts.Status, "status", "", "Filter by update status (unknown, upToDate, updateAvailable)")
	cmd.Flags().StringVar(&opts.Type, "type", "", "Filter by component type (container)")
	return cmd
}

func listUpdatesRun(ctx context.Context, w io.Writer, opts *listUpdatesOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewSystemClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &systemapi.ListSystemUpdatesParams{}
	if opts.Status != "" {
		s := systemapi.UpdateStatusFilter(opts.Status)
		params.Status = &s
	}
	if opts.Type != "" {
		ut := systemapi.UpdateTypeFilter(opts.Type)
		params.Type = &ut
	}
	resp, err := c.ListSystemUpdatesWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return updatesListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

// --- get ---

type getUpdateOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newGetUpdateCmd(f *cmdutil.Factory, runF func(*getUpdateOptions) error) *cobra.Command {
	opts := &getUpdateOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <update-id>",
		Short: "Show update details for a tracked component",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getUpdateRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getUpdateRun(ctx context.Context, w io.Writer, opts *getUpdateOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewSystemClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetSystemUpdateWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return updateGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

// --- check ---

type checkUpdatesOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
}

func newCheckUpdatesCmd(f *cmdutil.Factory, runF func(*checkUpdatesOptions) error) *cobra.Command {
	opts := &checkUpdatesOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "check",
		Short: "Force check for upstream updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return checkUpdatesRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func checkUpdatesRun(ctx context.Context, w io.Writer, opts *checkUpdatesOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewSystemClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.CheckSystemUpdatesWithResponse(ctx, &systemapi.CheckSystemUpdatesParams{})
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return apipkg.ParseError(resp.StatusCode(), resp.Body)
	}
	return updatesListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
```

Note: `CheckSystemUpdatesWithResponse` may return 200 with body (as existing code used `updatesListView.Render`), or it may return 204. Check the generated client's `Status` field if present. The existing code calls `Render` with `resp.JSON200`, so it returns 200. Adjust if the spec differs.

- [ ] **Step 8: Rewrite `updates_test.go`** — port all assertions from the existing file. Use `httpmock.JSONResponse(map[string]any{...})` with the same map keys the existing tests used for stub responses.

```go
package system

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

func TestNewListUpdatesCmd_statusFlag(t *testing.T) {
	var captured *listUpdatesOptions
	cmd := newListUpdatesCmd(cmdutil.TestFactory(t), func(o *listUpdatesOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"--status", "updateAvailable"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Status != "updateAvailable" {
		t.Errorf("expected Status=updateAvailable, got %q", captured.Status)
	}
}

func TestListUpdatesRun_tableOutput(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/system/updates"), httpmock.JSONResponse(map[string]any{
		"items": []map[string]any{{
			"id": "nas-1.homeassistant", "name": "homeassistant", "device": "nas-1",
			"type": "container", "status": "updateAvailable",
			"currentVersion": "2024.1.0", "latestVersion": "2024.2.0",
			"checkedAt": time.Now(),
		}},
	}))
	var out bytes.Buffer
	opts := &listUpdatesOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listUpdatesRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.homeassistant", "container", "updateAvailable", "2024.1.0", "2024.2.0"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetUpdateRun_containerType(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/system/updates/*"), httpmock.JSONResponse(map[string]any{
		"id": "nas-1.homeassistant", "name": "homeassistant", "device": "nas-1",
		"type": "container", "status": "updateAvailable",
		"currentVersion": "2024.1.0", "latestVersion": "2024.2.0",
		"checkedAt":  time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
		"publishedAt": time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
		"image": "ghcr.io/home-assistant/home-assistant",
	}))
	var out bytes.Buffer
	opts := &getUpdateOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.homeassistant",
	}
	if err := getUpdateRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.homeassistant", "homeassistant", "2024.1.0", "ghcr.io/home-assistant/home-assistant"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetUpdateRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/system/updates/*"), httpmock.StatusJSONResponse(http.StatusNotFound, map[string]any{
		"type": "https://homelab.local/problems/not-found", "title": "Not Found", "status": 404, "detail": "not found",
	}))
	var out bytes.Buffer
	opts := &getUpdateOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.foo",
	}
	err := getUpdateRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
	reg.Verify(t)
}

func TestCheckUpdatesRun_tableOutput(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("POST", "/system/updates/check"), httpmock.JSONResponse(map[string]any{
		"items": []map[string]any{{
			"id": "nas-1.homeassistant", "name": "homeassistant", "device": "nas-1",
			"type": "container", "status": "updateAvailable",
			"currentVersion": "2024.1.0", "latestVersion": "2024.2.0",
			"checkedAt": time.Now(),
		}},
	}))
	var out bytes.Buffer
	opts := &checkUpdatesOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := checkUpdatesRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "updateAvailable") {
		t.Errorf("expected 'updateAvailable' in output, got:\n%s", out.String())
	}
	reg.Verify(t)
}

func TestGetUpdateRun_jsonOutput(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/system/updates/*"), httpmock.JSONResponse(map[string]any{
		"id": "nas-1.homeassistant", "type": "container", "status": "updateAvailable",
		"currentVersion": "2024.1.0", "latestVersion": "2024.2.0",
		"image": "ghcr.io/home-assistant/home-assistant",
		"checkedAt": time.Now(),
	}))
	var out bytes.Buffer
	opts := &getUpdateOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatJSON },
		ID:         "nas-1.homeassistant",
	}
	if err := getUpdateRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "ghcr.io/home-assistant/home-assistant") {
		t.Errorf("expected image in JSON output, got:\n%s", out.String())
	}
}
```

- [ ] **Step 9: Update `system.go`** — remove InjectClient:

```go
package system

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "system", Short: "System health and information"}
	cmd.AddCommand(
		newHealthCmd(f, nil),
		newInfoCmd(f, nil),
		newUtilizationCmd(f, nil),
		newUpdatesCmd(f),
	)
	return cmd
}
```

- [ ] **Step 10: Delete `stub.go`**

```bash
rm internal/cli/system/stub.go
```

- [ ] **Step 11: Build and test**

```bash
make build && go test ./internal/cli/system/...
```

- [ ] **Step 12: Commit**

```bash
git add internal/cli/system/
git commit -m "refactor(system): Options+runF+httpmock, delete stub"
```

---

## Task 5: Network domain

**Files:**
- Modify: `internal/cli/network/clients.go` + `clients_test.go`
- Modify: `internal/cli/network/devices.go` + `devices_test.go`
- Modify: `internal/cli/network/topology.go` + `topology_test.go`
- Modify: `internal/cli/network/ssids.go` + `ssids_test.go`
- Modify: `internal/cli/network/vlans.go` + `vlans_test.go`
- Modify: `internal/cli/network/wans.go` + `wans_test.go`
- Modify: `internal/cli/network/network.go`
- Delete: `internal/cli/network/stub.go`

- [ ] **Step 1: Rewrite `clients.go`**

```go
package network

import (
	"context"
	"io"
	"net/http"

	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/watch"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	clientsListView = cmdutil.View{Templates: networkTemplates, Name: "clients_list.tmpl"}
	clientGetView   = cmdutil.PolymorphicView[networkapi.NetworkClientDetail]{
		Templates: networkTemplates,
		Variants: map[string]cmdutil.Variant[networkapi.NetworkClientDetail]{
			"wired":    {Template: "clients_get_wired.tmpl", Resolve: func(d networkapi.NetworkClientDetail) (any, error) { return d.AsWiredNetworkClientDetail() }},
			"wireless": {Template: "clients_get_wireless.tmpl", Resolve: func(d networkapi.NetworkClientDetail) (any, error) { return d.AsWirelessNetworkClientDetail() }},
		},
	}
)

type listClientsOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	Status     string
}

func newClientsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "clients", Short: "Network clients"}
	cmd.AddCommand(newListClientsCmd(f, nil), newGetClientCmd(f, nil))
	return cmd
}

func newListClientsCmd(f *cmdutil.Factory, runF func(*listClientsOptions) error) *cobra.Command {
	opts := &listClientsOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{Use: "list", Short: "List network clients"}
	cmd.Flags().StringVar(&opts.Status, "status", "", "Filter by status (online|offline)")
	cmd.RunE = watch.Wrap(f.Output, func(ctx context.Context, w io.Writer) error {
		if runF != nil {
			return runF(opts)
		}
		return listClientsRun(ctx, w, opts)
	})
	watch.RegisterFlags(cmd)
	return cmd
}

func listClientsRun(ctx context.Context, w io.Writer, opts *listClientsOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &networkapi.ListNetworkClientsParams{}
	if opts.Status != "" {
		s := networkapi.NetworkClientStatus(opts.Status)
		params.Status = &s
	}
	resp, err := c.ListNetworkClientsWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return clientsListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

type getClientOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newGetClientCmd(f *cmdutil.Factory, runF func(*getClientOptions) error) *cobra.Command {
	opts := &getClientOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <client-id>",
		Short: "Show network client details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getClientRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getClientRun(ctx context.Context, w io.Writer, opts *getClientOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetNetworkClientWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return clientGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
```

- [ ] **Step 2: Rewrite `clients_test.go`** — two-layer tests. Use `httpmock.REST("GET", "/network/clients")` for list, `"/network/clients/*"` for get. Port fixture data from the existing file.

- [ ] **Step 3: Rewrite `devices.go`**

```go
package network

import (
	"context"
	"io"
	"net/http"

	networkapi "github.com/bwilczynski/hlctl/internal/api/network"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var devicesListView = cmdutil.View{Templates: networkTemplates, Name: "devices_list.tmpl"}

type switchDetailView struct {
	networkapi.SwitchDetail
	Ports []switchPortView
}

type switchPortView struct {
	networkapi.SwitchPort
	ConnectedToName string
}

func buildSwitchPortViews(ports []networkapi.SwitchPort, allPorts bool) ([]switchPortView, error) {
	var out []switchPortView
	for _, p := range ports {
		if !allPorts && p.State != networkapi.NetworkPortStateUp {
			continue
		}
		connectedTo := "-"
		if p.ConnectedTo != nil {
			kind, err := p.ConnectedTo.Discriminator()
			if err != nil {
				return nil, err
			}
			switch kind {
			case "device":
				ref, err := p.ConnectedTo.AsNetworkDeviceRef()
				if err != nil {
					return nil, err
				}
				connectedTo = ref.Name
			case "client":
				ref, err := p.ConnectedTo.AsNetworkClientRef()
				if err != nil {
					return nil, err
				}
				connectedTo = ref.Name
			}
		}
		out = append(out, switchPortView{SwitchPort: p, ConnectedToName: connectedTo})
	}
	return out, nil
}

type listDevicesOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
}

func newDevicesCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "devices", Short: "Network devices"}
	cmd.AddCommand(newListDevicesCmd(f, nil), newGetDeviceCmd(f, nil))
	return cmd
}

func newListDevicesCmd(f *cmdutil.Factory, runF func(*listDevicesOptions) error) *cobra.Command {
	opts := &listDevicesOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "list",
		Short: "List network devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listDevicesRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func listDevicesRun(ctx context.Context, w io.Writer, opts *listDevicesOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.ListNetworkDevicesWithResponse(ctx)
	if err != nil {
		return err
	}
	return devicesListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

type getDeviceOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
	AllPorts   bool
}

func newGetDeviceCmd(f *cmdutil.Factory, runF func(*getDeviceOptions) error) *cobra.Command {
	opts := &getDeviceOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{
		Use:   "get <device-id>",
		Short: "Show network device details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getDeviceRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.AllPorts, "all-ports", false, "Show all ports (default: active ports only)")
	return cmd
}

func getDeviceRun(ctx context.Context, w io.Writer, opts *getDeviceOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	view := cmdutil.PolymorphicView[networkapi.NetworkDeviceDetail]{
		Templates: networkTemplates,
		Variants: map[string]cmdutil.Variant[networkapi.NetworkDeviceDetail]{
			"switch": {
				Template: "devices_get_switch.tmpl",
				Resolve: func(d networkapi.NetworkDeviceDetail) (any, error) {
					sw, err := d.AsSwitchDetail()
					if err != nil {
						return nil, err
					}
					portViews, err := buildSwitchPortViews(sw.Ports, opts.AllPorts)
					if err != nil {
						return nil, err
					}
					return switchDetailView{SwitchDetail: sw, Ports: portViews}, nil
				},
			},
			"accessPoint": {Template: "devices_get_accesspoint.tmpl", Resolve: func(d networkapi.NetworkDeviceDetail) (any, error) { return d.AsAccessPointDetail() }},
			"gateway":     {Template: "devices_get_gateway.tmpl", Resolve: func(d networkapi.NetworkDeviceDetail) (any, error) { return d.AsGatewayDetail() }},
			"unknown":     {Template: "devices_get_unknown.tmpl", Resolve: func(d networkapi.NetworkDeviceDetail) (any, error) { return d.AsUnknownDeviceDetail() }},
		},
	}
	resp, err := c.GetNetworkDeviceWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return view.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

// suppress unused import if output not used directly
var _ = output.FormatTable
```

Remove the `output` import line if `output` is not referenced in `devices.go` (it was used indirectly only). Check after writing.

- [ ] **Step 4: Rewrite `devices_test.go`** — Layer 1 tests for `--all-ports` flag; Layer 2 tests for `listDevicesRun` and `getDeviceRun` with `httpmock.REST("GET", "/network/devices")` and `"/network/devices/*"`. Port the full fixture data from the existing file (gateway, switch with active/all ports, access point, unknown).

```go
package network

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

func testHTTPClient(reg *httpmock.Registry) func() (*http.Client, string, error) {
	return func() (*http.Client, string, error) {
		return &http.Client{Transport: reg}, "http://localhost", nil
	}
}

func TestNewGetDeviceCmd_allPortsFlag(t *testing.T) {
	var captured *getDeviceOptions
	cmd := newGetDeviceCmd(cmdutil.TestFactory(t), func(o *getDeviceOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"unifi.switch-lr", "--all-ports"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !captured.AllPorts {
		t.Error("expected AllPorts=true")
	}
	if captured.ID != "unifi.switch-lr" {
		t.Errorf("expected ID=unifi.switch-lr, got %q", captured.ID)
	}
}

func TestListDevicesRun_tableOutput(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/devices"), httpmock.JSONResponse(map[string]any{
		"items": []map[string]any{
			{"id": "unifi.usg", "uri": "/network/devices/unifi.usg", "name": "USG", "mac": "aa:bb:cc:dd:00:01", "ip": "192.168.1.1", "type": "gateway", "status": "connected"},
			{"id": "unifi.ap-living-room", "uri": "/network/devices/unifi.ap-living-room", "name": "AP Living Room", "mac": "aa:bb:cc:dd:00:03", "ip": "192.168.1.3", "type": "accessPoint", "status": "connected"},
		},
	}))
	var out bytes.Buffer
	opts := &listDevicesOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listDevicesRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"unifi.usg", "unifi.ap-living-room", "gateway", "accessPoint"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetDeviceRun_gateway(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/devices/*"), httpmock.JSONResponse(map[string]any{
		"id": "unifi.usg", "uri": "/network/devices/unifi.usg",
		"name": "USG", "mac": "aa:bb:cc:dd:00:01", "ip": "192.168.1.1",
		"type": "gateway", "status": "connected",
		"model": "USG-3P", "firmwareVersion": "4.4.57", "uptime": 86400,
		"traffic": map[string]any{"rxBytesTotal": int64(12884901888), "txBytesTotal": int64(4294967296), "rxBytesPerSec": int64(125000), "txBytesPerSec": int64(50000)},
	}))
	var out bytes.Buffer
	opts := &getDeviceOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.usg",
	}
	if err := getDeviceRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"unifi.usg", "USG-3P", "4.4.57", "gateway", "TRAFFIC RX", "TRAFFIC TX", "1d"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetDeviceRun_switch_allPorts(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/network/devices/*"), httpmock.JSONResponse(map[string]any{
		"id": "unifi.switch-lr", "uri": "/network/devices/unifi.switch-lr",
		"name": "Switch Living Room", "mac": "aa:bb:cc:dd:00:10", "ip": "192.168.1.10",
		"type": "switch", "status": "connected",
		"model": "USW-24-PoE", "firmwareVersion": "6.2.14", "uptime": 3600,
		"traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)},
		"ports": []map[string]any{
			{"number": 1, "state": "up", "poeMode": "off", "traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)}},
			{"number": 2, "state": "down", "poeMode": "off", "traffic": map[string]any{"rxBytesTotal": int64(0), "txBytesTotal": int64(0), "rxBytesPerSec": int64(0), "txBytesPerSec": int64(0)}},
		},
	}))
	var out bytes.Buffer
	opts := &getDeviceOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "unifi.switch-lr",
		AllPorts:   true,
	}
	if err := getDeviceRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "down") {
		t.Errorf("expected 'down' port visible with --all-ports, got:\n%s", out.String())
	}
	reg.Verify(t)
}
```

Add remaining device type tests (access point, unknown, switch active ports only) following the same pattern with their respective fixture JSON from the existing `devices_test.go`.

- [ ] **Step 5: Rewrite `topology.go`** — `topologyOptions` struct, `newTopologyCmd(f, runF)`, `topologyRun(ctx, w, opts)`. Keep all the `buildTopologyTree`, `connectionRefID`, `topologyTree`, `topologyEdge` types and helper functions unchanged.

```go
// Options struct and constructor:
type topologyOptions struct {
	HTTPClient      func() (*http.Client, string, error)
	IO              *cmdutil.IOStreams
	Output          func() output.Format
	IncludeClients  bool
	IncludeWireless bool
}

func newTopologyCmd(f *cmdutil.Factory, runF func(*topologyOptions) error) *cobra.Command {
	opts := &topologyOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	cmd := &cobra.Command{Use: "topology", Short: "Show network topology"}
	cmd.Flags().BoolVar(&opts.IncludeClients, "include-clients", false, "Include wired clients in the topology")
	cmd.Flags().BoolVar(&opts.IncludeWireless, "include-wireless", false, "Also include wireless clients (implies --include-clients)")
	cmd.RunE = watch.Wrap(f.Output, func(ctx context.Context, w io.Writer) error {
		if runF != nil {
			return runF(opts)
		}
		return topologyRun(ctx, w, opts)
	})
	watch.RegisterFlags(cmd)
	return cmd
}

func topologyRun(ctx context.Context, w io.Writer, opts *topologyOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	params := &networkapi.GetNetworkTopologyParams{}
	if opts.IncludeClients || opts.IncludeWireless {
		t := true
		params.IncludeClients = &t
	}
	resp, err := c.GetNetworkTopologyWithResponse(ctx, params)
	if err != nil {
		return err
	}
	return topologyView.RenderWith(w, opts.Output(), resp.StatusCode(), resp.Body, func() (any, error) {
		return buildTopologyTree(*resp.JSON200, opts.IncludeWireless)
	})
}
```

Full `topology.go` retains the existing `buildTopologyTree`, `connectionRefID`, view var, and type defs; only the constructor + run func change.

- [ ] **Step 6: Rewrite `topology_test.go`** — Layer 1 test for `--include-clients`/`--include-wireless` flags; Layer 2 test for `topologyRun` with `httpmock.REST("GET", "/network/topology")`. Port the JSON fixture from the existing topology test.

- [ ] **Step 7: Rewrite `ssids.go`**

```go
package network

import (
	"context"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	ssidsListView = cmdutil.View{Templates: networkTemplates, Name: "ssids_list.tmpl"}
	ssidsGetView  = cmdutil.View{Templates: networkTemplates, Name: "ssids_get.tmpl"}
)

type listSsidsOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
}

func newSsidsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "ssids", Short: "WiFi networks (SSIDs)"}
	cmd.AddCommand(newListSsidsCmd(f, nil), newGetSsidCmd(f, nil))
	return cmd
}

func newListSsidsCmd(f *cmdutil.Factory, runF func(*listSsidsOptions) error) *cobra.Command {
	opts := &listSsidsOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "list",
		Short: "List WiFi networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listSsidsRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func listSsidsRun(ctx context.Context, w io.Writer, opts *listSsidsOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.ListSsidsWithResponse(ctx)
	if err != nil {
		return err
	}
	return ssidsListView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}

type getSsidOptions struct {
	HTTPClient func() (*http.Client, string, error)
	IO         *cmdutil.IOStreams
	Output     func() output.Format
	ID         string
}

func newGetSsidCmd(f *cmdutil.Factory, runF func(*getSsidOptions) error) *cobra.Command {
	opts := &getSsidOptions{HTTPClient: f.HTTPClient, IO: f.IOStreams, Output: f.Output}
	return &cobra.Command{
		Use:   "get <ssid-id>",
		Short: "Show WiFi network details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ID = args[0]
			if runF != nil {
				return runF(opts)
			}
			return getSsidRun(cmd.Context(), opts.IO.Out, opts)
		},
	}
}

func getSsidRun(ctx context.Context, w io.Writer, opts *getSsidOptions) error {
	httpClient, apiURL, err := opts.HTTPClient()
	if err != nil {
		return err
	}
	c, err := NewNetworkClient(httpClient, apiURL)
	if err != nil {
		return err
	}
	resp, err := c.GetSsidWithResponse(ctx, opts.ID)
	if err != nil {
		return err
	}
	return ssidsGetView.Render(w, opts.Output(), resp.StatusCode(), resp.Body, resp.JSON200)
}
```

- [ ] **Step 8: Rewrite `ssids_test.go`** — two-layer tests using `httpmock.REST("GET", "/network/ssids")` and `"/network/ssids/*"`. Port fixture from existing file.

- [ ] **Step 9: Rewrite `vlans.go`** — same Options+runF pattern as ssids, using `ListVlansWithResponse` / `GetVlanWithResponse`, `vlansListView`, `vlansGetView`. Follow the ssids.go template exactly with vlan names.

- [ ] **Step 10: Rewrite `vlans_test.go`** — two-layer tests using `httpmock.REST("GET", "/network/vlans")` and `"/network/vlans/*"`. Port fixture from existing file.

- [ ] **Step 11: Rewrite `wans.go`** — same Options+runF pattern as ssids, using `ListWansWithResponse` / `GetWanWithResponse`, `wansListView`, `wansGetView`.

- [ ] **Step 12: Rewrite `wans_test.go`** — two-layer tests using `httpmock.REST("GET", "/network/wans")` and `"/network/wans/*"`. Port fixture from existing file.

- [ ] **Step 13: Update `network.go`** — remove InjectClient, call sub-group constructors directly:

```go
package network

import (
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "network", Short: "Network devices and clients"}
	cmd.AddCommand(
		newDevicesCmd(f),
		newClientsCmd(f),
		newTopologyCmd(f, nil),
		newVlansCmd(f),
		newSsidsCmd(f),
		newWansCmd(f),
	)
	return cmd
}
```

- [ ] **Step 14: Delete `stub.go`**

```bash
rm internal/cli/network/stub.go
```

- [ ] **Step 15: Build and test**

```bash
make build && go test ./internal/cli/network/...
```

- [ ] **Step 16: Commit**

```bash
git add internal/cli/network/
git commit -m "refactor(network): Options+runF+httpmock, delete stub"
```

---

## Task 6: Delete old cmdutil infrastructure

**Files:**
- Delete: `internal/cli/cmdutil/client.go`
- Delete: `internal/cli/cmdutil/client_test.go`
- Delete: `internal/cli/cmdutil/action.go`
- Delete: `internal/cli/cmdutil/action_test.go`

- [ ] **Step 1: Delete the four files**

```bash
rm internal/cli/cmdutil/client.go internal/cli/cmdutil/client_test.go
rm internal/cli/cmdutil/action.go internal/cli/cmdutil/action_test.go
```

- [ ] **Step 2: Verify no remaining references**

```bash
grep -r "cmdutil\.InjectClient\|cmdutil\.Client\[\ ]*\|cmdutil\.SetClient\|cmdutil\.ActionCmd" internal/ --include="*.go"
```

Expected: no output. If any references remain, fix them before proceeding.

- [ ] **Step 3: Build and test everything**

```bash
make build && go test ./...
```

Expected: build succeeds, all tests pass.

- [ ] **Step 4: Run lint**

```bash
make lint
```

Expected: no errors.

- [ ] **Step 5: Verify no mutable package-level vars remain under `internal/cli/`**

```bash
grep -rn "^var " internal/cli/ --include="*.go" | grep -v "_test.go" | grep -v "View\|Templates\|Func\b"
```

Inspect results: all remaining `var` declarations should be view/template definitions, not mutable state.

- [ ] **Step 6: Commit**

```bash
git add -u internal/cli/cmdutil/
git commit -m "refactor: delete InjectClient, Client[C], SetClient[C], ActionCmd"
```

---

## Verification checklist

After all tasks complete:

- [ ] `make build` — binary builds
- [ ] `go test ./...` — all tests pass
- [ ] `make lint` — clean
- [ ] `hlctl --help` output identical to pre-refactor
- [ ] No `InjectClient`, `Client[`, `SetClient`, `ActionCmd` references remain in production code
- [ ] No `stub.go` files remain under `internal/cli/`
- [ ] No `cmdutil/client.go` or `cmdutil/action.go` remain
