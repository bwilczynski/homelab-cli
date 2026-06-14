package storage

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	storageapi "github.com/bwilczynski/hlctl/internal/api/storage"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/bwilczynski/hlctl/internal/cli/cmdutil/httpmock"
	"github.com/bwilczynski/hlctl/internal/output"
)

func testHTTPClient(reg *httpmock.Registry) func() (*http.Client, string, error) {
	return func() (*http.Client, string, error) {
		return &http.Client{Transport: reg}, "http://localhost", nil
	}
}

// Layer 1: flag/arg parsing

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

func TestNewGetVolumeCmd_argParsed(t *testing.T) {
	var captured *getVolumeOptions
	cmd := newGetVolumeCmd(cmdutil.TestFactory(t), func(o *getVolumeOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"nas-1.volume1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.ID != "nas-1.volume1" {
		t.Errorf("expected ID=nas-1.volume1, got %q", captured.ID)
	}
}

// Layer 2: business logic

func TestListVolumesRun_tableOutput(t *testing.T) {
	list := storageapi.VolumeList{Items: []storageapi.Volume{{
		Id:         "nas-1.volume1",
		Name:       "volume1",
		Device:     "nas-1",
		RaidType:   "SHR-2",
		Status:     storageapi.Normal,
		TotalBytes: 15_981_977_067_520,
		UsedBytes:  10_132_536_762_777,
		FileSystem: "ext4",
	}}}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/storage/volumes"), httpmock.JSONResponse(list))

	var out bytes.Buffer
	opts := &listVolumesOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	if err := listVolumesRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.volume1", "nas-1", "SHR-2", "normal"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}

func TestListVolumesRun_apiError(t *testing.T) {
	reg := httpmock.NewRegistry()
	reg.Register(
		httpmock.REST("GET", "/storage/volumes"),
		httpmock.StatusJSONResponse(http.StatusUnauthorized, map[string]any{
			"type": "https://homelab.local/problems/unauthorized", "title": "Unauthorized", "status": 401, "detail": "Bearer token missing",
		}),
	)
	var out bytes.Buffer
	opts := &listVolumesOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
	}
	err := listVolumesRun(context.Background(), &out, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' in error, got: %v", err)
	}
	reg.Verify(t)
}

func TestGetVolumeRun_tableOutput(t *testing.T) {
	detail := storageapi.VolumeDetail{
		Id:         "nas-1.volume1",
		Name:       "volume1",
		Device:     "nas-1",
		RaidType:   "SHR-2",
		Status:     storageapi.Normal,
		PoolStatus: storageapi.Normal,
		MountPath:  "/volume1",
		FileSystem: "ext4",
		TotalBytes: 15_981_977_067_520,
		UsedBytes:  10_132_536_762_777,
	}
	reg := httpmock.NewRegistry()
	reg.Register(httpmock.REST("GET", "/storage/volumes/*"), httpmock.JSONResponse(detail))

	var out bytes.Buffer
	opts := &getVolumeOptions{
		IO:         &cmdutil.IOStreams{Out: &out, ErrOut: &out},
		HTTPClient: testHTTPClient(reg),
		Output:     func() output.Format { return output.FormatTable },
		ID:         "nas-1.volume1",
	}
	if err := getVolumeRun(context.Background(), &out, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"nas-1.volume1", "volume1", "SHR-2", "/volume1"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, out.String())
		}
	}
	reg.Verify(t)
}
