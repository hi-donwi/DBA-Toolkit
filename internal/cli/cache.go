package cli

import (
	"context"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

var flagCacheLimit int

var cacheCmd = &cobra.Command{
	Use:     "cache",
	Aliases: []string{"buffer", "hit-ratio", "buffercache"},
	Short:   "Analyze PostgreSQL buffer cache hit ratios",
	Long: `Measures buffer cache hit ratios across the current database, user tables,
and indexes from pg_statio_user_tables. Emits findings for low overall cache hit ratio
(CRITICAL: CACHE-001, WARNING: CACHE-001) and tables with high physical disk reads (CACHE-002).
Read-only.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReport(cmd, "cache", cacheRun)
	},
}

func init() {
	cacheCmd.Flags().IntVar(&flagCacheLimit, "limit", 20, "Number of top tables by disk reads to inspect")
	addThresholds(cacheCmd, thresholdCacheHitWarn, thresholdCacheHitCrit)
}

func cacheRun(ctx context.Context, _ model.Connectivity, coll *collector.Collector, th evaluator.Thresholds) (any, []model.Finding, error) {
	cacheReport, err := coll.Cache(ctx, flagCacheLimit)
	if err != nil {
		return nil, nil, err
	}
	findings := evaluator.EvaluateCache(cacheReport, th)
	return report.CacheData{Cache: cacheReport}, findings, nil
}
