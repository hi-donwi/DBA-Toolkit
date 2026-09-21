package evaluator

import (
	"fmt"
	"strings"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// EvaluateLongQueries flags sessions that have been running at or above the
// threshold. Empty sessions produce a PASS finding.
func EvaluateLongQueries(sessions []model.Session, threshold time.Duration) []model.Finding {
	if len(sessions) == 0 {
		return []model.Finding{
			finding("LONGQ-001", model.SeverityPass, "Long-running queries",
				fmt.Sprintf("No queries running longer than %s", threshold)),
		}
	}
	out := make([]model.Finding, 0, len(sessions))
	for _, s := range sessions {
		m := map[string]string{
			"pid":      fmt.Sprintf("%d", s.PID),
			"database": s.Database,
			"user":     s.User,
		}
		var ev strings.Builder
		fmt.Fprintf(&ev, "PID: %d\nDuration: %s\nDatabase: %s\nUser: %s\nState: %s",
			s.PID, fmtDur(s.DurationSeconds), s.Database, s.User, s.State)
		if s.WaitEvent != "" {
			fmt.Fprintf(&ev, "\nWait event: %s", s.WaitEvent)
		}
		opts := []option{
			WithEvidence(ev.String()),
			WithRecommendation("Investigate the query for missing indexes, blocking, or excessive work_mem. dbakit is read-only and will not cancel it."),
		}
		for k, v := range m {
			opts = append(opts, WithMetadata(k, v))
		}
		out = append(out, finding("LONGQ-001", model.SeverityWarning, "Long-running query detected",
			fmt.Sprintf("Query on PID %d has been running for %s", s.PID, fmtDur(s.DurationSeconds)),
			opts...))
	}
	return out
}
