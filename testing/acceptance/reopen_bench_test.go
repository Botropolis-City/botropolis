package acceptance_test

import (
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/testing/helpers"
)

// The wall time of reopening last Friday's session, from opening the map
// to the resume being handed over. It reports and never gates: the test
// beside it gates on steps, ticks and work. Item 91.
//
// The terminal, for comparison: cd to the project, claude --resume, and
// search the list for the session — 15 s, timed by hand on 2026-10-16.
func BenchmarkReopenLastFriday(b *testing.B) {
	homes := []struct {
		name string
		home func(testing.TB) string
	}{
		{"week", func(tb testing.TB) string { return stage(tb, filepath.Join("testdata", "week.yaml"), monday) }},
		{"1000", func(tb testing.TB) string { return helpers.ParkedWeek(tb, 1000, monday, reopenTitle) }},
	}
	for _, h := range homes {
		b.Run(h.name, func(b *testing.B) {
			home := h.home(b)
			var r reopen
			for b.Loop() {
				r = runReopen(b, home, false)
			}
			b.ReportMetric(float64(r.steps), "steps/op")
			b.ReportMetric(float64(r.ticks), "ticks/op")
		})
	}
}
