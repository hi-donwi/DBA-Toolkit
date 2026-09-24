// Package collector runs read-only PostgreSQL queries and turns their rows into
// the normalized model data evaluators consume. It never writes, never
// terminates sessions, and never escapes the database.Queryer seam, so its
// row-parsing is testable without a real server.
package collector

import (
	"context"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/database"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// Collector issues the MVP's read-only probes against one connection.
type Collector struct {
	q    database.Queryer
	now  func() time.Time
	trim func(string, int) string
}

// New creates a collector bound to an open database connection.
func New(q database.Queryer) *Collector {
	return &Collector{q: q, now: time.Now, trim: func(s string, n int) string { return Truncate(s, n) }}
}

// Pinger pings the server for latency measurement.
func (c *Collector) Pinger(ctx context.Context) (time.Duration, error) {
	start := c.now()
	var one int
	if err := c.q.QueryRow(ctx, "SELECT 1", nil, &one); err != nil {
		return 0, err
	}
	return c.now().Sub(start), nil
}

// Connectivity collects version, current database, and (via Pinger) latency.
func (c *Collector) Connectivity(ctx context.Context) (model.Connectivity, error) {
	var out model.Connectivity
	latency, err := c.Pinger(ctx)
	if err != nil {
		return out, err
	}
	out.Connected = true
	out.LatencySeconds = latency.Seconds()

	verRaw := ""
	if err := c.q.QueryRow(ctx,
		"SELECT current_setting('server_version'), current_setting('server_version_num')::int, current_database()",
		nil, &verRaw, &out.VersionNum, &out.Database); err != nil {
		return out, err
	}
	out.Version = ShortVersion(verRaw)
	return out, nil
}

// Health collects uptime, max connections, and current database size.
func (c *Collector) Health(ctx context.Context) (model.Health, error) {
	var h model.Health
	verRaw := ""
	if err := c.q.QueryRow(ctx,
		"SELECT current_setting('server_version'), current_setting('server_version_num')::int, "+
			"current_database(), EXTRACT(EPOCH FROM (now() - pg_postmaster_start_time())), "+
			"current_setting('max_connections')::int, pg_database_size(current_database())",
		nil, &verRaw, &h.VersionNum, &h.CurrentDatabase, &h.UptimeSeconds,
		&h.MaxConnections, &h.DatabaseSizeBytes); err != nil {
		return h, err
	}
	h.Version = ShortVersion(verRaw)
	return h, nil
}

// Usage counts active and total backend connections (excluding this one).
func (c *Collector) Usage(ctx context.Context) (total, active int, err error) {
	rows, err := c.q.Query(ctx,
		"SELECT count(*) FILTER (WHERE state = 'active'), count(*) "+
			"FROM pg_stat_activity WHERE pid <> pg_backend_pid()")
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, 0, rows.Err()
	}
	if err := rows.Scan(&active, &total); err != nil {
		return 0, 0, err
	}
	return total, active, rows.Err()
}

// Sessions lists activity rows, longest-running query first.
func (c *Collector) Sessions(ctx context.Context, maxQueryLen int) ([]model.Session, error) {
	rows, err := c.q.Query(ctx,
		"SELECT pid, COALESCE(usename, ''), COALESCE(datname, ''), COALESCE(state, ''), "+
			"COALESCE(EXTRACT(EPOCH FROM (now() - query_start)), 0), "+
			"COALESCE(wait_event_type, ''), COALESCE(wait_event, ''), COALESCE(query, '') "+
			"FROM pg_stat_activity WHERE pid <> pg_backend_pid() "+
			"ORDER BY 5 DESC, 1 ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Session, 0)
	for rows.Next() {
		var s model.Session
		if err := rows.Scan(&s.PID, &s.User, &s.Database, &s.State,
			&s.DurationSeconds, &s.WaitType, &s.WaitEvent, &s.Query); err != nil {
			return nil, err
		}
		s.Query = c.trim(s.Query, maxQueryLen)
		out = append(out, s)
	}
	return out, rows.Err()
}

// LongQueries lists sessions that have been running for at least threshold.
func (c *Collector) LongQueries(ctx context.Context, threshold time.Duration, maxQueryLen int) ([]model.Session, error) {
	rows, err := c.q.Query(ctx,
		"SELECT pid, COALESCE(usename, ''), COALESCE(datname, ''), COALESCE(state, ''), "+
			"COALESCE(EXTRACT(EPOCH FROM (now() - query_start)), 0), "+
			"COALESCE(wait_event_type, ''), COALESCE(wait_event, ''), COALESCE(query, '') "+
			"FROM pg_stat_activity "+
			"WHERE pid <> pg_backend_pid() AND state IN ('active', 'idle in transaction', 'idle in transaction (aborted)') "+
			"AND COALESCE(EXTRACT(EPOCH FROM (now() - query_start)), 0) >= $1 "+
			"ORDER BY 5 DESC, 1 ASC", threshold.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Session, 0)
	for rows.Next() {
		var s model.Session
		if err := rows.Scan(&s.PID, &s.User, &s.Database, &s.State,
			&s.DurationSeconds, &s.WaitType, &s.WaitEvent, &s.Query); err != nil {
			return nil, err
		}
		s.Query = c.trim(s.Query, maxQueryLen)
		out = append(out, s)
	}
	return out, rows.Err()
}

// Locks lists blocked sessions and their blocking sessions. Blocked-first
// ordering is deterministic (wait time desc, pid asc). It never terminates
// anything — this is diagnostic information only.
func (c *Collector) Locks(ctx context.Context, maxQueryLen int) ([]model.BlockingPair, error) {
	rows, err := c.q.Query(ctx,
		"SELECT b.pid, COALESCE(b.usename, ''), c.pid, COALESCE(c.usename, ''), COALESCE(b.datname, ''), "+
			"COALESCE(EXTRACT(EPOCH FROM (now() - b.query_start)), 0), "+
			"COALESCE(b.state, ''), COALESCE(c.state, ''), "+
			"COALESCE(b.query, ''), COALESCE(c.query, '') "+
			"FROM pg_stat_activity b JOIN pg_stat_activity c ON c.pid = ANY(pg_blocking_pids(b.pid)) "+
			"WHERE b.pid <> pg_backend_pid() "+
			"ORDER BY 6 DESC, 1 ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.BlockingPair, 0)
	for rows.Next() {
		var p model.BlockingPair
		if err := rows.Scan(&p.BlockedPID, &p.BlockedUser, &p.BlockingPID, &p.BlockingUser, &p.Database,
			&p.WaitSeconds, &p.BlockedState, &p.BlockingState, &p.BlockedQuery, &p.BlockingQuery); err != nil {
			return nil, err
		}
		p.BlockedQuery = c.trim(p.BlockedQuery, maxQueryLen)
		p.BlockingQuery = c.trim(p.BlockingQuery, maxQueryLen)
		out = append(out, p)
	}
	return out, rows.Err()
}

// Replication inspects the server's role and replication lag. It is read-only;
// no failover, promotion, or repair.
func (c *Collector) Replication(ctx context.Context) (model.Replication, error) {
	var r model.Replication
	inRecovery := false
	if err := c.q.QueryRow(ctx, "SELECT pg_is_in_recovery()", nil, &inRecovery); err != nil {
		return r, err
	}
	r.IsInRecovery = inRecovery
	if inRecovery {
		r.Role = "standby"
		if err := c.q.QueryRow(ctx,
			"SELECT COALESCE(EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())), 0)",
			nil, &r.LocalReplayLagSeconds); err != nil {
			return r, err
		}
		return r, nil
	}
	r.Role = "primary"
	rows, err := c.q.Query(ctx,
		"SELECT COALESCE(application_name, ''), COALESCE(client_addr::text, ''), "+
			"COALESCE(state, ''), COALESCE(sync_state, ''), "+
			"COALESCE(EXTRACT(EPOCH FROM replay_lag), 0) "+
			"FROM pg_stat_replication ORDER BY 1 ASC")
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		var rep model.Replica
		if err := rows.Scan(&rep.ApplicationName, &rep.ClientAddr, &rep.State, &rep.SyncState,
			&rep.ReplayLagSeconds); err != nil {
			return r, err
		}
		r.Replicas = append(r.Replicas, rep)
	}
	r.ConnectedStandbys = len(r.Replicas)
	return r, rows.Err()
}

// Databases lists the server's non-template databases with owner, size, and
// connection count. Sizes are fetched per database so a size the user cannot
// read is reported as unknown instead of failing the whole listing.
func (c *Collector) Databases(ctx context.Context) ([]model.DatabaseInfo, error) {
	rows, err := c.q.Query(ctx,
		"SELECT d.datname, pg_get_userbyid(d.datdba), COUNT(s.datid) "+
			"FROM pg_database d LEFT JOIN pg_stat_database s ON s.datid = d.oid "+
			"WHERE NOT d.datistemplate GROUP BY d.datname, d.datdba ORDER BY d.datname")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.DatabaseInfo, 0)
	for rows.Next() {
		var di model.DatabaseInfo
		if err := rows.Scan(&di.Name, &di.Owner, &di.Connections); err != nil {
			return nil, err
		}
		out = append(out, di)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := c.q.QueryRow(ctx, "SELECT COALESCE(pg_database_size($1), 0)",
			[]any{out[i].Name}, &out[i].SizeBytes); err != nil {
			out[i].SizeUnknown = true
			continue
		}
	}
	return out, nil
}

// Settings reads the small allowlisted set of configuration settings.
func (c *Collector) Settings(ctx context.Context) ([]model.ConfigSetting, error) {
	rows, err := c.q.Query(ctx,
		"SELECT name, setting, COALESCE(unit, ''), COALESCE(source, ''), COALESCE(context, '') "+
			"FROM pg_settings "+
			"WHERE name IN ('max_connections','shared_buffers','work_mem','maintenance_work_mem',"+
			"'wal_level','max_wal_size','log_min_duration_statement') "+
			"ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.ConfigSetting, 0)
	for rows.Next() {
		var s model.ConfigSetting
		if err := rows.Scan(&s.Name, &s.Value, &s.Unit, &s.Source, &s.Context); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Indexes lists user indexes with size, scans, uniqueness, and validity.
func (c *Collector) Indexes(ctx context.Context) ([]model.IndexInfo, error) {
	rows, err := c.q.Query(ctx,
		"SELECT COALESCE(s.schemaname, ''), COALESCE(s.relname, ''), COALESCE(s.indexrelname, ''), "+
			"COALESCE(pg_relation_size(s.indexrelid), 0)::bigint, COALESCE(s.idx_scan, 0)::bigint, "+
			"i.indisunique, i.indisvalid, COALESCE(pg_get_indexdef(s.indexrelid), '') "+
			"FROM pg_stat_user_indexes s "+
			"JOIN pg_index i ON s.indexrelid = i.indexrelid "+
			"ORDER BY 4 DESC, 3 ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.IndexInfo, 0)
	for rows.Next() {
		var idx model.IndexInfo
		if err := rows.Scan(&idx.Schema, &idx.Table, &idx.Index, &idx.SizeBytes, &idx.Scans,
			&idx.IsUnique, &idx.IsValid, &idx.Definition); err != nil {
			return nil, err
		}
		out = append(out, idx)
	}
	return out, rows.Err()
}

// XID collects transaction ID age and wraparound risk for databases and oldest tables.
func (c *Collector) XID(ctx context.Context, topTablesLimit int) (model.XIDReport, error) {
	var rep model.XIDReport
	rep.AutovacuumFreezeMaxAge = 200000000 // default fallback
	_ = c.q.QueryRow(ctx, "SELECT setting::bigint FROM pg_settings WHERE name = 'autovacuum_freeze_max_age'", nil, &rep.AutovacuumFreezeMaxAge)

	dbRows, err := c.q.Query(ctx,
		"SELECT datname, age(datfrozenxid)::bigint "+
			"FROM pg_database WHERE NOT datistemplate "+
			"ORDER BY 2 DESC, 1 ASC")
	if err != nil {
		return rep, err
	}
	defer dbRows.Close()

	rep.Databases = make([]model.DatabaseXIDInfo, 0)
	for dbRows.Next() {
		var d model.DatabaseXIDInfo
		if err := dbRows.Scan(&d.Datname, &d.Age); err != nil {
			return rep, err
		}
		d.RemainingXIDs = 2147483647 - d.Age
		if d.Age > 0 {
			d.PercentWraparound = (float64(d.Age) / 2147483647.0) * 100.0
		}
		rep.Databases = append(rep.Databases, d)
	}
	if err := dbRows.Err(); err != nil {
		return rep, err
	}

	if topTablesLimit <= 0 {
		topTablesLimit = 10
	}

	tblRows, err := c.q.Query(ctx,
		"SELECT COALESCE(n.nspname, ''), COALESCE(c.relname, ''), age(c.relfrozenxid)::bigint, "+
			"COALESCE(pg_total_relation_size(c.oid), 0)::bigint "+
			"FROM pg_class c "+
			"JOIN pg_namespace n ON n.oid = c.relnamespace "+
			"WHERE c.relkind IN ('r', 't', 'm') "+
			"AND n.nspname NOT IN ('pg_catalog', 'information_schema') "+
			"AND n.nspname !~ '^pg_temp' "+
			"ORDER BY 3 DESC, 2 ASC "+
			"LIMIT $1", topTablesLimit)
	if err != nil {
		return rep, err
	}
	defer tblRows.Close()

	rep.OldestTables = make([]model.TableXIDInfo, 0)
	for tblRows.Next() {
		var t model.TableXIDInfo
		if err := tblRows.Scan(&t.Schema, &t.Table, &t.Age, &t.SizeBytes); err != nil {
			return rep, err
		}
		rep.OldestTables = append(rep.OldestTables, t)
	}
	return rep, tblRows.Err()
}
