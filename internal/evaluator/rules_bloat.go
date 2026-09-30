package evaluator

import (
	"fmt"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// EvaluateBloat evaluates user tables for dead tuple accumulation (BLOAT-001),
// autovacuum starvation / overdue maintenance (BLOAT-002), and overall bloat inventory (BLOAT-003).
func EvaluateBloat(rep model.BloatReport, th Thresholds) []model.Finding {
	if len(rep.Tables) == 0 {
		return []model.Finding{
			finding("BLOAT-003", model.SeverityInfo, "Bloat inventory",
				"No user tables found"),
		}
	}

	warnRatio := th.BloatDeadWarnRatio
	if warnRatio <= 0 {
		warnRatio = 20.0
	}
	critRatio := th.BloatDeadCritRatio
	if critRatio <= 0 {
		critRatio = 50.0
	}
	minDead := th.BloatMinDeadTuples
	if minDead <= 0 {
		minDead = 10_000
	}

	out := make([]model.Finding, 0)
	var hasIssue bool
	var totalDead int64
	var totalBytes int64

	for _, t := range rep.Tables {
		totalDead += t.DeadTuples
		totalBytes += t.TotalSizeBytes

		// BLOAT-001: Table dead tuple ratio
		if t.DeadTuples >= minDead {
			if t.DeadTupleRatio >= critRatio {
				hasIssue = true
				out = append(out, finding("BLOAT-001", model.SeverityCritical,
					fmt.Sprintf("Critical table bloat on %s.%s", t.Schema, t.Table),
					fmt.Sprintf("Table %q has %s dead tuples (%.1f%% of total tuples, %s total size)",
						t.Table, fmtNum(t.DeadTuples), t.DeadTupleRatio, fmtBytes(t.TotalSizeBytes)),
					WithEvidence(fmt.Sprintf("Schema: %s\nTable: %s\nLive tuples: %d\nDead tuples: %d\nDead tuple ratio: %.2f%%\nTable size: %d bytes\nTotal size: %d bytes\nLast vacuum: %s\nLast autovacuum: %s",
						t.Schema, t.Table, t.LiveTuples, t.DeadTuples, t.DeadTupleRatio, t.TableSizeBytes, t.TotalSizeBytes, t.LastVacuum, t.LastAutovacuum)),
					WithRecommendation(fmt.Sprintf("Over half the table is dead tuple bloat. Run VACUUM (VERBOSE, ANALYZE) %s.%s to clean dead rows, or use pg_repack to reclaim disk space without locking.",
						quoteIdent(t.Schema), quoteIdent(t.Table))),
					WithMetadata("schema", t.Schema),
					WithMetadata("table", t.Table),
					WithMetadata("dead_tuples", fmt.Sprintf("%d", t.DeadTuples)),
					WithMetadata("dead_tuple_ratio", fmt.Sprintf("%.2f", t.DeadTupleRatio))))
			} else if t.DeadTupleRatio >= warnRatio {
				hasIssue = true
				out = append(out, finding("BLOAT-001", model.SeverityWarning,
					fmt.Sprintf("High table bloat on %s.%s", t.Schema, t.Table),
					fmt.Sprintf("Table %q has %s dead tuples (%.1f%% of total tuples, %s total size)",
						t.Table, fmtNum(t.DeadTuples), t.DeadTupleRatio, fmtBytes(t.TotalSizeBytes)),
					WithEvidence(fmt.Sprintf("Schema: %s\nTable: %s\nLive tuples: %d\nDead tuples: %d\nDead tuple ratio: %.2f%%\nTable size: %d bytes\nTotal size: %d bytes\nLast vacuum: %s\nLast autovacuum: %s",
						t.Schema, t.Table, t.LiveTuples, t.DeadTuples, t.DeadTupleRatio, t.TableSizeBytes, t.TotalSizeBytes, t.LastVacuum, t.LastAutovacuum)),
					WithRecommendation(fmt.Sprintf("Table bloat exceeds recommended threshold. Run VACUUM %s.%s or tune autovacuum_vacuum_scale_factor for this table.",
						quoteIdent(t.Schema), quoteIdent(t.Table))),
					WithMetadata("schema", t.Schema),
					WithMetadata("table", t.Table),
					WithMetadata("dead_tuples", fmt.Sprintf("%d", t.DeadTuples)),
					WithMetadata("dead_tuple_ratio", fmt.Sprintf("%.2f", t.DeadTupleRatio))))
			}
		}

		// BLOAT-002: Autovacuum starvation (table has substantial dead tuples and never autovacuumed or never vacuumed)
		if t.DeadTuples >= minDead && t.LastAutovacuum == "" && t.LastVacuum == "" {
			hasIssue = true
			out = append(out, finding("BLOAT-002", model.SeverityWarning,
				fmt.Sprintf("Autovacuum starvation on %s.%s", t.Schema, t.Table),
				fmt.Sprintf("Table %q has %s dead tuples and has never been vacuumed",
					t.Table, fmtNum(t.DeadTuples)),
				WithEvidence(fmt.Sprintf("Schema: %s\nTable: %s\nDead tuples: %d\nLast vacuum: never\nLast autovacuum: never",
					t.Schema, t.Table, t.DeadTuples)),
				WithRecommendation(fmt.Sprintf("Ensure autovacuum is running and not blocked by long-running transactions. Run manual VACUUM ANALYZE %s.%s.",
					quoteIdent(t.Schema), quoteIdent(t.Table))),
				WithMetadata("schema", t.Schema),
				WithMetadata("table", t.Table),
				WithMetadata("dead_tuples", fmt.Sprintf("%d", t.DeadTuples))))
		}
	}

	// BLOAT-003: Bloat inventory
	if !hasIssue {
		out = append(out, finding("BLOAT-003", model.SeverityPass, "Bloat inventory",
			fmt.Sprintf("All %d inspected user tables have healthy dead tuple ratios (%s total size)",
				len(rep.Tables), fmtBytes(totalBytes)),
			WithEvidence(fmt.Sprintf("Tables inspected: %d\nTotal dead tuples: %d\nTotal size: %d bytes",
				len(rep.Tables), totalDead, totalBytes)),
			WithMetadata("tables_inspected", fmt.Sprintf("%d", len(rep.Tables))),
			WithMetadata("total_dead_tuples", fmt.Sprintf("%d", totalDead)),
			WithMetadata("total_size_bytes", fmt.Sprintf("%d", totalBytes))))
	} else {
		out = append(out, finding("BLOAT-003", model.SeverityInfo, "Bloat inventory",
			fmt.Sprintf("%d user tables inspected (%s total dead tuples, %s total size)",
				len(rep.Tables), fmtNum(totalDead), fmtBytes(totalBytes)),
			WithEvidence(fmt.Sprintf("Tables inspected: %d\nTotal dead tuples: %d\nTotal size: %d bytes",
				len(rep.Tables), totalDead, totalBytes)),
			WithMetadata("tables_inspected", fmt.Sprintf("%d", len(rep.Tables))),
			WithMetadata("total_dead_tuples", fmt.Sprintf("%d", totalDead)),
			WithMetadata("total_size_bytes", fmt.Sprintf("%d", totalBytes))))
	}

	return out
}
