package cli

import (
	"testing"
)

func TestTopQueriesCmdStructure(t *testing.T) {
	if topQueriesCmd.Use != "top-queries" {
		t.Errorf("expected Use 'top-queries', got %s", topQueriesCmd.Use)
	}
	aliases := map[string]bool{}
	for _, a := range topQueriesCmd.Aliases {
		aliases[a] = true
	}
	for _, want := range []string{"topq", "slow-queries", "statements"} {
		if !aliases[want] {
			t.Errorf("missing alias %s", want)
		}
	}
	for _, wantFlag := range []string{"limit", "mean-warn", "mean-crit"} {
		if topQueriesCmd.Flags().Lookup(wantFlag) == nil {
			t.Errorf("missing flag %s", wantFlag)
		}
	}
}
