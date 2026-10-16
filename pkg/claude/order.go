package claude

import (
	"regexp"
	"strings"
	"time"
)

// OrderLine is a line a session's place in its group's order can be
// read from: one naming a pull request, a Depends-On footer or a
// version. Command is whether it was in a command the session ran
// rather than in its words. Kept in memory only (item 90).
type OrderLine struct {
	Text    string    `json:"text"`
	Command bool      `json:"command,omitempty"`
	At      time.Time `json:"at"`
}

// OrderLinesKept is how many order lines a transcript keeps, newest.
const OrderLinesKept = 64

// orderLineRunes is the longest line kept.
const orderLineRunes = 300

var orderShape = regexp.MustCompile(`(?i)depends-on:|https?://\S+/(?:pull|pull-requests|-/merge_requests)/\d+|[\w.-]+/[\w.-]+#\d+|\bv\d+\.\d+(?:\.\d+)?\b`)

// OrderLines are the lines of text that name an order.
func OrderLines(text string, command bool, at time.Time) []OrderLine {
	var out []OrderLine
	for _, line := range strings.Split(text, "\n") {
		line = strings.Trim(strings.TrimSpace(line), `"'`)
		if line == "" || !orderShape.MatchString(line) {
			continue
		}
		if r := []rune(line); len(r) > orderLineRunes {
			line = string(r[:orderLineRunes])
		}
		out = append(out, OrderLine{Text: line, Command: command, At: at})
	}
	return out
}

func (s *transcriptScan) keepOrder(lines []OrderLine) {
	t := &s.transcript
	t.OrderLines = append(t.OrderLines, lines...)
	if over := len(t.OrderLines) - OrderLinesKept; over > 0 {
		t.OrderLines = append([]OrderLine(nil), t.OrderLines[over:]...)
	}
}
