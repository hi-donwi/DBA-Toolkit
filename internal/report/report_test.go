package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/config"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

func sampleReport() *model.Report {
	return &model.Report{
		Tool:      "dbakit",
		Version:   "0.1.0",
		Command:   "health",
		Timestamp: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
		Duration:  "42ms",
		Database:  model.Database{Engine: "postgresql", Version: "16.4", Name: "app"},
		Summary:   model.Summary{Critical: 1, Warning: 0, Info: 0, Pass: 1},
		Findings: []model.Finding{
			{
				ID:             "CONN-001",
				Severity:       model.SeverityCritical,
				Title:          "PostgreSQL connection failed",
				Summary:        "Could not establish a PostgreSQL connection",
				Evidence:       "dial tcp ...: connection refused",
				Recommendation: "Verify host, port, database, user, and credentials",
			},
			{ID: "CONN-002", Severity: model.SeverityPass, Title: "Connected"},
		},
		Data: HealthData{
			Connectivity: model.Connectivity{Connected: true, Version: "16.4", Database: "app", LatencySeconds: 0.008},
			Health:       model.Health{UptimeSeconds: 86400, TotalConnections: 12, MaxConnections: 100},
		},
	}
}

func TestJSONRoundTripShape(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, sampleReport()); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"tool", "version", "command", "database", "timestamp", "duration", "summary", "findings", "data"} {
		if _, ok := m[k]; !ok {
			t.Errorf("JSON missing top-level key %q", k)
		}
	}
	summary := m["summary"].(map[string]any)
	for _, k := range []string{"critical", "warning", "info", "pass"} {
		if summary[k] != float64(0) && !contains(summary, k) {
			t.Errorf("summary missing key %q", k)
		}
	}
	if got := summary["critical"]; got != float64(1) {
		t.Errorf("critical summary = %v", got)
	}
	db := m["database"].(map[string]any)
	if db["engine"] != "postgresql" || db["version"] != "16.4" {
		t.Errorf("database object wrong: %v", db)
	}
	if _, ok := m["findings"].([]any); !ok {
		t.Errorf("findings should be an array")
	}
	// Raw JSON must not contain unescaped HTML or tab characters.
	if strings.Contains(buf.String(), "\t") {
		t.Error("JSON must not contain raw tabs")
	}
}

func contains(m map[string]any, k string) bool {
	_, ok := m[k]
	return ok
}

func TestJSONIncludesPass(t *testing.T) {
	// --hide-pass is terminal-only; JSON stays complete so consumers decide.
	var buf bytes.Buffer
	if err := JSON(&buf, sampleReport()); err != nil {
		t.Fatal(err)
	}
	if strings.Count(buf.String(), `"severity": "PASS"`) != 1 {
		t.Errorf("expected exactly one PASS finding in JSON:\n%s", buf.String())
	}
}

func TestJSONNeverLeaksPassword(t *testing.T) {
	cfg := config.Config{Host: "db", Port: 5432, User: "u", Name: "n", Password: "sekrit-word"}
	r := sampleReport()
	r.Database.Engine = "postgresql"
	r.Database.Name = "app"
	// A finding whose evidence embeds a raw DSN containing the password.
	r.Findings[0].Evidence = "connect failed: " + cfg.ConnectionString()
	r.Findings[0].Evidence = config.Redact(r.Findings[0].Evidence)

	var buf bytes.Buffer
	if err := JSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "sekrit-word") {
		t.Fatalf("password leaked into JSON:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "password=***") {
		t.Fatalf("redacted marker missing from JSON:\n%s", buf.String())
	}
}

func TestHumanDiagnoseHeaderUsesDiagnoseData(t *testing.T) {
	// Regression: diagnose renders the header card from DiagnoseData, not
	// HealthData — a wrong type assertion used to print "Connection FAIL".
	r := sampleReport()
	r.Command = "diagnose"
	r.Data = DiagnoseData{
		Connectivity: model.Connectivity{Connected: true, Version: "16.4", Database: "app", LatencySeconds: 0.01},
		Health:       model.Health{UptimeSeconds: 100, TotalConnections: 5, MaxConnections: 100, ConnectionUsage: 5},
	}
	var buf bytes.Buffer
	if err := Human(&buf, r, Options{HidePass: true}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "Connection     FAIL") || strings.Contains(out, "Error") {
		t.Fatalf("diagnose header mis-rendered:\n%s", out)
	}
	if !strings.Contains(out, "Connection     PASS") {
		t.Fatalf("diagnose header should show PASS:\n%s", out)
	}
	if !strings.Contains(out, "5 / 100 (5.0%)") {
		t.Fatalf("diagnose header should show connection usage:\n%s", out)
	}
}

func TestHumanNeverLeaksPassword(t *testing.T) {
	cfg := config.Config{Host: "db", Port: 5432, User: "u", Name: "n", Password: "sekrit-word"}
	r := sampleReport()
	r.Findings[0].Evidence = config.Redact("connect failed: " + cfg.ConnectionString())
	var buf bytes.Buffer
	opts := Options{}
	if err := Human(&buf, r, opts); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "sekrit-word") {
		t.Fatalf("password leaked into human output:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "CRITICAL") {
		t.Fatalf("human output missing severity header:\n%s", buf.String())
	}
}

func TestHumanIndexes(t *testing.T) {
	r := sampleReport()
	r.Command = "indexes"
	r.Data = IndexesData{
		Indexes: []model.IndexInfo{
			{Schema: "public", Table: "users", Index: "idx_users_id", SizeBytes: 1048576, Scans: 100, IsUnique: true, IsValid: true},
			{Schema: "public", Table: "orders", Index: "idx_orders_status", SizeBytes: 20971520, Scans: 0, IsUnique: false, IsValid: false},
		},
	}
	var buf bytes.Buffer
	if err := Human(&buf, r, Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "User indexes") {
		t.Errorf("output missing 'User indexes' header:\n%s", out)
	}
	if !strings.Contains(out, "idx_users_id") || !strings.Contains(out, "idx_orders_status") {
		t.Errorf("output missing index names:\n%s", out)
	}
	if !strings.Contains(out, "1.0 MiB") || !strings.Contains(out, "20.0 MiB") {
		t.Errorf("output missing human byte sizes:\n%s", out)
	}
}

func TestHumanXID(t *testing.T) {
	r := sampleReport()
	r.Command = "xid"
	r.Data = XIDData{
		XID: model.XIDReport{
			AutovacuumFreezeMaxAge: 200000000,
			Databases: []model.DatabaseXIDInfo{
				{Datname: "postgres", Age: 1500000, RemainingXIDs: 2145983647, PercentWraparound: 0.1},
				{Datname: "app_prod", Age: 250000000, RemainingXIDs: 1897483647, PercentWraparound: 11.6},
			},
			OldestTables: []model.TableXIDInfo{
				{Schema: "public", Table: "events", Age: 250000000, SizeBytes: 1073741824},
			},
		},
	}
	var buf bytes.Buffer
	if err := Human(&buf, r, Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Transaction ID (XID) & Wraparound") {
		t.Errorf("output missing XID header:\n%s", out)
	}
	if !strings.Contains(out, "postgres") || !strings.Contains(out, "app_prod") {
		t.Errorf("output missing database names:\n%s", out)
	}
	if !strings.Contains(out, "Oldest tables (freeze horizon):") || !strings.Contains(out, "events") {
		t.Errorf("output missing oldest tables section:\n%s", out)
	}
	if !strings.Contains(out, "1.0 GiB") {
		t.Errorf("output missing table human size:\n%s", out)
	}
}

func TestHumanCache(t *testing.T) {
	r := sampleReport()
	r.Command = "cache"
	r.Data = CacheData{
		Cache: model.CacheReport{
			DatabaseName:  "app",
			OverallRatio:  98.50,
			HeapHitRatio:  98.10,
			IndexHitRatio: 99.20,
			ToastHitRatio: 100.0,
			Tables: []model.TableCacheInfo{
				{Schema: "public", Table: "orders", HeapReads: 1200, HeapHits: 50000, HeapHitRatio: 97.6, IndexReads: 300, IndexHits: 20000, IndexHitRatio: 98.5},
			},
		},
	}
	var buf bytes.Buffer
	if err := Human(&buf, r, Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Buffer Cache Hit Ratio") {
		t.Errorf("output missing Cache header:\n%s", out)
	}
	if !strings.Contains(out, "Overall hit ratio") || !strings.Contains(out, "98.50%") {
		t.Errorf("output missing overall ratio:\n%s", out)
	}
	if !strings.Contains(out, "orders") || !strings.Contains(out, "97.6%") {
		t.Errorf("output missing table stats:\n%s", out)
	}
}

func TestHumanTopQueries(t *testing.T) {
	r := sampleReport()
	r.Command = "top-queries"
	r.Data = TopQueriesData{
		TopQueries: model.TopQueriesReport{
			ExtensionAvailable: true,
			StatementsCount:    25,
			Queries: []model.TopQuery{
				{QueryID: 98765, Query: "SELECT * FROM users WHERE active = true", Calls: 500, MeanExecTimeMs: 150.0, TotalExecTimeMs: 75000.0, Rows: 500, SharedBlksHit: 900, SharedBlksRead: 100, TempBlksWritten: 50},
			},
		},
	}
	var buf bytes.Buffer
	if err := Human(&buf, r, Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Top Slow Queries") {
		t.Errorf("output missing TopQueries header:\n%s", out)
	}
	if !strings.Contains(out, "98765") || !strings.Contains(out, "users") {
		t.Errorf("output missing query data:\n%s", out)
	}
	if !strings.Contains(out, "150.0ms") {
		t.Errorf("output missing formatted mean execution time:\n%s", out)
	}

	// Test missing extension output
	rNoExt := sampleReport()
	rNoExt.Command = "top-queries"
	rNoExt.Data = TopQueriesData{
		TopQueries: model.TopQueriesReport{ExtensionAvailable: false},
	}
	var bufNoExt bytes.Buffer
	if err := Human(&bufNoExt, rNoExt, Options{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bufNoExt.String(), "extension is not installed") {
		t.Errorf("output missing extension not installed warning:\n%s", bufNoExt.String())
	}
}

func TestHumanBloat(t *testing.T) {
	r := sampleReport()
	r.Command = "bloat"
	r.Data = BloatData{
		Bloat: model.BloatReport{
			Tables: []model.TableBloatInfo{
				{Schema: "public", Table: "events", LiveTuples: 50000, DeadTuples: 25000, DeadTupleRatio: 33.3, TableSizeBytes: 20971520, TotalSizeBytes: 41943040, LastVacuum: "2026-09-28 10:00:00", LastAutovacuum: "2026-09-29 02:00:00"},
			},
		},
	}
	var buf bytes.Buffer
	if err := Human(&buf, r, Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Table Bloat & Dead Tuples") {
		t.Errorf("output missing Bloat header:\n%s", out)
	}
	if !strings.Contains(out, "events") || !strings.Contains(out, "33.3%") {
		t.Errorf("output missing table bloat data:\n%s", out)
	}
	if !strings.Contains(out, "20.0 MiB") || !strings.Contains(out, "40.0 MiB") {
		t.Errorf("output missing table human size:\n%s", out)
	}
}

var _ = Options{} // Options is a value type used above


