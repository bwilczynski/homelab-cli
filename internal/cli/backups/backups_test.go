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
