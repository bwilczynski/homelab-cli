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
						Status:     gen.Normal,
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
				Status:     gen.Normal,
				PoolStatus: gen.Normal,
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
				Status:     gen.Normal,
				PoolStatus: gen.Normal,
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
