package cli

import (
	"testing"
)

func TestXIDCmdStructure(t *testing.T) {
	if xidCmd.Use != "xid" {
		t.Errorf("expected Use 'xid', got %s", xidCmd.Use)
	}
	aliases := map[string]bool{}
	for _, a := range xidCmd.Aliases {
		aliases[a] = true
	}
	for _, want := range []string{"wraparound", "freeze", "vacuum"} {
		if !aliases[want] {
			t.Errorf("missing alias %s", want)
		}
	}
	for _, wantFlag := range []string{"warn-age", "crit-age", "top-tables"} {
		if xidCmd.Flags().Lookup(wantFlag) == nil {
			t.Errorf("missing flag %s", wantFlag)
		}
	}
}
