package ui_test

import (
	"fmt"
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

func TestDistrictSignBoard(t *testing.T) {
	foot := city.Point{X: 500, Y: 500}
	board := func(zoom float64) city.Rect {
		return ui.DistrictSignBoard("ratelimit-lib", "the agents' move", foot, zoom, measure7)
	}
	atOne := board(1)

	t.Run("when zoomed", func(t *testing.T) {
		for _, zoom := range append(append([]float64{}, city.ZoomSteps...), 4) {
			t.Run(fmt.Sprintf("to %v", zoom), func(t *testing.T) {
				t.Run("it should never draw the board taller than at zoom 1", func(t *testing.T) {
					assert.LessOrEqual(t, board(zoom).Height(), atOne.Height()+1e-9)
				})
			})
		}
	})

	t.Run("when zoomed in past zoom 1", func(t *testing.T) {
		for _, zoom := range []float64{1.5, 2, 4} {
			t.Run(fmt.Sprintf("to %v", zoom), func(t *testing.T) {
				t.Run("it should stand the board higher above its foot than at zoom 1", func(t *testing.T) {
					assert.Greater(t, foot.Y-board(zoom).Max.Y, foot.Y-atOne.Max.Y)
				})
			})
		}
	})

	t.Run("when zoomed out below the floor", func(t *testing.T) {
		_, ok := ui.LayoutDistrictSign("ratelimit-lib", "the agents' move", board(city.FitMinZoom), measure7)

		t.Run("it should leave the board off", func(t *testing.T) {
			assert.False(t, ok)
		})
	})

	t.Run("when zoomed to 1", func(t *testing.T) {
		w, _ := measure7("the agents' move", ui.Small)

		t.Run("it should give the board its readable size", func(t *testing.T) {
			assert.InDelta(t, w*ui.DistrictSignScale/ui.SignFill, atOne.Width(), 1e-9)
		})
	})
}
