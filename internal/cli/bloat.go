package cli

import (
	"context"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

var flagBloatLimit int

var bloatCmd = &cobra.Command{
	Use:     "bloat",
	Aliases: []string{"dead-tuples", "tables-bloat", "vacuum-needed"},
	Short:   "Inspect table bloat, dead tuples, and autovacuum status",
	Long: `Inspects user tables for dead tuple accumulation, table size, and autovacuum
lag from pg_stat_user_tables. Emits findings for high dead tuple ratios (BLOAT-001)
and autovacuum starvation on high-churn tables (BLOAT-002). Read-only.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReport(cmd, "bloat", bloatRun)
	},
}

func init() {
	bloatCmd.Flags().IntVar(&flagBloatLimit, "limit", 20, "Number of top tables by dead tuples to inspect")
	addThresholds(bloatCmd, thresholdBloatDeadWarn, thresholdBloatDeadCrit, thresholdBloatMinDead)
}

func bloatRun(ctx context.Context, _ model.Connectivity, coll *collector.Collector, th evaluator.Thresholds) (any, []model.Finding, error) {
	bloatReport, err := coll.Bloat(ctx, flagBloatLimit)
	if err != nil {
		return nil, nil, err
	}
	findings := evaluator.EvaluateBloat(bloatReport, th)
	return report.BloatData{Bloat: bloatReport}, findings, nil
}
