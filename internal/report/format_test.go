package report

import (
	"testing"
)

func TestFmtDurSubSecond(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0.0, "0s"},
		{-0.5, "0s"},
		{0.0000005, "<1µs"},
		{0.000350, "350µs"},
		{0.008, "8ms"},
		{0.0085, "8.50ms"},
		{0.5, "500ms"},
		{5.0, "5s"},
		{65.0, "1m 5s"},
		{3700.0, "1h 1m"},
	}

	for _, tc := range tests {
		got := fmtDur(tc.in)
		if got != tc.want {
			t.Errorf("fmtDur(%f) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
