package cli

import (
	"context"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

var flagTopQueriesLimit int

var topQueriesCmd = &cobra.Command{
	Use:     "top-queries",
	Aliases: []string{"topq", "slow-queries", "statements"},
	Short:   "Inspect slow and resource-heavy queries from pg_stat_statements",
	Long: `Inspects normalized query execution statistics from pg_stat_statements.
Ranks queries by execution time, flagging slow queries (CRITICAL/WARNING: TOPQ-002),
queries writing temporary files to disk (WARNING: TOPQ-003), or alerting if pg_stat_statements
is missing (WARNING: TOPQ-001). Read-only.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReport(cmd, "top-queries", topQueriesRun)
	},
}

func init() {
	topQueriesCmd.Flags().IntVar(&flagTopQueriesLimit, "limit", 10, "Number of top statements by total execution time to inspect")
	addThresholds(topQueriesCmd, thresholdMeanQueryWarn, thresholdMeanQueryCrit)
}

func topQueriesRun(ctx context.Context, _ model.Connectivity, coll *collector.Collector, th evaluator.Thresholds) (any, []model.Finding, error) {
	topQueriesReport, err := coll.TopQueries(ctx, flagTopQueriesLimit)
	if err != nil {
		return nil, nil, err
	}
	findings := evaluator.EvaluateTopQueries(topQueriesReport, th)
	return report.TopQueriesData{TopQueries: topQueriesReport}, findings, nil
}
