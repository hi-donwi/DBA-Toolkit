package report

import (
	"fmt"
	"time"
)

// fmtDur renders seconds as a compact duration: 8 → "8ms", 92 → "92s",
// 185 → "3m 5s", 7265 → "2h 1m", 115000 → "1d 7h". Negative or zero values
// render as "<1s" when measurable, matching an unreachable precision floor.
func fmtDur(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	if d < time.Second {
		return "0s"
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d/time.Minute), int((d%time.Minute)/time.Second))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d/time.Hour), int((d%time.Hour)/time.Minute))
	}
	return fmt.Sprintf("%dd %dh", int(d/(24*time.Hour)), int((d%(24*time.Hour))/time.Hour))
}

// humanBytes renders a byte count in the largest comfortable unit.
func humanBytes(n int64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
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

// humanInt renders an integer with comma thousands separators.
func humanInt(n int64) string {
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
