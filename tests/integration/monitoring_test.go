//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func fixtureConnection(t *testing.T, ctx context.Context) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(ctx, "postgres://dbakit:dbakit_pw@127.0.0.1:5433/dbakit?sslmode=disable")
	if err != nil {
		t.Fatalf("cannot connect to the loopback fixture: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

func TestRestrictedMonitoringRole(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	admin := fixtureConnection(t, ctx)
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	role := fmt.Sprintf("monitor_%x", random[:8])
	password := fmt.Sprintf("fixture-%x", random[:])
	identifier := pgx.Identifier{role}.Sanitize()
	for _, statement := range []string{
		fmt.Sprintf("CREATE ROLE %s LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD '%s'", identifier, password),
		"GRANT pg_monitor TO " + identifier,
		"ALTER ROLE " + identifier + " SET default_transaction_read_only = on",
	} {
		if _, err := admin.Exec(ctx, statement); err != nil {
			t.Fatalf("monitor fixture setup failed: %v", err)
		}
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP ROLE "+identifier) })
	var superuser, createDB, createRole, replication bool
	if err := admin.QueryRow(ctx, "SELECT rolsuper, rolcreatedb, rolcreaterole, rolreplication FROM pg_roles WHERE rolname=$1", role).
		Scan(&superuser, &createDB, &createRole, &replication); err != nil {
		t.Fatal(err)
	}
	if superuser || createDB || createRole || replication {
		t.Fatal("monitoring role has administrative privileges")
	}
	monitor, err := pgx.Connect(ctx, fmt.Sprintf("postgres://%s:%s@127.0.0.1:5433/dbakit?sslmode=disable", role, password))
	if err != nil {
		t.Fatalf("monitoring connection failed: %v", err)
	}
	t.Cleanup(func() { _ = monitor.Close(context.Background()) })
	var readOnly string
	if err := monitor.QueryRow(ctx, "SHOW transaction_read_only").Scan(&readOnly); err != nil || readOnly != "on" {
		t.Fatalf("monitoring connection is not read-only: %q (%v)", readOnly, err)
	}
	for _, args := range [][]string{
		{"health"}, {"diagnose"}, {"sessions"}, {"locks"}, {"replication"},
		{"databases"}, {"config"}, {"indexes"}, {"xid"}, {"cache"},
		{"top-queries"}, {"bloat"}, {"wal"}, {"top", "--once"},
	} {
		t.Run(args[0], func(t *testing.T) {
			rep, raw := runWithEnv(t, []string{"DBAKIT_DB_USER=" + role, "DBAKIT_DB_PASSWORD=" + password}, args...)
			if strings.Contains(raw, password) {
				t.Fatal("monitoring password leaked into the report")
			}
			if !containsID(findingIDs(rep), "CONN-001") || containsSeverity(rep, "CRITICAL") {
				t.Fatalf("read-only monitoring failed: %s", raw)
			}
		})
	}
}

func TestBlockedSessionIsReportedWithoutCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin := fixtureConnection(t, ctx)
	blocker := fixtureConnection(t, ctx)
	waiter := fixtureConnection(t, ctx)
	table := pgx.Identifier{fmt.Sprintf("fixture_locks_%d", time.Now().UnixNano())}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE TABLE "+table+" (id integer PRIMARY KEY, value integer); INSERT INTO "+table+" VALUES (1, 0)"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = waiter.Close(context.Background())
		_ = blocker.Close(context.Background())
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = admin.Exec(cleanup, "DROP TABLE "+table)
	})
	left, err := blocker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer left.Rollback(context.Background())
	if _, err := left.Exec(ctx, "UPDATE "+table+" SET value = 1 WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	right, err := waiter.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := right.Exec(ctx, "UPDATE "+table+" SET value = 2 WHERE id = 1")
		done <- err
	}()
	for {
		var waiting bool
		if err := admin.QueryRow(ctx, "SELECT COALESCE(wait_event_type = 'Lock', false) FROM pg_stat_activity WHERE pid=$1", waiter.PgConn().PID()).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("fixture did not acquire its expected lock wait")
		case <-time.After(20 * time.Millisecond):
		}
	}
	time.Sleep(30 * time.Millisecond)
	rep, raw := run(t, "locks", "--lock-wait-threshold", "10ms")
	reported := false
	for _, finding := range findings(rep) {
		metadata, _ := finding["metadata"].(map[string]any)
		if finding["id"] == "LOCK-001" && finding["severity"] == "CRITICAL" &&
			metadata["blocked_pid"] == fmt.Sprint(waiter.PgConn().PID()) &&
			metadata["blocking_pid"] == fmt.Sprint(blocker.PgConn().PID()) {
			reported = true
		}
	}
	if !reported {
		t.Fatalf("blocked fixture was not reported: %s", raw)
	}
	select {
	case err := <-done:
		t.Fatalf("diagnostics released or cancelled the waiting query: %v", err)
	default:
	}
	if err := left.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := right.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
}
