package collector

import (
	"regexp"
	"strings"
)

var versionRe = regexp.MustCompile(`PostgreSQL\s+(\d+(?:\.\d+)*)`)

// ShortVersion reduces "PostgreSQL 16.4 (Debian 16.4-1.pgdg120+1) on ..." to
// "16.4". Unparsable input is returned unchanged.
func ShortVersion(serverVersion string) string {
	if m := versionRe.FindStringSubmatch(serverVersion); len(m) == 2 {
		return m[1]
	}
	return strings.TrimSpace(serverVersion)
}

// Truncate cuts s to at most n runes, appending an ellipsis when truncated.
// It is the single place query text is shortened, so "never expose sensitive
// query parameters unnecessarily" is enforced consistently across every command
// and output format.
func Truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
