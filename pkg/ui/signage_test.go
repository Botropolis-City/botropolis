package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutSign(t *testing.T) {
	face := city.RectAt(100, 50, 92, 32)
	side := city.RectAt(130, 100, 32, 160)

	t.Run("when a short name fits the tank face", func(t *testing.T) {
		sign, ok := ui.LayoutSign("slack", face, side, measure7)
		require.True(t, ok)

		t.Run("it should run across the face", func(t *testing.T) {
			assert.False(t, sign.Vertical)
		})

		t.Run("it should be scaled to the face's height", func(t *testing.T) {
			assert.InDelta(t, 32*ui.SignFill/16, sign.Scale, 1e-9)
		})

		t.Run("it should be centred on the face", func(t *testing.T) {
			w, h := 5*7*sign.Scale, 16*sign.Scale
			assert.InDelta(t, face.Center().X-w/2, sign.At.X, 1e-9)
			assert.InDelta(t, face.Center().Y-h/2, sign.At.Y, 1e-9)
		})
	})

	t.Run("when a long name fits only the leg column", func(t *testing.T) {
		sign, ok := ui.LayoutSign("claude.ai Atlassian", face, side, measure7)
		require.True(t, ok)

		t.Run("it should run up the side", func(t *testing.T) {
			assert.True(t, sign.Vertical)
		})

		t.Run("it should be scaled to the column", func(t *testing.T) {
			assert.InDelta(t, 160*ui.SignFill/(19*7), sign.Scale, 1e-9)
		})

		t.Run("it should start at the column's foot, centred across it", func(t *testing.T) {
			w, h := 19*7*sign.Scale, 16*sign.Scale
			assert.InDelta(t, side.Center().X-h/2, sign.At.X, 1e-9)
			assert.InDelta(t, side.Center().Y+w/2, sign.At.Y, 1e-9)
		})
	})

	t.Run("when the tower is too small to read at this zoom", func(t *testing.T) {
		tiny := city.RectAt(0, 0, 9, 3)
		_, ok := ui.LayoutSign("slack", tiny, city.RectAt(0, 0, 3, 16), measure7)

		t.Run("it should hang no sign", func(t *testing.T) {
			assert.False(t, ok)
		})
	})
}

func TestLayoutBuildingSign(t *testing.T) {
	// A fascia band over the door, a strip up the flank, and the room
	// above the roofline, all in screen pixels.
	fascia := city.RectAt(100, 200, 120, 26)
	side := city.RectAt(100, 120, 26, 140)
	roof := city.RectAt(60, 40, 200, 70)

	t.Run("when the title is short enough for the fascia", func(t *testing.T) {
		sign, ok := ui.LayoutBuildingSign("ci queue", fascia, side, roof, measure7)
		require.True(t, ok)
		require.Len(t, sign.Lines, 1)

		t.Run("it should paint one line over the door", func(t *testing.T) {
			assert.False(t, sign.Lines[0].Vertical)
		})

		t.Run("it should hang no billboard", func(t *testing.T) {
			assert.False(t, sign.Billboard())
		})

		t.Run("it should keep the text inside the fascia", func(t *testing.T) {
			w, _ := measure7(sign.Lines[0].Text, ui.Small)
			assert.LessOrEqual(t, w*sign.Lines[0].Scale, fascia.Width())
		})
	})

	t.Run("when the title is too long for the fascia but the building is tall", func(t *testing.T) {
		tall := city.RectAt(100, 60, 26, 300)
		sign, ok := ui.LayoutBuildingSign("phase sixteen planting and the plaza", fascia, tall, roof, measure7)
		require.True(t, ok)
		require.Len(t, sign.Lines, 1)

		t.Run("it should run up the side", func(t *testing.T) {
			assert.True(t, sign.Lines[0].Vertical)
		})
	})

	t.Run("when the title fits neither face", func(t *testing.T) {
		narrow := city.RectAt(100, 200, 40, 10)
		thin := city.RectAt(100, 190, 8, 30)
		sign, ok := ui.LayoutBuildingSign("start ticket 613 worktree and land it", narrow, thin, roof, measure7)
		require.True(t, ok)

		t.Run("it should go on a rooftop billboard", func(t *testing.T) {
			assert.True(t, sign.Billboard())
		})

		t.Run("it should carry two lines", func(t *testing.T) {
			assert.Len(t, sign.Lines, ui.BillboardLines)
		})

		t.Run("it should ellipsise what does not fit", func(t *testing.T) {
			joined := sign.Lines[0].Text + sign.Lines[1].Text
			assert.Contains(t, joined, "…")
		})

		t.Run("it should stand the board on two posts down to the roof", func(t *testing.T) {
			for _, post := range sign.Posts {
				assert.Equal(t, sign.Panel.Max.Y, post.Min.Y)
				assert.Equal(t, roof.Max.Y, post.Max.Y)
			}
		})

		t.Run("it should keep every line inside the panel", func(t *testing.T) {
			for _, line := range sign.Lines {
				w, _ := measure7(line.Text, ui.Small)
				assert.LessOrEqual(t, line.At.X, sign.Panel.Max.X)
				assert.GreaterOrEqual(t, line.At.X+w*line.Scale, sign.Panel.Min.X)
			}
		})
	})

	t.Run("when even a billboard would be under seven pixels", func(t *testing.T) {
		_, ok := ui.LayoutBuildingSign("anything at all", city.RectAt(0, 0, 8, 3), city.RectAt(0, 0, 3, 8), city.RectAt(0, 0, 12, 6), measure7)

		t.Run("it should hang nothing and leave the name to the hover plate", func(t *testing.T) {
			assert.False(t, ok)
		})
	})

	t.Run("when the session has no title", func(t *testing.T) {
		_, ok := ui.LayoutBuildingSign("", fascia, side, roof, measure7)

		t.Run("it should hang nothing", func(t *testing.T) {
			assert.False(t, ok)
		})
	})
}
