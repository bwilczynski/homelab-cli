package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
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

// FormatBytes converts a byte count to a human-readable string using binary units.
func FormatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n := n / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), units[exp])
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
