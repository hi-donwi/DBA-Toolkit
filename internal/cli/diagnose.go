package cli

import (
	"context"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Run the full diagnostic battery: health, long queries, locks, replication",
	Long: `diagnose runs every MVP check against the target database: connectivity,
health and connection usage, long-running queries, locks/blocking, and
replication lag. Output includes a summary header and the normalized findings.
Read-only: it never terminates sessions, alters configuration, or changes
replication.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReport(cmd, "diagnose", diagnoseRun)
	},
}

func init() {
	addThresholds(diagnoseCmd,
		thresholdConnUsageWarn, thresholdConnUsageCrit,
		thresholdLongQuery, thresholdLockWait,
		thresholdLagWarn, thresholdLagCrit,
	)
}

func diagnoseRun(ctx context.Context, conn model.Connectivity, coll *collector.Collector, th evaluator.Thresholds) (any, []model.Finding, error) {
	findings := make([]model.Finding, 0, 8)

	health, err := coll.Health(ctx)
	if err != nil {
		return nil, nil, err
	}
	total, active, err := coll.Usage(ctx)
	if err != nil {
		return nil, nil, err
	}
	health.TotalConnections = total
	health.ActiveConnections = active
	if health.MaxConnections > 0 {
		health.ConnectionUsage = float64(total) / float64(health.MaxConnections) * 100
	}
	findings = append(findings, evaluator.EvaluateUsage(health.TotalConnections, health.ActiveConnections, health.MaxConnections, th)...)

	longQueries, err := coll.LongQueries(ctx, th.LongQuery, flagMaxQueryLen)
	if err != nil {
		return nil, nil, err
	}
	findings = append(findings, evaluator.EvaluateLongQueries(longQueries, th.LongQuery)...)

	pairs, err := coll.Locks(ctx, flagMaxQueryLen)
	if err != nil {
		return nil, nil, err
	}
	findings = append(findings, evaluator.EvaluateLocks(pairs, th.LockWait)...)

	repl, err := coll.Replication(ctx)
	if err != nil {
		return nil, nil, err
	}
	findings = append(findings, evaluator.EvaluateReplication(repl, th)...)

	data := report.DiagnoseData{
		Connectivity: conn,
		Health:       health,
		LongQueries:  longQueries,
		Locks:        pairs,
		Replication:  &repl,
	}
	return data, findings, nil
}
