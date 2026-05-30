package cmdutil_test

import (
	"testing"

	"github.com/bwilczynski/hlctl/internal/cli/cmdutil"
	"github.com/spf13/cobra"
)

func TestDeviceFlag_registersFlagAndReturnsPointer(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	device := cmdutil.DeviceFlag(cmd)

	if device == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *device != "" {
		t.Errorf("expected empty default, got %q", *device)
	}

	if err := cmd.Flags().Set("device", "nas-1"); err != nil {
		t.Fatalf("set device: %v", err)
	}
	if *device != "nas-1" {
		t.Errorf("expected pointer to track flag value, got %q", *device)
	}
}

func TestDeviceFlag_helpText(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmdutil.DeviceFlag(cmd)

	f := cmd.Flags().Lookup("device")
	if f == nil {
		t.Fatal("expected --device flag registered")
	}
	if f.Usage != "Filter by device ID" {
		t.Errorf("unexpected usage: %q", f.Usage)
	}
}
