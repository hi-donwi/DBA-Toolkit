package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/collector"
	"github.com/hi-donwi/DBA-Toolkit/internal/config"
	"github.com/hi-donwi/DBA-Toolkit/internal/evaluator"
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
	"github.com/hi-donwi/DBA-Toolkit/internal/report"
	"github.com/spf13/cobra"
)

var (
	flagTopRefresh time.Duration
	flagTopLimit   int
	flagTopSort    string
	flagTopOnce    bool
	flagTopCount   int
)

var topCmd = &cobra.Command{
	Use:     "top",
	Aliases: []string{"pgtop", "monitor", "live"},
	Short:   "Live interactive monitor of PostgreSQL activity and health",
	Long: `Provides a real-time terminal dashboard of PostgreSQL health, active connections,
running queries, lock contention, and replication lag. Refreshes periodically. Read-only.`,
	RunE: runTop,
}

func init() {
	topCmd.Flags().DurationVarP(&flagTopRefresh, "refresh", "r", 2*time.Second, "Refresh interval (e.g. 1s, 2s, 5s)")
	topCmd.Flags().IntVarP(&flagTopLimit, "limit", "l", 20, "Maximum number of active sessions to display")
	topCmd.Flags().StringVarP(&flagTopSort, "sort", "s", "duration", "Sort active sessions by: duration, pid, state, user")
	topCmd.Flags().BoolVar(&flagTopOnce, "once", false, "Print a single snapshot and exit immediately")
	topCmd.Flags().IntVarP(&flagTopCount, "count", "n", 0, "Number of snapshots to display before exiting (0 for continuous)")

	addThresholds(topCmd, thresholdConnUsageWarn, thresholdConnUsageCrit,
		thresholdLongQuery, thresholdLockWait, thresholdLagWarn, thresholdLagCrit)
}

func topRun(ctx context.Context, conn model.Connectivity, coll *collector.Collector, th evaluator.Thresholds) (any, []model.Finding, error) {
	snap, err := coll.TopSnapshot(ctx, flagMaxQueryLen)
	if err != nil {
		return nil, nil, err
	}

	sortSessions(snap.Sessions, flagTopSort)
	if flagTopLimit > 0 && len(snap.Sessions) > flagTopLimit {
		snap.Sessions = snap.Sessions[:flagTopLimit]
	}

	var findings []model.Finding
	findings = append(findings, evaluator.EvaluateUsage(snap.Health.TotalConnections, snap.Health.ActiveConnections, snap.Health.MaxConnections, th)...)
	findings = append(findings, evaluator.EvaluateLongQueries(snap.Sessions, th.LongQuery)...)
	findings = append(findings, evaluator.EvaluateLocks(snap.Locks, th.LockWait)...)
	findings = append(findings, evaluator.EvaluateReplication(snap.Replication, th)...)
	evaluator.Sort(findings)

	return report.TopData{
		Connectivity: conn,
		Snapshot:     snap,
	}, findings, nil
}

func sortSessions(sessions []model.Session, mode string) {
	switch strings.ToLower(mode) {
	case "pid":
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].PID < sessions[j].PID
		})
	case "state":
		sort.Slice(sessions, func(i, j int) bool {
			if sessions[i].State == sessions[j].State {
				return sessions[i].DurationSeconds > sessions[j].DurationSeconds
			}
			return sessions[i].State < sessions[j].State
		})
	case "user":
		sort.Slice(sessions, func(i, j int) bool {
			if sessions[i].User == sessions[j].User {
				return sessions[i].DurationSeconds > sessions[j].DurationSeconds
			}
			return sessions[i].User < sessions[j].User
		})
	default: // "duration"
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].DurationSeconds > sessions[j].DurationSeconds
		})
	}
}

func isCharDevice(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func runTop(cmd *cobra.Command, args []string) error {
	// If one-shot mode or count == 1, execute standard runReport
	if flagTopOnce || flagTopCount == 1 {
		return runReport(cmd, "top", topRun)
	}

	cfg := resolveConfig(cmd)
	th := resolveThresholds(cmd)
	opts := report.Options{NoColor: flagNoColor, HidePass: flagHidePass}

	// Context for graceful cancellation via SIGINT or SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	conn, db, err := connectProbe(ctx, cfg)
	if err != nil {
		// Output connectivity failure finding
		rep := &model.Report{
			Tool:      "dbakit",
			Version:   Version,
			Command:   "top",
			Timestamp: time.Now(),
			Database:  model.Database{Engine: "postgresql", Name: cfg.Name},
			Data:      report.HealthData{Connectivity: conn},
			Findings:  evaluator.EvaluateConnectivity(conn),
		}
		evaluator.Sort(rep.Findings)
		rep.Summary = evaluator.Summarize(rep.Findings)
		if flagJSON {
			return report.JSON(os.Stdout, rep)
		}
		return report.Human(os.Stdout, rep, opts)
	}
	defer db.Close()
	coll := collector.New(db)

	ticker := time.NewTicker(flagTopRefresh)
	defer ticker.Stop()

	iterations := 0
	interactive := isCharDevice(os.Stdout) && !flagJSON

	for {
		start := time.Now()
		rep := &model.Report{
			Tool:      "dbakit",
			Version:   Version,
			Command:   "top",
			Timestamp: start,
			Database:  model.Database{Engine: "postgresql", Name: cfg.Name, Version: conn.Version},
		}

		data, findings, runErr := topRun(ctx, conn, coll, th)
		if runErr != nil {
			return fmt.Errorf("top: %s", config.Redact(runErr.Error()))
		}
		findings = append(evaluator.EvaluateConnectivity(conn), findings...)
		evaluator.Sort(findings)
		rep.Data = data
		rep.Findings = findings
		rep.Summary = evaluator.Summarize(rep.Findings)
		rep.Duration = time.Since(start).Round(time.Millisecond).String()

		if flagJSON {
			if err := report.JSON(os.Stdout, rep); err != nil {
				return err
			}
		} else {
			if interactive {
				// Clear screen and home cursor
				fmt.Print("\033[H\033[2J")
			}
			if err := report.Human(os.Stdout, rep, opts); err != nil {
				return err
			}
		}

		iterations++
		if flagTopCount > 0 && iterations >= flagTopCount {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Recalculate latency on each tick if possible
			if lat, err := coll.Pinger(ctx); err == nil {
				conn.LatencySeconds = lat.Seconds()
			}
		}
	}
}
