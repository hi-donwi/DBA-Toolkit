package evaluator

import (
	"fmt"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// EvaluateConnectivity turns a connection result into a PASS or CRITICAL
// finding. The error text has already been scrubbed of credentials upstream.
func EvaluateConnectivity(c model.Connectivity) []model.Finding {
	if c.Connected {
		return []model.Finding{
			finding("CONN-001", model.SeverityPass, "Database connectivity",
				"Connection to PostgreSQL established",
				WithEvidence(fmt.Sprintf("PostgreSQL %s on database %q (%.0fms)",
					c.Version, c.Database, c.LatencySeconds*1000))),
		}
	}
	return []model.Finding{
		finding("CONN-001", model.SeverityCritical, "PostgreSQL connection failed",
			"Could not establish a PostgreSQL connection",
			WithEvidence(c.Error),
			WithRecommendation("Verify host, port, database, user, and credentials; ensure PostgreSQL is running and reachable.")),
	}
}

// EvaluateUsage scores total/active connections against the configured
// percentage thresholds (max == 0 means the setting could not be read).
func EvaluateUsage(total, active, maxConns int, t Thresholds) []model.Finding {
	if maxConns <= 0 {
		return []model.Finding{
			finding("USAGE-001", model.SeverityWarning, "Connection usage",
				"max_connections could not be read",
				WithEvidence("Cannot evaluate connection usage without max_connections")),
		}
	}
	use := float64(total) / float64(maxConns) * 100
	evidence := fmt.Sprintf("%d/%d connections (%.1f%%)", total, maxConns, use)
	switch {
	case use >= t.ConnUsageCritPercent:
		return []model.Finding{
			finding("USAGE-001", model.SeverityCritical, "Database is approaching connection limit",
				"Connection usage is at or above the critical threshold",
				WithEvidence(evidence),
				WithRecommendation("Increase max_connections or reduce concurrent connections before new clients are refused."),
				WithMetadata("total_connections", fmt.Sprintf("%d", total)),
				WithMetadata("active_connections", fmt.Sprintf("%d", active)),
				WithMetadata("max_connections", fmt.Sprintf("%d", maxConns))),
		}
	case use >= t.ConnUsageWarnPercent:
		return []model.Finding{
			finding("USAGE-001", model.SeverityWarning, "Connection usage is above configured threshold",
				"Connection usage is above the warning threshold",
				WithEvidence(evidence),
				WithRecommendation("Watch connection growth; plan capacity before the critical threshold is reached."),
				WithMetadata("total_connections", fmt.Sprintf("%d", total)),
				WithMetadata("active_connections", fmt.Sprintf("%d", active)),
				WithMetadata("max_connections", fmt.Sprintf("%d", maxConns))),
		}
	default:
		return []model.Finding{
			finding("USAGE-001", model.SeverityPass, "Connection usage",
				"Connection usage is within normal bounds",
				WithEvidence(evidence),
				WithMetadata("total_connections", fmt.Sprintf("%d", total)),
				WithMetadata("active_connections", fmt.Sprintf("%d", active)),
				WithMetadata("max_connections", fmt.Sprintf("%d", maxConns))),
		}
	}
}
