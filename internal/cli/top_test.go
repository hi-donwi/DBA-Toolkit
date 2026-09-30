package cli

import (
	"testing"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

func TestTopCmdStructure(t *testing.T) {
	if topCmd.Use != "top" {
		t.Errorf("expected Use 'top', got %s", topCmd.Use)
	}
	aliases := map[string]bool{}
	for _, a := range topCmd.Aliases {
		aliases[a] = true
	}
	for _, want := range []string{"pgtop", "monitor", "live"} {
		if !aliases[want] {
			t.Errorf("missing alias %s", want)
		}
	}
	for _, wantFlag := range []string{
		"refresh", "limit", "sort", "once", "count",
		"conn-usage-warn", "conn-usage-critical",
		"long-query-threshold", "lock-wait-threshold",
		"lag-threshold", "lag-critical",
	} {
		if topCmd.Flags().Lookup(wantFlag) == nil {
			t.Errorf("missing flag %s", wantFlag)
		}
	}
}

func TestSortSessions(t *testing.T) {
	sessions := []model.Session{
		{PID: 105, User: "charlie", State: "idle", DurationSeconds: 10.0},
		{PID: 102, User: "alice", State: "active", DurationSeconds: 50.0},
		{PID: 101, User: "bob", State: "active", DurationSeconds: 5.0},
	}

	// Test default duration sort (descending)
	sortSessions(sessions, "duration")
	if sessions[0].PID != 102 || sessions[1].PID != 105 || sessions[2].PID != 101 {
		t.Errorf("duration sort failed: %+v", sessions)
	}

	// Test PID sort (ascending)
	sortSessions(sessions, "pid")
	if sessions[0].PID != 101 || sessions[1].PID != 102 || sessions[2].PID != 105 {
		t.Errorf("pid sort failed: %+v", sessions)
	}

	// Test user sort (ascending)
	sortSessions(sessions, "user")
	if sessions[0].User != "alice" || sessions[1].User != "bob" || sessions[2].User != "charlie" {
		t.Errorf("user sort failed: %+v", sessions)
	}

	// Test state sort (ascending)
	sortSessions(sessions, "state")
	if sessions[0].State != "active" || sessions[1].State != "active" || sessions[2].State != "idle" {
		t.Errorf("state sort failed: %+v", sessions)
	}
}

func TestRunTopOnceFlag(t *testing.T) {
	// Verify default flag values
	if flagTopRefresh != 2*time.Second {
		t.Errorf("expected default refresh 2s, got %v", flagTopRefresh)
	}
	if flagTopLimit != 20 {
		t.Errorf("expected default limit 20, got %d", flagTopLimit)
	}
	if flagTopSort != "duration" {
		t.Errorf("expected default sort 'duration', got %s", flagTopSort)
	}
}
