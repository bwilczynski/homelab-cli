# Network Commands Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace stub implementations in `internal/cli/network/network.go` with real API calls following the containers domain pattern.

**Architecture:** Add a `NetworkClient` interface and `StubClient` following containers exactly. Add `FormatUptime` to the output package. Rewrite `network.go` so each command constructor accepts a `NetworkClient` parameter — `nil` in production (built inside `RunE`), stub in tests.

**Tech Stack:** Go, Cobra, oapi-codegen (generated client in `internal/network/api.gen.go`), `internal/apiclient` for HTTP client construction and error parsing.

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `internal/output/output.go` | Add `FormatUptime(seconds int) string` |
| Modify | `internal/output/output_test.go` | Tests for `FormatUptime` |
| Create | `internal/cli/network/client.go` | `NetworkClient` interface + `NewNetworkClient` factory |
| Create | `internal/cli/network/stub.go` | `StubClient` + `jsonResponse` helper |
| Rewrite | `internal/cli/network/network.go` | All four commands wired to real API |
| Create | `internal/cli/network/network_test.go` | Unit tests via `StubClient` |

---

### Task 1: Add FormatUptime to output package

**Files:**
- Modify: `internal/output/output.go`
- Modify: `internal/output/output_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/output/output_test.go`:

```go
func TestFormatUptime(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0s"},
		{45, "45s"},
		{3600, "1h 0m 0s"},
		{7200, "2h 0m 0s"},
		{3665, "1h 1m 5s"},
		{86400, "1d 0h 0m 0s"},
		{604800, "7d 0h 0m 0s"},
		{90061, "1d 1h 1m 1s"},
	}
	for _, tt := range tests {
		got := output.FormatUptime(tt.input)
		if got != tt.expected {
			t.Errorf("FormatUptime(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
```

- [ ] **Step 2: Run the tests to confirm failure**

```
go test ./internal/output/... -run TestFormatUptime -v
```

Expected: FAIL — `undefined: output.FormatUptime`

- [ ] **Step 3: Implement FormatUptime**

Append to `internal/output/output.go` (also add `"strings"` to the import block):

```go
// FormatUptime converts seconds to a human-readable duration string.
// Leading zero segments are skipped; seconds are always included.
// Examples: 86400 → "1d 0h 0m 0s", 7200 → "2h 0m 0s", 45 → "45s".
func FormatUptime(seconds int) string {
	d := seconds / 86400
	seconds %= 86400
	h := seconds / 3600
	seconds %= 3600
	m := seconds / 60
	s := seconds % 60

	type seg struct {
		val  int
		unit string
	}
	segs := []seg{{d, "d"}, {h, "h"}, {m, "m"}, {s, "s"}}
	var parts []string
	for _, sg := range segs {
		if len(parts) > 0 || sg.val > 0 || sg.unit == "s" {
			parts = append(parts, fmt.Sprintf("%d%s", sg.val, sg.unit))
		}
	}
	return strings.Join(parts, " ")
}
```

- [ ] **Step 4: Run tests to confirm pass**

```
go test ./internal/output/... -v
```

Expected: all tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/output/output.go internal/output/output_test.go
git commit -m "Add FormatUptime helper to output package"
```

---

### Task 2: Create NetworkClient interface and StubClient

**Files:**
- Create: `internal/cli/network/client.go`
- Create: `internal/cli/network/stub.go`

- [ ] **Step 1: Create client.go**

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
	ListNetworkClients(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkClient(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

// NewNetworkClient constructs a NetworkClient backed by the real API.
func NewNetworkClient(httpClient *http.Client, apiURL string) (NetworkClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
```

- [ ] **Step 2: Create stub.go**

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
	ListNetworkClientsFunc func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetNetworkClientFunc   func(ctx context.Context, clientId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func (s *StubClient) ListNetworkDevices(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListNetworkDevicesFunc(ctx, reqEditors...)
}

func (s *StubClient) GetNetworkDevice(ctx context.Context, deviceId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetNetworkDeviceFunc(ctx, deviceId, reqEditors...)
}

func (s *StubClient) ListNetworkClients(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListNetworkClientsFunc(ctx, reqEditors...)
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

- [ ] **Step 3: Verify compilation**

```
go build ./internal/cli/network/...
```

Expected: no errors (network.go still compiles because it doesn't import these files)

- [ ] **Step 4: Commit**

```bash
git add internal/cli/network/client.go internal/cli/network/stub.go
git commit -m "Add NetworkClient interface and StubClient for network domain"
```

---

### Task 3: Implement and test devices commands

**Files:**
- Create: `internal/cli/network/network_test.go` (devices tests)
- Rewrite: `internal/cli/network/network.go` (full file, devices commands wired)

- [ ] **Step 1: Write failing tests for devices commands**

Create `internal/cli/network/network_test.go`:

```go
package network

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	gen "github.com/bwilczynski/hlctl/internal/network"
)

func TestDevicesCmd_tableOutput(t *testing.T) {
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

	cmd := newDevicesCmd(stub)
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

func TestDevicesCmd_apiError(t *testing.T) {
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

	cmd := newDevicesCmd(stub)
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

func TestDeviceCmd_accessPoint(t *testing.T) {
	numClients := 5
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkDeviceDetail{
				Id:              "unifi.ap-living-room",
				Name:            "AP Living Room",
				Mac:             "aa:bb:cc:dd:ee:ff",
				Ip:              "192.168.1.3",
				Type:            gen.AccessPoint,
				Status:          gen.Connected,
				NumClients:      &numClients,
				Model:           "U6-Lite",
				FirmwareVersion: "6.6.77.14522",
				Uptime:          86400,
			}), nil
		},
	}

	cmd := newDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.ap-living-room"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"unifi.ap-living-room", "U6-Lite", "6.6.77.14522",
		"CLIENTS", "5", "1d 0h 0m 0s",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestDeviceCmd_gateway_noClientsRow(t *testing.T) {
	stub := &StubClient{
		GetNetworkDeviceFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkDeviceDetail{
				Id:              "unifi.usg",
				Name:            "USG",
				Mac:             "aa:bb:cc:dd:00:01",
				Ip:              "192.168.1.1",
				Type:            gen.Gateway,
				Status:          gen.Connected,
				Model:           "USG-3P",
				FirmwareVersion: "4.4.57",
				Uptime:          3600,
			}), nil
		},
	}

	cmd := newDeviceCmd(stub)
	cmd.SetArgs([]string{"unifi.usg"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "CLIENTS") {
		t.Errorf("expected no CLIENTS row for gateway, got:\n%s", out)
	}
	if !strings.Contains(out, "1h 0m 0s") {
		t.Errorf("expected formatted uptime in output, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```
go test ./internal/cli/network/... -v
```

Expected: FAIL — `newDevicesCmd` / `newDeviceCmd` do not accept a `NetworkClient` parameter yet.

- [ ] **Step 3: Rewrite network.go with devices commands**

Replace entire `internal/cli/network/network.go`:

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

	cmd.AddCommand(newDevicesCmd(nil))
	cmd.AddCommand(newDeviceCmd(nil))
	cmd.AddCommand(newClientsCmd(nil))
	cmd.AddCommand(newClientCmd(nil))
	return cmd
}

func buildClient() (NetworkClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewNetworkClient(httpClient, apiURL)
}

func newDevicesCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "devices",
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

func newDeviceCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "device <device-id>",
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

func newClientsCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "clients",
		Short: "List connected network clients",
		RunE: func(cmd *cobra.Command, args []string) error {
			panic("not yet implemented")
		},
	}
}

func newClientCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "client <client-id>",
		Short: "Show network client details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			panic("not yet implemented")
		},
	}
}
```

- [ ] **Step 4: Run the devices tests to confirm they pass**

```
go test ./internal/cli/network/... -run "TestDevices|TestDevice" -v
```

Expected: all four tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/network/network.go internal/cli/network/network_test.go
git commit -m "Implement network devices commands with real API calls"
```

---

### Task 4: Implement and test clients commands

**Files:**
- Modify: `internal/cli/network/network_test.go` (add clients tests)
- Modify: `internal/cli/network/network.go` (implement newClientsCmd + newClientCmd)

- [ ] **Step 1: Add failing tests for clients commands**

Append to `internal/cli/network/network_test.go`:

```go
func TestClientsCmd_tableOutput(t *testing.T) {
	ip1 := "192.168.1.101"
	ip2 := "192.168.1.10"
	stub := &StubClient{
		ListNetworkClientsFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.NetworkClientList{
				Items: []gen.NetworkClient{
					{
						Id:             "unifi.macbook-pro-3c",
						Name:           "MacBook Pro",
						Mac:            "3c:22:fb:09:aa:b1",
						Ip:             &ip1,
						ConnectionType: "wireless",
					},
					{
						Id:             "unifi.nas-1-68",
						Name:           "nas-1",
						Mac:            "68:d7:9a:12:bb:c2",
						Ip:             &ip2,
						ConnectionType: "wired",
					},
				},
			}), nil
		},
	}

	cmd := newClientsCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"unifi.macbook-pro-3c", "MacBook Pro", "wireless",
		"unifi.nas-1-68", "nas-1", "wired",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestClientsCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListNetworkClientsFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}

	cmd := newClientsCmd(stub)
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

func TestClientCmd_wireless(t *testing.T) {
	ip := "192.168.1.101"
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.WirelessNetworkClientDetail{
				ConnectionType: gen.Wireless,
				Id:             "unifi.macbook-pro-3c",
				Name:           "MacBook Pro",
				Mac:            "3c:22:fb:09:aa:b1",
				Ip:             &ip,
				Ssid:           "HomeNetwork",
				SignalStrength:  -62,
				Uptime:         7200,
			}), nil
		},
	}

	cmd := newClientCmd(stub)
	cmd.SetArgs([]string{"unifi.macbook-pro-3c"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"HomeNetwork", "-62 dBm", "2h 0m 0s", "wireless"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"SWITCH", "SWITCH PORT"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected no %q row for wireless client, got:\n%s", absent, out)
		}
	}
}

func TestClientCmd_wired(t *testing.T) {
	ip := "192.168.1.10"
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.WiredNetworkClientDetail{
				ConnectionType: gen.WiredNetworkClientDetailConnectionTypeWired,
				Id:             "unifi.nas-1-68",
				Name:           "nas-1",
				Mac:            "68:d7:9a:12:bb:c2",
				Ip:             &ip,
				SwitchName:     "Switch Living Room",
				SwitchPort:     8,
				Uptime:         604800,
			}), nil
		},
	}

	cmd := newClientCmd(stub)
	cmd.SetArgs([]string{"unifi.nas-1-68"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Switch Living Room", "8", "7d 0h 0m 0s", "wired"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	for _, absent := range []string{"SSID", "SIGNAL"} {
		if strings.Contains(out, absent) {
			t.Errorf("expected no %q row for wired client, got:\n%s", absent, out)
		}
	}
}

func TestClientCmd_notFound(t *testing.T) {
	stub := &StubClient{
		GetNetworkClientFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "client 'unifi.foo' does not exist or is offline",
			}), nil
		},
	}

	cmd := newClientCmd(stub)
	cmd.SetArgs([]string{"unifi.foo"})
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

- [ ] **Step 2: Run tests to confirm failure**

```
go test ./internal/cli/network/... -run "TestClients|TestClient" -v
```

Expected: FAIL — `newClientsCmd` / `newClientCmd` panic with "not yet implemented"

- [ ] **Step 3: Implement newClientsCmd in network.go**

Replace the `newClientsCmd` stub:

```go
func newClientsCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "clients",
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
```

- [ ] **Step 4: Implement newClientCmd in network.go**

Replace the `newClientCmd` stub:

```go
func newClientCmd(client NetworkClient) *cobra.Command {
	return &cobra.Command{
		Use:   "client <client-id>",
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

- [ ] **Step 5: Run all network tests**

```
go test ./internal/cli/network/... -v
```

Expected: all tests PASS

- [ ] **Step 6: Run full test suite**

```
go test ./...
```

Expected: all tests PASS

- [ ] **Step 7: Commit**

```bash
git add internal/cli/network/network.go internal/cli/network/network_test.go
git commit -m "Implement network clients and client commands with real API calls"
```
