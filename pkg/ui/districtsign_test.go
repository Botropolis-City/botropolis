package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutDistrictSign(t *testing.T) {
	t.Run("when the board is large enough to read", func(t *testing.T) {
		face := city.RectAt(100, 100, 200, 60)
		s, ok := ui.LayoutDistrictSign("ratelimit-lib", "your move", face, measure7)
		require.True(t, ok)

		t.Run("it should paint the name and the move on two lines inside the board", func(t *testing.T) {
			nameW, nameH := measure7(s.Name.Text, ui.Small)
			moveW, _ := measure7(s.Move.Text, ui.Small)
			inside := func(at city.Point, w, h, scale float64) bool {
				return face.Contains(at) && face.Contains(city.Point{X: at.X + w*scale, Y: at.Y + h*scale})
			}
			assert.True(t, s.Name.Text == "ratelimit-lib" && s.Move.Text == "your move" &&
				s.Move.At.Y >= s.Name.At.Y+nameH*s.Name.Scale &&
				inside(s.Name.At, nameW, nameH, s.Name.Scale) && inside(s.Move.At, moveW, nameH, s.Move.Scale))
		})
	})

	t.Run("when the board is too small to read", func(t *testing.T) {
		_, ok := ui.LayoutDistrictSign("ratelimit-lib", "your move", city.RectAt(0, 0, 20, 6), measure7)

		t.Run("it should decline", func(t *testing.T) {
			assert.False(t, ok)
		})
	})
}
