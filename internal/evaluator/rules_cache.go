package evaluator

import (
	"fmt"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// EvaluateCache evaluates database and per-table buffer cache hit ratios,
// flagging low hit ratios (CACHE-001), high disk-read miss tables (CACHE-002),
// and producing an inventory summary (CACHE-003).
func EvaluateCache(rep model.CacheReport, th Thresholds) []model.Finding {
	if rep.DatabaseName == "" && len(rep.Tables) == 0 {
		return []model.Finding{
			finding("CACHE-003", model.SeverityInfo, "Buffer cache inventory",
				"No cache statistics available"),
		}
	}

	out := make([]model.Finding, 0)
	var hasIssue bool

	warnTh := th.CacheHitWarnPercent
	if warnTh <= 0 {
		warnTh = 95.0
	}
	critTh := th.CacheHitCritPercent
	if critTh <= 0 {
		critTh = 90.0
	}

	// CACHE-001: Overall buffer cache hit ratio
	if rep.OverallRatio < critTh {
		hasIssue = true
		out = append(out, finding("CACHE-001", model.SeverityCritical,
			"Buffer cache hit ratio critically low",
			fmt.Sprintf("Database %q buffer cache hit ratio is %.2f%% (critical threshold %.1f%%)",
				rep.DatabaseName, rep.OverallRatio, critTh),
			WithEvidence(fmt.Sprintf("Database: %s\nOverall hit ratio: %.2f%%\nHeap hit ratio: %.2f%%\nIndex hit ratio: %.2f%%\nToast hit ratio: %.2f%%\nCritical threshold: %.1f%%",
				rep.DatabaseName, rep.OverallRatio, rep.HeapHitRatio, rep.IndexHitRatio, rep.ToastHitRatio, critTh)),
			WithRecommendation("The majority of reads are hitting physical disk instead of RAM. Consider increasing shared_buffers and check for queries running sequential scans on large tables."),
			WithMetadata("database", rep.DatabaseName),
			WithMetadata("overall_hit_ratio", fmt.Sprintf("%.2f", rep.OverallRatio)),
			WithMetadata("heap_hit_ratio", fmt.Sprintf("%.2f", rep.HeapHitRatio)),
			WithMetadata("index_hit_ratio", fmt.Sprintf("%.2f", rep.IndexHitRatio))))
	} else if rep.OverallRatio < warnTh {
		hasIssue = true
		out = append(out, finding("CACHE-001", model.SeverityWarning,
			"Buffer cache hit ratio below target",
			fmt.Sprintf("Database %q buffer cache hit ratio is %.2f%% (recommended threshold %.1f%%)",
				rep.DatabaseName, rep.OverallRatio, warnTh),
			WithEvidence(fmt.Sprintf("Database: %s\nOverall hit ratio: %.2f%%\nHeap hit ratio: %.2f%%\nIndex hit ratio: %.2f%%\nToast hit ratio: %.2f%%\nWarning threshold: %.1f%%",
				rep.DatabaseName, rep.OverallRatio, rep.HeapHitRatio, rep.IndexHitRatio, rep.ToastHitRatio, warnTh)),
			WithRecommendation("Cache hit ratio is lower than ideal. Review table-level disk reads and optimize indexing to keep hot data in shared buffers."),
			WithMetadata("database", rep.DatabaseName),
			WithMetadata("overall_hit_ratio", fmt.Sprintf("%.2f", rep.OverallRatio)),
			WithMetadata("heap_hit_ratio", fmt.Sprintf("%.2f", rep.HeapHitRatio)),
			WithMetadata("index_hit_ratio", fmt.Sprintf("%.2f", rep.IndexHitRatio))))
	} else {
		out = append(out, finding("CACHE-001", model.SeverityPass,
			"Buffer cache hit ratio healthy",
			fmt.Sprintf("Database %q buffer cache hit ratio is %.2f%% (heap: %.2f%%, index: %.2f%%)",
				rep.DatabaseName, rep.OverallRatio, rep.HeapHitRatio, rep.IndexHitRatio),
			WithEvidence(fmt.Sprintf("Overall: %.2f%%\nHeap: %.2f%%\nIndex: %.2f%%\nToast: %.2f%%",
				rep.OverallRatio, rep.HeapHitRatio, rep.IndexHitRatio, rep.ToastHitRatio)),
			WithMetadata("database", rep.DatabaseName),
			WithMetadata("overall_hit_ratio", fmt.Sprintf("%.2f", rep.OverallRatio))))
	}

	// CACHE-002: Table cache miss and disk reads
	for _, t := range rep.Tables {
		// Only flag tables with significant physical reads (>= 1,000 blocks read) and low hit ratio
		if t.HeapReads >= 1000 && t.HeapHitRatio < critTh {
			hasIssue = true
			out = append(out, finding("CACHE-002", model.SeverityWarning,
				fmt.Sprintf("Low table heap cache hit ratio on %s.%s", t.Schema, t.Table),
				fmt.Sprintf("Table %q has %s disk reads with only %.1f%% cache hit ratio",
					t.Table, fmtNum(t.HeapReads), t.HeapHitRatio),
				WithEvidence(fmt.Sprintf("Schema: %s\nTable: %s\nHeap reads: %d\nHeap hits: %d\nHeap hit ratio: %.2f%%",
					t.Schema, t.Table, t.HeapReads, t.HeapHits, t.HeapHitRatio)),
				WithRecommendation(fmt.Sprintf("Table %s.%s is repeatedly read from disk. Ensure missing indexes are added or tune work_mem to reduce repetitive sequential scans.",
					quoteIdent(t.Schema), quoteIdent(t.Table))),
				WithMetadata("schema", t.Schema),
				WithMetadata("table", t.Table),
				WithMetadata("heap_reads", fmt.Sprintf("%d", t.HeapReads)),
				WithMetadata("heap_hit_ratio", fmt.Sprintf("%.2f", t.HeapHitRatio))))
		}

		if t.IndexReads >= 1000 && t.IndexHitRatio < critTh {
			hasIssue = true
			out = append(out, finding("CACHE-002", model.SeverityWarning,
				fmt.Sprintf("Low table index cache hit ratio on %s.%s", t.Schema, t.Table),
				fmt.Sprintf("Indexes on table %q have %s disk reads with only %.1f%% cache hit ratio",
					t.Table, fmtNum(t.IndexReads), t.IndexHitRatio),
				WithEvidence(fmt.Sprintf("Schema: %s\nTable: %s\nIndex reads: %d\nIndex hits: %d\nIndex hit ratio: %.2f%%",
					t.Schema, t.Table, t.IndexReads, t.IndexHits, t.IndexHitRatio)),
				WithRecommendation(fmt.Sprintf("Indexes on %s.%s do not fit well in buffer cache. Check index sizes and consider index maintenance or increasing RAM allocation.",
					quoteIdent(t.Schema), quoteIdent(t.Table))),
				WithMetadata("schema", t.Schema),
				WithMetadata("table", t.Table),
				WithMetadata("index_reads", fmt.Sprintf("%d", t.IndexReads)),
				WithMetadata("index_hit_ratio", fmt.Sprintf("%.2f", t.IndexHitRatio))))
		}
	}

	// CACHE-003: Cache inventory
	if !hasIssue {
		out = append(out, finding("CACHE-003", model.SeverityPass, "Buffer cache inventory",
			fmt.Sprintf("All examined cache metrics meet performance targets (overall: %.2f%%)", rep.OverallRatio),
			WithEvidence(fmt.Sprintf("Tables examined: %d\nOverall hit ratio: %.2f%%\nHeap: %.2f%%\nIndex: %.2f%%",
				len(rep.Tables), rep.OverallRatio, rep.HeapHitRatio, rep.IndexHitRatio)),
			WithMetadata("tables_inspected", fmt.Sprintf("%d", len(rep.Tables))),
			WithMetadata("overall_hit_ratio", fmt.Sprintf("%.2f", rep.OverallRatio))))
	} else {
		out = append(out, finding("CACHE-003", model.SeverityInfo, "Buffer cache inventory",
			fmt.Sprintf("Buffer cache statistics: overall hit ratio %.2f%% across %d active tables",
				rep.OverallRatio, len(rep.Tables)),
			WithEvidence(fmt.Sprintf("Tables examined: %d\nOverall hit ratio: %.2f%%\nHeap: %.2f%%\nIndex: %.2f%%",
				len(rep.Tables), rep.OverallRatio, rep.HeapHitRatio, rep.IndexHitRatio)),
			WithMetadata("tables_inspected", fmt.Sprintf("%d", len(rep.Tables))),
			WithMetadata("overall_hit_ratio", fmt.Sprintf("%.2f", rep.OverallRatio))))
	}

	return out
}
