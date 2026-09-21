package cli

import (
	"context"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

var locksCmd = &cobra.Command{
	Use:   "locks",
	Short: "Detect blocked sessions and their blocking sessions",
	Long: `Lists every blocked session together with its blocking session (wait
duration, states, and truncated query text) and produces a CRITICAL finding per
blocking pair over the threshold. Diagnostic only — dbakit never terminates
sessions or releases locks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReport(cmd, "locks", locksRun)
	},
}

func init() {
	addThresholds(locksCmd, thresholdLockWait)
}

func locksRun(ctx context.Context, _ model.Connectivity, coll *collector.Collector, th evaluator.Thresholds) (any, []model.Finding, error) {
	pairs, err := coll.Locks(ctx, flagMaxQueryLen)
	if err != nil {
		return nil, nil, err
	}
	findings := evaluator.EvaluateLocks(pairs, th.LockWait)
	return report.LocksData{BlockingPairs: pairs}, findings, nil
}
