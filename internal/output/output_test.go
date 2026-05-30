package output_test

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"
	"time"

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

func TestRenderTemplate_derefHelpers(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte("{{ derefInt .IntPtr }}\n{{ derefFloat .FloatPtr }}\n{{ derefInt .NilInt }}\n{{ derefFloat .NilFloat }}")},
	}
	i := 42
	f := float32(3.14)
	type data struct {
		IntPtr   *int
		FloatPtr *float32
		NilInt   *int
		NilFloat *float32
	}

	var buf bytes.Buffer
	err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{IntPtr: &i, FloatPtr: &f})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "42") {
		t.Errorf("expected 42 in output, got:\n%s", out)
	}
	if !strings.Contains(out, "3.14") {
		// float32 3.14 may render as 3.1400001 due to precision; just check it has something
		if !strings.Contains(out, "3.1") {
			t.Errorf("expected ~3.14 in output, got:\n%s", out)
		}
	}
	// nil pointers produce 0
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	if strings.TrimSpace(lines[2]) != "0" {
		t.Errorf("expected 0 for nil int, got %q", lines[2])
	}
	if strings.TrimSpace(lines[3]) != "0" {
		t.Errorf("expected 0 for nil float, got %q", lines[3])
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

func TestRenderTemplate_sortedPairs(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte(
			"{{ range sortedPairs .M }}{{ index . 0 }}\t{{ index . 1 }}\n{{ end }}",
		)},
	}
	type data struct{ M map[string]string }
	var buf bytes.Buffer
	err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{M: map[string]string{"b": "2", "a": "1", "c": "3"}})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"a", "1", "b", "2", "c", "3"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Index(out, "a") > strings.Index(out, "b") || strings.Index(out, "b") > strings.Index(out, "c") {
		t.Errorf("expected sorted order a < b < c, got:\n%s", out)
	}
}

func TestRenderTemplate_sortedPairs_nil(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte("{{ if sortedPairs .M }}HAS{{ else }}EMPTY{{ end }}")},
	}
	type data struct{ M *map[string]string }
	var buf bytes.Buffer
	if err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{M: nil}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "EMPTY") {
		t.Errorf("expected EMPTY for nil map, got: %s", buf.String())
	}
}

func TestRenderTemplate_derefStrSlice(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte(`{{ join (derefStrSlice .S) ", " }}`)},
	}
	s := []string{"x", "y", "z"}
	type data struct{ S *[]string }
	var buf bytes.Buffer
	if err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{S: &s}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "x, y, z") {
		t.Errorf("expected 'x, y, z', got: %s", buf.String())
	}
}

func TestRenderTemplate_derefStrSlice_nil(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte(`{{ if derefStrSlice .S }}HAS{{ else }}EMPTY{{ end }}`)},
	}
	type data struct{ S *[]string }
	var buf bytes.Buffer
	if err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{S: nil}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "EMPTY") {
		t.Errorf("expected EMPTY for nil slice, got: %s", buf.String())
	}
}

func TestRenderTemplate_derefTimeAndInt64(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte("{{ formatTime (derefTime .T) }}\n{{ derefInt64 .N }}")},
	}
	ts := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	n := int64(42)
	type data struct {
		T *time.Time
		N *int64
	}
	var buf bytes.Buffer
	if err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{T: &ts, N: &n}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"2026-01-02", "42"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestRenderTemplate_derefInt64_nil(t *testing.T) {
	fsys := fstest.MapFS{
		"t.tmpl": &fstest.MapFile{Data: []byte("{{ derefInt64 .N }}")},
	}
	type data struct{ N *int64 }
	var buf bytes.Buffer
	if err := output.RenderTemplate(&buf, fsys, "t.tmpl", data{N: nil}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "0" {
		t.Errorf("expected 0 for nil int64, got: %s", buf.String())
	}
}
