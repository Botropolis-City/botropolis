package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 351, slice 4: a panel the hold reaches gives its keyed controls
// to the hold, rather than spelling their keys in a line.
func TestPanelsGiveTheHoldTheirControls(t *testing.T) {
	keysOf := func(controls []ui.Control) map[string][]string {
		out := map[string][]string{}
		for _, c := range controls {
			out[c.Name] = c.Keys
		}
		return out
	}

	t.Run("when close-up is open on a row with a box and a pin", func(t *testing.T) {
		rows := []closeUpRow{
			{Rect: city.RectAt(0, 0, 200, 20)},
			{Rect: city.RectAt(0, 20, 200, 20), Box: city.RectAt(4, 22, 12, 12), Pin: city.RectAt(180, 22, 12, 12), HasBox: true},
		}
		keys := keysOf(closeUpControls(rows, 1))

		t.Run("it should cap its box with p", func(t *testing.T) {
			assert.Equal(t, []string{"p"}, keys["pause"])
		})

		t.Run("it should cap its pin with space", func(t *testing.T) {
			assert.Equal(t, []string{"space"}, keys["pin"])
		})

		t.Run("and the focused row has no box", func(t *testing.T) {
			t.Run("it should cap no box", func(t *testing.T) {
				assert.NotContains(t, keysOf(closeUpControls(rows, 0)), "pause")
			})
		})
	})

	t.Run("when City Hall's board is open", func(t *testing.T) {
		tidy, quiet, fresh := city.RectAt(10, 10, 60, 20), city.RectAt(80, 10, 120, 20), city.RectAt(210, 10, 90, 20)
		keys := keysOf(boardControls(tidy, quiet, fresh, "shift+n"))

		for name, want := range map[string][]string{"tidy up": {"t"}, "pack up quiet": {"p"}, "new session": {"shift+n"}} {
			t.Run("it should cap its "+name+" button with "+keysName(want), func(t *testing.T) {
				assert.Equal(t, want, keys[name])
			})
		}
	})

	t.Run("when the journal is open on an entry", func(t *testing.T) {
		row := city.RectAt(10, 40, 300, 48)

		t.Run("it should cap the entry with r, which restores its desk", func(t *testing.T) {
			assert.Equal(t, []ui.Control{{Name: "restore", Rect: row, Keys: []string{"r"}}}, journalControls(row))
		})
	})

	t.Run("when the plant's long-range panel is open", func(t *testing.T) {
		p := ui.RangesPanel{
			Row:         ui.WindowRow{Short: []ui.Button{{Rect: city.RectAt(10, 10, 40, 20)}}, Long: []ui.Button{{Rect: city.RectAt(60, 10, 40, 20)}}},
			FilterRect:  city.RectAt(10, 40, 120, 16),
			CompareHead: ui.Text{Text: "same day last week", At: city.Point{X: 10, Y: 200}, Size: ui.Small},
		}
		keys := keysOf(rangesControls(p, func(s string, _ ui.Size) (float64, float64) { return 7 * float64(len(s)), 16 }))

		t.Run("it should cap the row of ranges with the arrows that choose one", func(t *testing.T) {
			assert.Equal(t, []string{"←", "→"}, keys["ranges"])
		})

		t.Run("it should cap the districts filter with d", func(t *testing.T) {
			assert.Equal(t, []string{"d"}, keys["districts"])
		})

		t.Run("it should cap the same-day comparison with s", func(t *testing.T) {
			assert.Equal(t, []string{"s"}, keys["same day"])
		})
	})
}
