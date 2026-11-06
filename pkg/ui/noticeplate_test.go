package ui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 352: a notice is one plate top-centre under the strip, with the
// tone's dot before its text and the key that acts on it as a cap after.
func TestLayoutNotice(t *testing.T) {
	th := ui.NewTheme(1)
	const width, strip = 1600.0, 32.0
	text := "packed up Sea conditions · undo"

	t.Run("when a notice with a tone and a key is laid out", func(t *testing.T) {
		p := ui.LayoutNotice(th, ui.Message{Text: text, Tone: ui.ToneNeedsYou, Key: "u"}, width, strip, measure7)
		require.Len(t, p.Lines, 1)

		t.Run("it should centre the plate on the window's width", func(t *testing.T) {
			assert.InDelta(t, width/2, p.Rect.Center().X, 0.5)
		})

		t.Run("it should sit one grid under the strip", func(t *testing.T) {
			assert.Equal(t, strip+th.Grid(), p.Rect.Min.Y)
		})

		t.Run("it should set the text in Body type", func(t *testing.T) {
			assert.Equal(t, ui.Text{Text: text, At: p.Lines[0].At, Size: ui.Body}, p.Lines[0])
		})

		t.Run("it should put the dot before the text", func(t *testing.T) {
			assert.True(t, p.Dot.Area() > 0 && p.Dot.Max.X <= p.Lines[0].At.X, "dot %v, text at %v", p.Dot, p.Lines[0].At)
		})

		t.Run("it should put the key cap after the text", func(t *testing.T) {
			textEnd := p.Lines[0].At.X + float64(len([]rune(text)))*7
			assert.True(t, p.Cap.Rect.Area() > 0 && p.Cap.Rect.Min.X >= textEnd, "cap %v, text ends %v", p.Cap.Rect, textEnd)
		})

		t.Run("it should name the key on the cap", func(t *testing.T) {
			assert.Equal(t, "u", p.Cap.Label.Text)
		})

		t.Run("it should keep the dot, the text and the cap inside the plate", func(t *testing.T) {
			assert.Equal(t, p.Rect, p.Rect.Union(p.Dot).Union(p.Cap.Rect))
		})
	})

	t.Run("when the notice has no tone", func(t *testing.T) {
		p := ui.LayoutNotice(th, ui.Message{Text: text, Key: "u"}, width, strip, measure7)

		t.Run("it should have no dot", func(t *testing.T) {
			assert.Zero(t, p.Dot.Area())
		})
	})

	t.Run("when the notice has no key", func(t *testing.T) {
		p := ui.LayoutNotice(th, ui.Message{Text: text, Tone: ui.ToneNeedsYou}, width, strip, measure7)

		t.Run("it should have no cap", func(t *testing.T) {
			assert.Zero(t, p.Cap.Rect.Area())
		})
	})

	t.Run("when the text is wider than the window", func(t *testing.T) {
		long := strings.TrimSpace(strings.Repeat("word ", 80))
		p := ui.LayoutNotice(th, ui.Message{Text: long, Tone: ui.ToneError, Key: "c"}, 600, strip, measure7)

		t.Run("it should wrap onto lines inside the window", func(t *testing.T) {
			assert.True(t, len(p.Lines) > 1 && p.Rect.Min.X >= 0 && p.Rect.Max.X <= 600, "%d lines, plate %v", len(p.Lines), p.Rect)
		})

		t.Run("it should cap at three lines", func(t *testing.T) {
			assert.LessOrEqual(t, len(p.Lines), 3)
		})

		t.Run("it should grow the plate to hold every line", func(t *testing.T) {
			last := p.Lines[len(p.Lines)-1]
			assert.LessOrEqual(t, last.At.Y+16, p.Rect.Max.Y)
		})
	})

	t.Run("when the text is empty", func(t *testing.T) {
		p := ui.LayoutNotice(th, ui.Message{Key: "u"}, width, strip, measure7)

		t.Run("it should lay out nothing", func(t *testing.T) {
			assert.Equal(t, ui.NoticePlate{}, p)
		})
	})
}

// Item 352: a key cap is a rect and a label, drawn like the footer's
// chip, in Small type; the notice, and item 350's menu, share it.
func TestLayoutKeyCap(t *testing.T) {
	th := ui.NewTheme(1)
	cap := ui.LayoutKeyCap(th, "esc", city.Point{X: 100, Y: 50}, measure7)

	t.Run("it should start where it is put", func(t *testing.T) {
		assert.Equal(t, city.Point{X: 100, Y: 50}, cap.Rect.Min)
	})

	t.Run("it should set the key in Small type", func(t *testing.T) {
		assert.Equal(t, ui.Small, cap.Label.Size)
	})

	t.Run("it should pad the key half a grid each side", func(t *testing.T) {
		assert.Equal(t, 3*7+8.0, cap.Rect.Width())
	})

	t.Run("it should keep the label inside the cap", func(t *testing.T) {
		assert.True(t, cap.Rect.Contains(cap.Label.At), "label at %v, cap %v", cap.Label.At, cap.Rect)
	})
}
