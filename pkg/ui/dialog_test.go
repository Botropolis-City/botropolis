package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 352: a prompt is a centred dialog, the quit panel's shape lifted
// out for every question with a choice or a loss, its keys under its
// buttons.
func TestLayoutDialog(t *testing.T) {
	th := ui.NewTheme(1)
	choices := []ui.DialogChoice{{"Demolish", []string{"d", "enter"}}, {"Close up…", []string{"c"}}, {"Cancel", []string{"esc", "n"}}}
	lines := []string{"Deletes this session and its worktree.", "Its conversation stays in Claude Code's files."}

	t.Run("when a dialog with a title, two lines and three choices is laid out in 1280×800", func(t *testing.T) {
		d := ui.LayoutDialog(th, "Demolish Fix the queue?", lines, choices, 1280, 800, measure7)
		require.Len(t, d.Buttons, 3)
		require.Len(t, d.Keys, 3)
		require.Len(t, d.Lines, 2)

		t.Run("it should centre the plate in the window", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 640, Y: 400}, d.Plate.Center())
		})

		t.Run("it should put the title first, the lines under it and the buttons under those", func(t *testing.T) {
			assert.True(t, d.Title.At.Y < d.Lines[0].At.Y && d.Lines[1].At.Y < d.Buttons[0].Rect.Min.Y, "title %v, lines %v %v, buttons %v", d.Title.At, d.Lines[0].At, d.Lines[1].At, d.Buttons[0].Rect)
		})

		t.Run("it should lay the buttons out in the choices' order", func(t *testing.T) {
			var labels []string
			for _, b := range d.Buttons {
				labels = append(labels, b.Label.Text)
			}
			assert.Equal(t, []string{"Demolish", "Close up…", "Cancel"}, labels)
		})

		t.Run("it should give each button one cap per key that presses it", func(t *testing.T) {
			var got [][]string
			for _, caps := range d.Keys {
				var labels []string
				for _, c := range caps {
					labels = append(labels, c.Label.Text)
				}
				got = append(got, labels)
			}
			assert.Equal(t, [][]string{{"d", "enter"}, {"c"}, {"esc", "n"}}, got)
		})

		t.Run("it should put each button's caps directly under that button", func(t *testing.T) {
			for i, b := range d.Buttons {
				for _, c := range d.Keys[i] {
					assert.True(t, c.Rect.Min.Y >= b.Rect.Max.Y && c.Rect.Min.X >= b.Rect.Min.X && c.Rect.Max.X <= b.Rect.Max.X, "button %d at %v, cap %q at %v", i, b.Rect, c.Label.Text, c.Rect)
				}
			}
		})

		t.Run("it should centre each button's caps under it", func(t *testing.T) {
			for i, b := range d.Buttons {
				caps := d.Keys[i]
				mid := (caps[0].Rect.Min.X + caps[len(caps)-1].Rect.Max.X) / 2
				assert.InDelta(t, b.Rect.Center().X, mid, 0.5, "button %d", i)
			}
		})

		t.Run("it should keep everything inside the plate", func(t *testing.T) {
			all := d.Plate
			for _, b := range d.Buttons {
				all = all.Union(b.Rect)
			}
			for _, caps := range d.Keys {
				for _, c := range caps {
					all = all.Union(c.Rect)
				}
			}
			assert.Equal(t, d.Plate, all)
		})
	})

	t.Run("when the window is narrower than the row of buttons", func(t *testing.T) {
		d := ui.LayoutDialog(th, "Demolish Fix the queue?", nil, choices, 300, 400, measure7)

		t.Run("it should keep the plate inside the window", func(t *testing.T) {
			assert.True(t, d.Plate.Min.X >= 0 && d.Plate.Max.X <= 300, "%v", d.Plate)
		})
	})

	t.Run("when the dialog has no lines", func(t *testing.T) {
		d := ui.LayoutDialog(th, "Quit Botropolis?", nil, choices, 1280, 800, measure7)

		t.Run("it should lay out none", func(t *testing.T) {
			assert.Empty(t, d.Lines)
		})
	})
}
