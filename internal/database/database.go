// Package database defines the small seam between PostgreSQL access and the
// rest of the toolkit. Collectors depend on the Queryer interface instead of a
// concrete driver, so their row-parsing is unit-testable against fixtures and
// a future engine (another driver) can implement the same interface.
package database

import "context"

// Rows is the subset of query result handling collectors need. Close mirrors
// pgx.Rows, which returns nothing on success.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// Queryer executes read-only queries. The MVP only ever issues SELECT /
// SHOW statements; the interface exists for testability and future engines.
type Queryer interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	// QueryRow scans a single row into dest. args are query parameters; the
	// remaining variadic dest are scan targets.
	QueryRow(ctx context.Context, sql string, args []any, dest ...any) error
	Close() error
}
