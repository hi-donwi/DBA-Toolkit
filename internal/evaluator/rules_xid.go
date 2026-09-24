package evaluator

import (
	"fmt"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// EvaluateXID evaluates transaction ID age across databases and oldest tables,
// flagging emergency wraparound risk (XID-001), autovacuum freeze age delays (XID-002),
// and specific tables holding back freeze horizon (XID-003).
func EvaluateXID(rep model.XIDReport, th Thresholds) []model.Finding {
	if len(rep.Databases) == 0 {
		return []model.Finding{
			finding("XID-004", model.SeverityInfo, "Transaction ID inventory",
				"No databases found"),
		}
	}

	warnThreshold := th.XIDWarnAge
	if warnThreshold <= 0 {
		if rep.AutovacuumFreezeMaxAge > 0 {
			warnThreshold = rep.AutovacuumFreezeMaxAge
		} else {
			warnThreshold = 200_000_000
		}
	}

	critThreshold := th.XIDCritAge
	if critThreshold <= 0 {
		critThreshold = 1_500_000_000
	}

	out := make([]model.Finding, 0)
	var hasCritical, hasWarning bool
	var maxDb model.DatabaseXIDInfo

	for _, d := range rep.Databases {
		if d.Age > maxDb.Age {
			maxDb = d
		}

		// XID-001: Emergency transaction ID wraparound risk
		if d.Age >= critThreshold || d.RemainingXIDs < 100_000_000 {
			hasCritical = true
			out = append(out, finding("XID-001", model.SeverityCritical,
				fmt.Sprintf("Emergency transaction ID wraparound risk on database %s", d.Datname),
				fmt.Sprintf("Database %q has reached %s XID age (only %s remaining, %.1f%% of wraparound limit)",
					d.Datname, fmtNum(d.Age), fmtNum(d.RemainingXIDs), d.PercentWraparound),
				WithEvidence(fmt.Sprintf("Database: %s\nAge: %d\nRemaining XIDs: %d\nWraparound capacity: %.2f%%\nCritical threshold: %d",
					d.Datname, d.Age, d.RemainingXIDs, d.PercentWraparound, critThreshold)),
				WithRecommendation(fmt.Sprintf("PostgreSQL stops accepting commands when remaining XIDs drop below 10M to prevent corruption. Run manual VACUUM FREEZE on database %s immediately.",
					quoteIdent(d.Datname))),
				WithMetadata("database", d.Datname),
				WithMetadata("age", fmt.Sprintf("%d", d.Age)),
				WithMetadata("remaining_xids", fmt.Sprintf("%d", d.RemainingXIDs)),
				WithMetadata("percent_wraparound", fmt.Sprintf("%.2f", d.PercentWraparound))))
			continue
		}

		// XID-002: Autovacuum freeze age threshold exceeded
		if d.Age >= warnThreshold {
			hasWarning = true
			out = append(out, finding("XID-002", model.SeverityWarning,
				fmt.Sprintf("Database %s exceeded autovacuum freeze age", d.Datname),
				fmt.Sprintf("Database %q age (%s) exceeds autovacuum freeze threshold (%s)",
					d.Datname, fmtNum(d.Age), fmtNum(warnThreshold)),
				WithEvidence(fmt.Sprintf("Database: %s\nAge: %d\nWarn threshold: %d\nAutovacuum freeze max age: %d",
					d.Datname, d.Age, warnThreshold, rep.AutovacuumFreezeMaxAge)),
				WithRecommendation("Autovacuum should be aggressively freezing old tuples. Check pg_stat_activity and lock contention to ensure autovacuum workers are not blocked."),
				WithMetadata("database", d.Datname),
				WithMetadata("age", fmt.Sprintf("%d", d.Age)),
				WithMetadata("warn_threshold", fmt.Sprintf("%d", warnThreshold))))
		}
	}

	// XID-003: Oldest tables holding back the freeze horizon
	for _, t := range rep.OldestTables {
		if t.Age >= warnThreshold {
			hasWarning = true
			out = append(out, finding("XID-003", model.SeverityWarning,
				fmt.Sprintf("Table %s.%s holding back freeze horizon", t.Schema, t.Table),
				fmt.Sprintf("Table %q in schema %q has age %s (size: %s), exceeding freeze threshold (%s)",
					t.Table, t.Schema, fmtNum(t.Age), fmtBytes(t.SizeBytes), fmtNum(warnThreshold)),
				WithEvidence(fmt.Sprintf("Schema: %s\nTable: %s\nAge: %d\nSize: %d bytes\nFreeze threshold: %d",
					t.Schema, t.Table, t.Age, t.SizeBytes, warnThreshold)),
				WithRecommendation(fmt.Sprintf("Vacuum and freeze table: VACUUM (FREEZE, VERBOSE) %s.%s; and verify autovacuum is not disabled in table options.",
					quoteIdent(t.Schema), quoteIdent(t.Table))),
				WithMetadata("schema", t.Schema),
				WithMetadata("table", t.Table),
				WithMetadata("age", fmt.Sprintf("%d", t.Age)),
				WithMetadata("size_bytes", fmt.Sprintf("%d", t.SizeBytes))))
		}
	}

	// XID-004: Transaction ID inventory status
	if !hasCritical && !hasWarning {
		out = append(out, finding("XID-004", model.SeverityPass, "Transaction ID health",
			fmt.Sprintf("All databases and tables are below freeze thresholds (oldest database: %s, age: %s, %.1f%% towards wraparound)",
				maxDb.Datname, fmtNum(maxDb.Age), maxDb.PercentWraparound),
			WithEvidence(fmt.Sprintf("Databases inspected: %d\nOldest database: %s\nMax age: %d\nWraparound percent: %.2f%%\nFreeze threshold: %d",
				len(rep.Databases), maxDb.Datname, maxDb.Age, maxDb.PercentWraparound, warnThreshold)),
			WithMetadata("oldest_database", maxDb.Datname),
			WithMetadata("max_age", fmt.Sprintf("%d", maxDb.Age)),
			WithMetadata("percent_wraparound", fmt.Sprintf("%.2f", maxDb.PercentWraparound))))
	} else {
		out = append(out, finding("XID-004", model.SeverityInfo, "Transaction ID inventory",
			fmt.Sprintf("%d databases and %d tables inspected (oldest database: %s, age: %s)",
				len(rep.Databases), len(rep.OldestTables), maxDb.Datname, fmtNum(maxDb.Age)),
			WithEvidence(fmt.Sprintf("Databases inspected: %d\nOldest database: %s\nMax age: %d\nFreeze threshold: %d",
				len(rep.Databases), maxDb.Datname, maxDb.Age, warnThreshold)),
			WithMetadata("oldest_database", maxDb.Datname),
			WithMetadata("max_age", fmt.Sprintf("%d", maxDb.Age))))
	}

	return out
}
