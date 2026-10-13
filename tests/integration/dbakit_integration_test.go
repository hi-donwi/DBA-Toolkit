//go:build integration

// Package integration runs the real dbakit binary against a live PostgreSQL
// (see docker-compose.yml). These tests are opt-in: they require Docker and a
// running server, so they never run as part of the default `go test ./...`.
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var env = []string{
	"DBAKIT_DB_HOST=localhost",
	"DBAKIT_DB_PORT=5433",
	"DBAKIT_DB_NAME=dbakit",
	"DBAKIT_DB_USER=dbakit",
	"DBAKIT_DB_PASSWORD=dbakit_pw",
	"DBAKIT_DB_SSLMODE=disable",
}

var binary string

// TestMain builds the real binary once for the whole suite.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "dbakit-integration-")
	if err != nil {
		fmt.Printf("cannot create integration build directory: %v\n", err)
		os.Exit(1)
	}
	binary = filepath.Join(dir, "dbakit")
	root := repoRoot()
	cmd := exec.Command("go", "build", "-tags", "integration", "-o", binary, "./cmd/dbakit")
	cmd.Dir = root
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("build failed (repo root: %s): %v\n%s\n", root, err, out)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// repoRoot walks up from the working directory to find the directory containing
// go.mod, which is the repository root. The previous implementation used
// filepath.Dir(wd) which returned tests/ instead of the actual root.
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic("cannot determine working directory: " + err.Error())
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("repoRoot: go.mod not found in any parent directory")
		}
		dir = parent
	}
}

func isolatedEnv(extra ...string) []string {
	base := make([]string, 0, len(os.Environ())+len(env)+len(extra)+1)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "DBAKIT_DB_") || strings.HasPrefix(item, "PG") {
			continue
		}
		base = append(base, item)
	}
	base = append(base, "DBAKIT_DB_URL=")
	base = append(base, env...)
	return append(base, extra...)
}

func run(t *testing.T, args ...string) (map[string]any, string) {
	t.Helper()
	return runWithEnv(t, nil, args...)
}

func runWithEnv(t *testing.T, extra []string, args ...string) (map[string]any, string) {
	t.Helper()
	cmd := exec.Command(binary, append(args, "--json", "--timeout", "30s")...)
	cmd.Env = isolatedEnv(extra...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("dbakit %v failed: %v\n%s", args, err, out.String())
	}
	var rep map[string]any
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("dbakit %v did not emit parseable JSON: %v\n%s", args, err, out.String())
	}
	return rep, out.String()
}

func TestConnectivity(t *testing.T) {
	rep, raw := run(t, "health")
	if got := rep["command"]; got != "health" {
		t.Fatalf("command = %v", got)
	}
	db := rep["database"].(map[string]any)
	if db["version"] == "" || db["engine"] != "postgresql" {
		t.Fatalf("database info wrong: %v", db)
	}
	summary := rep["summary"].(map[string]any)
	if summary["critical"] != float64(0) {
		t.Fatalf("connected health should have no critical findings: %s", raw)
	}
	ids := findingIDs(rep)
	if !containsID(ids, "CONN-001") {
		t.Fatalf("missing CONN-001 PASS finding: %s", raw)
	}
}

func TestAllReadOnlyCommandsAgainstServer(t *testing.T) {
	for _, cmd := range []string{"diagnose", "sessions", "locks", "replication", "databases", "config", "indexes", "xid", "cache", "top-queries", "bloat", "wal"} {
		rep, raw := run(t, cmd)
		if rep["command"] != cmd {
			t.Fatalf("%s: command=%v", cmd, rep["command"])
		}
		for _, title := range []string{"sup3r-secret-pw-abc", "dbakit_pw"} {
			if bytes.Contains([]byte(raw), []byte(title)) {
				t.Fatalf("%s output leaked the password", cmd)
			}
		}
		if !containsID(findingIDs(rep), "CONN-001") {
			t.Fatalf("%s missing connectivity finding: %s", cmd, raw)
		}
	}
}

func TestTopOnceAgainstServer(t *testing.T) {
	rep, raw := run(t, "top", "--once")
	if rep["command"] != "top" {
		t.Fatalf("command = %v\n%s", rep["command"], raw)
	}
	if bytes.Contains([]byte(raw), []byte("dbakit_pw")) {
		t.Fatal("top output leaked the password")
	}
}

func TestURLReportsActualDatabase(t *testing.T) {
	rep, raw := runWithEnv(t, []string{"DBAKIT_DB_NAME=postgres", "DBAKIT_DB_URL=postgres://dbakit:dbakit_pw@127.0.0.1:5433/dbakit?sslmode=disable"}, "health")
	if rep["database"].(map[string]any)["name"] != "dbakit" {
		t.Fatalf("report identity must match the actual database: %s", raw)
	}
}

func TestReplicaFixtureDoesNotFalseAlarm(t *testing.T) {
	if os.Getenv("DBAKIT_RUN_REPLICA") != "1" {
		t.Skip("set DBAKIT_RUN_REPLICA=1 with the replica compose fixture")
	}
	primary, primaryRaw := runWithEnv(t, []string{"DBAKIT_DB_PORT=5434"}, "replication")
	if roleOf(primary) != "primary" {
		t.Fatalf("primary role mismatch: %s", primaryRaw)
	}
	data := primary["data"].(map[string]any)["replication"].(map[string]any)
	if data["connected_standbys"] != float64(1) {
		t.Fatalf("primary must see a connected standby: %s", primaryRaw)
	}
	time.Sleep(2500 * time.Millisecond)
	standby, standbyRaw := runWithEnv(t, []string{"DBAKIT_DB_PORT=5435"}, "replication", "--lag-threshold", "1s", "--lag-critical", "2s")
	if roleOf(standby) != "standby" {
		t.Fatalf("standby role mismatch: %s", standbyRaw)
	}
	if containsSeverity(standby, "CRITICAL") {
		t.Fatalf("caught-up standby must not be a false critical: %s", standbyRaw)
	}
	local := standby["data"].(map[string]any)["replication"].(map[string]any)
	if local["local_replay_lag_known"] != true {
		t.Fatalf("streaming, caught-up standby must have known lag: %s", standbyRaw)
	}
}

func TestEveryCommandConnectionFailureIsACriticalFinding(t *testing.T) {
	base := []string{"health", "--json", "--timeout", "5s",
		"--db-host", "127.0.0.1", "--db-port", "1"}
	cmd := exec.Command(binary, base...)
	cmd.Dir = repoRoot()
	cmd.Env = isolatedEnv("DBAKIT_DB_PASSWORD=")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("connect-failure run should exit 0: %v\n%s", err, out.String())
	}
	var rep map[string]any
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("connect-failure JSON unparseable: %v", err)
	}
	summary := rep["summary"].(map[string]any)
	if summary["critical"] != float64(1) {
		t.Fatalf("expected 1 CRITICAL on connect failure: %s", out.String())
	}
}

func findings(rep map[string]any) []map[string]any {
	fs, _ := rep["findings"].([]any)
	out := make([]map[string]any, 0, len(fs))
	for _, finding := range fs {
		if item, ok := finding.(map[string]any); ok {
			out = append(out, item)
		}
	}
	return out
}

func containsSeverity(rep map[string]any, severity string) bool {
	for _, finding := range findings(rep) {
		if finding["severity"] == severity {
			return true
		}
	}
	return false
}

func roleOf(rep map[string]any) string {
	for _, finding := range findings(rep) {
		if finding["id"] != "REPL-001" {
			continue
		}
		meta, _ := finding["metadata"].(map[string]any)
		role, _ := meta["role"].(string)
		return role
	}
	return ""
}

func findingIDs(rep map[string]any) []string {
	fs, _ := rep["findings"].([]any)
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		m := f.(map[string]any)
		out = append(out, m["id"].(string))
	}
	return out
}

func containsID(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
