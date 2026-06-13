package storage

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	storageapi "github.com/bwilczynski/hlctl/internal/api/storage"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

// Layer 1: flag/arg parsing

func TestNewListBackupsCmd_deviceFlag(t *testing.T) {
	var captured *listBackupsOptions
	cmd := newListBackupsCmd(cmdutil.TestFactory(t), func(o *listBackupsOptions) error {
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

func TestNewGetBackupCmd_argParsed(t *testing.T) {
	var captured *getBackupOptions
	cmd := newGetBackupCmd(cmdutil.TestFactory(t), func(o *getBackupOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"nas-1.daily-backup"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.ID != "nas-1.daily-backup" {
		t.Errorf("expected ID=nas-1.daily-backup, got %q", captured.ID)
	}
}

// Layer 2: business logic

func TestListBackupsRun_tableOutput(t *testing.T) {
	list := storageapi.BackupTaskList{
		Items: []storageapi.BackupTask{
			{
				Id:         "nas-1.daily-backup",
				Name:       "Daily Backup",
				Device:     "nas-1",
				Status:     storageapi.Idle,
				LastResult: storageapi.BackupTaskResultSuccess,
				Type:       "hyperBackup",
			},
		},
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/storage/backups"), httpmock.JSONResponse(list))

	var out bytes.Buffer
	opts := &listBackupsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listBackupsRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.daily-backup", "Daily Backup", "nas-1", "idle", "success", "hyperBackup"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestListBackupsRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(
		httpmock.REST("GET", "/storage/backups"),
		httpmock.StatusJSONResponse(http.StatusUnauthorized, map[string]any{
			"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized", "status": 401, "detail": "Bearer token missing",
		}),
	)
	var out bytes.Buffer
	opts := &listBackupsOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	err := listBackupsRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
	reg.Verify(t)
}

func TestGetBackupRun_withDates(t *testing.T) {
	lastRun := time.Date(2026, 4, 30, 3, 0, 0, 0, time.UTC)
	nextRun := time.Date(2026, 5, 1, 3, 0, 0, 0, time.UTC)
	detail := storageapi.BackupTaskDetail{
		Id:         "nas-1.daily-backup",
		Name:       "Daily Backup",
		Device:     "nas-1",
		Status:     storageapi.Idle,
		LastResult: storageapi.BackupTaskResultSuccess,
		Type:       "hyperBackup",
		LastRunAt:  &lastRun,
		NextRunAt:  &nextRun,
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/storage/backups/*"), httpmock.JSONResponse(detail))

	var out bytes.Buffer
	opts := &getBackupOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.daily-backup",
	}
	if err := getBackupRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.daily-backup", "hyperBackup", "LAST RUN", "NEXT RUN", "2026-04-30", "2026-05-01"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetBackupRun_withSizeAndFolders(t *testing.T) {
	size := storageapi.Bytes(10737418240)
	folders := []string{"/volume1/photos", "/volume1/documents"}
	detail := storageapi.BackupTaskDetail{
		Id:         "nas-1.daily-backup",
		Name:       "Daily Backup",
		Device:     "nas-1",
		Status:     storageapi.Idle,
		LastResult: storageapi.BackupTaskResultSuccess,
		Type:       "hyperBackup",
		Size:       &size,
		Folders:    &folders,
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/storage/backups/*"), httpmock.JSONResponse(detail))

	var out bytes.Buffer
	opts := &getBackupOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.daily-backup",
	}
	if err := getBackupRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"SIZE", "10.0 GB", "FOLDERS", "/volume1/photos", "/volume1/documents"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestGetBackupRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(
		httpmock.REST("GET", "/storage/backups/*"),
		httpmock.StatusJSONResponse(http.StatusNotFound, map[string]any{
			"type": "https://homelab.local/problems/not-found", "title": "Not Found", "status": 404, "detail": "backup 'nas-1.foo' not found",
		}),
	)
	var out bytes.Buffer
	opts := &getBackupOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.foo",
	}
	err := getBackupRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected 'Not Found' in error, got: %v", err)
	}
	reg.Verify(t)
}
