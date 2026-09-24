package evaluator

import (
	"fmt"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// EvaluateIndexes evaluates user indexes for invalid definitions (IDX-002),
// unused space consumption (IDX-001), and overall inventory health (IDX-003).
func EvaluateIndexes(indexes []model.IndexInfo, th Thresholds) []model.Finding {
	if len(indexes) == 0 {
		return []model.Finding{
			finding("IDX-003", model.SeverityInfo, "Index inventory",
				"No user indexes found"),
		}
	}

	out := make([]model.Finding, 0)
	var totalBytes int64
	var hasIssue bool

	for _, idx := range indexes {
		totalBytes += idx.SizeBytes

		// IDX-002: Invalid index (failed CREATE INDEX CONCURRENTLY or corrupted)
		if !idx.IsValid {
			hasIssue = true
			out = append(out, finding("IDX-002", model.SeverityCritical,
				fmt.Sprintf("Invalid index %s.%s", idx.Schema, idx.Index),
				fmt.Sprintf("Index %q on table %q is marked invalid", idx.Index, idx.Table),
				WithEvidence(fmt.Sprintf("Schema: %s\nTable: %s\nIndex: %s\nSize: %d bytes\nDefinition: %s",
					idx.Schema, idx.Table, idx.Index, idx.SizeBytes, idx.Definition)),
				WithRecommendation(fmt.Sprintf("Rebuild the invalid index concurrently: REINDEX INDEX CONCURRENTLY %s.%s; or drop it if redundant.",
					quoteIdent(idx.Schema), quoteIdent(idx.Index))),
				WithMetadata("schema", idx.Schema),
				WithMetadata("table", idx.Table),
				WithMetadata("index", idx.Index),
				WithMetadata("size_bytes", fmt.Sprintf("%d", idx.SizeBytes)),
				WithMetadata("is_valid", "false")))
		}

		// IDX-001: Unused index (0 scans on non-unique index with size >= threshold)
		// Unique indexes are excluded because they enforce constraints even without explicit scans.
		if idx.IsValid && idx.Scans == 0 && !idx.IsUnique && idx.SizeBytes >= th.UnusedIndexMinSize {
			hasIssue = true
			out = append(out, finding("IDX-001", model.SeverityWarning,
				fmt.Sprintf("Unused index %s.%s", idx.Schema, idx.Index),
				fmt.Sprintf("Index %q on table %q has 0 scans (%s)", idx.Index, idx.Table, fmtBytes(idx.SizeBytes)),
				WithEvidence(fmt.Sprintf("Schema: %s\nTable: %s\nIndex: %s\nSize: %d bytes\nScans: 0\nDefinition: %s",
					idx.Schema, idx.Table, idx.Index, idx.SizeBytes, idx.Definition)),
				WithRecommendation(fmt.Sprintf("Verify whether this index is required by periodic batch workloads or foreign keys. If unused, consider dropping it to save storage and write overhead: DROP INDEX CONCURRENTLY %s.%s;",
					quoteIdent(idx.Schema), quoteIdent(idx.Index))),
				WithMetadata("schema", idx.Schema),
				WithMetadata("table", idx.Table),
				WithMetadata("index", idx.Index),
				WithMetadata("size_bytes", fmt.Sprintf("%d", idx.SizeBytes)),
				WithMetadata("scans", "0")))
		}
	}

	// IDX-003: Index inventory summary
	if !hasIssue {
		out = append(out, finding("IDX-003", model.SeverityPass, "Index inventory",
			fmt.Sprintf("All %d user indexes are valid and active (%s total)", len(indexes), fmtBytes(totalBytes)),
			WithEvidence(fmt.Sprintf("Total indexes: %d\nTotal index size: %d bytes", len(indexes), totalBytes)),
			WithMetadata("total_indexes", fmt.Sprintf("%d", len(indexes))),
			WithMetadata("total_size_bytes", fmt.Sprintf("%d", totalBytes))))
	} else {
		out = append(out, finding("IDX-003", model.SeverityInfo, "Index inventory",
			fmt.Sprintf("%d user indexes inspected (%s total)", len(indexes), fmtBytes(totalBytes)),
			WithEvidence(fmt.Sprintf("Total indexes: %d\nTotal index size: %d bytes", len(indexes), totalBytes)),
			WithMetadata("total_indexes", fmt.Sprintf("%d", len(indexes))),
			WithMetadata("total_size_bytes", fmt.Sprintf("%d", totalBytes))))
	}

	return out
}

func quoteIdent(s string) string {
	return `"` + s + `"`
}
