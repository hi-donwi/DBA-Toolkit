package evaluator

import (
	"fmt"
	"strings"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// EvaluateWAL evaluates WAL and checkpoint durability and performance metrics.
func EvaluateWAL(rep model.WALReport) []model.Finding {
	out := make([]model.Finding, 0)
	s := rep.Settings

	// WAL-001: fsync = off (CRITICAL)
	if strings.EqualFold(s.Fsync, "off") {
		out = append(out, finding("WAL-001", model.SeverityCritical,
			"PostgreSQL fsync is Disabled",
			"The fsync configuration parameter is turned off.",
			WithEvidence("Current setting: fsync = off"),
			WithRecommendation("Enable fsync (fsync = on) in postgresql.conf and reload or restart immediately. Running with fsync=off risks unrecoverable database corruption on power loss or host crash."),
			WithMetadata("fsync", s.Fsync)))
	} else if strings.EqualFold(s.Fsync, "on") {
		out = append(out, finding("WAL-001", model.SeverityPass,
			"PostgreSQL fsync is Enabled",
			"The fsync setting is active, ensuring WAL writes are flushed to disk.",
			WithEvidence("Current setting: fsync = on")))
	}

	// WAL-002: full_page_writes = off (CRITICAL)
	if strings.EqualFold(s.FullPageWrites, "off") {
		out = append(out, finding("WAL-002", model.SeverityCritical,
			"PostgreSQL full_page_writes is Disabled",
			"The full_page_writes configuration parameter is turned off.",
			WithEvidence("Current setting: full_page_writes = off"),
			WithRecommendation("Enable full_page_writes (full_page_writes = on) in postgresql.conf. Without full_page_writes, an operating system crash during a page write can cause torn pages that cannot be recovered by WAL."),
			WithMetadata("full_page_writes", s.FullPageWrites)))
	} else if strings.EqualFold(s.FullPageWrites, "on") {
		out = append(out, finding("WAL-002", model.SeverityPass,
			"PostgreSQL full_page_writes is Enabled",
			"The full_page_writes setting is active, protecting against torn pages.",
			WithEvidence("Current setting: full_page_writes = on")))
	}

	// WAL-003: synchronous_commit = off (WARNING)
	if strings.EqualFold(s.SynchronousCommit, "off") {
		out = append(out, finding("WAL-003", model.SeverityWarning,
			"PostgreSQL synchronous_commit is Disabled",
			"Transactions report commit before WAL records are flushed to durable storage.",
			WithEvidence("Current setting: synchronous_commit = off"),
			WithRecommendation("Ensure synchronous_commit = on for transactional consistency unless this database is explicitly an ephemeral cache or non-critical scratch environment."),
			WithMetadata("synchronous_commit", s.SynchronousCommit)))
	} else if s.SynchronousCommit != "" {
		out = append(out, finding("WAL-003", model.SeverityPass,
			"PostgreSQL synchronous_commit is Active",
			fmt.Sprintf("synchronous_commit is set to %q.", s.SynchronousCommit),
			WithEvidence(fmt.Sprintf("Current setting: synchronous_commit = %s", s.SynchronousCommit))))
	}

	// WAL-004: Frequent forced/requested checkpoints (WARNING)
	totalCheckpoints := rep.Checkpoint.CheckpointsTimed + rep.Checkpoint.CheckpointsReq
	if totalCheckpoints >= 5 {
		reqRatio := float64(rep.Checkpoint.CheckpointsReq) / float64(totalCheckpoints) * 100.0
		if reqRatio > 20.0 {
			out = append(out, finding("WAL-004", model.SeverityWarning,
				"Frequent Forced Checkpoints Detected",
				fmt.Sprintf("%.1f%% of checkpoints are requested/forced (%d of %d).", reqRatio, rep.Checkpoint.CheckpointsReq, totalCheckpoints),
				WithEvidence(fmt.Sprintf("Timed checkpoints: %d\nRequested checkpoints: %d\nTotal checkpoints: %d\nForced ratio: %.1f%%\nWrite time: %.2f ms\nSync time: %.2f ms",
					rep.Checkpoint.CheckpointsTimed, rep.Checkpoint.CheckpointsReq, totalCheckpoints, reqRatio, rep.Checkpoint.WriteTimeMs, rep.Checkpoint.SyncTimeMs)),
				WithRecommendation("Increase max_wal_size and set checkpoint_completion_target = 0.9 to prevent premature checkpoints triggered by WAL volume. Checkpoints should primarily occur due to checkpoint_timeout."),
				WithMetadata("checkpoints_timed", fmt.Sprintf("%d", rep.Checkpoint.CheckpointsTimed)),
				WithMetadata("checkpoints_req", fmt.Sprintf("%d", rep.Checkpoint.CheckpointsReq)),
				WithMetadata("forced_percent", fmt.Sprintf("%.1f", reqRatio))))
		} else {
			out = append(out, finding("WAL-004", model.SeverityPass,
				"Checkpoint Cadence Healthy",
				fmt.Sprintf("Checkpoints are predominantly timed (%.1f%% timed out of %d).", 100.0-reqRatio, totalCheckpoints),
				WithEvidence(fmt.Sprintf("Timed checkpoints: %d\nRequested checkpoints: %d", rep.Checkpoint.CheckpointsTimed, rep.Checkpoint.CheckpointsReq))))
		}
	}

	// WAL-005: track_io_timing = off (INFO)
	if strings.EqualFold(s.TrackIOTiming, "off") {
		out = append(out, finding("WAL-005", model.SeverityInfo,
			"track_io_timing is Disabled",
			"Block and WAL I/O read/write timings are not recorded.",
			WithEvidence("Current setting: track_io_timing = off"),
			WithRecommendation("Enable track_io_timing (track_io_timing = on) in postgresql.conf to capture accurate I/O latency metrics for query execution and WAL flushing."),
			WithMetadata("track_io_timing", s.TrackIOTiming)))
	} else if strings.EqualFold(s.TrackIOTiming, "on") {
		out = append(out, finding("WAL-005", model.SeverityPass,
			"track_io_timing is Enabled",
			"I/O read/write execution and sync timings are tracked.",
			WithEvidence("Current setting: track_io_timing = on")))
	}

	return out
}
