// Package model defines the shared, engine-agnostic types produced by the
// DBA-Toolkit pipeline: raw collected data, normalized findings, and the final
// report. Collectors fill the *Data types, evaluators turn them into findings,
// and reporters serialize the report for a terminal or JSON consumer.
package model

import "time"

// Severity is the severity level of a finding. The toolkit stays deliberately
// small: INFO, WARNING, and CRITICAL, plus PASS for a check that passed.
type Severity string

const (
	SeverityInfo     Severity = "INFO"     // Factual, informational result
	SeverityWarning  Severity = "WARNING"  // Needs attention
	SeverityCritical Severity = "CRITICAL" // Requires immediate attention
	SeverityPass     Severity = "PASS"     // Diagnostic check passed
)

// Rank orders severities most-urgent first, for deterministic reporting.
func (s Severity) Rank() int {
	switch s {
	case SeverityCritical:
		return 0
	case SeverityWarning:
		return 1
	case SeverityInfo:
		return 2
	default:
		return 3
	}
}

// Finding is a single, self-contained diagnostic result.
type Finding struct {
	ID             string            `json:"id"`
	Severity       Severity          `json:"severity"`
	Title          string            `json:"title"`
	Summary        string            `json:"summary"`
	Evidence       string            `json:"evidence,omitempty"`
	Recommendation string            `json:"recommendation,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// Summary counts findings per severity.
type Summary struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
	Pass     int `json:"pass"`
}

// Database describes the database the report applies to.
type Database struct {
	Engine  string `json:"engine"` // "postgresql" in the MVP
	Version string `json:"version"`
	Name    string `json:"name,omitempty"`
}

// Report is the complete, stable output of a dbakit run. Its JSON serialization
// is consumed by scripts, CI, and monitoring integrations, so it is kept
// deterministic and additive-only.
type Report struct {
	Tool      string    `json:"tool"`
	Version   string    `json:"version"`
	Command   string    `json:"command"`
	Database  Database  `json:"database"`
	Timestamp time.Time `json:"timestamp"`
	Duration  string    `json:"duration"`
	Summary   Summary   `json:"summary"`
	Findings  []Finding `json:"findings"`
	Data      any       `json:"data,omitempty"`
}
