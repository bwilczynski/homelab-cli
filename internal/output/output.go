package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
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
		return "10M"
	case "fe":
		return "100M"
	case "gbe1":
		return "1GbE"
	case "gbe2_5":
		return "2.5GbE"
	case "gbe5":
		return "5GbE"
	case "gbe10":
		return "10GbE"
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
