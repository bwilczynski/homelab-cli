package output_test

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"

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

func TestFormatBytesPerSec(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B/s"},
		{500, "500 B/s"},
		{1024, "1.0 KB/s"},
		{125000, "122.1 KB/s"},
		{1048576, "1.0 MB/s"},
		{1073741824, "1.0 GB/s"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := output.FormatBytesPerSec(tt.input)
			if got != tt.expected {
				t.Errorf("FormatBytesPerSec(%d) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatLinkSpeed(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"e", "10 Mbps"},
		{"fe", "100 Mbps"},
		{"gbe1", "1 GbE"},
		{"gbe2_5", "2.5 GbE"},
		{"gbe5", "5 GbE"},
		{"gbe10", "10 GbE"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := output.FormatLinkSpeed(tt.input)
			if got != tt.expected {
				t.Errorf("FormatLinkSpeed(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRenderTemplate_list(t *testing.T) {
	fsys := fstest.MapFS{
		"list.tmpl": &fstest.MapFile{Data: []byte("NAME\tCOUNT\n{{ range .Items }}{{ .Name }}\t{{ .Count }}\n{{ end }}")},
	}
	type row struct {
		Name  string
		Count int
	}
	type data struct{ Items []row }

	var buf bytes.Buffer
	err := output.RenderTemplate(&buf, fsys, "list.tmpl", data{Items: []row{{"foo", 1}, {"bar", 2}}})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"NAME", "COUNT", "foo", "bar"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestRenderTemplate_formatFuncs(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte("{{ formatUptime .Uptime }}\n{{ formatBands .Bands }}\n{{ derefStr .Ptr }}")},
	}
	ptr := "hello"
	type data struct {
		Uptime int
		Bands  []string
		Ptr    *string
	}

	var buf bytes.Buffer
	err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{Uptime: 86400, Bands: []string{"band2g", "band5g"}, Ptr: &ptr})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"1d", "2.4 GHz", "5 GHz", "hello"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestRenderTemplate_unknownTemplate(t *testing.T) {
	fsys := fstest.MapFS{
		"a.tmpl": &fstest.MapFile{Data: []byte("hello")},
	}
	err := output.RenderTemplate(&bytes.Buffer{}, fsys, "missing.tmpl", nil)
	if err == nil {
		t.Fatal("expected error for missing template name")
	}
}

func TestRenderTemplate_flush(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte("A\tB\nfoo\tbar\n{{ flush }}\nC\tD\nbaz\tqux\n")},
	}
	var buf bytes.Buffer
	if err := output.RenderTemplate(&buf, fsys, "t.tmpl", nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"A", "B", "foo", "bar", "C", "D", "baz", "qux"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}
