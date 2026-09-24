// Package evaluator turns normalized model data into model.Finding values. It
// is the rules engine of the toolkit: every function is pure (no I/O, no
// database) so each rule is unit-testable straight from fixtures. Rules are
// compiled in for the MVP — there is no custom rule DSL yet.
package evaluator

import (
	"sort"
	"time"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// Rule documents one diagnostic rule: its stable ID, title, and group. The
// catalog keeps the rule surface honest and is rendered in `dbakit rules`
// and used by tests to prove every emitted finding has a declared rule.
type Rule struct {
	ID    string
	Title string
	Group string
}

// Rules is the compiled-in rule catalog.
var Rules = []Rule{
	{ID: "CONN-001", Title: "Database connectivity", Group: "connectivity"},
	{ID: "USAGE-001", Title: "Connection usage", Group: "health"},
	{ID: "LONGQ-001", Title: "Long-running queries", Group: "diagnostics"},
	{ID: "LOCK-001", Title: "Locks and blocking", Group: "diagnostics"},
	{ID: "REPL-001", Title: "Replication role", Group: "replication"},
	{ID: "REPL-002", Title: "Replica replay lag", Group: "replication"},
	{ID: "DB-001", Title: "Database inventory", Group: "databases"},
	{ID: "IDX-001", Title: "Unused index", Group: "indexes"},
	{ID: "IDX-002", Title: "Invalid index", Group: "indexes"},
	{ID: "IDX-003", Title: "Index inventory", Group: "indexes"},
	{ID: "CFG-001", Title: "Configuration setting", Group: "configuration"},
	{ID: "CFG-WAL-LEVEL", Title: "WAL level", Group: "configuration"},
	{ID: "CFG-LOG-DURATION", Title: "Slow-statement logging", Group: "configuration"},
}

// Thresholds carries the configurable boundaries every evaluator honours. All
// defaults are conservative and documented in the README.
type Thresholds struct {
	ConnUsageWarnPercent float64       // 80: warn above this %
	ConnUsageCritPercent float64       // 95: critical above this %
	LongQuery            time.Duration // 60s: minimum duration flagged
	LockWait             time.Duration // 5s:  minimum blocked-wait flagged
	ReplLagWarn          time.Duration // 30s: warn above replay lag
	ReplLagCrit          time.Duration // 120s: critical above replay lag
	UnusedIndexMinSize   int64         // 10MB: minimum index size to flag as unused
}

// DefaultThresholds returns the MVP defaults.
func DefaultThresholds() Thresholds {
	return Thresholds{
		ConnUsageWarnPercent: 80,
		ConnUsageCritPercent: 95,
		LongQuery:            60 * time.Second,
		LockWait:             5 * time.Second,
		ReplLagWarn:          30 * time.Second,
		ReplLagCrit:          2 * time.Minute,
		UnusedIndexMinSize:   10 * 1024 * 1024,
	}
}

// option mutates a finding under construction.
type option func(*model.Finding)

// WithEvidence attaches evidence text.
func WithEvidence(s string) option { return func(f *model.Finding) { f.Evidence = s } }

// WithRecommendation attaches recommendation text.
func WithRecommendation(s string) option { return func(f *model.Finding) { f.Recommendation = s } }

// WithMetadata appends one metadata key/value.
func WithMetadata(k, v string) option {
	return func(f *model.Finding) {
		if f.Metadata == nil {
			f.Metadata = map[string]string{}
		}
		f.Metadata[k] = v
	}
}

// finding builds a Finding with the given identity and text.
func finding(id string, sev model.Severity, title, summary string, opts ...option) model.Finding {
	f := model.Finding{ID: id, Severity: sev, Title: title, Summary: summary}
	for _, o := range opts {
		o(&f)
	}
	return f
}

// RuleIDs returns the set of declared rule IDs for tests.
func RuleIDs() map[string]bool {
	set := make(map[string]bool, len(Rules))
	for _, r := range Rules {
		set[r.ID] = true
	}
	return set
}

// Sort orders findings deterministically: severity (most urgent first), then
// rule ID, then summary text. This makes JSON and terminal output stable.
func Sort(findings []model.Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if ra, rb := a.Severity.Rank(), b.Severity.Rank(); ra != rb {
			return ra < rb
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Summary < b.Summary
	})
}

// Summarize tallies findings per severity.
func Summarize(findings []model.Finding) model.Summary {
	var s model.Summary
	for _, f := range findings {
		switch f.Severity {
		case model.SeverityCritical:
			s.Critical++
		case model.SeverityWarning:
			s.Warning++
		case model.SeverityInfo:
			s.Info++
		case model.SeverityPass:
			s.Pass++
		}
	}
	return s
}
