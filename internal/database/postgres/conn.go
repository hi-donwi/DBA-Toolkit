// Package postgres implements the database.Queryer seam for PostgreSQL via
// pgx/v5. It only opens and verifies connections and executes queries; all
// business and evaluation logic lives in collector and evaluator packages.
//
// The layer is intentionally thin and read-only: dbakit never issues DDL, DML,
// or administrative commands.
package postgres

import (
	"context"
	"fmt"

	"github.com/hi-donwi/DBA-Toolkit/internal/config"
	"github.com/hi-donwi/DBA-Toolkit/internal/database"
	"github.com/jackc/pgx/v5"
)

// DB wraps a live pgx connection.
type DB struct {
	conn *pgx.Conn
}

// Connect opens a PostgreSQL connection from cfg. The returned closed error is
// already scrubbed of credentials via config.Redact.
func Connect(ctx context.Context, cfg config.Config) (*DB, error) {
	connString := cfg.ConnectionString()
	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("postgres connection failed: %s", config.Redact(err.Error()))
	}
	return &DB{conn: conn}, nil
}

// Query implements database.Queryer.
func (d *DB) Query(ctx context.Context, sql string, args ...any) (database.Rows, error) {
	return d.conn.Query(ctx, sql, args...)
}

// QueryRow implements database.Queryer.
func (d *DB) QueryRow(ctx context.Context, sql string, args []any, dest ...any) error {
	return d.conn.QueryRow(ctx, sql, args...).Scan(dest...)
}

// Close closes the underlying connection.
func (d *DB) Close() error {
	return d.conn.Close(context.Background())
}
