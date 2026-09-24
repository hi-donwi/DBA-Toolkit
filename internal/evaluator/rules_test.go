package evaluator

import (
	"strings"
	"testing"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

func TestEvaluateConnectivityPass(t *testing.T) {
	fs := EvaluateConnectivity(model.Connectivity{
		Connected: true, Version: "16.4", Database: "app", LatencySeconds: 0.008,
	})
	if len(fs) != 1 || fs[0].Severity != model.SeverityPass || fs[0].ID != "CONN-001" {
		t.Fatalf("unexpected findings: %+v", fs)
	}
	if !strings.Contains(fs[0].Evidence, "16.4") {
		t.Errorf("evidence should mention version: %q", fs[0].Evidence)
	}
}

func TestEvaluateConnectivityFailure(t *testing.T) {
	fs := EvaluateConnectivity(model.Connectivity{
		Connected: false, Error: "postgres connection failed: connection refused",
	})
	if len(fs) != 1 || fs[0].Severity != model.SeverityCritical {
		t.Fatalf("expected CRITICAL connectivity finding: %+v", fs)
	}
	if fs[0].Recommendation == "" {
		t.Error("failure finding should carry a recommendation")
	}
}

func TestEvaluateUsageThresholds(t *testing.T) {
	th := DefaultThresholds() // warn 80, crit 95
	cases := []struct {
		total int
		max   int
		want  model.Severity
	}{
		{50, 100, model.SeverityPass},
		{80, 100, model.SeverityWarning}, // boundary inclusive
		{94, 100, model.SeverityWarning},
		{95, 100, model.SeverityCritical}, // boundary inclusive
		{100, 100, model.SeverityCritical},
	}
	for _, c := range cases {
		fs := EvaluateUsage(c.total, 1, c.max, th)
		if len(fs) != 1 || fs[0].Severity != c.want {
			t.Errorf("total=%d max=%d: got %+v, want severity %s", c.total, c.max, fs, c.want)
		}
	}
}

func TestEvaluateUsageUnknownMax(t *testing.T) {
	fs := EvaluateUsage(5, 1, 0, DefaultThresholds())
	if len(fs) != 1 || fs[0].Severity != model.SeverityWarning {
		t.Fatalf("expected warning for unknown max_connections: %+v", fs)
	}
}

func TestEvaluateLongQueriesNone(t *testing.T) {
	fs := EvaluateLongQueries(nil, time.Minute)
	if len(fs) != 1 || fs[0].Severity != model.SeverityPass {
		t.Fatalf("expected PASS when no long queries: %+v", fs)
	}
}

func TestEvaluateLongQueriesDetects(t *testing.T) {
	fs := EvaluateLongQueries([]model.Session{
		{PID: 12345, User: "app_user", Database: "app", State: "active", DurationSeconds: 182},
		{PID: 42, User: "other", Database: "app", State: "idle in transaction", DurationSeconds: 3},
	}, time.Minute)
	if len(fs) != 2 {
		t.Fatalf("want a finding per session, got %+v", fs)
	}
	if fs[0].Severity != model.SeverityWarning || fs[0].ID != "LONGQ-001" {
		t.Fatalf("want WARNING LONGQ-001: %+v", fs[0])
	}
	if fs[0].Metadata["pid"] != "12345" || fs[0].Metadata["database"] != "app" {
		t.Fatalf("metadata wrong: %+v", fs[0].Metadata)
	}
	if !strings.Contains(fs[0].Summary, "3m 2s") {
		t.Errorf("summary should mention the humanized duration: %q", fs[0].Summary)
	}
	if fs[0].Recommendation == "" {
		t.Error("long query finding should carry a recommendation")
	}
}

func TestEvaluateLocksFilterAndSeverity(t *testing.T) {
	th := 5 * time.Second
	pairs := []model.BlockingPair{
		{BlockedPID: 111, BlockingPID: 222, WaitSeconds: 30}, // above threshold → CRITICAL
		{BlockedPID: 333, BlockingPID: 444, WaitSeconds: 1},  // below threshold → excluded
	}
	fs := EvaluateLocks(pairs, th)
	if len(fs) != 1 || fs[0].Severity != model.SeverityCritical || fs[0].ID != "LOCK-001" {
		t.Fatalf("expected one CRITICAL LOCK-001: %+v", fs)
	}
	if fs[0].Metadata["blocked_pid"] != "111" || fs[0].Metadata["blocking_pid"] != "222" {
		t.Fatalf("lock metadata wrong: %+v", fs[0].Metadata)
	}
	if !strings.Contains(fs[0].Summary, "111") || !strings.Contains(fs[0].Summary, "222") {
		t.Errorf("lock summary should name both pids: %q", fs[0].Summary)
	}
	if strings.Contains(fs[0].Evidence, "/bin/") {
		t.Error("lock evidence must not contain termination commands")
	}
}

func TestEvaluateLocksNone(t *testing.T) {
	fs := EvaluateLocks([]model.BlockingPair{{BlockedPID: 1, BlockingPID: 2, WaitSeconds: 1}}, 5*time.Second)
	if len(fs) != 1 || fs[0].Severity != model.SeverityPass {
		t.Fatalf("below-threshold wait should be PASS: %+v", fs)
	}
	fs = EvaluateLocks(nil, 5*time.Second)
	if len(fs) != 1 || fs[0].Severity != model.SeverityPass {
		t.Fatalf("no pairs should be PASS: %+v", fs)
	}
}

func TestEvaluateReplicationPrimaryLag(t *testing.T) {
	th := DefaultThresholds() // warn 30s, crit 120s
	r := model.Replication{
		Role: "primary", IsInRecovery: false, ConnectedStandbys: 2,
		Replicas: []model.Replica{
			{ApplicationName: "db-replica-01", State: "streaming", ReplayLagSeconds: 10},  // PASS
			{ApplicationName: "db-replica-02", State: "streaming", ReplayLagSeconds: 92},  // WARNING
			{ApplicationName: "db-replica-03", State: "streaming", ReplayLagSeconds: 300}, // CRITICAL
		},
	}
	fs := EvaluateReplication(r, th)
	// Exactly one REPL-002 finding per replica, with each of the three tiers.
	var byLag []model.Severity
	for _, f := range fs {
		if f.ID == "REPL-002" {
			byLag = append(byLag, f.Severity)
		}
	}
	want := map[model.Severity]bool{
		model.SeverityPass: true, model.SeverityWarning: true, model.SeverityCritical: true,
	}
	for _, s := range byLag {
		if !want[s] {
			t.Fatalf("unexpected lag severity %s in %+v", s, fs)
		}
	}
	if len(byLag) != 3 {
		t.Fatalf("want 3 repl lag findings, got %d", len(byLag))
	}
	// A warning finding must carry the human title used in docs/examples.
	anyWarnTitle := false
	for _, f := range fs {
		if f.Severity == model.SeverityWarning && f.Title == "Replica replay lag detected" {
			anyWarnTitle = true
		}
	}
	if !anyWarnTitle {
		t.Fatalf("no expected warning title in %+v", fs)
	}
}

func TestEvaluateReplicationRoleInfoAlwaysPresent(t *testing.T) {
	fs := EvaluateReplication(model.Replication{Role: "primary", ConnectedStandbys: 0}, DefaultThresholds())
	found := false
	for _, f := range fs {
		if f.ID == "REPL-001" && f.Metadata["role"] == "primary" {
			found = true
		}
	}
	if !found {
		t.Fatalf("role info finding missing: %+v", fs)
	}
}

func TestEvaluateReplicationStandby(t *testing.T) {
	r := model.Replication{Role: "standby", IsInRecovery: true, LocalReplayLagSeconds: 5}
	fs := EvaluateReplication(r, DefaultThresholds())
	var got []model.Severity
	for _, f := range fs {
		if f.ID == "REPL-002" {
			got = append(got, f.Severity)
		}
	}
	if len(got) != 1 || got[0] != model.SeverityPass {
		t.Fatalf("low local lag should PASS: %+v", fs)
	}
	r.LocalReplayLagSeconds = 500
	fs = EvaluateReplication(r, DefaultThresholds())
	for _, f := range fs {
		if f.ID == "REPL-002" && f.Severity == model.SeverityCritical {
			return
		}
	}
	t.Fatalf("high local lag should be CRITICAL: %+v", fs)
}

func TestEvaluateDatabases(t *testing.T) {
	fs := EvaluateDatabases([]model.DatabaseInfo{
		{Name: "app", Owner: "app_owner", SizeBytes: 1 << 30, Connections: 3},
		{Name: "templess", Owner: "postgres", SizeBytes: 0, SizeUnknown: true},
	})
	if len(fs) != 2 {
		t.Fatalf("want one finding per db: %+v", fs)
	}
	if fs[0].ID != "DB-001" || fs[0].Severity != model.SeverityInfo {
		t.Fatalf("want INFO DB-001: %+v", fs[0])
	}
	if !strings.Contains(fs[0].Evidence, "app_owner") {
		t.Errorf("evidence should name owner: %q", fs[0].Evidence)
	}
	if !strings.Contains(fs[1].Evidence, "unknown") {
		t.Errorf("unknown size must be marked: %q", fs[1].Evidence)
	}
}

func TestEvaluateSettingsWarns(t *testing.T) {
	fs := EvaluateSettings([]model.ConfigSetting{
		{Name: "max_connections", Value: "100", Source: "postmaster"},
		{Name: "wal_level", Value: "minimal"},
		{Name: "log_min_duration_statement", Value: "-1"},
	})
	var ids []string
	for _, f := range fs {
		ids = append(ids, f.ID)
	}
	for _, want := range []string{"CFG-001", "CFG-WAL-LEVEL", "CFG-LOG-DURATION"} {
		found := false
		for _, id := range ids {
			if id == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing rule %s in %v", want, ids)
		}
	}
	for _, f := range fs {
		if f.ID == "CFG-WAL-LEVEL" && f.Severity != model.SeverityWarning {
			t.Errorf("minimal wal_level should warn: %+v", f)
		}
		if f.ID == "CFG-LOG-DURATION" && f.Severity != model.SeverityWarning {
			t.Errorf("-1 log_min_duration_statement should warn: %+v", f)
		}
	}
}

func TestEvaluateSettingsNoFalseWarnings(t *testing.T) {
	fs := EvaluateSettings([]model.ConfigSetting{
		{Name: "wal_level", Value: "replica"},
		{Name: "log_min_duration_statement", Value: "1000"},
	})
	for _, f := range fs {
		if f.ID == "CFG-WAL-LEVEL" || f.ID == "CFG-LOG-DURATION" {
			t.Errorf("no warning expected for healthy settings: %+v", f)
		}
	}
}

func TestEvaluateIndexes(t *testing.T) {
	th := DefaultThresholds() // UnusedIndexMinSize = 10MB

	// 1. Empty indexes
	emptyFs := EvaluateIndexes(nil, th)
	if len(emptyFs) != 1 || emptyFs[0].ID != "IDX-003" || emptyFs[0].Severity != model.SeverityInfo {
		t.Fatalf("unexpected empty indexes findings: %+v", emptyFs)
	}

	// 2. All valid, active indexes -> PASS
	healthy := []model.IndexInfo{
		{Schema: "public", Table: "users", Index: "idx_users_id", SizeBytes: 1048576, Scans: 1000, IsUnique: true, IsValid: true},
		{Schema: "public", Table: "orders", Index: "idx_orders_user", SizeBytes: 5242880, Scans: 250, IsUnique: false, IsValid: true},
	}
	healthyFs := EvaluateIndexes(healthy, th)
	if len(healthyFs) != 1 || healthyFs[0].ID != "IDX-003" || healthyFs[0].Severity != model.SeverityPass {
		t.Fatalf("expected PASS finding for healthy indexes, got: %+v", healthyFs)
	}

	// 3. Problematic indexes: invalid + unused (large) + unused unique (should not warn) + unused small (should not warn)
	mixed := []model.IndexInfo{
		// Invalid index -> CRITICAL (IDX-002)
		{Schema: "public", Table: "items", Index: "idx_items_broken", SizeBytes: 20971520, Scans: 0, IsUnique: false, IsValid: false, Definition: "CREATE INDEX idx_items_broken ON public.items (name)"},
		// Unused large non-unique index (15MB >= 10MB, 0 scans) -> WARNING (IDX-001)
		{Schema: "public", Table: "logs", Index: "idx_logs_old", SizeBytes: 15728640, Scans: 0, IsUnique: false, IsValid: true, Definition: "CREATE INDEX idx_logs_old ON public.logs (created_at)"},
		// Unused unique index -> exempt from unused warning
		{Schema: "public", Table: "users", Index: "idx_users_uuid", SizeBytes: 20971520, Scans: 0, IsUnique: true, IsValid: true},
		// Unused small index (< 10MB) -> exempt from unused warning
		{Schema: "public", Table: "tags", Index: "idx_tags_name", SizeBytes: 1048576, Scans: 0, IsUnique: false, IsValid: true},
	}
	mixedFs := EvaluateIndexes(mixed, th)

	var hasInvalid, hasUnused, hasInventory bool
	for _, f := range mixedFs {
		switch f.ID {
		case "IDX-002":
			hasInvalid = true
			if f.Severity != model.SeverityCritical {
				t.Errorf("invalid index should be CRITICAL, got %s", f.Severity)
			}
			if !strings.Contains(f.Recommendation, "REINDEX INDEX CONCURRENTLY") {
				t.Errorf("recommendation should mention REINDEX: %q", f.Recommendation)
			}
		case "IDX-001":
			hasUnused = true
			if f.Severity != model.SeverityWarning {
				t.Errorf("unused index should be WARNING, got %s", f.Severity)
			}
			if !strings.Contains(f.Recommendation, "DROP INDEX CONCURRENTLY") {
				t.Errorf("recommendation should mention DROP: %q", f.Recommendation)
			}
		case "IDX-003":
			hasInventory = true
			if f.Severity != model.SeverityInfo {
				t.Errorf("inventory with issues should be INFO, got %s", f.Severity)
			}
		}
	}

	if !hasInvalid {
		t.Error("expected IDX-002 finding for invalid index")
	}
	if !hasUnused {
		t.Error("expected IDX-001 finding for unused large index")
	}
	if !hasInventory {
		t.Error("expected IDX-003 inventory finding")
	}
}

func TestEvaluateXID(t *testing.T) {
	th := DefaultThresholds() // XIDWarnAge = 200M, XIDCritAge = 1.5B

	// 1. Empty report
	emptyFs := EvaluateXID(model.XIDReport{}, th)
	if len(emptyFs) != 1 || emptyFs[0].ID != "XID-004" || emptyFs[0].Severity != model.SeverityInfo {
		t.Fatalf("unexpected empty report findings: %+v", emptyFs)
	}

	// 2. Healthy databases & tables -> PASS
	healthy := model.XIDReport{
		AutovacuumFreezeMaxAge: 200000000,
		Databases: []model.DatabaseXIDInfo{
			{Datname: "postgres", Age: 1500000, RemainingXIDs: 2145983647, PercentWraparound: 0.07},
			{Datname: "app_prod", Age: 50000000, RemainingXIDs: 2097483647, PercentWraparound: 2.33},
		},
		OldestTables: []model.TableXIDInfo{
			{Schema: "public", Table: "users", Age: 50000000, SizeBytes: 104857600},
		},
	}
	healthyFs := EvaluateXID(healthy, th)
	if len(healthyFs) != 1 || healthyFs[0].ID != "XID-004" || healthyFs[0].Severity != model.SeverityPass {
		t.Fatalf("expected PASS finding for healthy XID, got: %+v", healthyFs)
	}

	// 3. Database warning + table warning
	warnReport := model.XIDReport{
		AutovacuumFreezeMaxAge: 200000000,
		Databases: []model.DatabaseXIDInfo{
			{Datname: "app_prod", Age: 250000000, RemainingXIDs: 1897483647, PercentWraparound: 11.64},
		},
		OldestTables: []model.TableXIDInfo{
			{Schema: "public", Table: "events", Age: 250000000, SizeBytes: 524288000},
			{Schema: "public", Table: "users", Age: 50000000, SizeBytes: 104857600},
		},
	}
	warnFs := EvaluateXID(warnReport, th)
	var hasDbWarn, hasTblWarn, hasInventory bool
	for _, f := range warnFs {
		switch f.ID {
		case "XID-002":
			hasDbWarn = true
			if f.Severity != model.SeverityWarning {
				t.Errorf("XID-002 should be WARNING, got %s", f.Severity)
			}
		case "XID-003":
			hasTblWarn = true
			if f.Severity != model.SeverityWarning {
				t.Errorf("XID-003 should be WARNING, got %s", f.Severity)
			}
			if !strings.Contains(f.Recommendation, "VACUUM (FREEZE, VERBOSE)") {
				t.Errorf("recommendation should mention VACUUM FREEZE: %q", f.Recommendation)
			}
		case "XID-004":
			hasInventory = true
			if f.Severity != model.SeverityInfo {
				t.Errorf("inventory with issues should be INFO, got %s", f.Severity)
			}
		}
	}
	if !hasDbWarn {
		t.Error("expected XID-002 finding for database exceeding freeze threshold")
	}
	if !hasTblWarn {
		t.Error("expected XID-003 finding for table exceeding freeze threshold")
	}
	if !hasInventory {
		t.Error("expected XID-004 finding")
	}

	// 4. Critical wraparound danger
	critReport := model.XIDReport{
		AutovacuumFreezeMaxAge: 200000000,
		Databases: []model.DatabaseXIDInfo{
			{Datname: "legacy_db", Age: 1600000000, RemainingXIDs: 547483647, PercentWraparound: 74.51},
		},
	}
	critFs := EvaluateXID(critReport, th)
	var hasCrit bool
	for _, f := range critFs {
		if f.ID == "XID-001" {
			hasCrit = true
			if f.Severity != model.SeverityCritical {
				t.Errorf("XID-001 should be CRITICAL, got %s", f.Severity)
			}
			if !strings.Contains(f.Recommendation, "Run manual VACUUM FREEZE") {
				t.Errorf("recommendation should urge manual vacuum freeze: %q", f.Recommendation)
			}
		}
	}
	if !hasCrit {
		t.Error("expected XID-001 finding for critical wraparound risk")
	}
}
