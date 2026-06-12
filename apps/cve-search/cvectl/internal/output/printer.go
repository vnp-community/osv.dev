// Package output handles formatted output for cvectl commands.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"
)

// Format is the output format type.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
)

// Printer writes formatted output to stdout.
type Printer struct {
	format Format
	w      io.Writer
}

// New creates a Printer with the given format.
func New(format string) *Printer {
	f := Format(format)
	if f != FormatTable && f != FormatJSON {
		f = FormatTable
	}
	return &Printer{format: f, w: os.Stdout}
}

// PrintJSON prints any value as indented JSON.
func (p *Printer) PrintJSON(v any) {
	enc := json.NewEncoder(p.w)
	enc.SetIndent("", "  ")
	enc.Encode(v) //nolint:errcheck
}

// PrintTable prints rows as a tab-aligned table.
// headers is the column header row; rows is the data.
func (p *Printer) PrintTable(headers []string, rows [][]string) {
	if p.format == FormatJSON {
		// Convert to array of objects
		var objects []map[string]string
		for _, row := range rows {
			obj := make(map[string]string, len(headers))
			for i, h := range headers {
				if i < len(row) {
					obj[h] = row[i]
				}
			}
			objects = append(objects, obj)
		}
		p.PrintJSON(objects)
		return
	}

	tw := tabwriter.NewWriter(p.w, 0, 0, 2, ' ', 0)
	// Header
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprint(tw, h)
	}
	fmt.Fprintln(tw)
	// Separator
	for i := range headers {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		for range len(headers[i]) {
			fmt.Fprint(tw, "-")
		}
	}
	fmt.Fprintln(tw)
	// Data rows
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, cell)
		}
		fmt.Fprintln(tw)
	}
	tw.Flush()
}

// PrintSuccess prints a success message.
func (p *Printer) PrintSuccess(msg string) {
	if p.format == FormatJSON {
		fmt.Fprintf(p.w, `{"status":"ok","message":%q}`+"\n", msg)
		return
	}
	fmt.Fprintln(p.w, "✓ "+msg)
}

// PrintError prints an error message.
func PrintError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}

// FormatTime formats a time value for display.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04")
}

// FormatBool formats a bool for display.
func FormatBool(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
