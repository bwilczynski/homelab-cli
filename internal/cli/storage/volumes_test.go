package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	gen "github.com/bwilczynski/hlctl/internal/storage"
)

func okVolumesResp(list gen.VolumeList) *gen.ListStorageVolumesResponse {
	b, _ := json.Marshal(list)
	return &gen.ListStorageVolumesResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &list}
}

func errVolumesResp(status int, body map[string]any) *gen.ListStorageVolumesResponse {
	b, _ := json.Marshal(body)
	return &gen.ListStorageVolumesResponse{HTTPResponse: &http.Response{StatusCode: status}, Body: b}
}

func okVolumeResp(data gen.VolumeDetail) *gen.GetStorageVolumeResponse {
	b, _ := json.Marshal(data)
	return &gen.GetStorageVolumeResponse{HTTPResponse: &http.Response{StatusCode: http.StatusOK}, Body: b, JSON200: &data}
}

func TestListVolumesCmd_tableOutput(t *testing.T) {
	list := gen.VolumeList{
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
	}
	stub := &StubClient{
		ListStorageVolumesWithResponseFunc: func(_ context.Context, _ *gen.ListStorageVolumesParams, _ ...gen.RequestEditorFn) (*gen.ListStorageVolumesResponse, error) {
			return okVolumesResp(list), nil
		},
	}

	cmd := newListVolumesCmd()
	cmdutil.SetClient[StorageClient](cmd, stub)
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

func TestListVolumesCmd_apiError(t *testing.T) {
	stub := &StubClient{
		ListStorageVolumesWithResponseFunc: func(_ context.Context, _ *gen.ListStorageVolumesParams, _ ...gen.RequestEditorFn) (*gen.ListStorageVolumesResponse, error) {
			return errVolumesResp(http.StatusUnauthorized, map[string]any{
				"type":   "https://homelab.local/problems/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Bearer token missing",
			}), nil
		},
	}
	cmd := newListVolumesCmd()
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

func TestGetVolumeCmd_tableOutput(t *testing.T) {
	detail := gen.VolumeDetail{
		Id:         "nas-1.volume1",
		Name:       "volume1",
		Device:     "nas-1",
		RaidType:   "SHR-2",
		Status:     gen.Normal,
		PoolStatus: gen.Normal,
		MountPath:  "/volume1",
		FileSystem: "ext4",
		TotalBytes: 15_981_977_067_520,
		UsedBytes:  10_132_536_762_777,
	}
	stub := &StubClient{
		GetStorageVolumeWithResponseFunc: func(_ context.Context, _ string, _ ...gen.RequestEditorFn) (*gen.GetStorageVolumeResponse, error) {
			return okVolumeResp(detail), nil
		},
	}

	cmd := newGetVolumeCmd()
	cmdutil.SetClient[StorageClient](cmd, stub)
	cmd.SetArgs([]string{"nas-1.volume1"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"nas-1.volume1", "volume1", "SHR-2", "/volume1"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
