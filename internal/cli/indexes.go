package cli

import (
	"context"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

var indexesCmd = &cobra.Command{
	Use:     "indexes",
	Aliases: []string{"idx", "index"},
	Short:   "List user indexes with size, scan count, and validity",
	Long: `Lists user indexes with their table, size, scan count, uniqueness,
and validity. Emits findings for invalid indexes (CRITICAL: IDX-002) and
unused indexes consuming significant storage (WARNING: IDX-001). Read-only.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReport(cmd, "indexes", indexesRun)
	},
}

func init() {
	addThresholds(indexesCmd, thresholdUnusedIndexMinSize)
}

func indexesRun(ctx context.Context, _ model.Connectivity, coll *collector.Collector, th evaluator.Thresholds) (any, []model.Finding, error) {
	indexes, err := coll.Indexes(ctx)
	if err != nil {
		return nil, nil, err
	}
	findings := evaluator.EvaluateIndexes(indexes, th)
	return report.IndexesData{Indexes: indexes}, findings, nil
}
