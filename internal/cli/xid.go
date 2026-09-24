package cli

import (
	"context"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

var flagTopTables int

var xidCmd = &cobra.Command{
	Use:     "xid",
	Aliases: []string{"wraparound", "freeze", "vacuum"},
	Short:   "Analyze transaction ID age and wraparound headroom",
	Long: `Inspects transaction ID (XID) age across all databases and identifies
the oldest tables holding back the autovacuum freeze horizon. Emits findings for
emergency wraparound danger (CRITICAL: XID-001), databases exceeding autovacuum
freeze threshold (WARNING: XID-002), and lagging tables (WARNING: XID-003). Read-only.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReport(cmd, "xid", xidRun)
	},
}

func init() {
	xidCmd.Flags().IntVar(&flagTopTables, "top-tables", 10, "Number of oldest tables to inspect for freeze horizon")
	addThresholds(xidCmd, thresholdXIDWarnAge, thresholdXIDCritAge)
}

func xidRun(ctx context.Context, _ model.Connectivity, coll *collector.Collector, th evaluator.Thresholds) (any, []model.Finding, error) {
	xidReport, err := coll.XID(ctx, flagTopTables)
	if err != nil {
		return nil, nil, err
	}
	findings := evaluator.EvaluateXID(xidReport, th)
	return report.XIDData{XID: xidReport}, findings, nil
}
