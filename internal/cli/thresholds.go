package cli

import (
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/spf13/cobra"
)

// Threshold flag variables, shared by the commands that expose them.
var (
	flagConnUsageWarn      float64
	flagConnUsageCrit      float64
	flagLongQuery          time.Duration
	flagLockWait           time.Duration
	flagLagWarn            time.Duration
	flagLagCrit            time.Duration
	flagUnusedIndexBytes   int64
	flagXIDWarnAge         int64
	flagXIDCritAge         int64
	flagCacheHitWarn       float64
	flagCacheHitCrit       float64
	flagMeanQueryWarn      time.Duration
	flagMeanQueryCrit      time.Duration
	flagBloatDeadWarnRatio float64
	flagBloatDeadCritRatio float64
	flagBloatMinDeadTuples int64
)

const (
	thresholdConnUsageWarn      = "conn-usage-warn"
	thresholdConnUsageCrit      = "conn-usage-critical"
	thresholdLongQuery          = "long-query-threshold"
	thresholdLockWait           = "lock-wait-threshold"
	thresholdLagWarn            = "lag-threshold"
	thresholdLagCrit            = "lag-critical"
	thresholdUnusedIndexMinSize = "unused-min-size"
	thresholdXIDWarnAge         = "warn-age"
	thresholdXIDCritAge         = "crit-age"
	thresholdCacheHitWarn       = "cache-hit-warn"
	thresholdCacheHitCrit       = "cache-hit-crit"
	thresholdMeanQueryWarn      = "mean-warn"
	thresholdMeanQueryCrit      = "mean-crit"
	thresholdBloatDeadWarn      = "dead-ratio-warn"
	thresholdBloatDeadCrit      = "dead-ratio-crit"
	thresholdBloatMinDead       = "min-dead-tuples"
)

// addThresholds registers the requested threshold flags on a command.
func addThresholds(cmd *cobra.Command, names ...string) {
	f := cmd.Flags()
	for _, n := range names {
		switch n {
		case thresholdConnUsageWarn:
			f.Float64Var(&flagConnUsageWarn, thresholdConnUsageWarn, 0, "Warn when connection usage exceeds this percent")
		case thresholdConnUsageCrit:
			f.Float64Var(&flagConnUsageCrit, thresholdConnUsageCrit, 0, "Critical when connection usage exceeds this percent")
		case thresholdLongQuery:
			f.DurationVar(&flagLongQuery, thresholdLongQuery, 0, "Flag queries running longer than this duration (e.g. 60s)")
		case thresholdLockWait:
			f.DurationVar(&flagLockWait, thresholdLockWait, 0, "Flag blocking waits longer than this duration (e.g. 5s)")
		case thresholdLagWarn:
			f.DurationVar(&flagLagWarn, thresholdLagWarn, 0, "Warn when replica replay lag exceeds this duration (e.g. 30s)")
		case thresholdLagCrit:
			f.DurationVar(&flagLagCrit, thresholdLagCrit, 0, "Critical when replica replay lag exceeds this duration (e.g. 120s)")
		case thresholdUnusedIndexMinSize:
			f.Int64Var(&flagUnusedIndexBytes, thresholdUnusedIndexMinSize, 0, "Minimum index size in bytes to flag as unused (default 10485760 / 10MB)")
		case thresholdXIDWarnAge:
			f.Int64Var(&flagXIDWarnAge, thresholdXIDWarnAge, 0, "Warn when database or table XID age exceeds this number (default 200000000 / 200M)")
		case thresholdXIDCritAge:
			f.Int64Var(&flagXIDCritAge, thresholdXIDCritAge, 0, "Critical when database XID age exceeds this number (default 1500000000 / 1.5B)")
		case thresholdCacheHitWarn:
			f.Float64Var(&flagCacheHitWarn, thresholdCacheHitWarn, 0, "Warn when buffer cache hit ratio falls below this percent (default 95.0)")
		case thresholdCacheHitCrit:
			f.Float64Var(&flagCacheHitCrit, thresholdCacheHitCrit, 0, "Critical when buffer cache hit ratio falls below this percent (default 90.0)")
		case thresholdMeanQueryWarn:
			f.DurationVar(&flagMeanQueryWarn, thresholdMeanQueryWarn, 0, "Warn when mean query execution time exceeds this duration (default 500ms)")
		case thresholdMeanQueryCrit:
			f.DurationVar(&flagMeanQueryCrit, thresholdMeanQueryCrit, 0, "Critical when mean query execution time exceeds this duration (default 2s)")
		case thresholdBloatDeadWarn:
			f.Float64Var(&flagBloatDeadWarnRatio, thresholdBloatDeadWarn, 0, "Warn when table dead tuple ratio exceeds this percent (default 20.0)")
		case thresholdBloatDeadCrit:
			f.Float64Var(&flagBloatDeadCritRatio, thresholdBloatDeadCrit, 0, "Critical when table dead tuple ratio exceeds this percent (default 50.0)")
		case thresholdBloatMinDead:
			f.Int64Var(&flagBloatMinDeadTuples, thresholdBloatMinDead, 0, "Minimum dead tuples to flag table bloat (default 10000)")
		}
	}
}

// resolveThresholds applies any threshold flags the user set over the defaults.
func resolveThresholds(cmd *cobra.Command) evaluator.Thresholds {
	th := evaluator.DefaultThresholds()
	f := cmd.Flags()
	if f.Changed(thresholdConnUsageWarn) && flagConnUsageWarn > 0 {
		th.ConnUsageWarnPercent = flagConnUsageWarn
	}
	if f.Changed(thresholdConnUsageCrit) && flagConnUsageCrit > 0 {
		th.ConnUsageCritPercent = flagConnUsageCrit
	}
	if f.Changed(thresholdLongQuery) && flagLongQuery > 0 {
		th.LongQuery = flagLongQuery
	}
	if f.Changed(thresholdLockWait) && flagLockWait > 0 {
		th.LockWait = flagLockWait
	}
	if f.Changed(thresholdLagWarn) && flagLagWarn > 0 {
		th.ReplLagWarn = flagLagWarn
	}
	if f.Changed(thresholdLagCrit) && flagLagCrit > 0 {
		th.ReplLagCrit = flagLagCrit
	}
	if f.Changed(thresholdUnusedIndexMinSize) && flagUnusedIndexBytes > 0 {
		th.UnusedIndexMinSize = flagUnusedIndexBytes
	}
	if f.Changed(thresholdXIDWarnAge) && flagXIDWarnAge > 0 {
		th.XIDWarnAge = flagXIDWarnAge
	}
	if f.Changed(thresholdXIDCritAge) && flagXIDCritAge > 0 {
		th.XIDCritAge = flagXIDCritAge
	}
	if f.Changed(thresholdCacheHitWarn) && flagCacheHitWarn > 0 {
		th.CacheHitWarnPercent = flagCacheHitWarn
	}
	if f.Changed(thresholdCacheHitCrit) && flagCacheHitCrit > 0 {
		th.CacheHitCritPercent = flagCacheHitCrit
	}
	if f.Changed(thresholdMeanQueryWarn) && flagMeanQueryWarn > 0 {
		th.MeanQueryWarn = flagMeanQueryWarn
	}
	if f.Changed(thresholdMeanQueryCrit) && flagMeanQueryCrit > 0 {
		th.MeanQueryCrit = flagMeanQueryCrit
	}
	if f.Changed(thresholdBloatDeadWarn) && flagBloatDeadWarnRatio > 0 {
		th.BloatDeadWarnRatio = flagBloatDeadWarnRatio
	}
	if f.Changed(thresholdBloatDeadCrit) && flagBloatDeadCritRatio > 0 {
		th.BloatDeadCritRatio = flagBloatDeadCritRatio
	}
	if f.Changed(thresholdBloatMinDead) && flagBloatMinDeadTuples > 0 {
		th.BloatMinDeadTuples = flagBloatMinDeadTuples
	}
	return th
}
