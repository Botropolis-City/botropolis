package ui

import (
	"fmt"
	"strings"

	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/stats"
)

// The one way usage is put into words, wherever it is shown: cost at API
// list prices, with the messages beside it, and the counters under it,
// each apart because each is priced apart. Item 116.

func dollars(usd float64) string {
	if usd > 0 && usd < 0.005 {
		return "<$0.01"
	}
	return fmt.Sprintf("$%.2f", usd)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// Headline is a stretch's cost and messages: one figure when every
// write's lifetime is known, a range when some are not, and how many
// messages nothing prices.
func Headline(c stats.Cost, messages int) string {
	if messages == 0 {
		return "no messages"
	}
	if c.Unpriced >= messages {
		return "cost unknown · " + plural(messages, "message", "messages")
	}
	cost := dollars(c.Low)
	if dollars(c.High) != cost {
		cost += "–" + dollars(c.High)
	}
	line := cost + " at API list prices · " + plural(messages, "message", "messages")
	if c.Unpriced > 0 {
		line += fmt.Sprintf(" · cost unknown for %d", c.Unpriced)
	}
	return line
}

func count(n int64) string {
	if n == 0 {
		return "0"
	}
	return format.Tokens(float64(n))
}

// CountersLine is the counters apart, never summed.
func CountersLine(c stats.Counters) string {
	parts := []string{"input " + count(c.Input), "5 m writes " + count(c.Write5m), "1 h writes " + count(c.Write1h)}
	if c.WriteUnsplit > 0 {
		parts = append(parts, "writes of unknown lifetime "+count(c.WriteUnsplit))
	}
	parts = append(parts, "reads "+count(c.Read), "output "+count(c.Output))
	return strings.Join(parts, " · ")
}

// PausedLine is why statistics are paused, or "" when they are not.
func PausedLine(reason string) string {
	if reason == "" {
		return ""
	}
	return "statistics paused: " + reason
}
