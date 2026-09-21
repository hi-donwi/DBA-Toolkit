package evaluator

import (
	"fmt"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// EvaluateDatabases emits one INFO finding per database. Sizes are presented
// in bytes (machine-ready); only the reporter formats them for humans.
func EvaluateDatabases(dbs []model.DatabaseInfo) []model.Finding {
	if len(dbs) == 0 {
		return []model.Finding{
			finding("DB-001", model.SeverityInfo, "Database inventory",
				"No databases found"),
		}
	}
	out := make([]model.Finding, 0, len(dbs))
	for _, d := range dbs {
		size := "<unknown>"
		if !d.SizeUnknown {
			size = fmt.Sprintf("%d bytes", d.SizeBytes)
		}
		out = append(out, finding("DB-001", model.SeverityInfo, "Database "+d.Name,
			fmt.Sprintf("Database %q", d.Name),
			WithEvidence(fmt.Sprintf("Owner: %s\nSize: %s\nConnections: %d", d.Owner, size, d.Connections)),
			WithMetadata("owner", d.Owner),
			WithMetadata("size_bytes", fmt.Sprintf("%d", d.SizeBytes)),
			WithMetadata("connections", fmt.Sprintf("%d", d.Connections))))
	}
	return out
}
