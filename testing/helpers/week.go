package helpers

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// ParkedWeek writes a home of n parked sessions spread over the week
// before now, across ten projects. One of them is titled target and last
// spoke on the Friday before now; the rest are titled by number. Item 91.
func ParkedWeek(t testing.TB, n int, now time.Time, target string) string {
	t.Helper()
	h := NewHome(t)
	parkedWeek(h, n, now, target)
	return h.Path
}

// ParkedWeekIn writes the same sessions into an existing home, beside
// whatever it holds.
func ParkedWeekIn(t testing.TB, home string, n int, now time.Time, target string) {
	t.Helper()
	parkedWeek(&Home{t: t, Path: home}, n, now, target)
}

func parkedWeek(h *Home, n int, now time.Time, target string) {
	h.t.Helper()
	friday := now.AddDate(0, 0, -1)
	for friday.Weekday() != time.Friday {
		friday = friday.AddDate(0, 0, -1)
	}
	for i := range n {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
		cwd := filepath.Join("/home/user/work", fmt.Sprintf("project-%02d", i%10))
		title := fmt.Sprintf("Session number %04d", i)
		at := now.Add(-time.Duration(i+1) * 6 * 24 * time.Hour / time.Duration(n+1))
		if i == n/2 {
			title, at = target, time.Date(friday.Year(), friday.Month(), friday.Day(), 16, 0, 0, 0, now.Location())
		}
		ts := at.UTC().Format(time.RFC3339Nano)
		h.Transcript(id, cwd, UserPrompt(id, cwd, ts), AssistantReply(id, fmt.Sprintf("msg_%d", i), ts), AITitle(id, title))
	}
}
