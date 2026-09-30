package cli

import (
	"testing"
)

func TestCacheCmdStructure(t *testing.T) {
	if cacheCmd.Use != "cache" {
		t.Errorf("expected Use 'cache', got %s", cacheCmd.Use)
	}
	aliases := map[string]bool{}
	for _, a := range cacheCmd.Aliases {
		aliases[a] = true
	}
	for _, want := range []string{"buffer", "hit-ratio", "buffercache"} {
		if !aliases[want] {
			t.Errorf("missing alias %s", want)
		}
	}
	for _, wantFlag := range []string{"limit", "cache-hit-warn", "cache-hit-crit"} {
		if cacheCmd.Flags().Lookup(wantFlag) == nil {
			t.Errorf("missing flag %s", wantFlag)
		}
	}
}
