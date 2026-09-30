package report

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// FormatTopSnapshot formats a single snapshot into a complete terminal screen string.
func FormatTopSnapshot(snap model.TopSnapshot, conn model.Connectivity, findings []model.Finding, noColor bool) string {
	var buf bytes.Buffer
	c := ansi(noColor)

	// Header line
	fmt.Fprintf(&buf, "%s%s\n", c.Bold("DBA-Toolkit PostgreSQL Live Monitor (dbakit top)"), topDbSuffix(conn))
	fmt.Fprintln(&buf, c.Gray(strings.Repeat("-", 78)))

	// Metrics card
	h := snap.Health
	idleConns := h.TotalConnections - h.ActiveConnections
	if idleConns < 0 {
		idleConns = 0
	}

	roleStr := snap.Replication.Role
	if roleStr == "" {
		roleStr = "standalone"
	}

	fmt.Fprintf(&buf, "PostgreSQL: %-8s  Role: %-10s  Database: %-12s  Latency: %.2fms\n",
		conn.Version, roleStr, conn.Database, conn.LatencySeconds*1000)
	fmt.Fprintf(&buf, "Uptime:     %-8s  Database Size: %-12s  Timestamp: %s\n",
		fmtDur(h.UptimeSeconds), humanBytes(h.DatabaseSizeBytes), snap.Timestamp)

	connUsageStr := fmt.Sprintf("%d/%d (%.1f%%) [Active: %d, Idle: %d]",
		h.TotalConnections, h.MaxConnections, h.ConnectionUsage, h.ActiveConnections, idleConns)
	if h.ConnectionUsage >= 90.0 {
		connUsageStr = c.Red(connUsageStr)
	} else if h.ConnectionUsage >= 80.0 {
		connUsageStr = c.Yellow(connUsageStr)
	}
	fmt.Fprintf(&buf, "Connections: %s\n", connUsageStr)

	// Replication details if applicable
	if snap.Replication.ConnectedStandbys > 0 || snap.Replication.Role == "standby" {
		if snap.Replication.Role == "standby" {
			fmt.Fprintf(&buf, "Standby Replay Lag: %s\n", fmtDur(snap.Replication.LocalReplayLagSeconds))
		} else {
			fmt.Fprintf(&buf, "Connected Standbys: %d\n", snap.Replication.ConnectedStandbys)
		}
	}

	// Active Lock Contention
	if len(snap.Locks) > 0 {
		fmt.Fprintf(&buf, "\n%s %s\n", c.Red("Contention:"), fmt.Sprintf("%d blocked lock pair(s) detected", len(snap.Locks)))
		tw := tabwriter.NewWriter(&buf, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "BLOCKED PID\tBLOCKING PID\tDATABASE\tWAIT\tBLOCKED QUERY")
		for _, l := range snap.Locks {
			fmt.Fprintf(tw, "%d\t%d\t%s\t%s\t%s\n",
				l.BlockedPID, l.BlockingPID, l.Database, fmtDur(l.WaitSeconds), oneLine(l.BlockedQuery))
		}
		tw.Flush()
	}

	// Active Diagnostic Alerts
	if len(findings) > 0 {
		hasAlerts := false
		for _, f := range findings {
			if f.Severity != model.SeverityPass {
				hasAlerts = true
				break
			}
		}
		if hasAlerts {
			fmt.Fprintf(&buf, "\n%s\n", c.Bold("Active Diagnostics"))
			for _, f := range findings {
				if f.Severity == model.SeverityPass {
					continue
				}
				fmt.Fprintf(&buf, "  %s %s: %s\n", c.tag(f.Severity), f.ID, f.Summary)
			}
		}
	}

	// Active Sessions
	fmt.Fprintf(&buf, "\n%s (%d)\n", c.Bold("Active Sessions"), len(snap.Sessions))
	if len(snap.Sessions) == 0 {
		fmt.Fprintln(&buf, "  No active sessions.")
	} else {
		tw := tabwriter.NewWriter(&buf, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "PID\tUSER\tDATABASE\tSTATE\tDURATION\tWAIT\tQUERY")
		for _, s := range snap.Sessions {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
				s.PID, s.User, s.Database, s.State, fmtDur(s.DurationSeconds), s.WaitEvent, oneLine(s.Query))
		}
		tw.Flush()
	}

	return buf.String()
}

func topDbSuffix(conn model.Connectivity) string {
	if conn.Database == "" {
		return ""
	}
	return " — " + conn.Database + " (" + conn.Version + ")"
}

func renderTop(w io.Writer, rep *model.Report, data TopData, c colors, opts Options) {
	out := FormatTopSnapshot(data.Snapshot, data.Connectivity, rep.Findings, opts.NoColor)
	fmt.Fprint(w, out)
}
