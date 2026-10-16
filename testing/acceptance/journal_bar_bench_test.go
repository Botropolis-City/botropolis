package acceptance_test

import (
	"testing"

	"botropolis.city/botropolis/testing/helpers"
)

// The wall time of seeing what each piece of work was doing and what is
// next, from opening the map to the journal drawn. It reports and never
// gates: the test beside it gates on steps, ticks and work. Item 91.
//
// The terminal, for comparison: memory, and opening each terminal — about
// 10 s, timed by hand by Aria on 2026-10-16.
func BenchmarkWhatEachPieceOfWorkWasDoing(b *testing.B) {
	homes := []struct {
		name string
		home func(testing.TB) string
	}{
		{"week", func(tb testing.TB) string { home, _ := helpers.JournalWeek(tb, monday); return home }},
		{"1000", func(tb testing.TB) string { return helpers.ParkedWeek(tb, 1000, monday, reopenTitle) }},
	}
	for _, h := range homes {
		b.Run(h.name, func(b *testing.B) {
			home := h.home(b)
			var o opened
			for b.Loop() {
				o = openAfterTheWeekend(b, home)
			}
			b.ReportMetric(float64(o.steps), "steps/op")
			b.ReportMetric(float64(o.ticks), "ticks/op")
		})
	}
}
