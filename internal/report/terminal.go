package report

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// renderTableCommand prints the per-command table views for sessions, locks,
// replication, databases, and config, followed by the Findings section. When
// the command produced no data (e.g. a connection failure), only the findings
// are printed.
func renderTableCommand(w io.Writer, rep *model.Report, opts Options) {
	c := ansi(opts.NoColor)
	switch rep.Command {
	case "sessions":
		if data, ok := rep.Data.(SessionsData); ok {
			fmt.Fprintf(w, "%s%s\n\n", c.Bold("Active sessions"), dbSuffix(rep))
			renderSessions(w, data.Sessions, c)
		}
	case "locks":
		if data, ok := rep.Data.(LocksData); ok {
			fmt.Fprintf(w, "%s%s\n\n", c.Bold("Blocking sessions"), dbSuffix(rep))
			renderLocks(w, data.BlockingPairs, c)
		}
	case "replication":
		if data, ok := rep.Data.(ReplicationData); ok {
			fmt.Fprintf(w, "%s%s\n\n", c.Bold("Replication"), dbSuffix(rep))
			renderReplication(w, data.Replication, c)
		}
	case "databases":
		if data, ok := rep.Data.(DatabasesData); ok {
			fmt.Fprintf(w, "%s%s\n\n", c.Bold("Databases"), dbSuffix(rep))
			renderDatabases(w, data.Databases, c)
		}
	case "config":
		if data, ok := rep.Data.(ConfigData); ok {
			fmt.Fprintf(w, "%s%s\n\n", c.Bold("Configuration"), dbSuffix(rep))
			renderSettings(w, data.Settings, c)
		}
	case "indexes":
		if data, ok := rep.Data.(IndexesData); ok {
			fmt.Fprintf(w, "%s%s\n\n", c.Bold("User indexes"), dbSuffix(rep))
			renderIndexes(w, data.Indexes, c)
		}
	case "xid":
		if data, ok := rep.Data.(XIDData); ok {
			fmt.Fprintf(w, "%s%s\n\n", c.Bold("Transaction ID (XID) & Wraparound"), dbSuffix(rep))
			renderXID(w, data.XID, c)
		}
	}
	if len(rep.Findings) > 0 {
		fmt.Fprintln(w)
		renderFindings(w, rep, c, opts)
	}
}

func dbSuffix(rep *model.Report) string {
	if rep.Database.Name == "" {
		return ""
	}
	return " — " + rep.Database.Name + " (PostgreSQL " + rep.Database.Version + ")"
}

func renderSessions(w io.Writer, sessions []model.Session, c colors) {
	if len(sessions) == 0 {
		fmt.Fprintln(w, "No sessions.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PID\tUSER\tDATABASE\tSTATE\tDURATION\tWAIT\tQUERY")
	for _, s := range sessions {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			s.PID, s.User, s.Database, s.State, fmtDur(s.DurationSeconds), s.WaitEvent, oneLine(s.Query))
	}
	tw.Flush()
}

func renderLocks(w io.Writer, pairs []model.BlockingPair, c colors) {
	if len(pairs) == 0 {
		fmt.Fprintln(w, "No blocking detected.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "BLOCKED PID\tBLOCKING PID\tDATABASE\tWAIT\tBLOCKED STATE\tBLOCKING STATE")
	for _, p := range pairs {
		fmt.Fprintf(tw, "%d\t%d\t%s\t%s\t%s\t%s\n",
			p.BlockedPID, p.BlockingPID, p.Database, fmtDur(p.WaitSeconds), p.BlockedState, p.BlockingState)
	}
	tw.Flush()
}

func renderReplication(w io.Writer, r model.Replication, c colors) {
	fmt.Fprintf(w, "%-14s %s\n", "Role", c.Bold(r.Role))
	fmt.Fprintf(w, "%-14s %d\n", "Standbys", r.ConnectedStandbys)
	if r.Role == "standby" {
		fmt.Fprintf(w, "%-14s %s\n", "Replay lag", fmtDur(r.LocalReplayLagSeconds))
		fmt.Fprintln(w)
		return
	}
	if len(r.Replicas) == 0 {
		fmt.Fprintln(w)
		return
	}
	fmt.Fprintln(w)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "APPLICATION\tSTATE\tSYNC\tREPLAY LAG")
	for _, rep := range r.Replicas {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			rep.ApplicationName, rep.State, rep.SyncState, fmtDur(rep.ReplayLagSeconds))
	}
	tw.Flush()
}

func renderDatabases(w io.Writer, dbs []model.DatabaseInfo, c colors) {
	if len(dbs) == 0 {
		fmt.Fprintln(w, "No databases found.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "DATABASE\tSIZE\tOWNER\tCONNECTIONS")
	for _, d := range dbs {
		size := "<unknown>"
		if !d.SizeUnknown {
			size = humanBytes(d.SizeBytes)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", d.Name, size, d.Owner, d.Connections)
	}
	tw.Flush()
}

func renderSettings(w io.Writer, settings []model.ConfigSetting, c colors) {
	if len(settings) == 0 {
		fmt.Fprintln(w, "No settings matched the allowlist.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SETTING\tVALUE\tSOURCE")
	for _, s := range settings {
		value := s.Value + " " + s.Unit
		fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Name, value, s.Source)
	}
	tw.Flush()
}

func renderIndexes(w io.Writer, indexes []model.IndexInfo, c colors) {
	if len(indexes) == 0 {
		fmt.Fprintln(w, "No user indexes.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SCHEMA\tTABLE\tINDEX\tSIZE\tSCANS\tVALID\tUNIQUE")
	for _, idx := range indexes {
		validStr := "YES"
		if !idx.IsValid {
			validStr = c.Red("NO")
		}
		uniqueStr := "NO"
		if idx.IsUnique {
			uniqueStr = "YES"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			idx.Schema, idx.Table, idx.Index, humanBytes(idx.SizeBytes), idx.Scans, validStr, uniqueStr)
	}
	tw.Flush()
}

func renderXID(w io.Writer, xid model.XIDReport, c colors) {
	if len(xid.Databases) == 0 {
		fmt.Fprintln(w, "No database XID information.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "DATABASE\tAGE\tREMAINING\tWRAPAROUND %\tSTATUS")
	for _, d := range xid.Databases {
		status := "OK"
		if d.Age >= 1500000000 || d.RemainingXIDs < 100000000 {
			status = c.Red("CRITICAL")
		} else if d.Age >= xid.AutovacuumFreezeMaxAge && xid.AutovacuumFreezeMaxAge > 0 {
			status = c.Yellow("WARN (freeze)")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%.1f%%\t%s\n",
			d.Datname, humanInt(d.Age), humanInt(d.RemainingXIDs), d.PercentWraparound, status)
	}
	tw.Flush()

	if len(xid.OldestTables) > 0 {
		fmt.Fprintf(w, "\n%s\n", c.Bold("Oldest tables (freeze horizon):"))
		twTbl := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(twTbl, "SCHEMA\tTABLE\tAGE\tSIZE\tSTATUS")
		for _, t := range xid.OldestTables {
			status := "OK"
			if t.Age >= xid.AutovacuumFreezeMaxAge && xid.AutovacuumFreezeMaxAge > 0 {
				status = c.Yellow("WARN")
			}
			fmt.Fprintf(twTbl, "%s\t%s\t%s\t%s\t%s\n",
				t.Schema, t.Table, humanInt(t.Age), humanBytes(t.SizeBytes), status)
		}
		twTbl.Flush()
	}
}

func oneLine(s string) string {
	if len(s) > 60 {
		s = s[:57] + "..."
	}
	return s
}
