package report

import (
	"github.com/hi-donwi/DBA-Toolkit/internal/model"
)

// colors holds ANSI escape sequences; sequences are empty when NoColor is set.
type colors struct {
	eRed, eGreen, eYellow, eCyan, eGray, eReset, eBold string
}

// ansi returns escape sequences; all empty when color is disabled.
func ansi(enabled bool) colors {
	if !enabled {
		return colors{}
	}
	return colors{
		eRed:    "\x1b[31m",
		eGreen:  "\x1b[32m",
		eYellow: "\x1b[33m",
		eCyan:   "\x1b[36m",
		eGray:   "\x1b[90m",
		eReset:  "\x1b[0m",
		eBold:   "\x1b[1m",
	}
}

func (c colors) Bold(s string) string  { return c.eBold + s + c.eReset }
func (c colors) Red(s string) string   { return c.eRed + s + c.eReset }
func (c colors) Green(s string) string { return c.eGreen + s + c.eReset }
func (c colors) Yellow(s string) string {
	return c.eYellow + s + c.eReset
}
func (c colors) Cyan(s string) string { return c.eCyan + s + c.eReset }
func (c colors) Gray(s string) string { return c.eGray + s + c.eReset }

// tag renders a severity as its bracketed colored label.
func (c colors) tag(sev model.Severity) string {
	switch sev {
	case model.SeverityCritical:
		return c.Red("[" + string(sev) + "]")
	case model.SeverityWarning:
		return c.Yellow("[" + string(sev) + "]")
	case model.SeverityPass:
		return c.Green("[" + string(sev) + "]")
	default:
		return c.Cyan("[" + string(sev) + "]")
	}
}
