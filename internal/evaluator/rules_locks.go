package evaluator

import (
	"fmt"
	"strings"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// EvaluateLocks flags blocking pairs whose wait is at or above the threshold.
// Pairs below the threshold are excluded from findings but remain in the data
// tables. Detection only — the tool never terminates sessions.
func EvaluateLocks(pairs []model.BlockingPair, threshold time.Duration) []model.Finding {
	flag := make([]model.BlockingPair, 0, len(pairs))
	for _, p := range pairs {
		if time.Duration(p.WaitSeconds*float64(time.Second)) >= threshold {
			flag = append(flag, p)
		}
	}
	if len(flag) == 0 {
		return []model.Finding{
			finding("LOCK-001", model.SeverityPass, "Locks and blocking",
				fmt.Sprintf("No blocking detected within %s", threshold)),
		}
	}
	out := make([]model.Finding, 0, len(flag))
	for _, p := range flag {
		ev := fmt.Sprintf("Blocked PID: %d\nBlocking PID: %d\nDatabase: %s\nWait: %s\nBlocked state: %s\nBlocking state: %s",
			p.BlockedPID, p.BlockingPID, p.Database, fmtDur(p.WaitSeconds), p.BlockedState, p.BlockingState)
		if p.BlockedQuery != "" {
			ev += "\nBlocked query:\n  " + fmtQuery(p.BlockedQuery)
		}
		if p.BlockingQuery != "" {
			ev += "\nBlocking query:\n  " + fmtQuery(p.BlockingQuery)
		}
		out = append(out, finding("LOCK-001", model.SeverityCritical, "Blocking session detected",
			fmt.Sprintf("PID %d has been blocked by PID %d for %s",
				p.BlockedPID, p.BlockingPID, fmtDur(p.WaitSeconds)),
			WithEvidence(ev),
			WithRecommendation("Review the transaction on the blocking PID before any action. dbakit is read-only and will not terminate it."),
			WithMetadata("blocked_pid", fmt.Sprintf("%d", p.BlockedPID)),
			WithMetadata("blocking_pid", fmt.Sprintf("%d", p.BlockingPID)),
			WithMetadata("database", p.Database)))
	}
	return out
}

// fmtQuery collapses a query to a single space-joined line for evidence blocks.
func fmtQuery(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
