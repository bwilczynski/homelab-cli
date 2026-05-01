# Missing Domains Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement real API calls for the three stubbed domains: `backups`, `storage`, and `system`.

**Architecture:** Each domain follows the same pattern as `containers` and `network`: a narrow `<Domain>Client` interface in `client.go`, a `StubClient` in `stub.go` for tests, and the commands refactored from hardcoded stubs to real API calls in the existing `<domain>.go`. Tests use `StubClient` with injected function fields.

**Tech Stack:** Go, Cobra, oapi-codegen generated clients in `internal/{backups,storage,system}/api.gen.go`

---

## File Map

**Backups:**
- Create: `internal/cli/backups/client.go`
- Create: `internal/cli/backups/stub.go`
- Create: `internal/cli/backups/backups_test.go`
- Modify: `internal/cli/backups/backups.go`

**Storage:**
- Create: `internal/cli/storage/client.go`
- Create: `internal/cli/storage/stub.go`
- Create: `internal/cli/storage/storage_test.go`
- Modify: `internal/cli/storage/storage.go`

**System:**
- Create: `internal/cli/system/client.go`
- Create: `internal/cli/system/stub.go`
- Create: `internal/cli/system/system_test.go`
- Modify: `internal/cli/system/system.go`

---

## Task 1: Backups — client.go and stub.go

**Files:**
- Create: `internal/cli/backups/client.go`
- Create: `internal/cli/backups/stub.go`

- [ ] **Step 1: Create client.go**

```go
// internal/cli/backups/client.go
package backups

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/backups"
)

// BackupsClient is the interface used by backups commands.
type BackupsClient interface {
	ListBackupTasks(ctx context.Context, params *gen.ListBackupTasksParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetBackupTask(ctx context.Context, taskId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

// NewBackupsClient constructs a BackupsClient backed by the real API.
func NewBackupsClient(httpClient *http.Client, apiURL string) (BackupsClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
```

- [ ] **Step 2: Create stub.go**

```go
// internal/cli/backups/stub.go
package backups

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gen "github.com/bwilczynski/hlctl/internal/backups"
)

// StubClient is a BackupsClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListBackupTasksFunc func(ctx context.Context, params *gen.ListBackupTasksParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetBackupTaskFunc   func(ctx context.Context, taskId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func (s *StubClient) ListBackupTasks(ctx context.Context, params *gen.ListBackupTasksParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListBackupTasksFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetBackupTask(ctx context.Context, taskId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetBackupTaskFunc(ctx, taskId, reqEditors...)
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

- [ ] **Step 3: Verify it compiles**

```bash
go build ./internal/cli/backups/...
```

Expected: no output (clean compile)

---

## Task 2: Backups — failing tests

**Files:**
- Create: `internal/cli/backups/backups_test.go`

- [ ] **Step 1: Create backups_test.go**

```go
// internal/cli/backups/backups_test.go
package backups

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	gen "github.com/bwilczynski/hlctl/internal/backups"
)

func TestTasksCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListBackupTasksFunc: func(_ context.Context, _ *gen.ListBackupTasksParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
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

	cmd := newTasksCmd(stub)
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

func TestTasksCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListBackupTasksFunc: func(_ context.Context, _ *gen.ListBackupTasksParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}

	cmd := newTasksCmd(stub)
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

func TestTaskCmd_withDates(t *testing.T) {
	lastRun := time.Date(2026, 4, 30, 3, 0, 0, 0, time.UTC)
	nextRun := time.Date(2026, 5, 1, 3, 0, 0, 0, time.UTC)
	stub := &StubClient{
		GetBackupTaskFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
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

	cmd := newTaskCmd(stub)
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

func TestTaskCmd_noDates(t *testing.T) {
	stub := &StubClient{
		GetBackupTaskFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.BackupTaskDetail{
				Id:         "nas-1.daily-backup",
				Name:       "Daily Backup",
				Device:     "nas-1",
				Status:     gen.Running,
				LastResult: gen.Unknown,
				Type:       "hyperBackup",
			}), nil
		},
	}

	cmd := newTaskCmd(stub)
	cmd.SetArgs([]string{"nas-1.daily-backup"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "LAST RUN") {
		t.Errorf("expected no LAST RUN row when LastRunAt is nil, got:\n%s", out)
	}
}

func TestTaskCmd_apiError(t *testing.T) {
	stub := &StubClient{
		GetBackupTaskFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "task 'nas-1.foo' not found",
			}), nil
		},
	}

	cmd := newTaskCmd(stub)
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

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/cli/backups/... 2>&1 | head -20
```

Expected: FAIL — `newTasksCmd` and `newTaskCmd` do not accept a client argument yet

---

## Task 3: Backups — implement real commands

**Files:**
- Modify: `internal/cli/backups/backups.go`

- [ ] **Step 1: Replace backups.go with real implementation**

```go
// internal/cli/backups/backups.go
package backups

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/backups"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backups",
		Short: "Backup tasks and history",
	}

	cmd.AddCommand(newTasksCmd(nil))
	cmd.AddCommand(newTaskCmd(nil))
	return cmd
}

func buildClient() (BackupsClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewBackupsClient(httpClient, apiURL)
}

func newTasksCmd(client BackupsClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "List backup tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client
			if c == nil {
				var err error
				c, err = buildClient()
				if err != nil {
					return err
				}
			}

			params := &gen.ListBackupTasksParams{}
			if device != "" {
				params.Device = &device
			}

			resp, err := c.ListBackupTasks(context.Background(), params)
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

func newTaskCmd(client BackupsClient) *cobra.Command {
	return &cobra.Command{
		Use:   "task <task-id>",
		Short: "Show backup task details",
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

			resp, err := c.GetBackupTask(context.Background(), args[0])
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
				rows = append(rows, []string{"LAST RUN", detail.LastRunAt.Format(time.RFC3339)})
			}
			if detail.NextRunAt != nil {
				rows = append(rows, []string{"NEXT RUN", detail.NextRunAt.Format(time.RFC3339)})
			}
			return output.Print(cmd.OutOrStdout(), flags.GetOutputFormat(), detail, headers, rows)
		},
	}
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/cli/backups/... -v 2>&1
```

Expected: all 5 tests PASS

- [ ] **Step 3: Commit**

```bash
git add internal/cli/backups/ && git commit -m "feat(backups): implement real API calls with tests"
```

---

## Task 4: Storage — client.go and stub.go

**Files:**
- Create: `internal/cli/storage/client.go`
- Create: `internal/cli/storage/stub.go`

- [ ] **Step 1: Create client.go**

```go
// internal/cli/storage/client.go
package storage

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/storage"
)

// StorageClient is the interface used by storage commands.
type StorageClient interface {
	ListStorageVolumes(ctx context.Context, params *gen.ListStorageVolumesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetStorageVolume(ctx context.Context, volumeId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

// NewStorageClient constructs a StorageClient backed by the real API.
func NewStorageClient(httpClient *http.Client, apiURL string) (StorageClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
```

- [ ] **Step 2: Create stub.go**

```go
// internal/cli/storage/stub.go
package storage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gen "github.com/bwilczynski/hlctl/internal/storage"
)

// StubClient is a StorageClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	ListStorageVolumesFunc func(ctx context.Context, params *gen.ListStorageVolumesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetStorageVolumeFunc   func(ctx context.Context, volumeId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func (s *StubClient) ListStorageVolumes(ctx context.Context, params *gen.ListStorageVolumesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListStorageVolumesFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetStorageVolume(ctx context.Context, volumeId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetStorageVolumeFunc(ctx, volumeId, reqEditors...)
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

- [ ] **Step 3: Verify it compiles**

```bash
go build ./internal/cli/storage/...
```

Expected: no output (clean compile)

---

## Task 5: Storage — failing tests

**Files:**
- Create: `internal/cli/storage/storage_test.go`

- [ ] **Step 1: Create storage_test.go**

```go
// internal/cli/storage/storage_test.go
package storage

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	gen "github.com/bwilczynski/hlctl/internal/storage"
)

func TestVolumesCmd_tableOutput(t *testing.T) {
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

	cmd := newVolumesCmd(stub)
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

func TestVolumesCmd_apiError(t *testing.T) {
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

	cmd := newVolumesCmd(stub)
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

func TestVolumeCmd_withDisks(t *testing.T) {
	stub := &StubClient{
		GetStorageVolumeFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.VolumeDetail{
				Id:         "nas-1.volume1",
				Name:       "volume1",
				Device:     "nas-1",
				FileSystem: "ext4",
				RaidType:   "SHR-2",
				Status:     gen.VolumeStatusNormal,
				PoolStatus: gen.VolumeStatusNormal,
				MountPath:  "/volume1",
				TotalBytes: 15_981_977_067_520,
				UsedBytes:  10_132_536_762_777,
				Disks: []gen.VolumeDisk{
					{
						Id:                 "disk1",
						Model:              "WDC WD40EFAX",
						Status:             gen.DiskStatusNormal,
						TemperatureCelsius: 36,
						TotalBytes:         4_000_787_030_016,
					},
				},
			}), nil
		},
	}

	cmd := newVolumeCmd(stub)
	cmd.SetArgs([]string{"nas-1.volume1"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"nas-1.volume1", "ext4", "SHR-2", "/volume1",
		"DISKS", "disk1", "WDC WD40EFAX", "36",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestVolumeCmd_noDisks(t *testing.T) {
	stub := &StubClient{
		GetStorageVolumeFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.VolumeDetail{
				Id:         "nas-1.volume1",
				Name:       "volume1",
				Device:     "nas-1",
				FileSystem: "ext4",
				RaidType:   "basic",
				Status:     gen.VolumeStatusNormal,
				PoolStatus: gen.VolumeStatusNormal,
				MountPath:  "/volume1",
				TotalBytes: 1_000_000_000,
				UsedBytes:  500_000_000,
				Disks:      []gen.VolumeDisk{},
			}), nil
		},
	}

	cmd := newVolumeCmd(stub)
	cmd.SetArgs([]string{"nas-1.volume1"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "DISKS") {
		t.Errorf("expected no DISKS section when disks are empty, got:\n%s", out)
	}
}

func TestVolumeCmd_apiError(t *testing.T) {
	stub := &StubClient{
		GetStorageVolumeFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "volume 'nas-1.foo' not found",
			}), nil
		},
	}

	cmd := newVolumeCmd(stub)
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

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/cli/storage/... 2>&1 | head -20
```

Expected: FAIL — `newVolumesCmd` and `newVolumeCmd` do not accept a client argument yet

---

## Task 6: Storage — implement real commands

**Files:**
- Modify: `internal/cli/storage/storage.go`

- [ ] **Step 1: Replace storage.go with real implementation**

```go
// internal/cli/storage/storage.go
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/storage"
	"github.com/bwilczynski/hlctl/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "NAS storage volumes",
	}

	cmd.AddCommand(newVolumesCmd(nil))
	cmd.AddCommand(newVolumeCmd(nil))
	return cmd
}

func buildClient() (StorageClient, error) {
	httpClient, apiURL, err := apiclient.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	return NewStorageClient(httpClient, apiURL)
}

func newVolumesCmd(client StorageClient) *cobra.Command {
	var device string

	cmd := &cobra.Command{
		Use:   "volumes",
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

func newVolumeCmd(client StorageClient) *cobra.Command {
	return &cobra.Command{
		Use:   "volume <volume-id>",
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
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/cli/storage/... -v 2>&1
```

Expected: all 5 tests PASS

- [ ] **Step 3: Commit**

```bash
git add internal/cli/storage/ && git commit -m "feat(storage): implement real API calls with tests"
```

---

## Task 7: System — client.go and stub.go

**Files:**
- Create: `internal/cli/system/client.go`
- Create: `internal/cli/system/stub.go`

- [ ] **Step 1: Create client.go**

```go
// internal/cli/system/client.go
package system

import (
	"context"
	"net/http"

	gen "github.com/bwilczynski/hlctl/internal/system"
)

// SystemClient is the interface used by system commands.
type SystemClient interface {
	GetSystemHealth(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSystemInfo(ctx context.Context, params *gen.ListSystemInfoParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSystemUtilization(ctx context.Context, params *gen.ListSystemUtilizationParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSystemUpdates(ctx context.Context, params *gen.ListSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetSystemUpdate(ctx context.Context, updateId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	CheckSystemUpdates(ctx context.Context, params *gen.CheckSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

// NewSystemClient constructs a SystemClient backed by the real API.
func NewSystemClient(httpClient *http.Client, apiURL string) (SystemClient, error) {
	return gen.NewClient(apiURL, gen.WithHTTPClient(httpClient))
}
```

- [ ] **Step 2: Create stub.go**

```go
// internal/cli/system/stub.go
package system

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gen "github.com/bwilczynski/hlctl/internal/system"
)

// StubClient is a SystemClient that delegates each method to a configurable
// function field. Use in tests to inject controlled responses.
// When a function field is nil the method panics — always set the field under test.
type StubClient struct {
	GetSystemHealthFunc       func(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSystemInfoFunc        func(ctx context.Context, params *gen.ListSystemInfoParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSystemUtilizationFunc func(ctx context.Context, params *gen.ListSystemUtilizationParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	ListSystemUpdatesFunc     func(ctx context.Context, params *gen.ListSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	GetSystemUpdateFunc       func(ctx context.Context, updateId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
	CheckSystemUpdatesFunc    func(ctx context.Context, params *gen.CheckSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error)
}

func (s *StubClient) GetSystemHealth(ctx context.Context, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetSystemHealthFunc(ctx, reqEditors...)
}

func (s *StubClient) ListSystemInfo(ctx context.Context, params *gen.ListSystemInfoParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListSystemInfoFunc(ctx, params, reqEditors...)
}

func (s *StubClient) ListSystemUtilization(ctx context.Context, params *gen.ListSystemUtilizationParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListSystemUtilizationFunc(ctx, params, reqEditors...)
}

func (s *StubClient) ListSystemUpdates(ctx context.Context, params *gen.ListSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.ListSystemUpdatesFunc(ctx, params, reqEditors...)
}

func (s *StubClient) GetSystemUpdate(ctx context.Context, updateId string, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.GetSystemUpdateFunc(ctx, updateId, reqEditors...)
}

func (s *StubClient) CheckSystemUpdates(ctx context.Context, params *gen.CheckSystemUpdatesParams, reqEditors ...gen.RequestEditorFn) (*http.Response, error) {
	return s.CheckSystemUpdatesFunc(ctx, params, reqEditors...)
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

- [ ] **Step 3: Verify it compiles**

```bash
go build ./internal/cli/system/...
```

Expected: no output (clean compile)

---

## Task 8: System — failing tests

**Files:**
- Create: `internal/cli/system/system_test.go`

- [ ] **Step 1: Create system_test.go**

```go
// internal/cli/system/system_test.go
package system

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	gen "github.com/bwilczynski/hlctl/internal/system"
)

func TestHealthCmd_tableOutput(t *testing.T) {
	msg := "disk failing"
	stub := &StubClient{
		GetSystemHealthFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.Health{
				Status:    gen.Healthy,
				CheckedAt: time.Now(),
				Components: []gen.ComponentHealth{
					{Name: "nas-1", Status: gen.Healthy},
					{Name: "unifi", Status: gen.Degraded, Message: &msg},
				},
			}), nil
		},
	}

	cmd := newHealthCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1", "healthy", "unifi", "degraded"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestHealthCmd_apiError(t *testing.T) {
	stub := &StubClient{
		GetSystemHealthFunc: func(_ context.Context, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}

	cmd := newHealthCmd(stub)
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

func TestInfoCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListSystemInfoFunc: func(_ context.Context, _ *gen.ListSystemInfoParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.SystemInfoList{
				Items: []gen.SystemInfo{
					{
						Device:        "nas-1",
						Model:         "DS920+",
						Firmware:      "7.2.1-69057",
						RamMb:         4096,
						UptimeSeconds: 3_931_200,
					},
				},
			}), nil
		},
	}

	cmd := newInfoCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1", "DS920+", "7.2.1-69057", "4096 MB"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestUtilizationCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListSystemUtilizationFunc: func(_ context.Context, _ *gen.ListSystemUtilizationParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.SystemUtilizationList{
				Items: []gen.SystemUtilization{
					{
						Device:    "nas-1",
						SampledAt: time.Now(),
						Cpu:       gen.CpuUsage{TotalPercent: 12},
						Memory: gen.MemoryUsage{
							UsedPercent:    68,
							SwapTotalBytes: 2_147_483_648,
							SwapUsedBytes:  0,
						},
					},
				},
			}), nil
		},
	}

	cmd := newUtilizationCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1", "12%", "68%"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestUpdatesCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		ListSystemUpdatesFunc: func(_ context.Context, _ *gen.ListSystemUpdatesParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.SystemUpdateList{
				Items: []gen.SystemUpdate{
					{
						Id:             "nas-1.homeassistant",
						Name:           "homeassistant",
						Device:         "nas-1",
						Type:           gen.Container,
						Status:         gen.UpdateAvailable,
						CurrentVersion: "2024.1.0",
						LatestVersion:  "2024.2.0",
						CheckedAt:      time.Now(),
					},
				},
			}), nil
		},
	}

	cmd := newUpdatesCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.homeassistant", "nas-1", "container", "updateAvailable", "2024.1.0", "2024.2.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestUpdateCmd_containerType(t *testing.T) {
	stub := &StubClient{
		GetSystemUpdateFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.ContainerSystemUpdateDetail{
				Id:             "nas-1.homeassistant",
				Name:           "homeassistant",
				Device:         "nas-1",
				Type:           gen.ContainerSystemUpdateDetailTypeContainer,
				Status:         gen.UpdateAvailable,
				CurrentVersion: "2024.1.0",
				LatestVersion:  "2024.2.0",
				CheckedAt:      time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
				PublishedAt:    time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
				Image:          "ghcr.io/home-assistant/home-assistant",
				Source:         "https://github.com/home-assistant/core",
				ReleaseUrl:     "https://github.com/home-assistant/core/releases/tag/2024.2.0",
			}), nil
		},
	}

	cmd := newUpdateCmd(stub)
	cmd.SetArgs([]string{"nas-1.homeassistant"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"nas-1.homeassistant", "homeassistant", "container",
		"2024.1.0", "2024.2.0",
		"ghcr.io/home-assistant/home-assistant",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestUpdateCmd_apiError(t *testing.T) {
	stub := &StubClient{
		GetSystemUpdateFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "update 'nas-1.foo' not found",
			}), nil
		},
	}

	cmd := newUpdateCmd(stub)
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

func TestCheckUpdatesCmd_tableOutput(t *testing.T) {
	stub := &StubClient{
		CheckSystemUpdatesFunc: func(_ context.Context, _ *gen.CheckSystemUpdatesParams, _ ...gen.RequestEditorFn) (*http.Response, error) {
			return jsonResponse(http.StatusOK, gen.SystemUpdateList{
				Items: []gen.SystemUpdate{
					{
						Id:             "nas-1.homeassistant",
						Name:           "homeassistant",
						Device:         "nas-1",
						Type:           gen.Container,
						Status:         gen.UpdateAvailable,
						CurrentVersion: "2024.1.0",
						LatestVersion:  "2024.2.0",
						CheckedAt:      time.Now(),
					},
				},
			}), nil
		},
	}

	cmd := newCheckUpdatesCmd(stub)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.homeassistant", "updateAvailable"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/cli/system/... 2>&1 | head -20
```

Expected: FAIL — command constructors do not accept a client argument yet

---

## Task 9: System — implement real commands

**Files:**
- Modify: `internal/cli/system/system.go`

- [ ] **Step 1: Replace system.go with real implementation**

```go
// internal/cli/system/system.go
package system

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bwilczynski/hlctl/internal/apiclient"
	"github.com/bwilczynski/hlctl/internal/cli/flags"
	gen "github.com/bwilczynski/hlctl/internal/system"
	"github.com/bwilczynski/hlctl/internal/output"
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
	cmd.AddCommand(newUpdatesCmd(nil))
	cmd.AddCommand(newUpdateCmd(nil))
	cmd.AddCommand(newCheckUpdatesCmd(nil))
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
					fmt.Sprintf("%d MB", info.RamMb),
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

func newUpdatesCmd(client SystemClient) *cobra.Command {
	var status, updateType string

	cmd := &cobra.Command{
		Use:   "updates",
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

func printUpdateList(w interface{ Write([]byte) (int, error) }, list gen.SystemUpdateList) error {
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

func newUpdateCmd(client SystemClient) *cobra.Command {
	return &cobra.Command{
		Use:   "update <update-id>",
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
					{"CHECKED AT", d.CheckedAt.Format(time.RFC3339)},
					{"PUBLISHED AT", d.PublishedAt.Format(time.RFC3339)},
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
		Use:   "check-updates",
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

**Note on `printUpdateList`:** It takes `io.Writer` — use `cmd.OutOrStdout()` which satisfies `io.Writer`. The `w interface{ Write([]byte) (int, error) }` signature is equivalent. Change it to `io.Writer` and add `"io"` to imports:

```go
// Change the signature to:
func printUpdateList(w io.Writer, list gen.SystemUpdateList) error {
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/cli/system/... -v 2>&1
```

Expected: all 8 tests PASS

- [ ] **Step 3: Run all tests to confirm nothing is broken**

```bash
go test ./... 2>&1
```

Expected: all tests PASS (backups, storage, system, network, containers, output, apiclient)

- [ ] **Step 4: Build the binary**

```bash
make build 2>&1
```

Expected: builds `bin/hlctl` with no errors

- [ ] **Step 5: Commit**

```bash
git add internal/cli/system/ && git commit -m "feat(system): implement real API calls with tests"
```
