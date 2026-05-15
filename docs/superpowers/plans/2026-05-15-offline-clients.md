# Offline Client Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update the `hlctl network clients` commands to surface the new `status` field, support filtering by status, and handle optional session fields for offline clients.

**Architecture:** Advance the spec submodule, regenerate the API client, update the `NetworkClient` interface and stub to add params to `ListNetworkClients`, then update the list and get commands with TDD.

**Tech Stack:** Go, Cobra, oapi-codegen, testify-style table tests in stdlib

---

## File Map

| File | Change |
|------|--------|
| `spec` (submodule) | Advance to `4a073a5` |
| `internal/network/api.gen.go` | Regenerated — new `NetworkClientStatus`, `ListNetworkClientsParams`, optional pointer fields |
| `internal/cli/network/client.go` | `ListNetworkClients` gains `params *gen.ListNetworkClientsParams` |
| `internal/cli/network/stub.go` | Matching update to `ListNetworkClientsFunc` and method |
| `internal/cli/network/network.go` | Add `--status` flag + `STATUS` column; handle optional fields in `get` |
| `internal/cli/network/network_test.go` | Update existing fixtures; add 4 new tests |

---

### Task 1: Advance spec submodule and regenerate API client

**Files:**
- Modify: `spec` (submodule pointer)
- Regenerate: `internal/network/api.gen.go` (gitignored, not committed)

- [ ] **Step 1: Advance submodule and regenerate**

```bash
cd /path/to/repo
git -C spec fetch origin
git -C spec checkout 4a073a5
make generate
```

Expected: no errors; `internal/network/api.gen.go` is updated.

- [ ] **Step 2: Verify the key generated types**

```bash
grep -n "NetworkClientStatus\|ListNetworkClientsParams\|SwitchName\|SwitchPort\|SignalStrength" internal/network/api.gen.go | head -30
```

Expected output includes:
- `type NetworkClientStatus string`
- `NetworkClientStatusOnline NetworkClientStatus = "online"`
- `NetworkClientStatusOffline NetworkClientStatus = "offline"`
- `type ListNetworkClientsParams struct`
- `Status *NetworkClientStatus`
- `SwitchName *string` (was non-pointer before)
- `SwitchPort *int` (was non-pointer before)
- `SignalStrength *int` (was non-pointer before)

> If field names differ from the above (oapi-codegen sometimes uses different casing), note the actual names — they must match exactly in all subsequent tasks.

- [ ] **Step 3: Confirm build currently fails (interface mismatch)**

```bash
make build 2>&1 | head -20
```

Expected: compile error like `cannot use ... ListNetworkClients` — the interface in `client.go` does not yet match the regenerated signature. This confirms regeneration worked.

- [ ] **Step 4: Commit submodule pointer**

```bash
git add spec
git commit -m "chore: advance spec submodule to 4a073a5 (offline client support)"
```

---

### Task 2: Update NetworkClient interface and stub

**Files:**
- Modify: `internal/cli/network/client.go`
- Modify: `internal/cli/network/stub.go`

- [ ] **Step 1: Update the interface in `client.go`**

Replace the `ListNetworkClients` line:

```go
// Before
ListNetworkClients(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)

// After
ListNetworkClients(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
```

Full updated file:

```go
package network

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/network"
)

// NetworkClient is the interface used by network commands.
// It matches the subset of gen.ClientInterface that network commands need.
type NetworkClient interface {
	ListNetworkDevices(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkDevice(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListNetworkClients(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkClient(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

// NewNetworkClient constructs a NetworkClient backed by the real API.
func NewNetworkClient(httpClient *http.Client, apiURL string) (NetworkClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
```

- [ ] **Step 2: Update the stub in `stub.go`**

Full updated file:

```go
package network

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gen "github.com/bwilczynski/hlctl/internal/network"
)

// StubClient is a NetworkClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListNetworkDevicesFunc func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkDeviceFunc   func(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListNetworkClientsFunc func(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkClientFunc   func(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func (s *StubClient) ListNetworkDevices(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListNetworkDevicesFunc(ctx, reqEditors...)
}

func (s *StubClient) GetNetworkDevice(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetNetworkDeviceFunc(ctx, deviceId, reqEditors...)
}

func (s *StubClient) ListNetworkClients(ctx context.Context, params *gen.ListNetworkClientsParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListNetworkClientsFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetNetworkClient(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetNetworkClientFunc(ctx, clientId, reqEditors...)
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

- [ ] **Step 3: Confirm build still fails (network.go not yet updated)**

```bash
make build 2>&1 | head -20
```

Expected: compile error about `ListNetworkClients` call in `network.go` — still passes wrong args. Interface and stub are correct now.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/network/client.go internal/cli/network/stub.go
git commit -m "feat: update NetworkClient interface for ListNetworkClients params"
```

---

### Task 3: Add --status flag and STATUS column to list command (TDD)

**Files:**
- Modify: `internal/cli/network/network_test.go`
- Modify: `internal/cli/network/network.go`

- [ ] **Step 1: Update existing list test to add Status field and assert STATUS column**

In `network_test.go`, update `TestListClientsCmd_tableOutput`:

```go
func TestListClientsCmd_tableOutput(t *testing.T) {
	ip := "192.168.1.50"
	stub := &StubClient{
		ListNetworkClientsFunc: func(_ context.Context, _ *gen.ListNetworkClientsParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkClientList{
				Items: []gen.NetworkClient{
					{
						Id:             "unifi.aa:bb:cc:dd:ee:01",
						Name:           "laptop",
						Mac:            "aa:bb:cc:dd:ee:01",
						Ip:             &ip,
						ConnectionType: gen.NetworkClientConnectionTypeWired,
						Status:         gen.NetworkClientStatusOnline,
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
	for _, want := range []string{"unifi.aa:bb:cc:dd:ee:01", "laptop", "192.168.1.50", "wired", "online"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
```

Also update `TestListClientsCmd_apiError` to match the new stub signature:

```go
func TestListClientsCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListNetworkClientsFunc: func(_ context.Context, _ *gen.ListNetworkClientsParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListClientsCmd(stub)
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

- [ ] **Step 2: Add new status filter test**

Add `TestListClientsCmd_statusFilter` to `network_test.go`:

```go
func TestListClientsCmd_statusFilter(t *testing.T) {
	var capturedParams *gen.ListNetworkClientsParams
	stub := &StubClient{
		ListNetworkClientsFunc: func(_ context.Context, params *gen.ListNetworkClientsParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			capturedParams = params
			return jsonResponse(http.StatusOK, gen.NetworkClientList{Items: []gen.NetworkClient{}}), nil
		},
	}

	cmd := newListClientsCmd(stub)
	cmd.SetArgs([]string{"--status", "online"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedParams == nil || capturedParams.Status == nil {
		t.Fatal("expected Status param to be set")
	}
	if *capturedParams.Status != gen.NetworkClientStatusOnline {
		t.Errorf("expected status=online, got %q", *capturedParams.Status)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd internal/cli/network && go test ./... 2>&1
```

Expected: compile errors because `network.go` still uses old `ListNetworkClients` signature.

- [ ] **Step 4: Update `newListClientsCmd` in `network.go`**

Replace the `newListClientsCmd` function:

```go
func newListClientsCmd(client NetworkClient) *cobra.Command {
	var statusFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List network clients",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			resp, err := c.ListNetworkClients(context.Background(), params)
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

			headers := []string{"ID", "NAME", "MAC", "IP", "STATUS", "CONNECTION"}
			var rows [][]string
			for _, cl := range list.Items {
				ip := ""
				if cl.Ip != nil {
					ip = *cl.Ip
				}
				rows = append(rows, []string{
					cl.Id, cl.Name, cl.Mac, ip,
					string(cl.Status),
					string(cl.ConnectionType),
				})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), list, headers, rows)
		},
	}
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (online|offline)")
	return cmd
}
```

- [ ] **Step 5: Run tests**

```bash
cd internal/cli/network && go test ./... -run TestListClients -v 2>&1
```

Expected: all `TestListClients*` tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/network/network.go internal/cli/network/network_test.go
git commit -m "feat: add --status flag and STATUS column to network clients list"
```

---

### Task 4: Add STATUS row to get command and handle optional fields (TDD)

**Files:**
- Modify: `internal/cli/network/network_test.go`
- Modify: `internal/cli/network/network.go`

- [ ] **Step 1: Update existing wired and wireless get tests**

In `network_test.go`, update `TestGetClientCmd_wired` to include `status` and assert `STATUS` row:

```go
func TestGetClientCmd_wired(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:01",
				"name":           "laptop",
				"mac":            "aa:bb:cc:dd:ee:01",
				"ip":             "192.168.1.50",
				"connectionType": "wired",
				"status":         "online",
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
	for _, want := range []string{"laptop", "switch-1", fmt.Sprintf("%d", 3), "online"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
```

Update `TestGetClientCmd_wireless` to include `status`:

```go
func TestGetClientCmd_wireless(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:02",
				"name":           "phone",
				"mac":            "aa:bb:cc:dd:ee:02",
				"ip":             "192.168.1.51",
				"connectionType": "wireless",
				"status":         "online",
				"ssid":           "HomeNet",
				"signalStrength": -65,
				"uptime":         1800,
			}), nil
		},
	}

	cmd := newGetClientCmd(stub)
	cmd.SetArgs([]string{"unifi.aa:bb:cc:dd:ee:02"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"phone", "HomeNet", "-65 dBm", "online"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "SWITCH") {
		t.Errorf("expected no SWITCH row in wireless output, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Add offline wired test**

Add `TestGetClientCmd_offline_wired` to `network_test.go`:

```go
func TestGetClientCmd_offline_wired(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:03",
				"name":           "printer",
				"mac":            "aa:bb:cc:dd:ee:03",
				"ip":             "192.168.1.60",
				"connectionType": "wired",
				"status":         "offline",
			}), nil
		},
	}

	cmd := newGetClientCmd(stub)
	cmd.SetArgs([]string{"unifi.aa:bb:cc:dd:ee:03"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"printer", "offline"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"SWITCH PORT", "UPTIME"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected %q to be absent for offline wired client, got:\n%s", absent, out)
		}
	}
}
```

- [ ] **Step 3: Add offline wireless test**

Add `TestGetClientCmd_offline_wireless` to `network_test.go`:

```go
func TestGetClientCmd_offline_wireless(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":             "unifi.aa:bb:cc:dd:ee:04",
				"name":           "tablet",
				"mac":            "aa:bb:cc:dd:ee:04",
				"ip":             "192.168.1.70",
				"connectionType": "wireless",
				"status":         "offline",
			}), nil
		},
	}

	cmd := newGetClientCmd(stub)
	cmd.SetArgs([]string{"unifi.aa:bb:cc:dd:ee:04"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"tablet", "offline"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"SSID", "SIGNAL", "UPTIME"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected %q to be absent for offline wireless client, got:\n%s", absent, out)
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

```bash
cd internal/cli/network && go test ./... -run TestGetClient -v 2>&1
```

Expected: existing tests may pass but new offline tests fail (fields currently treated as required non-pointers).

- [ ] **Step 5: Update `newGetClientCmd` in `network.go`**

Replace the `newGetClientCmd` function:

```go
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
					{"STATUS", string(d.Status)},
				}
				if d.SwitchName != nil {
					rows = append(rows, []string{"SWITCH", *d.SwitchName})
				}
				if d.SwitchPort != nil {
					rows = append(rows, []string{"SWITCH PORT", fmt.Sprintf("%d", *d.SwitchPort)})
				}
				if d.Uptime != nil {
					rows = append(rows, []string{"UPTIME", output.FormatUptime(*d.Uptime)})
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
					{"STATUS", string(d.Status)},
				}
				if d.Ssid != nil {
					rows = append(rows, []string{"SSID", *d.Ssid})
				}
				if d.SignalStrength != nil {
					rows = append(rows, []string{"SIGNAL", fmt.Sprintf("%d dBm", *d.SignalStrength)})
				}
				if d.Uptime != nil {
					rows = append(rows, []string{"UPTIME", output.FormatUptime(*d.Uptime)})
				}
			default:
				return fmt.Errorf("unknown connection type: %s", disc)
			}

			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)
		},
	}
}
```

- [ ] **Step 6: Run all network tests**

```bash
cd internal/cli/network && go test ./... -v 2>&1
```

Expected: all tests pass.

- [ ] **Step 7: Build the binary**

```bash
make build 2>&1
```

Expected: `bin/hlctl` built with no errors.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/network/network.go internal/cli/network/network_test.go
git commit -m "feat: handle offline clients in network clients get command"
```

---

### Task 5: Final verification

- [ ] **Step 1: Run full test suite**

```bash
go test ./... 2>&1
```

Expected: all tests pass, no compile errors.

- [ ] **Step 2: Smoke test the binary**

```bash
./bin/hlctl network clients --help
./bin/hlctl network clients list --help
```

Expected: `--status` flag appears in `network clients list --help`.
