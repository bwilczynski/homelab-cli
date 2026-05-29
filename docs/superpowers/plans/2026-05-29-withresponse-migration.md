# WithResponse Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace manual HTTP plumbing (ReadAll + Unmarshal + status check) in all CLI domain commands with the generated `*WithResponse` typed client methods.

**Architecture:** Each domain's client interface is updated to declare `WithResponse` methods returning typed `*gen.XxxResponse` structs. Command bodies drop 4 lines of plumbing per API call and read from `resp.Body` (bytes) and `resp.JSON200` (typed pointer) directly. `apiclient.ParseError` is updated to accept `(statusCode int, body []byte)` since `WithResponse` already reads and closes the response body.

**Tech Stack:** Go, oapi-codegen generated `ClientWithResponses`, Cobra

---

## File Map

| File | Change |
|------|--------|
| `internal/apiclient/errors.go` | New `ParseError(int, []byte)`, rename old to `ParseErrorResponse` |
| `internal/apiclient/errors_test.go` | Update tests for new signature |
| `internal/cli/network/client.go` | Interface → `WithResponse` methods; `NewNetworkClient` → `gen.NewClientWithResponses` |
| `internal/cli/network/stub.go` | StubClient fields/methods → `WithResponse`; remove `jsonResponse` |
| `internal/cli/network/network.go` | Remove manual plumbing in all 5 commands |
| `internal/cli/network/ssids.go` | Remove manual plumbing in 2 commands |
| `internal/cli/network/vlans.go` | Remove manual plumbing in 2 commands |
| `internal/cli/network/wans.go` | Remove manual plumbing in 2 commands |
| `internal/cli/network/network_test.go` | Update all stub initializations |
| `internal/cli/system/client.go` | Same pattern as network |
| `internal/cli/system/stub.go` | Same pattern |
| `internal/cli/system/system.go` | Same pattern, 6 commands |
| `internal/cli/system/system_test.go` | Same pattern |
| `internal/cli/storage/client.go` | Same pattern |
| `internal/cli/storage/stub.go` | Same pattern |
| `internal/cli/storage/storage.go` | Same pattern, 4 commands |
| `internal/cli/storage/storage_test.go` | Same pattern |
| `internal/cli/docker/client.go` | Same pattern |
| `internal/cli/docker/stub.go` | Same pattern |
| `internal/cli/docker/docker.go` | Same pattern, 9 commands (3 are 204 No Content) |
| `internal/cli/docker/docker_test.go` | Same pattern |

---

## Task 1: Update `apiclient.ParseError` API

**Files:**
- Modify: `internal/apiclient/errors.go`
- Modify: `internal/apiclient/errors_test.go`
- Modify: `internal/cli/network/network.go`, `ssids.go`, `vlans.go`, `wans.go`
- Modify: `internal/cli/system/system.go`
- Modify: `internal/cli/storage/storage.go`
- Modify: `internal/cli/docker/docker.go`

- [ ] **Step 1: Update `errors.go`**

Replace the entire file content:

```go
package apiclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type problem struct {
	Title  string  `json:"title"`
	Detail *string `json:"detail,omitempty"`
}

// ParseError parses an RFC 9457 Problem Details body from already-read bytes and returns
// a user-friendly error. Call this on any non-2xx response using resp.StatusCode() and resp.Body.
func ParseError(statusCode int, body []byte) error {
	var p problem
	if err := json.Unmarshal(body, &p); err != nil || p.Title == "" {
		return fmt.Errorf("unexpected status %d", statusCode)
	}
	if p.Detail != nil && *p.Detail != "" {
		return fmt.Errorf("%s — %s", p.Title, *p.Detail)
	}
	return errors.New(p.Title)
}

// ParseErrorResponse reads an RFC 9457 Problem Details body from resp and returns
// a user-friendly error. Deprecated: only use before WithResponse migration is complete.
func ParseErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return ParseError(resp.StatusCode, body)
}
```

- [ ] **Step 2: Update `errors_test.go`**

Replace the entire file content:

```go
package apiclient_test

import (
	"testing"

	"github.com/bwilczynski/hlctl/internal/apiclient"
)

func TestParseError_withDetail(t *testing.T) {
	body := []byte(`{"type":"https://example.com/problem","title":"Not Found","status":404,"detail":"container 'nas-1.foo' does not exist"}`)
	err := apiclient.ParseError(404, body)
	want := "Not Found — container 'nas-1.foo' does not exist"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}

func TestParseError_withoutDetail(t *testing.T) {
	body := []byte(`{"type":"https://example.com/problem","title":"Unauthorized","status":401}`)
	err := apiclient.ParseError(401, body)
	want := "Unauthorized"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}

func TestParseError_invalidBody(t *testing.T) {
	err := apiclient.ParseError(500, []byte("not json"))
	want := "unexpected status 500"
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}
```

- [ ] **Step 3: Rename all existing callers to `ParseErrorResponse`**

Run this sed command from the repo root to rename all `apiclient.ParseError(resp)` call sites that still pass `*http.Response`:

```bash
find internal/cli -name "*.go" ! -name "*_test.go" ! -name "stub.go" \
  -exec sed -i '' 's/apiclient\.ParseError(resp)/apiclient.ParseErrorResponse(resp)/g' {} +
```

- [ ] **Step 4: Verify compilation and tests pass**

```bash
make build
go test ./internal/apiclient/...
```

Expected: build succeeds, 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/apiclient/errors.go internal/apiclient/errors_test.go \
  internal/cli/network/network.go internal/cli/network/ssids.go \
  internal/cli/network/vlans.go internal/cli/network/wans.go \
  internal/cli/system/system.go internal/cli/storage/storage.go \
  internal/cli/docker/docker.go
git commit -m "refactor: add ParseError(statusCode, body) and rename old to ParseErrorResponse"
```

---

## Task 2: Migrate network domain

**Files:**
- Modify: `internal/cli/network/client.go`
- Modify: `internal/cli/network/stub.go`
- Modify: `internal/cli/network/network.go`
- Modify: `internal/cli/network/ssids.go`
- Modify: `internal/cli/network/vlans.go`
- Modify: `internal/cli/network/wans.go`
- Modify: `internal/cli/network/network_test.go`

- [ ] **Step 1: Update `internal/cli/network/client.go`**

```go
package network

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/network"
)

// NetworkClient is the interface used by network commands.
type NetworkClient interface {
	ListNetworkDevicesWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListNetworkDevicesResponse, error)
	GetNetworkDeviceWithResponse(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkDeviceResponse, error)
	ListNetworkClientsWithResponse(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*gen.ListNetworkClientsResponse, error)
	GetNetworkClientWithResponse(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkClientResponse, error)
	GetNetworkTopologyWithResponse(ctx context.Context, params *gen.GetNetworkTopologyParams, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkTopologyResponse, error)
	ListVlansWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListVlansResponse, error)
	GetVlanWithResponse(ctx context.Context, vlanId string, reqEditors ...gen.RequestEditorFn) (*gen.GetVlanResponse, error)
	ListSsidsWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListSsidsResponse, error)
	GetSsidWithResponse(ctx context.Context, ssidId string, reqEditors ...gen.RequestEditorFn) (*gen.GetSsidResponse, error)
	ListWansWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListWansResponse, error)
	GetWanWithResponse(ctx context.Context, wanId string, reqEditors ...gen.RequestEditorFn) (*gen.GetWanResponse, error)
}

// NewNetworkClient constructs a NetworkClient backed by the real API.
func NewNetworkClient(httpClient *http.Client, apiURL string) (NetworkClient, error) {
	return gen.NewClientWithResponses(apiURL, gen.WithHTTPClient(httpClient))
}
```

- [ ] **Step 2: Update `internal/cli/network/stub.go`**

```go
package network

import (
	"context"
	"encoding/json"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/network"
)

// StubClient is a NetworkClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListNetworkDevicesWithResponseFunc func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListNetworkDevicesResponse, error)
	GetNetworkDeviceWithResponseFunc   func(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkDeviceResponse, error)
	ListNetworkClientsWithResponseFunc func(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*gen.ListNetworkClientsResponse, error)
	GetNetworkClientWithResponseFunc   func(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkClientResponse, error)
	GetNetworkTopologyWithResponseFunc func(ctx context.Context, params *gen.GetNetworkTopologyParams, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkTopologyResponse, error)
	ListVlansWithResponseFunc          func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListVlansResponse, error)
	GetVlanWithResponseFunc            func(ctx context.Context, vlanId string, reqEditors ...gen.RequestEditorFn) (*gen.GetVlanResponse, error)
	ListSsidsWithResponseFunc          func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListSsidsResponse, error)
	GetSsidWithResponseFunc            func(ctx context.Context, ssidId string, reqEditors ...gen.RequestEditorFn) (*gen.GetSsidResponse, error)
	ListWansWithResponseFunc           func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListWansResponse, error)
	GetWanWithResponseFunc             func(ctx context.Context, wanId string, reqEditors ...gen.RequestEditorFn) (*gen.GetWanResponse, error)
}

func (s *StubClient) ListNetworkDevicesWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListNetworkDevicesResponse, error) {
	return s.ListNetworkDevicesWithResponseFunc(ctx, reqEditors...)
}
func (s *StubClient) GetNetworkDeviceWithResponse(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkDeviceResponse, error) {
	return s.GetNetworkDeviceWithResponseFunc(ctx, deviceId, reqEditors...)
}
func (s *StubClient) ListNetworkClientsWithResponse(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*gen.ListNetworkClientsResponse, error) {
	return s.ListNetworkClientsWithResponseFunc(ctx, params, reqEditors...)
}
func (s *StubClient) GetNetworkClientWithResponse(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkClientResponse, error) {
	return s.GetNetworkClientWithResponseFunc(ctx, clientId, reqEditors...)
}
func (s *StubClient) GetNetworkTopologyWithResponse(ctx context.Context, params *gen.GetNetworkTopologyParams, reqEditors ...gen.RequestEditorFn) (*gen.GetNetworkTopologyResponse, error) {
	return s.GetNetworkTopologyWithResponseFunc(ctx, params, reqEditors...)
}
func (s *StubClient) ListVlansWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListVlansResponse, error) {
	return s.ListVlansWithResponseFunc(ctx, reqEditors...)
}
func (s *StubClient) GetVlanWithResponse(ctx context.Context, vlanId string, reqEditors ...gen.RequestEditorFn) (*gen.GetVlanResponse, error) {
	return s.GetVlanWithResponseFunc(ctx, vlanId, reqEditors...)
}
func (s *StubClient) ListSsidsWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListSsidsResponse, error) {
	return s.ListSsidsWithResponseFunc(ctx, reqEditors...)
}
func (s *StubClient) GetSsidWithResponse(ctx context.Context, ssidId string, reqEditors ...gen.RequestEditorFn) (*gen.GetSsidResponse, error) {
	return s.GetSsidWithResponseFunc(ctx, ssidId, reqEditors...)
}
func (s *StubClient) ListWansWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.ListWansResponse, error) {
	return s.ListWansWithResponseFunc(ctx, reqEditors...)
}
func (s *StubClient) GetWanWithResponse(ctx context.Context, wanId string, reqEditors ...gen.RequestEditorFn) (*gen.GetWanResponse, error) {
	return s.GetWanWithResponseFunc(ctx, wanId, reqEditors...)
}

// okResponse marshals v to JSON and returns a typed response with StatusOK and JSON200 set.
// T must be the inner payload type (e.g. gen.NetworkDeviceList).
// R must be the response wrapper (e.g. *gen.ListNetworkDevicesResponse).
// Because Go generics cannot express the relationship between T and R, each caller
// constructs the response manually — this helper is intentionally absent.
// Instead, use the inline pattern shown in the test stub examples below.
```

- [ ] **Step 3: Update `internal/cli/network/network.go` — replace all command bodies**

Remove imports `"encoding/json"` and `"io"` from the import block. Keep all others.

Replace `newListDevicesCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.ListNetworkDevicesWithResponse(context.Background())
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_list.tmpl", *resp.JSON200)
},
```

Replace `newGetDeviceCmd` RunE body (keep all the discriminator switch logic unchanged, just replace the API call + plumbing):
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.GetNetworkDeviceWithResponse(context.Background(), args[0])
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }

    detail := *resp.JSON200
    disc, err := detail.Discriminator()
    if err != nil {
        return err
    }

    switch disc {
    case "switch":
        d, err := detail.AsSwitchDetail()
        if err != nil {
            return err
        }
        var portViews []switchPortView
        for _, p := range d.Ports {
            if !allPorts && p.State != gen.NetworkPortStateUp {
                continue
            }
            connectedTo := "-"
            if p.ConnectedTo != nil {
                kind, err := p.ConnectedTo.Discriminator()
                if err != nil {
                    return err
                }
                switch kind {
                case "device":
                    ref, err := p.ConnectedTo.AsNetworkDeviceRef()
                    if err != nil {
                        return err
                    }
                    connectedTo = ref.Name
                case "client":
                    ref, err := p.ConnectedTo.AsNetworkClientRef()
                    if err != nil {
                        return err
                    }
                    connectedTo = ref.Name
                }
            }
            portViews = append(portViews, switchPortView{SwitchPort: p, ConnectedToName: connectedTo})
        }
        return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_get_switch.tmpl",
            switchDetailView{SwitchDetail: d, Ports: portViews})

    case "accessPoint":
        d, err := detail.AsAccessPointDetail()
        if err != nil {
            return err
        }
        return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_get_accesspoint.tmpl", d)

    case "gateway":
        d, err := detail.AsGatewayDetail()
        if err != nil {
            return err
        }
        return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_get_gateway.tmpl", d)

    case "unknown":
        d, err := detail.AsUnknownDeviceDetail()
        if err != nil {
            return err
        }
        return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "devices_get_unknown.tmpl", d)

    default:
        return fmt.Errorf("unknown device type: %s", disc)
    }
},
```

Replace `newListClientsCmd` RunE body (inside `watch.Wrap`):
```go
func(ctx context.Context, w io.Writer) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    params := &gen.ListNetworkClientsParams{}
    if statusFilter != "" {
        s := gen.NetworkClientStatus(statusFilter)
        params.Status = &s
    }

    resp, err := c.ListNetworkClientsWithResponse(ctx, params)
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(w, string(resp.Body))
        return nil
    }
    return output.RenderTemplate(w, networkTemplates, "clients_list.tmpl", *resp.JSON200)
}
```

Replace `newGetClientCmd` RunE body (keep the discriminator switch unchanged):
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.GetNetworkClientWithResponse(context.Background(), args[0])
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }

    detail := *resp.JSON200
    disc, err := detail.Discriminator()
    if err != nil {
        return err
    }

    switch disc {
    case "wired":
        d, err := detail.AsWiredNetworkClientDetail()
        if err != nil {
            return err
        }
        return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "clients_get_wired.tmpl", d)

    case "wireless":
        d, err := detail.AsWirelessNetworkClientDetail()
        if err != nil {
            return err
        }
        return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "clients_get_wireless.tmpl", d)

    default:
        return fmt.Errorf("unknown connection type: %s", disc)
    }
},
```

Replace `newTopologyCmd` RunE body (inside `watch.Wrap`):
```go
func(ctx context.Context, w io.Writer) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    params := &gen.GetNetworkTopologyParams{}
    if includeClients || includeWireless {
        t := true
        params.IncludeClients = &t
    }

    resp, err := c.GetNetworkTopologyWithResponse(ctx, params)
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(w, string(resp.Body))
        return nil
    }

    tree, err := buildTopologyTree(*resp.JSON200, includeWireless)
    if err != nil {
        return err
    }
    return output.RenderTemplate(w, networkTemplates, "topology.tmpl", tree)
}
```

- [ ] **Step 4: Update `internal/cli/network/ssids.go`**

Remove imports `"encoding/json"` and `"io"`. Keep all others.

Replace `newListSsidsCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.ListSsidsWithResponse(context.Background())
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "ssids_list.tmpl", *resp.JSON200)
},
```

Replace `newGetSsidCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.GetSsidWithResponse(context.Background(), args[0])
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "ssids_get.tmpl", *resp.JSON200)
},
```

- [ ] **Step 5: Update `internal/cli/network/vlans.go`**

Remove imports `"encoding/json"` and `"io"`. Keep all others.

Replace `newListVlansCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.ListVlansWithResponse(context.Background())
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "vlans_list.tmpl", *resp.JSON200)
},
```

Replace `newGetVlanCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.GetVlanWithResponse(context.Background(), args[0])
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "vlans_get.tmpl", *resp.JSON200)
},
```

- [ ] **Step 6: Update `internal/cli/network/wans.go`**

Remove imports `"encoding/json"` and `"io"`. Keep all others.

Replace `newListWansCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.ListWansWithResponse(context.Background())
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "wans_list.tmpl", *resp.JSON200)
},
```

Replace `newGetWanCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.GetWanWithResponse(context.Background(), args[0])
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }
    return output.RenderTemplate(cmd.OutOrStdout(), networkTemplates, "wans_get.tmpl", *resp.JSON200)
},
```

- [ ] **Step 7: Update `internal/cli/network/network_test.go` — stub initializations**

Every stub struct literal in the test file must be updated. The transformation rule for each field:

**Success case (was):**
```go
ListNetworkDevicesFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
    return jsonResponse(http.StatusOK, gen.NetworkDeviceList{Items: ...}), nil
},
```

**Success case (now):**
```go
ListNetworkDevicesWithResponseFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*gen.ListNetworkDevicesResponse, error) {
    list := gen.NetworkDeviceList{Items: ...}
    b, _ := json.Marshal(list)
    return &gen.ListNetworkDevicesResponse{
        HTTPResponse: &http.Response{StatusCode: http.StatusOK},
        Body:         b,
        JSON200:      &list,
    }, nil
},
```

**Error case (was):**
```go
ListNetworkDevicesFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
    return jsonResponse(http.StatusUnauthorized, map[string]any{"title": "Unauthorized", ...}), nil
},
```

**Error case (now):**
```go
ListNetworkDevicesWithResponseFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*gen.ListNetworkDevicesResponse, error) {
    b, _ := json.Marshal(map[string]any{"title": "Unauthorized", "status": 401, "detail": "Bearer token missing"})
    return &gen.ListNetworkDevicesResponse{
        HTTPResponse: &http.Response{StatusCode: http.StatusUnauthorized},
        Body:         b,
    }, nil
},
```

**For `GetNetworkDevice` tests that pass raw `map[string]any` (discriminated union):**
```go
GetNetworkDeviceWithResponseFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*gen.GetNetworkDeviceResponse, error) {
    payload := map[string]any{"id": "unifi.usg", "type": "gateway", ...}
    b, _ := json.Marshal(payload)
    var detail gen.NetworkDeviceDetail
    _ = json.Unmarshal(b, &detail)
    return &gen.GetNetworkDeviceResponse{
        HTTPResponse: &http.Response{StatusCode: http.StatusOK},
        Body:         b,
        JSON200:      &detail,
    }, nil
},
```

Apply the same pattern for all other stub fields. The complete list of field renames:
- `ListNetworkDevicesFunc` → `ListNetworkDevicesWithResponseFunc`
- `GetNetworkDeviceFunc` → `GetNetworkDeviceWithResponseFunc`
- `ListNetworkClientsFunc` → `ListNetworkClientsWithResponseFunc`
- `GetNetworkClientFunc` → `GetNetworkClientWithResponseFunc`
- `GetNetworkTopologyFunc` → `GetNetworkTopologyWithResponseFunc`
- `ListVlansFunc` → `ListVlansWithResponseFunc`
- `GetVlanFunc` → `GetVlanWithResponseFunc`
- `ListSsidsFunc` → `ListSsidsWithResponseFunc`
- `GetSsidFunc` → `GetSsidWithResponseFunc`
- `ListWansFunc` → `ListWansWithResponseFunc`
- `GetWanFunc` → `GetWanWithResponseFunc`

Add `"encoding/json"` to imports in `network_test.go`. Remove `"io"`, `"strings"`.

- [ ] **Step 8: Run tests**

```bash
go test ./internal/cli/network/...
```

Expected: all tests pass.

- [ ] **Step 9: Commit**

```bash
git add internal/cli/network/
git commit -m "refactor: migrate network domain to WithResponse client methods"
```

---

## Task 3: Migrate system domain

**Files:**
- Modify: `internal/cli/system/client.go`
- Modify: `internal/cli/system/stub.go`
- Modify: `internal/cli/system/system.go`
- Modify: `internal/cli/system/system_test.go`

- [ ] **Step 1: Update `internal/cli/system/client.go`**

```go
package system

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/system"
)

// SystemClient is the interface used by system commands.
type SystemClient interface {
	GetSystemHealthWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.GetSystemHealthResponse, error)
	ListSystemInfoWithResponse(ctx context.Context, params *gen.ListSystemInfoParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemInfoResponse, error)
	ListSystemUtilizationWithResponse(ctx context.Context, params *gen.ListSystemUtilizationParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemUtilizationResponse, error)
	ListSystemUpdatesWithResponse(ctx context.Context, params *gen.ListSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemUpdatesResponse, error)
	GetSystemUpdateWithResponse(ctx context.Context, updateId string, reqEditors ...gen.RequestEditorFn) (*gen.GetSystemUpdateResponse, error)
	CheckSystemUpdatesWithResponse(ctx context.Context, params *gen.CheckSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*gen.CheckSystemUpdatesResponse, error)
}

// NewSystemClient constructs a SystemClient backed by the real API.
func NewSystemClient(httpClient *http.Client, apiURL string) (SystemClient, error) {
	return gen.NewClientWithResponses(apiURL, gen.WithHTTPClient(httpClient))
}
```

- [ ] **Step 2: Update `internal/cli/system/stub.go`**

```go
package system

import (
	"context"

	gen "github.com/bwilczynski/hlctl/internal/system"
)

// StubClient is a SystemClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	GetSystemHealthWithResponseFunc       func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.GetSystemHealthResponse, error)
	ListSystemInfoWithResponseFunc        func(ctx context.Context, params *gen.ListSystemInfoParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemInfoResponse, error)
	ListSystemUtilizationWithResponseFunc func(ctx context.Context, params *gen.ListSystemUtilizationParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemUtilizationResponse, error)
	ListSystemUpdatesWithResponseFunc     func(ctx context.Context, params *gen.ListSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemUpdatesResponse, error)
	GetSystemUpdateWithResponseFunc       func(ctx context.Context, updateId string, reqEditors ...gen.RequestEditorFn) (*gen.GetSystemUpdateResponse, error)
	CheckSystemUpdatesWithResponseFunc    func(ctx context.Context, params *gen.CheckSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*gen.CheckSystemUpdatesResponse, error)
}

func (s *StubClient) GetSystemHealthWithResponse(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*gen.GetSystemHealthResponse, error) {
	return s.GetSystemHealthWithResponseFunc(ctx, reqEditors...)
}
func (s *StubClient) ListSystemInfoWithResponse(ctx context.Context, params *gen.ListSystemInfoParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemInfoResponse, error) {
	return s.ListSystemInfoWithResponseFunc(ctx, params, reqEditors...)
}
func (s *StubClient) ListSystemUtilizationWithResponse(ctx context.Context, params *gen.ListSystemUtilizationParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemUtilizationResponse, error) {
	return s.ListSystemUtilizationWithResponseFunc(ctx, params, reqEditors...)
}
func (s *StubClient) ListSystemUpdatesWithResponse(ctx context.Context, params *gen.ListSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListSystemUpdatesResponse, error) {
	return s.ListSystemUpdatesWithResponseFunc(ctx, params, reqEditors...)
}
func (s *StubClient) GetSystemUpdateWithResponse(ctx context.Context, updateId string, reqEditors ...gen.RequestEditorFn) (*gen.GetSystemUpdateResponse, error) {
	return s.GetSystemUpdateWithResponseFunc(ctx, updateId, reqEditors...)
}
func (s *StubClient) CheckSystemUpdatesWithResponse(ctx context.Context, params *gen.CheckSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*gen.CheckSystemUpdatesResponse, error) {
	return s.CheckSystemUpdatesWithResponseFunc(ctx, params, reqEditors...)
}
```

- [ ] **Step 3: Update `internal/cli/system/system.go` — all command bodies**

Remove imports `"encoding/json"` and `"io"` from the import block. Keep all others.

Replace `newHealthCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.GetSystemHealthWithResponse(context.Background())
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }

    health := *resp.JSON200
    headers := []string{"COMPONENT", "STATUS"}
    var rows [][]string
    for _, comp := range health.Components {
        rows = append(rows, []string{comp.Name, string(comp.Status)})
    }
    return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), health, headers, rows)
},
```

Replace `newInfoCmd` RunE body:
```go
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

    resp, err := c.ListSystemInfoWithResponse(context.Background(), params)
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }

    list := *resp.JSON200
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
```

Replace `newUtilizationCmd` RunE body (inside `watch.Wrap`): keep the table-building logic unchanged, replace only the API call and plumbing:
```go
func(ctx context.Context, w io.Writer) error {
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

    resp, err := c.ListSystemUtilizationWithResponse(ctx, params)
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(w, string(resp.Body))
        return nil
    }

    list := *resp.JSON200
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
    return output.Print(w, flags.GetOutputFormat(), list, headers, rows)
}
```

Replace `newListUpdatesCmd` RunE body:
```go
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

    resp, err := c.ListSystemUpdatesWithResponse(context.Background(), params)
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }
    return printUpdateList(cmd.OutOrStdout(), *resp.JSON200)
},
```

Replace `newGetUpdateCmd` RunE body (keep discriminator switch unchanged):
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.GetSystemUpdateWithResponse(context.Background(), args[0])
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }

    detail := *resp.JSON200
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
            {"ID", d.Id}, {"NAME", d.Name}, {"DEVICE", d.Device},
            {"TYPE", string(d.Type)}, {"STATUS", string(d.Status)},
            {"CURRENT", d.CurrentVersion}, {"LATEST", d.LatestVersion},
            {"CHECKED AT", output.FormatTime(d.CheckedAt)},
            {"PUBLISHED AT", output.FormatTime(d.PublishedAt)},
            {"IMAGE", d.Image}, {"SOURCE", d.Source}, {"RELEASE URL", d.ReleaseUrl},
        }
        return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)
    default:
        return fmt.Errorf("unknown update type: %s", disc)
    }
},
```

Replace `newCheckUpdatesCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.CheckSystemUpdatesWithResponse(context.Background(), &gen.CheckSystemUpdatesParams{})
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }
    return printUpdateList(cmd.OutOrStdout(), *resp.JSON200)
},
```

- [ ] **Step 4: Update `internal/cli/system/system_test.go` — stub initializations**

Field renames (same success/error pattern as shown in Task 2 Step 7):
- `GetSystemHealthFunc` → `GetSystemHealthWithResponseFunc` → returns `*gen.GetSystemHealthResponse`
- `ListSystemInfoFunc` → `ListSystemInfoWithResponseFunc` → returns `*gen.ListSystemInfoResponse`
- `ListSystemUtilizationFunc` → `ListSystemUtilizationWithResponseFunc` → returns `*gen.ListSystemUtilizationResponse`
- `ListSystemUpdatesFunc` → `ListSystemUpdatesWithResponseFunc` → returns `*gen.ListSystemUpdatesResponse`
- `GetSystemUpdateFunc` → `GetSystemUpdateWithResponseFunc` → returns `*gen.GetSystemUpdateResponse`
- `CheckSystemUpdatesFunc` → `CheckSystemUpdatesWithResponseFunc` → returns `*gen.CheckSystemUpdatesResponse`

For each stub: marshal the payload to bytes, set `HTTPResponse`, `Body`, and `JSON200` for success cases; set only `HTTPResponse` and `Body` for error cases. Add `"encoding/json"` to imports. Remove `"io"`, `"strings"`.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/cli/system/...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/system/
git commit -m "refactor: migrate system domain to WithResponse client methods"
```

---

## Task 4: Migrate storage domain

**Files:**
- Modify: `internal/cli/storage/client.go`
- Modify: `internal/cli/storage/stub.go`
- Modify: `internal/cli/storage/storage.go`
- Modify: `internal/cli/storage/storage_test.go`

- [ ] **Step 1: Update `internal/cli/storage/client.go`**

```go
package storage

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/storage"
)

// StorageClient is the interface used by storage commands.
type StorageClient interface {
	ListStorageVolumesWithResponse(ctx context.Context, params *gen.ListStorageVolumesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListStorageVolumesResponse, error)
	GetStorageVolumeWithResponse(ctx context.Context, volumeId string, reqEditors ...gen.RequestEditorFn) (*gen.GetStorageVolumeResponse, error)
	ListBackupsWithResponse(ctx context.Context, params *gen.ListBackupsParams, reqEditors ...gen.RequestEditorFn) (*gen.ListBackupsResponse, error)
	GetBackupWithResponse(ctx context.Context, backupId string, reqEditors ...gen.RequestEditorFn) (*gen.GetBackupResponse, error)
}

// NewStorageClient constructs a StorageClient backed by the real API.
func NewStorageClient(httpClient *http.Client, apiURL string) (StorageClient, error) {
	return gen.NewClientWithResponses(apiURL, gen.WithHTTPClient(httpClient))
}
```

- [ ] **Step 2: Update `internal/cli/storage/stub.go`**

```go
package storage

import (
	"context"

	gen "github.com/bwilczynski/hlctl/internal/storage"
)

// StubClient is a StorageClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListStorageVolumesWithResponseFunc func(ctx context.Context, params *gen.ListStorageVolumesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListStorageVolumesResponse, error)
	GetStorageVolumeWithResponseFunc   func(ctx context.Context, volumeId string, reqEditors ...gen.RequestEditorFn) (*gen.GetStorageVolumeResponse, error)
	ListBackupsWithResponseFunc        func(ctx context.Context, params *gen.ListBackupsParams, reqEditors ...gen.RequestEditorFn) (*gen.ListBackupsResponse, error)
	GetBackupWithResponseFunc          func(ctx context.Context, backupId string, reqEditors ...gen.RequestEditorFn) (*gen.GetBackupResponse, error)
}

func (s *StubClient) ListStorageVolumesWithResponse(ctx context.Context, params *gen.ListStorageVolumesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListStorageVolumesResponse, error) {
	return s.ListStorageVolumesWithResponseFunc(ctx, params, reqEditors...)
}
func (s *StubClient) GetStorageVolumeWithResponse(ctx context.Context, volumeId string, reqEditors ...gen.RequestEditorFn) (*gen.GetStorageVolumeResponse, error) {
	return s.GetStorageVolumeWithResponseFunc(ctx, volumeId, reqEditors...)
}
func (s *StubClient) ListBackupsWithResponse(ctx context.Context, params *gen.ListBackupsParams, reqEditors ...gen.RequestEditorFn) (*gen.ListBackupsResponse, error) {
	return s.ListBackupsWithResponseFunc(ctx, params, reqEditors...)
}
func (s *StubClient) GetBackupWithResponse(ctx context.Context, backupId string, reqEditors ...gen.RequestEditorFn) (*gen.GetBackupResponse, error) {
	return s.GetBackupWithResponseFunc(ctx, backupId, reqEditors...)
}
```

- [ ] **Step 3: Update `internal/cli/storage/storage.go` — all command bodies**

Remove imports `"encoding/json"` and `"io"`. Keep all others.

Replace `newListVolumesCmd` RunE body:
```go
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

    resp, err := c.ListStorageVolumesWithResponse(context.Background(), params)
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }

    list := *resp.JSON200
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
```

Replace `newGetVolumeCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.GetStorageVolumeWithResponse(context.Background(), args[0])
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }
    return printVolumeDetail(cmd, *resp.JSON200)
},
```

Replace `newListBackupsCmd` RunE body:
```go
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

    resp, err := c.ListBackupsWithResponse(context.Background(), params)
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }

    list := *resp.JSON200
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
```

Replace `newGetBackupCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.GetBackupWithResponse(context.Background(), args[0])
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }

    detail := *resp.JSON200
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
```

- [ ] **Step 4: Update `internal/cli/storage/storage_test.go` — stub initializations**

Field renames:
- `ListStorageVolumesFunc` → `ListStorageVolumesWithResponseFunc` → returns `*gen.ListStorageVolumesResponse` (`JSON200` is `*gen.VolumeList`)
- `GetStorageVolumeFunc` → `GetStorageVolumeWithResponseFunc` → returns `*gen.GetStorageVolumeResponse` (`JSON200` is `*gen.VolumeDetail`)
- `ListBackupsFunc` → `ListBackupsWithResponseFunc` → returns `*gen.ListBackupsResponse` (`JSON200` is `*gen.BackupTaskList`)
- `GetBackupFunc` → `GetBackupWithResponseFunc` → returns `*gen.GetBackupResponse` (`JSON200` is `*gen.BackupTaskDetail`)

Apply the same success/error construction pattern from Task 2 Step 7. Add `"encoding/json"` import. Remove `"io"`, `"strings"`.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/cli/storage/...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/storage/
git commit -m "refactor: migrate storage domain to WithResponse client methods"
```

---

## Task 5: Migrate docker domain

**Files:**
- Modify: `internal/cli/docker/client.go`
- Modify: `internal/cli/docker/stub.go`
- Modify: `internal/cli/docker/docker.go`
- Modify: `internal/cli/docker/docker_test.go`

- [ ] **Step 1: Update `internal/cli/docker/client.go`**

```go
package docker

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/docker"
)

// DockerClient is the interface used by all docker subcommands.
type DockerClient interface {
	ListContainersWithResponse(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*gen.ListContainersResponse, error)
	GetContainerWithResponse(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*gen.GetContainerResponse, error)
	StartContainerWithResponse(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.StartContainerResponse, error)
	StopContainerWithResponse(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.StopContainerResponse, error)
	RestartContainerWithResponse(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.RestartContainerResponse, error)
	ListDockerNetworksWithResponse(ctx context.Context, params *gen.ListDockerNetworksParams, reqEditors ...gen.RequestEditorFn) (*gen.ListDockerNetworksResponse, error)
	GetDockerNetworkWithResponse(ctx context.Context, networkId string, reqEditors ...gen.RequestEditorFn) (*gen.GetDockerNetworkResponse, error)
	ListDockerImagesWithResponse(ctx context.Context, params *gen.ListDockerImagesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListDockerImagesResponse, error)
	GetDockerImageWithResponse(ctx context.Context, imageId string, reqEditors ...gen.RequestEditorFn) (*gen.GetDockerImageResponse, error)
}

// NewDockerClient constructs a DockerClient backed by the real API.
func NewDockerClient(httpClient *http.Client, apiURL string) (DockerClient, error) {
	return gen.NewClientWithResponses(apiURL, gen.WithHTTPClient(httpClient))
}
```

- [ ] **Step 2: Update `internal/cli/docker/stub.go`**

```go
package docker

import (
	"context"

	gen "github.com/bwilczynski/hlctl/internal/docker"
)

// StubClient is a DockerClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListContainersWithResponseFunc     func(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*gen.ListContainersResponse, error)
	GetContainerWithResponseFunc       func(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*gen.GetContainerResponse, error)
	StartContainerWithResponseFunc     func(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.StartContainerResponse, error)
	StopContainerWithResponseFunc      func(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.StopContainerResponse, error)
	RestartContainerWithResponseFunc   func(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.RestartContainerResponse, error)
	ListDockerNetworksWithResponseFunc func(ctx context.Context, params *gen.ListDockerNetworksParams, reqEditors ...gen.RequestEditorFn) (*gen.ListDockerNetworksResponse, error)
	GetDockerNetworkWithResponseFunc   func(ctx context.Context, networkId string, reqEditors ...gen.RequestEditorFn) (*gen.GetDockerNetworkResponse, error)
	ListDockerImagesWithResponseFunc   func(ctx context.Context, params *gen.ListDockerImagesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListDockerImagesResponse, error)
	GetDockerImageWithResponseFunc     func(ctx context.Context, imageId string, reqEditors ...gen.RequestEditorFn) (*gen.GetDockerImageResponse, error)
}

func (s *StubClient) ListContainersWithResponse(ctx context.Context, params *gen.ListContainersParams, reqEditors ...gen.RequestEditorFn) (*gen.ListContainersResponse, error) {
	return s.ListContainersWithResponseFunc(ctx, params, reqEditors...)
}
func (s *StubClient) GetContainerWithResponse(ctx context.Context, containerId string, reqEditors ...gen.RequestEditorFn) (*gen.GetContainerResponse, error) {
	return s.GetContainerWithResponseFunc(ctx, containerId, reqEditors...)
}
func (s *StubClient) StartContainerWithResponse(ctx context.Context, containerId string, params *gen.StartContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.StartContainerResponse, error) {
	return s.StartContainerWithResponseFunc(ctx, containerId, params, reqEditors...)
}
func (s *StubClient) StopContainerWithResponse(ctx context.Context, containerId string, params *gen.StopContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.StopContainerResponse, error) {
	return s.StopContainerWithResponseFunc(ctx, containerId, params, reqEditors...)
}
func (s *StubClient) RestartContainerWithResponse(ctx context.Context, containerId string, params *gen.RestartContainerParams, reqEditors ...gen.RequestEditorFn) (*gen.RestartContainerResponse, error) {
	return s.RestartContainerWithResponseFunc(ctx, containerId, params, reqEditors...)
}
func (s *StubClient) ListDockerNetworksWithResponse(ctx context.Context, params *gen.ListDockerNetworksParams, reqEditors ...gen.RequestEditorFn) (*gen.ListDockerNetworksResponse, error) {
	return s.ListDockerNetworksWithResponseFunc(ctx, params, reqEditors...)
}
func (s *StubClient) GetDockerNetworkWithResponse(ctx context.Context, networkId string, reqEditors ...gen.RequestEditorFn) (*gen.GetDockerNetworkResponse, error) {
	return s.GetDockerNetworkWithResponseFunc(ctx, networkId, reqEditors...)
}
func (s *StubClient) ListDockerImagesWithResponse(ctx context.Context, params *gen.ListDockerImagesParams, reqEditors ...gen.RequestEditorFn) (*gen.ListDockerImagesResponse, error) {
	return s.ListDockerImagesWithResponseFunc(ctx, params, reqEditors...)
}
func (s *StubClient) GetDockerImageWithResponse(ctx context.Context, imageId string, reqEditors ...gen.RequestEditorFn) (*gen.GetDockerImageResponse, error) {
	return s.GetDockerImageWithResponseFunc(ctx, imageId, reqEditors...)
}
```

- [ ] **Step 3: Update `internal/cli/docker/docker.go` — all command bodies**

Remove imports `"encoding/json"` and `"io"` from the import block. Keep all others.

Replace `newListCmd` RunE body (inside `watch.Wrap`):
```go
func(ctx context.Context, w io.Writer) error {
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

    resp, err := c.ListContainersWithResponse(ctx, params)
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(w, string(resp.Body))
        return nil
    }

    list := *resp.JSON200
    headers := []string{"ID", "IMAGE", "STATUS", "CPU", "MEMORY"}
    var rows [][]string
    for _, c := range list.Items {
        rows = append(rows, []string{
            c.Id, c.Image, string(c.Status),
            fmt.Sprintf("%.1f%%", c.Resources.CpuPercent),
            output.FormatBytes(c.Resources.MemoryBytes),
        })
    }
    return output.Print(w, flags.GetOutputFormat(), list, headers, rows)
}
```

Replace `newGetCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.GetContainerWithResponse(context.Background(), args[0])
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }
    return printContainerDetail(cmd, *resp.JSON200)
},
```

Replace `newStartCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }
    resp, err := c.StartContainerWithResponse(context.Background(), args[0], &gen.StartContainerParams{})
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusNoContent {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    fmt.Fprintf(cmd.OutOrStdout(), "Container %s started\n", args[0])
    return nil
},
```

Replace `newStopCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }
    resp, err := c.StopContainerWithResponse(context.Background(), args[0], &gen.StopContainerParams{})
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusNoContent {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    fmt.Fprintf(cmd.OutOrStdout(), "Container %s stopped\n", args[0])
    return nil
},
```

Replace `newRestartCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }
    resp, err := c.RestartContainerWithResponse(context.Background(), args[0], &gen.RestartContainerParams{})
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusNoContent {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    fmt.Fprintf(cmd.OutOrStdout(), "Container %s restarted\n", args[0])
    return nil
},
```

Replace `newListNetworksCmd` RunE body:
```go
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

    resp, err := c.ListDockerNetworksWithResponse(context.Background(), params)
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }

    list := *resp.JSON200
    headers := []string{"ID", "NAME", "DEVICE", "CONTAINERS"}
    var rows [][]string
    for _, n := range list.Items {
        rows = append(rows, []string{n.Id, n.Name, n.Device, fmt.Sprintf("%d", n.ConnectedContainers)})
    }
    return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
},
```

Replace `newGetNetworkCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.GetDockerNetworkWithResponse(context.Background(), args[0])
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }
    return printNetworkDetail(cmd, *resp.JSON200)
},
```

Replace `newListImagesCmd` RunE body:
```go
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

    resp, err := c.ListDockerImagesWithResponse(context.Background(), params)
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }

    list := *resp.JSON200
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
```

Replace `newGetImageCmd` RunE body:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    c := client
    if c == nil {
        var err error
        c, err = buildClient()
        if err != nil {
            return err
        }
    }

    resp, err := c.GetDockerImageWithResponse(context.Background(), args[0])
    if err != nil {
        return err
    }
    if resp.StatusCode() != http.StatusOK {
        return apiclient.ParseError(resp.StatusCode(), resp.Body)
    }
    if flags.GetOutputFormat() == output.FormatJSON {
        fmt.Fprint(cmd.OutOrStdout(), string(resp.Body))
        return nil
    }
    return printImageDetail(cmd, *resp.JSON200)
},
```

- [ ] **Step 4: Update `internal/cli/docker/docker_test.go` — stub initializations**

Field renames:
- `ListContainersFunc` → `ListContainersWithResponseFunc` → returns `*gen.ListContainersResponse` (`JSON200` is `*gen.ContainerList`)
- `GetContainerFunc` → `GetContainerWithResponseFunc` → returns `*gen.GetContainerResponse` (`JSON200` is `*gen.ContainerDetail`)
- `StartContainerFunc` → `StartContainerWithResponseFunc` → returns `*gen.StartContainerResponse` (no `JSON200`; success is 204)
- `StopContainerFunc` → `StopContainerWithResponseFunc` → returns `*gen.StopContainerResponse` (no `JSON200`; success is 204)
- `RestartContainerFunc` → `RestartContainerWithResponseFunc` → returns `*gen.RestartContainerResponse` (no `JSON200`; success is 204)
- `ListDockerNetworksFunc` → `ListDockerNetworksWithResponseFunc` → returns `*gen.ListDockerNetworksResponse` (`JSON200` is `*gen.DockerNetworkList`)
- `GetDockerNetworkFunc` → `GetDockerNetworkWithResponseFunc` → returns `*gen.GetDockerNetworkResponse` (`JSON200` is `*gen.DockerNetworkDetail`)
- `ListDockerImagesFunc` → `ListDockerImagesWithResponseFunc` → returns `*gen.ListDockerImagesResponse` (`JSON200` is `*gen.DockerImageList`)
- `GetDockerImageFunc` → `GetDockerImageWithResponseFunc` → returns `*gen.GetDockerImageResponse` (`JSON200` is `*gen.DockerImageDetail`)

For 204 action commands (start/stop/restart), success stubs omit `JSON200` and use `http.StatusNoContent`:
```go
StartContainerWithResponseFunc: func(_ context.Context, _ string, _ *gen.StartContainerParams, _ ...gen.RequestEditorFn) (*gen.StartContainerResponse, error) {
    return &gen.StartContainerResponse{
        HTTPResponse: &http.Response{StatusCode: http.StatusNoContent},
        Body:         []byte{},
    }, nil
},
```

For error stubs on action commands:
```go
StartContainerWithResponseFunc: func(_ context.Context, _ string, _ *gen.StartContainerParams, _ ...gen.RequestEditorFn) (*gen.StartContainerResponse, error) {
    b, _ := json.Marshal(map[string]any{"title": "Not Found", "status": 404})
    return &gen.StartContainerResponse{
        HTTPResponse: &http.Response{StatusCode: http.StatusNotFound},
        Body:         b,
    }, nil
},
```

Add `"encoding/json"` to imports. Remove `"io"`, `"strings"`.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/cli/docker/...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/docker/
git commit -m "refactor: migrate docker domain to WithResponse client methods"
```

---

## Task 6: Remove `ParseErrorResponse`

**Files:**
- Modify: `internal/apiclient/errors.go`
- Modify: `internal/apiclient/errors_test.go`

- [ ] **Step 1: Remove `ParseErrorResponse` from `errors.go`**

Remove the `ParseErrorResponse` function and its `"io"` and `"net/http"` imports (no longer needed). Final `errors.go`:

```go
package apiclient

import (
	"encoding/json"
	"errors"
	"fmt"
)

type problem struct {
	Title  string  `json:"title"`
	Detail *string `json:"detail,omitempty"`
}

// ParseError parses an RFC 9457 Problem Details body and returns a user-friendly error.
// Call this on any non-2xx response using resp.StatusCode() and resp.Body.
func ParseError(statusCode int, body []byte) error {
	var p problem
	if err := json.Unmarshal(body, &p); err != nil || p.Title == "" {
		return fmt.Errorf("unexpected status %d", statusCode)
	}
	if p.Detail != nil && *p.Detail != "" {
		return fmt.Errorf("%s — %s", p.Title, *p.Detail)
	}
	return errors.New(p.Title)
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./...
```

Expected: all tests pass with no compilation errors.

- [ ] **Step 3: Commit**

```bash
git add internal/apiclient/errors.go internal/apiclient/errors_test.go
git commit -m "refactor: remove ParseErrorResponse now all domains use ParseError(statusCode, body)"
```
