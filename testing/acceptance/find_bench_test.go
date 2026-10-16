package acceptance_test

import (
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/testing/helpers"
)

// The wall time of finding a session by three words, from opening the
// map to the resume being handed over. It reports and never gates: the
// test beside it gates on steps, ticks and work. Item 91.
//
// The terminal, for comparison: open a Claude session and ask it to
// search — not yet timed by hand.
func BenchmarkFindByThreeWords(b *testing.B) {
	homes := []struct {
		name string
		home func(testing.TB) string
	}{
		{"week", func(tb testing.TB) string { return stage(tb, filepath.Join("testdata", "week.yaml"), monday) }},
		{"1000", func(tb testing.TB) string { return helpers.ParkedWeek(tb, 1000, monday, generatedOn) }},
	}
	for _, h := range homes {
		b.Run(h.name, func(b *testing.B) {
			home := h.home(b)
			var f found
			for b.Loop() {
				f = runFind(b, home, false)
			}
			b.ReportMetric(float64(f.steps), "steps/op")
			b.ReportMetric(float64(f.ticks), "ticks/op")
		})
	}
}
