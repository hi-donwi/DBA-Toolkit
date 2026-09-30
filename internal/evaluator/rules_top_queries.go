package evaluator

import (
	"fmt"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// EvaluateTopQueries evaluates pg_stat_statements statistics for slow queries,
// temp file disk spills, and general query performance health.
func EvaluateTopQueries(rep model.TopQueriesReport, th Thresholds) []model.Finding {
	// TOPQ-001: Query statistics extension availability
	if !rep.ExtensionAvailable {
		return []model.Finding{
			finding("TOPQ-001", model.SeverityWarning, "pg_stat_statements not installed",
				"The pg_stat_statements extension is not installed or enabled in this database",
				WithEvidence("pg_extension check for 'pg_stat_statements' returned false"),
				WithRecommendation("Enable query tracking: add 'pg_stat_statements' to shared_preload_libraries in postgresql.conf, restart PostgreSQL, and run: CREATE EXTENSION pg_stat_statements;")),
		}
	}

	if rep.StatementsCount == 0 || len(rep.Queries) == 0 {
		return []model.Finding{
			finding("TOPQ-004", model.SeverityPass, "Top queries inventory",
				"No statement execution data recorded in pg_stat_statements"),
		}
	}

	warnTh := th.MeanQueryWarn
	if warnTh <= 0 {
		warnTh = 500 * time.Millisecond
	}
	critTh := th.MeanQueryCrit
	if critTh <= 0 {
		critTh = 2 * time.Second
	}

	out := make([]model.Finding, 0)
	var hasIssue bool

	for _, q := range rep.Queries {
		meanDur := time.Duration(q.MeanExecTimeMs * float64(time.Millisecond))

		// TOPQ-002: Slow query mean latency
		if meanDur >= critTh {
			hasIssue = true
			out = append(out, finding("TOPQ-002", model.SeverityCritical,
				fmt.Sprintf("Critical slow query (query ID %d)", q.QueryID),
				fmt.Sprintf("Query has mean execution time of %s (%d calls, %s total)",
					fmtDur(q.MeanExecTimeMs/1000.0), q.Calls, fmtDur(q.TotalExecTimeMs/1000.0)),
				WithEvidence(fmt.Sprintf("Query ID: %d\nCalls: %d\nMean time: %.2f ms\nMax time: %.2f ms\nTotal time: %.2f ms\nRows: %d\nQuery: %s",
					q.QueryID, q.Calls, q.MeanExecTimeMs, q.MaxExecTimeMs, q.TotalExecTimeMs, q.Rows, q.Query)),
				WithRecommendation("Analyze the slow query with EXPLAIN (ANALYZE, BUFFERS) to find full table scans, suboptimal joins, or missing indexes."),
				WithMetadata("query_id", fmt.Sprintf("%d", q.QueryID)),
				WithMetadata("calls", fmt.Sprintf("%d", q.Calls)),
				WithMetadata("mean_time_ms", fmt.Sprintf("%.2f", q.MeanExecTimeMs))))
		} else if meanDur >= warnTh {
			hasIssue = true
			out = append(out, finding("TOPQ-002", model.SeverityWarning,
				fmt.Sprintf("Slow query (query ID %d)", q.QueryID),
				fmt.Sprintf("Query has mean execution time of %s (%d calls, %s total)",
					fmtDur(q.MeanExecTimeMs/1000.0), q.Calls, fmtDur(q.TotalExecTimeMs/1000.0)),
				WithEvidence(fmt.Sprintf("Query ID: %d\nCalls: %d\nMean time: %.2f ms\nMax time: %.2f ms\nTotal time: %.2f ms\nRows: %d\nQuery: %s",
					q.QueryID, q.Calls, q.MeanExecTimeMs, q.MaxExecTimeMs, q.TotalExecTimeMs, q.Rows, q.Query)),
				WithRecommendation("Review execution plan with EXPLAIN to evaluate whether indexes or query refactoring can improve latency."),
				WithMetadata("query_id", fmt.Sprintf("%d", q.QueryID)),
				WithMetadata("calls", fmt.Sprintf("%d", q.Calls)),
				WithMetadata("mean_time_ms", fmt.Sprintf("%.2f", q.MeanExecTimeMs))))
		}

		// TOPQ-003: Query temp disk spills
		if q.TempBlksWritten > 0 {
			hasIssue = true
			out = append(out, finding("TOPQ-003", model.SeverityWarning,
				fmt.Sprintf("Query spilling to temp files (query ID %d)", q.QueryID),
				fmt.Sprintf("Query spilled %s temp blocks to disk across %d executions",
					fmtNum(q.TempBlksWritten), q.Calls),
				WithEvidence(fmt.Sprintf("Query ID: %d\nTemp blocks written: %d\nCalls: %d\nQuery: %s",
					q.QueryID, q.TempBlksWritten, q.Calls, q.Query)),
				WithRecommendation("Queries writing to temporary files indicate work_mem is too small for sort, hash join, or aggregation operations. Consider increasing work_mem or tuning query."),
				WithMetadata("query_id", fmt.Sprintf("%d", q.QueryID)),
				WithMetadata("temp_blks_written", fmt.Sprintf("%d", q.TempBlksWritten))))
		}
	}

	// TOPQ-004: Top queries inventory
	if !hasIssue {
		out = append(out, finding("TOPQ-004", model.SeverityPass, "Top queries inventory",
			fmt.Sprintf("All %d top queries are within acceptable latency thresholds (%d statements tracked)",
				len(rep.Queries), rep.StatementsCount),
			WithEvidence(fmt.Sprintf("Queries inspected: %d\nTotal statements in pg_stat_statements: %d",
				len(rep.Queries), rep.StatementsCount)),
			WithMetadata("queries_inspected", fmt.Sprintf("%d", len(rep.Queries))),
			WithMetadata("total_statements", fmt.Sprintf("%d", rep.StatementsCount))))
	} else {
		out = append(out, finding("TOPQ-004", model.SeverityInfo, "Top queries inventory",
			fmt.Sprintf("%d top queries analyzed from %d total recorded statements",
				len(rep.Queries), rep.StatementsCount),
			WithEvidence(fmt.Sprintf("Queries inspected: %d\nTotal statements in pg_stat_statements: %d",
				len(rep.Queries), rep.StatementsCount)),
			WithMetadata("queries_inspected", fmt.Sprintf("%d", len(rep.Queries))),
			WithMetadata("total_statements", fmt.Sprintf("%d", rep.StatementsCount))))
	}

	return out
}
