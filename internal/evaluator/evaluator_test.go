package evaluator

import (
	"reflect"
	"testing"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

func TestSummarize(t *testing.T) {
	fs := []model.Finding{
		{ID: "A", Severity: model.SeverityCritical},
		{ID: "B", Severity: model.SeverityWarning},
		{ID: "C", Severity: model.SeverityInfo},
		{ID: "D", Severity: model.SeverityPass},
		{ID: "E", Severity: model.SeverityPass},
	}
	s := Summarize(fs)
	if s.Critical != 1 || s.Warning != 1 || s.Info != 1 || s.Pass != 2 {
		t.Fatalf("summarize wrong: %+v", s)
	}
}

func TestSortDeterministic(t *testing.T) {
	makeFs := func() []model.Finding {
		return []model.Finding{
			{ID: "CONN-001", Severity: model.SeverityPass, Summary: "b"},
			{ID: "LOCK-001", Severity: model.SeverityCritical, Summary: "z"},
			{ID: "LOCK-001", Severity: model.SeverityCritical, Summary: "a"},
			{ID: "USAGE-001", Severity: model.SeverityWarning, Summary: "x"},
			{ID: "REPL-001", Severity: model.SeverityInfo, Summary: "y"},
		}
	}
	a, b := makeFs(), makeFs()
	Sort(a)
	Sort(b)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("sort not deterministic: %+v vs %+v", a, b)
	}
	// Most urgent first: critical before warning before info before pass.
	got := severityOrder(a)
	want := []model.Severity{
		model.SeverityCritical, model.SeverityCritical,
		model.SeverityWarning, model.SeverityInfo, model.SeverityPass,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("severity ordering wrong: %v", got)
	}
	// Equal severity, equal ID: text order.
	if a[0].Summary != "a" {
		t.Fatalf("text tiebreak wrong: %q", a[0].Summary)
	}
}

func severityOrder(fs []model.Finding) []model.Severity {
	out := make([]model.Severity, len(fs))
	for i, f := range fs {
		out[i] = f.Severity
	}
	return out
}

func TestFindingIDsAreDeclared(t *testing.T) {
	declared := RuleIDs()
	collect := func(fs []model.Finding) {
		for _, f := range fs {
			if !declared[f.ID] {
				t.Errorf("finding ID %q not in rule catalog", f.ID)
			}
		}
	}
	collect(EvaluateConnectivity(model.Connectivity{Connected: true}))
	collect(EvaluateConnectivity(model.Connectivity{}))
	collect(EvaluateUsage(50, 10, 100, DefaultThresholds()))
	collect(EvaluateLongQueries(nil, time.Minute))
	collect(EvaluateLocks(nil, 5*time.Second))
	collect(EvaluateReplication(model.Replication{Role: "primary"}, DefaultThresholds()))
	collect(EvaluateDatabases([]model.DatabaseInfo{{Name: "a"}}))
	collect(EvaluateSettings([]model.ConfigSetting{
		{Name: "wal_level", Value: "minimal"},
		{Name: "log_min_duration_statement", Value: "-1"},
	}))
	collect(EvaluateIndexes([]model.IndexInfo{
		{Schema: "public", Table: "t", Index: "i", SizeBytes: 20000000, Scans: 0, IsUnique: false, IsValid: false},
		{Schema: "public", Table: "t2", Index: "i2", SizeBytes: 20000000, Scans: 0, IsUnique: false, IsValid: true},
		{Schema: "public", Table: "t3", Index: "i3", SizeBytes: 1000, Scans: 100, IsUnique: true, IsValid: true},
	}, DefaultThresholds()))
	collect(EvaluateXID(model.XIDReport{
		AutovacuumFreezeMaxAge: 200000000,
		Databases: []model.DatabaseXIDInfo{
			{Datname: "a", Age: 300000000, RemainingXIDs: 1800000000},
			{Datname: "b", Age: 1600000000, RemainingXIDs: 50000000},
		},
		OldestTables: []model.TableXIDInfo{
			{Schema: "public", Table: "t", Age: 300000000, SizeBytes: 1000},
		},
	}, DefaultThresholds()))
	collect(EvaluateCache(model.CacheReport{
		DatabaseName: "app",
		OverallRatio: 88.0,
		Tables: []model.TableCacheInfo{
			{Schema: "public", Table: "t", HeapReads: 5000, HeapHits: 1000, HeapHitRatio: 16.6, IndexReads: 2000, IndexHits: 500, IndexHitRatio: 20.0},
		},
	}, DefaultThresholds()))
	collect(EvaluateCache(model.CacheReport{}, DefaultThresholds()))
	collect(EvaluateTopQueries(model.TopQueriesReport{
		ExtensionAvailable: false,
	}, DefaultThresholds()))
	collect(EvaluateTopQueries(model.TopQueriesReport{
		ExtensionAvailable: true,
		StatementsCount:    2,
		Queries: []model.TopQuery{
			{QueryID: 1, MeanExecTimeMs: 3000.0, Calls: 10, TempBlksWritten: 100},
			{QueryID: 2, MeanExecTimeMs: 600.0, Calls: 50, TempBlksWritten: 0},
			{QueryID: 3, MeanExecTimeMs: 10.0, Calls: 100, TempBlksWritten: 0},
		},
	}, DefaultThresholds()))
	collect(EvaluateTopQueries(model.TopQueriesReport{ExtensionAvailable: true}, DefaultThresholds()))
}

