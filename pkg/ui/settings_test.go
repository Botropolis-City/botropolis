package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func someSettings() ui.Settings {
	return ui.NewSettings([]ui.Setting{
		{Key: "projection", Label: "projection", Options: []string{"iso", "top"}, Value: "iso"},
		{Key: "reduced_motion", Label: "reduced motion", Options: []string{"off", "on"}, Value: "off"},
		{Key: "parked_days", Label: "parked days", Options: []string{"0", "7", "30"}, Value: "7"},
	})
}

func TestSettings(t *testing.T) {
	t.Run("when moving the cursor", func(t *testing.T) {
		t.Run("it should step to the next row", func(t *testing.T) {
			s := someSettings()
			s.Move(1)
			assert.Equal(t, 1, s.Cursor)
		})

		t.Run("it should stop at the last row", func(t *testing.T) {
			s := someSettings()
			s.Move(5)
			assert.Equal(t, 2, s.Cursor)
		})

		t.Run("it should stop at the first row", func(t *testing.T) {
			s := someSettings()
			s.Move(-1)
			assert.Equal(t, 0, s.Cursor)
		})
	})

	t.Run("when adjusting the selected row", func(t *testing.T) {
		t.Run("it should cycle to the next option", func(t *testing.T) {
			s := someSettings()
			s.Adjust(1)
			assert.Equal(t, "top", s.Items[0].Value)
		})

		t.Run("it should wrap from the last option to the first", func(t *testing.T) {
			s := someSettings()
			s.Adjust(1)
			s.Adjust(1)
			assert.Equal(t, "iso", s.Items[0].Value)
		})

		t.Run("it should cycle back with a negative step", func(t *testing.T) {
			s := someSettings()
			s.Move(2)
			s.Adjust(-1)
			assert.Equal(t, "0", s.Items[2].Value)
		})

		t.Run("it should report the changed setting", func(t *testing.T) {
			s := someSettings()
			s.Move(1)
			changed := s.Adjust(1)
			assert.Equal(t, ui.Setting{Key: "reduced_motion", Label: "reduced motion", Options: []string{"off", "on"}, Value: "on"}, changed)
		})

		t.Run("it should hold a value that is not among the options", func(t *testing.T) {
			s := ui.NewSettings([]ui.Setting{{Key: "terminal", Label: "terminal", Options: []string{"auto", "kitty"}, Value: "wezterm start --"}})
			s.Adjust(1)
			assert.Equal(t, "auto", s.Items[0].Value)
		})
	})
}

func TestLayoutSettings(t *testing.T) {
	th := ui.NewTheme(1)
	s := someSettings()
	s.Move(1)

	t.Run("when the panel is laid out", func(t *testing.T) {
		p := ui.LayoutSettings(th, 800, 600, s, measure7)

		t.Run("it should centre the panel", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 400, Y: 300}, p.Rect.Center())
		})

		t.Run("it should title it", func(t *testing.T) {
			assert.Equal(t, ui.Text{Text: "Settings", At: p.Rect.Min.Add(city.Point{X: 24, Y: 24}), Size: ui.Title}, p.Title)
		})

		t.Run("it should stack one row per setting", func(t *testing.T) {
			require.Len(t, p.Rows, 3)
			assert.Equal(t, 16+8+4.0, p.Rows[1].Label.At.Y-p.Rows[0].Label.At.Y)
		})

		t.Run("it should put the values in one column past the widest label", func(t *testing.T) {
			assert.Equal(t, p.Rect.Min.X+24+14*7+24, p.Rows[2].Value.At.X)
		})

		t.Run("it should mark the selected row's value with chevrons", func(t *testing.T) {
			assert.Equal(t, "‹ off ›", p.Rows[1].Value.Text)
		})

		t.Run("it should leave the other values plain", func(t *testing.T) {
			assert.Equal(t, "iso", p.Rows[0].Value.Text)
		})

		t.Run("it should flag the selected row", func(t *testing.T) {
			assert.True(t, p.Rows[1].Selected)
		})

		t.Run("it should size the panel to the widest row plus padding", func(t *testing.T) {
			assert.Equal(t, 24+14*7+24+7*7+24.0, p.Rect.Width())
		})
	})

	// Item 84: what a row has to say — a combination held elsewhere, or a
	// desktop with no way to register one — is said in the panel.
	t.Run("when the settings carry a note", func(t *testing.T) {
		plain := ui.LayoutSettings(th, 800, 600, s, measure7)
		noted := s
		noted.Note = "another application holds ctrl+alt+f; choose another, or bind botropolis find in the desktop's own keyboard settings"
		p := ui.LayoutSettings(th, 800, 600, noted, measure7)
		require.NotEmpty(t, p.Note)

		t.Run("it should lay it out under the last row", func(t *testing.T) {
			assert.Greater(t, p.Note[0].At.Y, p.Rows[2].Label.At.Y)
		})

		t.Run("it should wrap it inside the panel", func(t *testing.T) {
			for _, line := range p.Note {
				w, _ := measure7(line.Text, ui.Body)
				require.LessOrEqual(t, line.At.X+w, p.Rect.Max.X-24, line.Text)
			}
		})

		t.Run("it should grow the panel to hold it", func(t *testing.T) {
			assert.Greater(t, p.Rect.Height(), plain.Rect.Height())
		})
	})
}
