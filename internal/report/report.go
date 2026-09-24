// Package report renders a model.Report for humans (concise terminal output)
// and for machines (stable JSON). Renderers only read the report; they never
// touch the database, so JSON serialization is fully unit-testable.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// Options controls terminal rendering.
type Options struct {
	NoColor   bool
	HidePass  bool
	MaxDetail bool
}

// Human writes the concise terminal view of rep.
func Human(w io.Writer, rep *model.Report, opts Options) error {
	switch rep.Command {
	case "sessions", "locks", "replication", "databases", "config", "indexes":
		renderTableCommand(w, rep, opts)
	case "diagnose", "health":
		renderHealth(w, rep, opts)
	default:
		fmt.Fprintf(w, "dbakit %s\n", rep.Command)
	}
	return nil
}

// JSON writes the stable machine-readable report as indented JSON.
func JSON(w io.Writer, rep *model.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// healthHeader is implemented by every data wrapper that carries the shared
// connectivity/health header card (health and diagnose).
type healthHeader interface {
	connectivity() model.Connectivity
	health() model.Health
}

// renderHealth prints the header card followed by the Findings section.
func renderHealth(w io.Writer, rep *model.Report, opts Options) {
	c := ansi(opts.NoColor)
	fmt.Fprintln(w, c.Bold("DBA-Toolkit PostgreSQL Health Check"))
	fmt.Fprintln(w)

	data, _ := rep.Data.(healthHeader)
	conn := data.connectivity()
	h := data.health()
	if !conn.Connected {
		fmt.Fprintf(w, "%-14s %s\n", "Connection", c.Red("FAIL"))
		fmt.Fprintf(w, "%-14s %s\n", "Error", conn.Error)
		fmt.Fprintln(w)
	} else {
		fmt.Fprintf(w, "%-14s %s\n", "Connection", c.Green("PASS"))
		fmt.Fprintf(w, "%-14s %s\n", "PostgreSQL", conn.Version)
		fmt.Fprintf(w, "%-14s %s\n", "Database", conn.Database)
		fmt.Fprintf(w, "%-14s %s\n", "Latency", fmtDur(conn.LatencySeconds))
		if h.UptimeSeconds > 0 {
			fmt.Fprintf(w, "%-14s %s\n", "Uptime", fmtDur(h.UptimeSeconds))
		}
		if h.MaxConnections > 0 {
			fmt.Fprintf(w, "%-14s %d / %d (%.1f%%)\n", "Connections",
				h.TotalConnections, h.MaxConnections, h.ConnectionUsage)
		}
		if h.DatabaseSizeBytes > 0 {
			fmt.Fprintf(w, "%-14s %s\n", "Size", humanBytes(h.DatabaseSizeBytes))
		}
		fmt.Fprintln(w)
	}
	renderFindings(w, rep, c, opts)
}

// renderFindings prints the Findings section with severity tags.
func renderFindings(w io.Writer, rep *model.Report, c colors, opts Options) {
	fmt.Fprintln(w, c.Bold("Findings"))
	fmt.Fprintln(w, c.Gray("--------"))
	fmt.Fprintln(w)

	printed := 0
	for _, f := range rep.Findings {
		if f.Severity == model.SeverityPass && opts.HidePass {
			continue
		}
		printed++
		tag := c.tag(f.Severity)
		fmt.Fprintf(w, "%s %s\n", tag, c.Bold(f.Title))
		if f.Evidence != "" && f.Severity.Rank() <= model.SeverityInfo.Rank() {
			if f.Severity == model.SeverityInfo {
				fmt.Fprintf(w, "    %s\n", strings.ReplaceAll(f.Evidence, "\n", " · "))
			} else {
				for _, line := range strings.Split(f.Evidence, "\n") {
					fmt.Fprintf(w, "    %s\n", line)
				}
			}
		}
		if f.Summary != "" && f.Severity == model.SeverityInfo && f.Evidence == "" {
			fmt.Fprintf(w, "    %s\n", f.Summary)
		}
		if f.Recommendation != "" {
			fmt.Fprintf(w, "    %s%s%s\n", c.Cyan("→ "), f.Recommendation, c.eReset)
		}
		fmt.Fprintln(w)
	}
	if printed == 0 {
		fmt.Fprintln(w, "No findings.")
		fmt.Fprintln(w)
	}
}
