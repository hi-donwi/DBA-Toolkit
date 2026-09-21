package evaluator

import (
	"fmt"
	"time"
)

// fmtDur renders seconds as a compact, human-readable duration, e.g. 92 →
// "92s", 185 → "3m 5s", 7265 → "2h 1m". Used only in human-facing evidence
// text; machine outputs always carry raw seconds.
func fmtDur(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	if d < time.Hour {
		m := d / time.Minute
		s := (d % time.Minute) / time.Second
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	return fmt.Sprintf("%dh %dm", h, m)
}

// pct renders a percentage with one decimal place.
func pct(use float64) string {
	return fmt.Sprintf("%.1f%%", use)
}
