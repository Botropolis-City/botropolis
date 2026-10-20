package format

import (
	"fmt"
	"time"
)

// Count is a number with its noun agreeing: "1 PR", "2 PRs".
func Count(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func Dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// Unknown is what stands in for a number nothing reported.
const Unknown = "\u2014"

// USD is a cost estimate to the cent, or a dash when no cost-state was
// ever seen: an unknown cost is never a number.
func USD(usd float64, known bool) string {
	if !known {
		return Unknown
	}
	return fmt.Sprintf("~$%.2f", usd)
}

func Percent(p float64) string {
	if p == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", p)
}

func Tokens(perHour float64) string {
	switch {
	case perHour == 0:
		return "-"
	case perHour >= 1_000_000:
		return fmt.Sprintf("%.1fM", perHour/1_000_000)
	case perHour >= 1_000:
		return fmt.Sprintf("%.0fk", perHour/1_000)
	}
	return fmt.Sprintf("%.0f", perHour)
}

func Age(d time.Duration) string {
	switch {
	case d < 0:
		return "-"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
}
