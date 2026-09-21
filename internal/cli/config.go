package cli

import (
	"context"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Report a small set of PostgreSQL configuration settings",
	Long: `Reports facts and obvious warnings for a small allowlist of settings
(max_connections, shared_buffers, work_mem, maintenance_work_mem, wal_level,
max_wal_size, log_min_duration_statement). It reports facts, not "optimal"
values — every workload differs. Read-only.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReport(cmd, "config", configRun)
	},
}

func configRun(ctx context.Context, _ model.Connectivity, coll *collector.Collector, _ evaluator.Thresholds) (any, []model.Finding, error) {
	settings, err := coll.Settings(ctx)
	if err != nil {
		return nil, nil, err
	}
	findings := evaluator.EvaluateSettings(settings)
	return report.ConfigData{Settings: settings}, findings, nil
}
