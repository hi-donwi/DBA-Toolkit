package evaluator

import (
	"fmt"

	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// EvaluateSettings reports each setting as a factual INFO finding and flags the
// two obvious configuration hazards (WAL level too weak to support PITR or
// replication, slow-statement logging disabled). It deliberately does not
// pretend to know the "perfect" value for any workload.
func EvaluateSettings(settings []model.ConfigSetting) []model.Finding {
	if len(settings) == 0 {
		return []model.Finding{
			finding("CFG-001", model.SeverityInfo, "Configuration",
				"No settings matched the allowlist"),
		}
	}
	out := make([]model.Finding, 0, len(settings)+2)
	for _, s := range settings {
		value := s.Value
		if s.Unit != "" {
			value += " " + s.Unit
		}
		evidence := fmt.Sprintf("%s = %s", s.Name, value)
		if s.Source != "" {
			evidence += fmt.Sprintf(" (source: %s)", s.Source)
		}
		if s.Context != "" {
			evidence += fmt.Sprintf(" (context: %s)", s.Context)
		}
		out = append(out, finding("CFG-001", model.SeverityInfo, s.Name,
			fmt.Sprintf("%s is set to %s", s.Name, value),
			WithEvidence(evidence),
			WithMetadata("value", value)))

		switch {
		case s.Name == "wal_level" && s.Value == "minimal":
			out = append(out, finding("CFG-WAL-LEVEL", model.SeverityWarning,
				"WAL level is 'minimal'",
				"Point-in-time recovery and replication are not available",
				WithEvidence("wal_level = minimal"),
				WithRecommendation("Set wal_level to 'replica' (or 'logical') to enable PITR and replication.")))
		case s.Name == "log_min_duration_statement" && s.Value == "-1":
			out = append(out, finding("CFG-LOG-DURATION", model.SeverityWarning,
				"Slow statements are not logged",
				"log_min_duration_statement = -1 disables slow-query logging",
				WithEvidence("log_min_duration_statement = -1"),
				WithRecommendation("Set a positive log_min_duration_statement (e.g. 1000ms) to capture slow queries for diagnosis.")))
		}
	}
	return out
}
