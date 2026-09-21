package cli

import (
	"context"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

var databasesCmd = &cobra.Command{
	Use:     "databases",
	Aliases: []string{"dbs"},
	Short:   "List databases with size, owner, and connection counts",
	Long: `Lists non-template databases with their owner, size (fetched
per-database; unreadable sizes are reported as unknown), and current connection
count. Read-only.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReport(cmd, "databases", databasesRun)
	},
}

func databasesRun(ctx context.Context, _ model.Connectivity, coll *collector.Collector, _ evaluator.Thresholds) (any, []model.Finding, error) {
	dbs, err := coll.Databases(ctx)
	if err != nil {
		return nil, nil, err
	}
	findings := evaluator.EvaluateDatabases(dbs)
	return report.DatabasesData{Databases: dbs}, findings, nil
}
