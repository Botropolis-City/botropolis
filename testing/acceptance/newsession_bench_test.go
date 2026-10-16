package acceptance_test

import (
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/testing/helpers"
)

// The wall time of starting a session in a directory with nothing
// running, from opening the map to the start being handed over. It
// reports and never gates: the test beside it gates on steps, ticks and
// work. Item 91.
//
// The terminal, for comparison: cd to the directory and run claude —
// about 10 s, timed by hand by Aria on 2026-10-16.
func BenchmarkStartASessionAnywhere(b *testing.B) {
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
			var s started
			for b.Loop() {
				s = runStart(b, home)
			}
			b.ReportMetric(float64(s.steps), "steps/op")
			b.ReportMetric(float64(s.ticks), "ticks/op")
		})
	}
}
