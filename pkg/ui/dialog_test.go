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
	choices := []ui.DialogChoice{{"Demolish", "d · enter"}, {"Close up…", "c"}, {"Cancel", "esc · n"}}
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

		t.Run("it should put each button's keys directly under that button", func(t *testing.T) {
			for i, b := range d.Buttons {
				k := d.Keys[i]
				assert.True(t, k.Text == choices[i].Keys && k.At.Y >= b.Rect.Max.Y && k.At.X >= b.Rect.Min.X && k.At.X < b.Rect.Max.X, "button %d at %v, keys %q at %v", i, b.Rect, k.Text, k.At)
			}
		})

		t.Run("it should keep everything inside the plate", func(t *testing.T) {
			all := d.Plate
			for _, b := range d.Buttons {
				all = all.Union(b.Rect)
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
