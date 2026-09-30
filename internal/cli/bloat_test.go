package cli

import (
	"testing"
)

func TestBloatCmdStructure(t *testing.T) {
	if bloatCmd.Use != "bloat" {
		t.Errorf("expected Use 'bloat', got %s", bloatCmd.Use)
	}
	aliases := map[string]bool{}
	for _, a := range bloatCmd.Aliases {
		aliases[a] = true
	}
	for _, want := range []string{"dead-tuples", "tables-bloat", "vacuum-needed"} {
		if !aliases[want] {
			t.Errorf("missing alias %s", want)
		}
	}
	for _, wantFlag := range []string{"limit", "dead-ratio-warn", "dead-ratio-crit", "min-dead-tuples"} {
		if bloatCmd.Flags().Lookup(wantFlag) == nil {
			t.Errorf("missing flag %s", wantFlag)
		}
	}
}
