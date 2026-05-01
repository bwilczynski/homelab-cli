package output_test

import (
	"testing"

	"github.com/bwilczynski/hlctl/internal/output"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{268435456, "256.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := output.FormatBytes(tt.input)
			if got != tt.expected {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

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
