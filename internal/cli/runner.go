package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/config"
	"github.com/hi-donwi/DBA-Toolkit/internal/database/postgres"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

// runFunc collects data and findings for one command against a live collector.
// The evaluator/collector separation means runFunc must stay free of rendering.
type runFunc func(ctx context.Context, conn model.Connectivity, coll *collector.Collector, th evaluator.Thresholds) (any, []model.Finding, error)

// connectProbe opens the connection, measures latency, and collects the basic
// connectivity facts (version, current database). On failure the returned
// Connectivity carries a redacted error and err is non-nil.
func connectProbe(ctx context.Context, cfg config.Config) (model.Connectivity, *postgres.DB, error) {
	var conn model.Connectivity
	start := time.Now()
	db, err := postgres.Connect(ctx, cfg)
	if err != nil {
		conn.Error = config.Redact(err.Error())
		return conn, nil, err
	}
	conn.LatencySeconds = time.Since(start).Seconds()
	coll := collector.New(db)
	probe, perr := coll.Connectivity(ctx)
	if perr != nil {
		db.Close()
		conn.Error = config.Redact(perr.Error())
		return conn, nil, perr
	}
	probe.LatencySeconds = conn.LatencySeconds
	return probe, db, nil
}

// runReport executes the shared pipeline for one command and writes output.
func runReport(cmd *cobra.Command, command string, run runFunc) error {
	cfg := resolveConfig(cmd)
	th := resolveThresholds(cmd)
	opts := report.Options{NoColor: flagNoColor, HidePass: flagHidePass}

	ctx, cancel := commandContext()
	defer cancel()

	start := time.Now()
	rep := &model.Report{
		Tool:      "dbakit",
		Version:   Version,
		Command:   command,
		Timestamp: start,
		Database:  model.Database{Engine: "postgresql", Name: cfg.Name},
	}

	conn, db, err := connectProbe(ctx, cfg)
	if err != nil {
		// A failed connection is a finding, not a crash: the JSON report stays
		// parseable so monitoring can react to a CRITICAL connectivity result.
		rep.Database.Version = ""
		rep.Data = report.HealthData{Connectivity: conn}
		rep.Findings = evaluator.EvaluateConnectivity(conn)
	} else {
		rep.Database.Version = conn.Version
		defer db.Close()
		data, findings, runErr := run(ctx, conn, collector.New(db), th)
		if runErr != nil {
			return fmt.Errorf("%s: %s", command, config.Redact(runErr.Error()))
		}
		findings = append(evaluator.EvaluateConnectivity(conn), findings...)
		rep.Data = data
		rep.Findings = findings
	}

	evaluator.Sort(rep.Findings)
	rep.Summary = evaluator.Summarize(rep.Findings)
	rep.Duration = time.Since(start).Round(time.Millisecond).String()

	if flagJSON {
		return report.JSON(os.Stdout, rep)
	}
	return report.Human(os.Stdout, rep, opts)
}
