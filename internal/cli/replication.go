package cli

import (
	"context"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

var replicationCmd = &cobra.Command{
	Use:     "replication",
	Aliases: []string{"repl"},
	Short:   "Inspect PostgreSQL replication role and replay lag",
	Long: `Reports whether the server is a primary or a standby, lists connected
standbys with their state and replay lag (from a primary), or shows the local
WAL replay lag (on a standby). Detection only — no failover, promotion, or
repair.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReport(cmd, "replication", replicationRun)
	},
}

func init() {
	addThresholds(replicationCmd, thresholdLagWarn, thresholdLagCrit)
}

func replicationRun(ctx context.Context, _ model.Connectivity, coll *collector.Collector, th evaluator.Thresholds) (any, []model.Finding, error) {
	repl, err := coll.Replication(ctx)
	if err != nil {
		return nil, nil, err
	}
	findings := evaluator.EvaluateReplication(repl, th)
	return report.ReplicationData{Replication: repl}, findings, nil
}
