package cli

import (
	"testing"
)

func TestWALCmdStructure(t *testing.T) {
	if walCmd.Use != "wal" {
		t.Errorf("expected Use 'wal', got %s", walCmd.Use)
	}
	aliases := map[string]bool{}
	for _, a := range walCmd.Aliases {
		aliases[a] = true
	}
	for _, want := range []string{"checkpoints", "durability", "checkpoint"} {
		if !aliases[want] {
			t.Errorf("missing alias %s", want)
		}
	}
}
