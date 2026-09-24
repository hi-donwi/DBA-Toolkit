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

// fmtBytes renders a byte count in human units for finding summaries.
func fmtBytes(n int64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %s", n, units[i])
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

// fmtNum renders an integer with comma thousands separators.
func fmtNum(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	in := fmt.Sprintf("%d", n)
	if len(in) <= 3 {
		return sign + in
	}
	out := ""
	rem := len(in) % 3
	if rem > 0 {
		out = in[:rem] + ","
		in = in[rem:]
	}
	for len(in) > 3 {
		out += in[:3] + ","
		in = in[3:]
	}
	out += in
	return sign + out
}
