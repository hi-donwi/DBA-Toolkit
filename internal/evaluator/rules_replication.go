package evaluator

import (
	"fmt"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// EvaluateReplication reports the server role and scores replay lag for each
// standby (from a primary) or the local lag (on a standby). Detection only —
// no failover, promotion, or repair.
func EvaluateReplication(r model.Replication, t Thresholds) []model.Finding {
	out := make([]model.Finding, 0, 2+len(r.Replicas))
	out = append(out, finding("REPL-001", model.SeverityInfo, "Replication role",
		fmt.Sprintf("PostgreSQL server is %s", r.Role),
		WithMetadata("role", r.Role)))

	switch {
	case r.Role == "primary" && len(r.Replicas) == 0:
		out = append(out, finding("REPL-002", model.SeverityInfo, "Replication",
			"Primary has no connected standby servers",
			WithEvidence("pg_stat_replication is empty")))
	case r.Role == "primary":
		for _, rep := range r.Replicas {
			out = append(out, replicaLagFinding("REPL-002",
				rep.ApplicationName, rep.ReplayLagSeconds, t,
				fmt.Sprintf("Standby %q (state %s, sync %s)", rep.ApplicationName, rep.State, rep.SyncState)))
		}
	default: // standby
		out = append(out, replicaLagFinding("REPL-002",
			"local", r.LocalReplayLagSeconds, t, "Local WAL replay on this standby"))
	}
	return out
}

// replicaLagFinding scores one lag measurement against warn/crit boundaries,
// reporting PASS when the lag is below both.
func replicaLagFinding(id, name string, lagSeconds float64, t Thresholds, evidence string) model.Finding {
	lag := time.Duration(lagSeconds * float64(time.Second))
	lagTxt := fmtDur(lagSeconds)
	switch {
	case lag >= t.ReplLagCrit:
		return finding(id, model.SeverityCritical, "Replica replay lag critical",
			fmt.Sprintf("Replica %q replay lag is %s (above %s critical threshold)", name, lagTxt, t.ReplLagCrit),
			WithEvidence(evidence), WithMetadata("lag_seconds", fmt.Sprintf("%.1f", lagSeconds)))
	case lag >= t.ReplLagWarn:
		return finding(id, model.SeverityWarning, "Replica replay lag detected",
			fmt.Sprintf("Replica %q replay lag is %s", name, lagTxt),
			WithEvidence(evidence), WithMetadata("lag_seconds", fmt.Sprintf("%.1f", lagSeconds)))
	default:
		return finding(id, model.SeverityPass, "Replication",
			fmt.Sprintf("Replica %q replay lag is %s", name, lagTxt),
			WithEvidence(evidence), WithMetadata("lag_seconds", fmt.Sprintf("%.1f", lagSeconds)))
	}
}
