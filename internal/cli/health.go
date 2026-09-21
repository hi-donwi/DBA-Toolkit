package cli

import (
	"context"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check PostgreSQL connectivity and basic database health",
	Long: `Collects connectivity (version, database, latency), connection usage,
uptime, and current database size, then evaluates them against the configured
thresholds. Read-only.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReport(cmd, "health", healthRun)
	},
}

func init() {
	addThresholds(healthCmd, thresholdConnUsageWarn, thresholdConnUsageCrit)
}

func healthRun(ctx context.Context, conn model.Connectivity, coll *collector.Collector, th evaluator.Thresholds) (any, []model.Finding, error) {
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

	findings := evaluator.EvaluateUsage(health.TotalConnections, health.ActiveConnections, health.MaxConnections, th)
	return report.HealthData{Connectivity: conn, Health: health}, findings, nil
}
