// Package cli wires the dbakit command tree. The pipeline for every command is
// the same: resolve config → connect (measure latency) → collect → evaluate →
// sort+summarize → render. Connection failures are findings (exit 0) so the
// JSON report stays parseable for monitoring; operational errors (bad flags,
// a query the server refuses) exit non-zero with a message on stderr.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/config"
	"github.com/spf13/cobra"
)

// Version is stamped into builds via ldflags; the bare value is the MVP default.
var (
	Version   = "0.1.0"
	GitCommit = "dev"
	BuildDate = "unknown"
)

// Global flag variables.
var (
	flagDBHost     string
	flagDBPort     int
	flagDBName     string
	flagDBUser     string
	flagDBPassword string
	flagDBSSLMode  string
	flagDBURL      string

	flagJSON        bool
	flagNoColor     bool
	flagHidePass    bool
	flagMaxQueryLen int
	flagTimeout     time.Duration
)

// RootCmd is the base command.
var RootCmd = &cobra.Command{
	Use:   "dbakit",
	Short: "dbakit — PostgreSQL diagnostics, health checks, and troubleshooting (read-only)",
	Long: `dbakit is a small, read-only command-line toolkit for PostgreSQL
diagnostics, health checks, and troubleshooting. It collects normalized data,
runs a small set of rules, and reports findings as concise terminal output or
stable JSON for scripts and monitoring. It never modifies the database.`,
}

// ExitError carries an integer exit code for predictable CI behaviour.
type ExitError struct {
	Code int
	Msg  string
}

func (e *ExitError) Error() string { return e.Msg }

// Execute runs the root command and exits with the right code.
func Execute() {
	RootCmd.SilenceUsage = true
	RootCmd.SilenceErrors = true
	if err := RootCmd.Execute(); err != nil {
		if ee, ok := err.(*ExitError); ok {
			if ee.Msg != "" {
				fmt.Fprintln(os.Stderr, ee.Msg)
			}
			os.Exit(ee.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// commandContext returns a context bound to the global timeout and to
// SIGINT/SIGTERM, so a run against a wedged server terminates instead of
// hanging the operator's shell.
func commandContext() (context.Context, context.CancelFunc) {
	ctx, cancelSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if flagTimeout <= 0 {
		return ctx, cancelSignals
	}
	ctx, cancelTimeout := context.WithTimeout(ctx, flagTimeout)
	return ctx, func() {
		cancelTimeout()
		cancelSignals()
	}
}

// resolveConfig merges flags (highest precedence), DBAKIT_DB_* environment
// variables, a connection URL, and defaults.
func resolveConfig(cmd *cobra.Command) config.Config {
	cfg := config.Load()
	v := config.Values{}
	if cmd.Flags().Changed("db-host") {
		v.Host = &flagDBHost
	}
	if cmd.Flags().Changed("db-port") {
		v.Port = &flagDBPort
	}
	if cmd.Flags().Changed("db-name") {
		v.Name = &flagDBName
	}
	if cmd.Flags().Changed("db-user") {
		v.User = &flagDBUser
	}
	if cmd.Flags().Changed("db-password") {
		v.Password = &flagDBPassword
	}
	if cmd.Flags().Changed("db-sslmode") {
		v.SSLMode = &flagDBSSLMode
	}
	if cmd.Flags().Changed("db-url") {
		v.URL = &flagDBURL
	}
	return cfg.WithValues(v)
}

func init() {
	pf := RootCmd.PersistentFlags()
	pf.StringVar(&flagDBHost, "db-host", "", "PostgreSQL host (env DBAKIT_DB_HOST, default localhost)")
	pf.IntVar(&flagDBPort, "db-port", 0, "PostgreSQL port (env DBAKIT_DB_PORT, default 5432)")
	pf.StringVar(&flagDBName, "db-name", "", "Database name (env DBAKIT_DB_NAME, default postgres)")
	pf.StringVar(&flagDBUser, "db-user", "", "Database user (env DBAKIT_DB_USER, default postgres)")
	pf.StringVar(&flagDBPassword, "db-password", "", "Database password (env DBAKIT_DB_PASSWORD; never printed)")
	pf.StringVar(&flagDBSSLMode, "db-sslmode", "", "SSL mode (env DBAKIT_DB_SSLMODE, default prefer)")
	pf.StringVar(&flagDBURL, "db-url", "", "libpq connection URL/DSN; overrides the individual settings above (env DBAKIT_DB_URL)")

	pf.BoolVar(&flagJSON, "json", false, "Output the report as JSON")
	pf.BoolVar(&flagNoColor, "no-color", false, "Disable ANSI color output")
	pf.BoolVar(&flagHidePass, "hide-pass", false, "Hide PASS findings from terminal output")
	pf.IntVar(&flagMaxQueryLen, "max-query-length", 200, "Truncate query text in output to this many runes")
	pf.DurationVar(&flagTimeout, "timeout", 15*time.Second, "Overall deadline for a run (0 disables)")

	RootCmd.AddCommand(healthCmd)
	RootCmd.AddCommand(diagnoseCmd)
	RootCmd.AddCommand(sessionsCmd)
	RootCmd.AddCommand(locksCmd)
	RootCmd.AddCommand(replicationCmd)
	RootCmd.AddCommand(databasesCmd)
	RootCmd.AddCommand(indexesCmd)
	RootCmd.AddCommand(configCmd)
	RootCmd.AddCommand(rulesCmd)
	RootCmd.AddCommand(versionCmd)
}
