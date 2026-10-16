package acceptance_test

import (
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/testing/helpers"
)

// The wall time of answering a session, from opening the map to the
// attach being handed over by a click on the needs-you line. It reports
// and never gates: the test beside it gates on steps, ticks and work.
// Item 91.
//
// The terminal, for comparison: find the session's terminal on whichever
// workspace it is on — about 10 s, timed by hand by Aria on 2026-10-16.
func BenchmarkAnswerAndComeBack(b *testing.B) {
	hamlet := filepath.Join("..", "..", "demo", "scenarios", "hamlet.yaml")
	homes := []struct {
		name string
		home func(testing.TB) string
	}{
		{"hamlet", func(tb testing.TB) string { return stage(tb, hamlet, monday) }},
		{"1000", func(tb testing.TB) string {
			home := stage(tb, hamlet, monday)
			helpers.ParkedWeekIn(tb, home, 1000, monday, reopenTitle)
			return home
		}},
	}
	for _, h := range homes {
		b.Run(h.name, func(b *testing.B) {
			home := h.home(b)
			var a answered
			for b.Loop() {
				a = runAnswer(b, home)
			}
			b.ReportMetric(float64(a.steps), "steps/op")
			b.ReportMetric(float64(a.ticks), "ticks/op")
		})
	}
}
