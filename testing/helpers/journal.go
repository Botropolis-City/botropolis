package helpers

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// JournalWeek is a home of a week's work before now for the journal:
// three sessions that share a ticket key across two directories, which
// group detection joins, and four on their own, each in its own
// directory and on its own day, with nothing in common. It returns the
// home and the grouped sessions' ids. Item 91.
func JournalWeek(t testing.TB, now time.Time) (string, []string) {
	t.Helper()
	h := NewHome(t)
	session := func(i int, dir, prompt, title string, at time.Time) string {
		id := fmt.Sprintf("91000000-0000-4000-8000-%012d", i)
		cwd := filepath.Join(h.Path, "src", dir)
		ts := at.UTC().Format(time.RFC3339Nano)
		h.Transcript(id, cwd,
			UserPrompt(id, cwd, ts),
			askingReply(id, fmt.Sprintf("msg_journal_%d", i), at.Add(time.Minute).UTC().Format(time.RFC3339Nano), "Shall I "+title+" next?"),
			AITitle(id, title))
		h.Prompt(id, cwd, prompt, at)
		return id
	}
	day := func(n int, hour int) time.Time {
		d := now.AddDate(0, 0, -n)
		return time.Date(d.Year(), d.Month(), d.Day(), hour, 0, 0, 0, now.Location())
	}
	grouped := []string{
		session(0, "orchard", "PLOT-7: the plot planner drops the last row", "Plot planner drops a row", day(3, 10)),
		session(1, "orchard-api", "PLOT-7: the API returns the short plot too", "API returns the short plot", day(3, 15)),
		session(2, "orchard", "PLOT-7 follow-up: test the ten by ten plot", "Test the ten by ten plot", day(4, 11)),
	}
	session(3, "kiln", "why does the glaze report round the hold time", "Glaze report rounds the hold", day(1, 9))
	session(4, "beacon", "add a battery line to the status page", "Battery line on the status page", day(2, 14))
	session(5, "lanternfish", "which port does the lantern server use", "Lantern server port", day(5, 16))
	session(6, "quillmark", "split the markdown block parser into handlers", "Split the block parser", day(6, 12))
	return h.Path, grouped
}

// askingReply is a reply whose last paragraph asks what to do next.
func askingReply(sessionID, id, ts, question string) string {
	return fmt.Sprintf(`{"type":"assistant","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
		`"message":{"id":%q,"model":"claude-opus-5","role":"assistant","stop_reason":"end_turn",`+
		`"content":[{"type":"text","text":%q}],"usage":{"input_tokens":2,"output_tokens":5}}}`,
		ts, sessionID, id, "The first pass is done.\n\n"+question)
}
