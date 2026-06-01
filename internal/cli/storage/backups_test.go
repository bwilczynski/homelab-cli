package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	gen "github.com/bwilczynski/hlctl/internal/storage"
)

func okBackupsResp(list gen.BackupTaskList) *gen.ListBackupsResponse {
	b, _ := json.Marshal(list)
	return &gen.ListBackupsResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func errBackupsResp(status int, body map[string]any) *gen.ListBackupsResponse {
	b, _ := json.Marshal(body)
	return &gen.ListBackupsResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func okBackupResp(data gen.BackupTaskDetail) *gen.GetBackupResponse {
	b, _ := json.Marshal(data)
	return &gen.GetBackupResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &data}
}

func errBackupResp(status int, body map[string]any) *gen.GetBackupResponse {
	b, _ := json.Marshal(body)
	return &gen.GetBackupResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func TestListBackupsCmd_tableOutput(t *testing.T) {
	list := gen.BackupTaskList{
		Items: []gen.BackupTask{
			{
				Id:         "nas-1.daily-backup",
				Name:       "Daily Backup",
				Device:     "nas-1",
				Status:     gen.Idle,
				LastResult: gen.BackupTaskResultSuccess,
				Type:       "hyperBackup",
			},
		},
	}
	stub := &StubClient{
		ListBackupsWithResponseFunc: func(_ context.Context, _ *gen.ListBackupsParams, _ ...gen.RequestEditorFn) (*gen.ListBackupsResponse, error) {
			return okBackupsResp(list), nil
		},
	}

	cmd := newListBackupsCmd()
	cmdutil.SetClient[StorageClient](cmd, stub)
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
		ListBackupsWithResponseFunc: func(_ context.Context, _ *gen.ListBackupsParams, _ ...gen.RequestEditorFn) (*gen.ListBackupsResponse, error) {
			return errBackupsResp(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListBackupsCmd()
	cmdutil.SetClient[StorageClient](cmd, stub)
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
	detail := gen.BackupTaskDetail{
		Id:         "nas-1.daily-backup",
		Name:       "Daily Backup",
		Device:     "nas-1",
		Status:     gen.Idle,
		LastResult: gen.BackupTaskResultSuccess,
		Type:       "hyperBackup",
		LastRunAt:  &lastRun,
		NextRunAt:  &nextRun,
	}
	stub := &StubClient{
		GetBackupWithResponseFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*gen.GetBackupResponse, error) {
			return okBackupResp(detail), nil
		},
	}

	cmd := newGetBackupCmd()
	cmdutil.SetClient[StorageClient](cmd, stub)
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
	detail := gen.BackupTaskDetail{
		Id:         "nas-1.daily-backup",
		Name:       "Daily Backup",
		Device:     "nas-1",
		Status:     gen.Idle,
		LastResult: gen.BackupTaskResultSuccess,
		Type:       "hyperBackup",
		Size:       &size,
		Folders:    &folders,
	}
	stub := &StubClient{
		GetBackupWithResponseFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*gen.GetBackupResponse, error) {
			return okBackupResp(detail), nil
		},
	}

	cmd := newGetBackupCmd()
	cmdutil.SetClient[StorageClient](cmd, stub)
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
		GetBackupWithResponseFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*gen.GetBackupResponse, error) {
			return errBackupResp(http.StatusNotFound, map[string]any{
				"type":   "https://homelab.local/problems/not-found",
				"title":  "Not Found",
				"status": 404,
				"detail": "backup 'nas-1.foo' not found",
			}), nil
		},
	}
	cmd := newGetBackupCmd()
	cmdutil.SetClient[StorageClient](cmd, stub)
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
