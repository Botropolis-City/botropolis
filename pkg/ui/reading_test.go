package ui_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 159: while a first launch reads the history, the footer says how
// far it has got, on a bar.
func TestReadingLine(t *testing.T) {
	scan := state.ScanProgress{Done: 42, Known: 100}

	for _, tc := range []struct {
		name    string
		left    time.Duration
		settled bool
		want    string
	}{
		{"when the rate has not settled", 0, false, "Reading your history · 42% · statistics fill in as it reads"},
		{"when about six minutes are left", 5*time.Minute + 20*time.Second, true, "Reading your history · 42% · about 6 min left · statistics fill in as it reads"},
		{"when under a minute is left", 40 * time.Second, true, "Reading your history · 42% · under a minute left · statistics fill in as it reads"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run(fmt.Sprintf("it should say %q", tc.want), func(t *testing.T) {
				assert.Equal(t, tc.want, ui.ReadingLine(scan, tc.left, tc.settled))
			})
		})
	}

	t.Run("when the last bytes are still being read", func(t *testing.T) {
		t.Run("it should not say 100%", func(t *testing.T) {
			assert.Equal(t, "Reading your history · 99% · statistics fill in as it reads", ui.ReadingLine(state.ScanProgress{Done: 999, Known: 1000}, 0, false))
		})
	})
}

func TestScanRate(t *testing.T) {
	const mib = 1 << 20
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	observe := func(seconds int) *ui.ScanRate {
		r := &ui.ScanRate{}
		for s := 0; s <= seconds; s++ {
			r.Observe(at.Add(time.Duration(s)*time.Second), int64(s)*mib)
		}
		return r
	}

	t.Run("when it has watched 1 MiB a second for 10 s", func(t *testing.T) {
		_, settled := observe(10).Remaining(10*mib, 370*mib)

		t.Run("it should not yet estimate", func(t *testing.T) {
			assert.False(t, settled)
		})
	})

	t.Run("when it has watched 1 MiB a second for 30 s, with 360 MiB left", func(t *testing.T) {
		left, settled := observe(30).Remaining(30*mib, 390*mib)

		t.Run("it should have settled", func(t *testing.T) {
			assert.True(t, settled)
		})

		t.Run("it should estimate six minutes", func(t *testing.T) {
			assert.Equal(t, 6*time.Minute, left)
		})
	})

	t.Run("when a new scan starts below where the last one was", func(t *testing.T) {
		r := observe(30)
		r.Observe(at.Add(31*time.Second), 0)
		_, settled := r.Remaining(0, 100*mib)

		t.Run("it should start watching again", func(t *testing.T) {
			assert.False(t, settled)
		})
	})
}

func TestFooterWithReading(t *testing.T) {
	th := ui.NewTheme(1)
	keys := []ui.Key{{"drag", "pan"}}

	t.Run("when the history is being read and a session waits", func(t *testing.T) {
		plain := ui.LayoutFooter(th, 800, 600, "", keys, measure7)
		waiting := plain.WithWaiting(th, 800, "needs-you: Sea conditions", measure7)
		f := waiting.WithReading(th, 800, "Reading your history · 42%", 0.42, measure7)

		t.Run("it should show the line", func(t *testing.T) {
			assert.Equal(t, "Reading your history · 42%", f.Reading.Text)
		})

		t.Run("it should fill the bar to the fraction read", func(t *testing.T) {
			assert.InDelta(t, 0.42*f.ReadingBar.Width(), f.ReadingFill.Width(), 0.001)
		})

		t.Run("it should start the fill where the bar starts", func(t *testing.T) {
			assert.Equal(t, f.ReadingBar.Min, f.ReadingFill.Min)
		})

		t.Run("it should sit above the needs-you line", func(t *testing.T) {
			assert.LessOrEqual(t, f.ReadingBar.Max.Y, f.WaitingRect.Min.Y)
		})

		t.Run("it should grow the footer to hold it", func(t *testing.T) {
			assert.LessOrEqual(t, f.Rect.Min.Y, f.Reading.At.Y)
		})

		t.Run("it should leave the key row where it was", func(t *testing.T) {
			assert.Equal(t, plain.KeyRow, f.KeyRow)
		})
	})

	t.Run("when nothing is being read", func(t *testing.T) {
		plain := ui.LayoutFooter(th, 800, 600, "", keys, measure7)

		t.Run("it should leave the footer alone", func(t *testing.T) {
			assert.Equal(t, plain, plain.WithReading(th, 800, "", 0, measure7))
		})
	})
}
