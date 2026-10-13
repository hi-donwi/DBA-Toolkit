package cli

import (
	"context"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

var walCmd = &cobra.Command{
	Use:     "wal",
	Aliases: []string{"checkpoints", "durability", "checkpoint"},
	Short:   "Inspect WAL generation, commit durability settings, and checkpoint cadence",
	Long: `Inspects WAL volume, durability parameters (fsync, synchronous_commit, full_page_writes,
wal_sync_method, track_io_timing), and checkpoint frequency (pg_stat_checkpointer on PG17+,
pg_stat_bgwriter on earlier versions). Emits findings for disabled fsync (CRITICAL: WAL-001),
disabled full_page_writes (CRITICAL: WAL-002), asynchronous commits (WARNING: WAL-003),
and excessive forced checkpoints (WARNING: WAL-004). Read-only.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReport(cmd, "wal", walRun)
	},
}

func walRun(ctx context.Context, conn model.Connectivity, coll *collector.Collector, th evaluator.Thresholds) (any, []model.Finding, error) {
	walRep, err := coll.WAL(ctx, conn.VersionNum)
	if err != nil {
		return nil, nil, err
	}
	findings := evaluator.EvaluateWAL(walRep)
	return report.WALData{WAL: walRep}, findings, nil
}
