package collector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/database"
)

// fakeRow is one result row of any values.
type fakeRow []any

// fakeRows serves canned rows and simulates Scan like a driver row iterator.
type fakeRows struct {
	rows  []fakeRow
	index int
	err   error
}

func (f *fakeRows) Next() bool {
	if f.index >= len(f.rows) {
		return false
	}
	f.index++
	return true
}

func (f *fakeRows) Scan(dest ...any) error {
	if f.err != nil {
		return f.err
	}
	if f.index == 0 || f.index > len(f.rows) {
		return fmt.Errorf("Scan called before first Next")
	}
	src := f.rows[f.index-1]
	for i, d := range dest {
		switch v := d.(type) {
		case *int:
			switch n := src[i].(type) {
			case int:
				*v = n
			case int64:
				*v = int(n)
			case float64:
				*v = int(n)
			default:
				return fmt.Errorf("unexpected int source %T", src[i])
			}
		case *int64:
			switch n := src[i].(type) {
			case int64:
				*v = n
			case int:
				*v = int64(n)
			case float64:
				*v = int64(n)
			default:
				return fmt.Errorf("unexpected int64 source %T", src[i])
			}
		case *float64:
			switch n := src[i].(type) {
			case float64:
				*v = n
			case int:
				*v = float64(n)
			default:
				return fmt.Errorf("unexpected float source %T", src[i])
			}
		case *string:
			*v = src[i].(string)
		case *bool:
			*v = src[i].(bool)
		default:
			return fmt.Errorf("unsupported dest type %T", d)
		}
	}
	return nil
}

func (f *fakeRows) Err() error { return f.err }
func (f *fakeRows) Close()     {}

// fakeQueryer serves a fixture per exact SQL string; unknown SQL errors.
type fakeQueryer struct {
	queryRow map[string]fakeRow // single-row QueryRow fixtures
	query    map[string]fakeRows
	calls    int
	lastArgs []any
}

func (f *fakeQueryer) Query(_ context.Context, sql string, args ...any) (database.Rows, error) {
	f.calls++
	f.lastArgs = args
	fx, ok := f.query[sql]
	if !ok {
		return nil, fmt.Errorf("no Query fixture: %.40q", sql)
	}
	return &fx, nil
}

func (f *fakeQueryer) QueryRow(_ context.Context, sql string, args []any, dest ...any) error {
	f.calls++
	f.lastArgs = args
	fx, ok := f.queryRow[sql]
	if !ok {
		return fmt.Errorf("no QueryRow fixture: %.40q", sql)
	}
	fr := &fakeRows{rows: []fakeRow{fx}}
	if !fr.Next() {
		return fmt.Errorf("empty row: %.40q", sql)
	}
	return fr.Scan(dest...)
}

func (f *fakeQueryer) Close() error { return nil }

func TestConnectivity(t *testing.T) {
	q := &fakeQueryer{queryRow: map[string]fakeRow{
		selectOne:         {1},
		connectivityQuery: {"PostgreSQL 16.4 (Debian) on x86_64", 160004, "app"},
	}}
	c := New(q)
	out, err := c.Connectivity(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if !out.Connected || out.Version != "16.4" || out.VersionNum != 160004 || out.Database != "app" {
		t.Fatalf("connectivity parse wrong: %+v", out)
	}
	if out.LatencySeconds <= 0 {
		t.Fatalf("expected measured latency, got %v", out.LatencySeconds)
	}
}

func TestHealthAndUsage(t *testing.T) {
	q := &fakeQueryer{
		queryRow: map[string]fakeRow{
			healthQuery: {"PostgreSQL 16.4 (Debian) on x86_64", 160004, "app", 86400.0, 100, 1048576},
		},
		query: map[string]fakeRows{
			usageQuery: {rows: []fakeRow{{3, 12}}},
		},
	}
	c := New(q)
	h, err := c.Health(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if h.Version != "16.4" || h.VersionNum != 160004 || h.CurrentDatabase != "app" ||
		h.UptimeSeconds != 86400 || h.MaxConnections != 100 || h.DatabaseSizeBytes != 1048576 {
		t.Fatalf("health parse wrong: %+v", h)
	}
	total, active, err := c.Usage(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if total != 12 || active != 3 {
		t.Fatalf("usage parse wrong: total=%d active=%d", total, active)
	}
}

func TestSessionsParseAndTruncate(t *testing.T) {
	longQuery := "SELECT * FROM big_table WHERE id = " + repeat("1", 500)
	q := &fakeQueryer{query: map[string]fakeRows{
		sessionsQuery: {rows: []fakeRow{
			{9, "app_user", "app", "active", 182.0, "Client", "ClientRead", longQuery},
			{7, "postgres", "app", "idle", 0.0, "", "", ""},
		}},
	}}
	c := New(q)
	sessions, err := c.Sessions(ctx(), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(sessions))
	}
	if sessions[0].PID != 9 || sessions[0].DurationSeconds != 182 {
		t.Fatalf("session parse wrong: %+v", sessions[0])
	}
	if got := len([]rune(sessions[0].Query)); got > 31 {
		t.Fatalf("query not truncated: %d runes", got)
	}
}

func fakeRowsRows(vals ...any) []fakeRow { return []fakeRow{vals} }

func TestLongQueriesPassesThresholdArg(t *testing.T) {
	q := &fakeQueryer{query: map[string]fakeRows{
		longQueriesQuery: {rows: fakeRowsRows(5, "u", "d", "active", 120.0, "", "Lock", "select 1")},
	}}
	c := New(q)
	got, err := c.LongQueries(ctx(), 60*time.Second, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PID != 5 {
		t.Fatalf("long query parse wrong: %+v", got)
	}
	if len(q.lastArgs) != 1 || q.lastArgs[0] != 60.0 {
		t.Fatalf("threshold arg not passed as seconds: %v", q.lastArgs)
	}
}

func TestLocksParse(t *testing.T) {
	q := &fakeQueryer{query: map[string]fakeRows{
		locksQuery: {rows: []fakeRow{{111, "buser", 222, "guser", "app", 45.0, "active", "idle in transaction",
			"select * from x", "update y set z=1"}}},
	}}
	c := New(q)
	pairs, err := c.Locks(ctx(), 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 1 {
		t.Fatalf("want 1 pair, got %d", len(pairs))
	}
	p := pairs[0]
	if p.BlockedPID != 111 || p.BlockingPID != 222 || p.Database != "app" || p.WaitSeconds != 45 {
		t.Fatalf("lock parse wrong: %+v", p)
	}
	if p.BlockedQuery != "select * from x" || p.BlockingQuery != "update y set z=1" {
		t.Fatalf("lock queries wrong: %+v", p)
	}
}

func TestReplicationPrimary(t *testing.T) {
	q := &fakeQueryer{
		queryRow: map[string]fakeRow{roleQuery: {false}},
		query: map[string]fakeRows{
			replicasQuery: {rows: []fakeRow{{"db-replica-01", "10.0.0.5", "streaming", "async", 92.0}}},
		},
	}
	c := New(q)
	r, err := c.Replication(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if r.Role != "primary" || r.ConnectedStandbys != 1 {
		t.Fatalf("primary parse wrong: %+v", r)
	}
	if r.Replicas[0].ApplicationName != "db-replica-01" || r.Replicas[0].ReplayLagSeconds != 92 {
		t.Fatalf("replica parse wrong: %+v", r.Replicas[0])
	}
}

func TestReplicationStandby(t *testing.T) {
	q := &fakeQueryer{queryRow: map[string]fakeRow{
		roleQuery:       {true},
		standbyLagQuery: {30.0},
	}}
	c := New(q)
	r, err := c.Replication(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if r.Role != "standby" || !r.IsInRecovery || r.LocalReplayLagSeconds != 30 {
		t.Fatalf("standby parse wrong: %+v", r)
	}
}

func TestDatabasesParse(t *testing.T) {
	q := &fakeQueryer{
		query: map[string]fakeRows{
			databasesQuery: {rows: []fakeRow{{"app", "app_owner", 3}, {"postgres", "postgres", 1}}},
		},
		queryRow: map[string]fakeRow{sizeQuery: {1048576}},
	}
	c := New(q)
	dbs, err := c.Databases(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if len(dbs) != 2 {
		t.Fatalf("want 2 dbs, got %d", len(dbs))
	}
	if dbs[0].Name != "app" || dbs[0].Owner != "app_owner" || dbs[0].Connections != 3 ||
		dbs[0].SizeBytes != 1048576 || dbs[0].SizeUnknown {
		t.Fatalf("db parse wrong: %+v", dbs[0])
	}
	if q.calls != 3 { // 1 listing + 2 size probes
		t.Fatalf("expected 3 queries, got %d", q.calls)
	}
}

func TestSettingsParse(t *testing.T) {
	q := &fakeQueryer{query: map[string]fakeRows{
		settingsQuery: {rows: []fakeRow{{"max_connections", "100", "", "postmaster", "postmaster"},
			{"wal_level", "replica", "", "default", "postmaster"}}},
	}}
	c := New(q)
	settings, err := c.Settings(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if len(settings) != 2 || settings[0].Name != "max_connections" || settings[0].Value != "100" {
		t.Fatalf("settings parse wrong: %+v", settings)
	}
}

func TestShortVersion(t *testing.T) {
	cases := map[string]string{
		"PostgreSQL 16.4 (Debian 16.4-1.pgdg120+1) on x86_64": "16.4",
		"PostgreSQL 15.2": "15.2",
		"garbage":         "garbage",
	}
	for in, want := range cases {
		if got := ShortVersion(in); got != want {
			t.Errorf("ShortVersion(%q)=%q want %q", in, got, want)
		}
	}
}

func ctx() context.Context { return context.Background() }

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

// Query constants must match what the collector sends, so a typo in either is
// a test failure, not a silent fixture miss.
const (
	selectOne         = "SELECT 1"
	connectivityQuery = "SELECT current_setting('server_version'), current_setting('server_version_num')::int, current_database()"
	healthQuery       = "SELECT current_setting('server_version'), current_setting('server_version_num')::int, " +
		"current_database(), EXTRACT(EPOCH FROM (now() - pg_postmaster_start_time())), " +
		"current_setting('max_connections')::int, pg_database_size(current_database())"
	usageQuery = "SELECT count(*) FILTER (WHERE state = 'active'), count(*) " +
		"FROM pg_stat_activity WHERE pid <> pg_backend_pid()"
	sessionsQuery = "SELECT pid, COALESCE(usename, ''), COALESCE(datname, ''), COALESCE(state, ''), " +
		"COALESCE(EXTRACT(EPOCH FROM (now() - query_start)), 0), " +
		"COALESCE(wait_event_type, ''), COALESCE(wait_event, ''), COALESCE(query, '') " +
		"FROM pg_stat_activity WHERE pid <> pg_backend_pid() " +
		"ORDER BY 5 DESC, 1 ASC"
	longQueriesQuery = "SELECT pid, COALESCE(usename, ''), COALESCE(datname, ''), COALESCE(state, ''), " +
		"COALESCE(EXTRACT(EPOCH FROM (now() - query_start)), 0), " +
		"COALESCE(wait_event_type, ''), COALESCE(wait_event, ''), COALESCE(query, '') " +
		"FROM pg_stat_activity " +
		"WHERE pid <> pg_backend_pid() AND state IN ('active', 'idle in transaction', 'idle in transaction (aborted)') " +
		"AND COALESCE(EXTRACT(EPOCH FROM (now() - query_start)), 0) >= $1 " +
		"ORDER BY 5 DESC, 1 ASC"
	locksQuery = "SELECT b.pid, COALESCE(b.usename, ''), c.pid, COALESCE(c.usename, ''), COALESCE(b.datname, ''), " +
		"COALESCE(EXTRACT(EPOCH FROM (now() - b.query_start)), 0), " +
		"COALESCE(b.state, ''), COALESCE(c.state, ''), " +
		"COALESCE(b.query, ''), COALESCE(c.query, '') " +
		"FROM pg_stat_activity b JOIN pg_stat_activity c ON c.pid = ANY(pg_blocking_pids(b.pid)) " +
		"WHERE b.pid <> pg_backend_pid() " +
		"ORDER BY 6 DESC, 1 ASC"
	roleQuery       = "SELECT pg_is_in_recovery()"
	standbyLagQuery = "SELECT COALESCE(EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())), 0)"
	replicasQuery   = "SELECT COALESCE(application_name, ''), COALESCE(client_addr::text, ''), " +
		"COALESCE(state, ''), COALESCE(sync_state, ''), " +
		"COALESCE(EXTRACT(EPOCH FROM replay_lag), 0) " +
		"FROM pg_stat_replication ORDER BY 1 ASC"
	databasesQuery = "SELECT d.datname, pg_get_userbyid(d.datdba), COUNT(s.datid) " +
		"FROM pg_database d LEFT JOIN pg_stat_database s ON s.datid = d.oid " +
		"WHERE NOT d.datistemplate GROUP BY d.datname, d.datdba ORDER BY d.datname"
	sizeQuery     = "SELECT COALESCE(pg_database_size($1), 0)"
	settingsQuery = "SELECT name, setting, COALESCE(unit, ''), COALESCE(source, ''), COALESCE(context, '') " +
		"FROM pg_settings " +
		"WHERE name IN ('max_connections','shared_buffers','work_mem','maintenance_work_mem'," +
		"'wal_level','max_wal_size','log_min_duration_statement') " +
		"ORDER BY name"
)
