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
	"testing"
)

var env = []string{
	"DBAKIT_DB_HOST=localhost",
	"DBAKIT_DB_PORT=5433",
	"DBAKIT_DB_NAME=dbakit",
	"DBAKIT_DB_USER=dbakit",
	"DBAKIT_DB_PASSWORD=dbakit_pw",
	"DBAKIT_DB_SSLMODE=disable",
}

const binary = "/tmp/dbakit-integration"

// TestMain builds the real binary once for the whole suite.
func TestMain(m *testing.M) {
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Println("SKIP: docker not available")
		os.Exit(0)
	}
	cmd := exec.Command("go", "build", "-tags", "integration", "-o", binary, "../..")
	cmd.Dir = repoRoot()
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("build failed: %v\n%s\n", err, out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func repoRoot() string {
	wd, _ := os.Getwd()
	return filepath.Dir(wd)
}

func run(t *testing.T, args ...string) (map[string]any, string) {
	t.Helper()
	cmd := exec.Command(binary, append(args, "--json", "--timeout", "30s")...)
	cmd.Env = append(os.Environ(), env...)
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
	for _, cmd := range []string{"diagnose", "sessions", "locks", "replication", "databases", "config"} {
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

func TestEveryCommandConnectionFailureIsACriticalFinding(t *testing.T) {
	base := []string{"health", "--json", "--timeout", "5s",
		"--db-host", "127.0.0.1", "--db-port", "1"}
	cmd := exec.Command(binary, base...)
	cmd.Dir = repoRoot()
	cmd.Env = append(os.Environ(), "DBAKIT_DB_PASSWORD=")
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
