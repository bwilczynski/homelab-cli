package output

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"reflect"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
)

// Print renders data in the specified format.
// For table format, headers and rows are used.
// For JSON format, data is marshalled directly.
func Print(w io.Writer, format Format, data any, headers []string, rows [][]string) error {
	switch format {
	case FormatJSON:
		return printJSON(w, data)
	default:
		return printTable(w, headers, rows)
	}
}

func printJSON(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func printTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprint(tw, h)
	}
	fmt.Fprintln(tw)

	for _, row := range rows {
		for i, col := range row {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, col)
		}
		fmt.Fprintln(tw)
	}
	return tw.Flush()
}

// formatBinaryUnits is a private helper that formats byte counts using binary units.
// baseUnit is "B" and suffix is either "" or "/s" to create the full unit names.
func formatBinaryUnits(n int64, baseUnit, suffix string) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d %s%s", n, baseUnit, suffix)
	}
	div, exp := int64(unit), 0
	for n := n / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"K", "M", "G", "T", "P"}
	return fmt.Sprintf("%.1f %s%s%s", float64(n)/float64(div), units[exp], baseUnit, suffix)
}

// FormatBytes converts a byte count to a human-readable string using binary units.
func FormatBytes(n int64) string {
	return formatBinaryUnits(n, "B", "")
}

// FormatBytesPerSec converts a byte count per second to a human-readable string using binary units.
func FormatBytesPerSec(n int64) string {
	return formatBinaryUnits(n, "B", "/s")
}

// FormatLinkSpeed converts network speed abbreviations to their full representation.
func FormatLinkSpeed(s string) string {
	switch s {
	case "e":
		return "10 Mbps"
	case "fe":
		return "100 Mbps"
	case "gbe1":
		return "1 GbE"
	case "gbe2_5":
		return "2.5 GbE"
	case "gbe5":
		return "5 GbE"
	case "gbe10":
		return "10 GbE"
	default:
		return s
	}
}

// FormatTime formats a time value in the local timezone using RFC3339 layout,
// which includes the UTC offset (e.g. "2026-04-30T05:00:00+02:00").
func FormatTime(t time.Time) string {
	return t.Local().Format(time.RFC3339)
}

// FormatUptime converts seconds to a human-readable duration string.
// Leading zero segments are skipped; seconds are always included.
// Examples: 86400 → "1d 0h 0m 0s", 7200 → "2h 0m 0s", 45 → "45s".
func FormatUptime(seconds int) string {
	d := seconds / 86400
	seconds %= 86400
	h := seconds / 3600
	seconds %= 3600
	m := seconds / 60
	s := seconds % 60

	type seg struct {
		val  int
		unit string
	}
	segs := []seg{{d, "d"}, {h, "h"}, {m, "m"}, {s, "s"}}
	var parts []string
	for _, sg := range segs {
		if len(parts) > 0 || sg.val > 0 || sg.unit == "s" {
			parts = append(parts, fmt.Sprintf("%d%s", sg.val, sg.unit))
		}
	}
	return strings.Join(parts, " ")
}

// RenderTemplate executes the named template from fsys into w, with a tabwriter
// for column alignment. Call {{ flush }} in the template between independent
// table sections to reset column-width tracking.
//
// The template set is re-parsed on every call because the flush func must close
// over the per-call tabwriter — caching the parsed template would prevent this.
func RenderTemplate(w io.Writer, fsys fs.FS, name string, data any) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	funcMap := template.FuncMap{
		"formatUptime":      FormatUptime,
		"formatBytes":       FormatBytes,
		"formatBytesPerSec": FormatBytesPerSec,
		"formatLinkSpeed":   FormatLinkSpeed,
		"formatTime":        FormatTime,
		"join":              strings.Join,
		"derefStr": func(v any) string {
			if v == nil {
				return ""
			}
			rv := reflect.ValueOf(v)
			if rv.Kind() == reflect.Ptr {
				if rv.IsNil() {
					return ""
				}
				rv = rv.Elem()
			}
			if rv.Kind() == reflect.String {
				return rv.String()
			}
			return fmt.Sprintf("%v", rv.Interface())
		},
		"derefInt": func(v any) int {
			if v == nil {
				return 0
			}
			rv := reflect.ValueOf(v)
			if rv.Kind() == reflect.Ptr {
				if rv.IsNil() {
					return 0
				}
				rv = rv.Elem()
			}
			switch rv.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return int(rv.Int())
			}
			return 0
		},
		"derefFloat": func(v any) float64 {
			if v == nil {
				return 0
			}
			rv := reflect.ValueOf(v)
			if rv.Kind() == reflect.Ptr {
				if rv.IsNil() {
					return 0
				}
				rv = rv.Elem()
			}
			switch rv.Kind() {
			case reflect.Float32, reflect.Float64:
				return rv.Float()
			}
			return 0
		},
		"string": func(v any) string {
			rv := reflect.ValueOf(v)
			if rv.Kind() == reflect.String {
				return rv.String()
			}
			return fmt.Sprintf("%v", v)
		},
		"formatBands": func(bands any) string {
			rv := reflect.ValueOf(bands)
			if rv.Kind() != reflect.Slice {
				return ""
			}
			parts := make([]string, 0, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				elem := rv.Index(i)
				var s string
				if elem.Kind() == reflect.String {
					s = elem.String()
				} else {
					s = fmt.Sprintf("%v", elem.Interface())
				}
				switch s {
				case "band2g":
					parts = append(parts, "2.4 GHz")
				case "band5g":
					parts = append(parts, "5 GHz")
				case "band6g":
					parts = append(parts, "6 GHz")
				default:
					parts = append(parts, s)
				}
			}
			return strings.Join(parts, ", ")
		},
		"dict": func(args ...any) (map[string]any, error) {
			if len(args)%2 != 0 {
				return nil, fmt.Errorf("dict requires an even number of arguments")
			}
			m := make(map[string]any, len(args)/2)
			for i := 0; i < len(args); i += 2 {
				k, ok := args[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				m[k] = args[i+1]
			}
			return m, nil
		},
		"isLast": func(i int, slice any) bool {
			rv := reflect.ValueOf(slice)
			if rv.Kind() != reflect.Slice {
				return false
			}
			return i == rv.Len()-1
		},
		"connector": func(isLast bool) string {
			if isLast {
				return "└── "
			}
			return "├── "
		},
		"childPrefix": func(prefix string, isLast bool) string {
			if isLast {
				return prefix + "    "
			}
			return prefix + "│   "
		},
		"flush": func() (string, error) {
			return "", tw.Flush()
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(fsys, "*.tmpl")
	if err != nil {
		return err
	}

	t := tmpl.Lookup(name)
	if t == nil {
		return fmt.Errorf("template %q not found", name)
	}

	if err := t.Execute(tw, data); err != nil {
		return err
	}
	return tw.Flush()
}
