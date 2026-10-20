package render

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A window's glow is an additive disc, so two that overlap are brighter
// than either and a column of them saturates to white. At fit that is
// the point — what matters is which lights are on, not their size — but
// the radius grows with the zoom and the storeys it sits between do not
// grow faster, so at zoom 2 and above a five-storey building wore five
// merged discs and vanished behind its own windows.
func TestWindowGlowFitsItsStorey(t *testing.T) {
	t.Run("when the radius sits comfortably inside a storey", func(t *testing.T) {
		t.Run("it should be left alone", func(t *testing.T) {
			assert.InDelta(t, 8.0, windowGlow(8, 2.5, 40), 1e-9)
		})
	})

	t.Run("when the radius is wider than the storey it lights", func(t *testing.T) {
		got := windowGlow(32, 2.5, 26)

		t.Run("it should be held to half a storey, so two do not merge", func(t *testing.T) {
			assert.InDelta(t, 13.0, got, 1e-9)
		})

		t.Run("it should never exceed the gap between storeys", func(t *testing.T) {
			for _, storey := range []float64{4, 12, 26, 60, 140} {
				require.LessOrEqual(t, windowGlow(1000, 2.5, storey), storey)
			}
		})
	})

	t.Run("when the view is so far out that the building is a few pixels", func(t *testing.T) {
		t.Run("it should keep its floor, so a lit building still reads", func(t *testing.T) {
			assert.InDelta(t, 2.5, windowGlow(0.3, 2.5, 40), 1e-9)
		})

		t.Run("it should not let the floor outgrow the storey either", func(t *testing.T) {
			assert.InDelta(t, 1.0, windowGlow(0.3, 2.5, 2), 1e-9)
		})
	})

	t.Run("when a building has one storey and no room at all", func(t *testing.T) {
		t.Run("it should still draw something rather than nothing", func(t *testing.T) {
			assert.Greater(t, windowGlow(5, 2.5, 0), 0.0)
		})
	})
}

// Item 74: a building that has not arrived, or is still going up, lit
// every window of its finished height over an empty lot.
func TestDimLight(t *testing.T) {
	light := color.NRGBA{0xff, 0xc8, 0x70, 0x40}

	t.Run("when a light is dimmed by its building's lights", func(t *testing.T) {
		for _, c := range []struct {
			lights string
			k      float64
			want   uint8
		}{
			{"fully on", 1, 0x40},
			{"half on", 0.5, 0x20},
			{"off", 0, 0},
		} {
			t.Run("and the lights are "+c.lights, func(t *testing.T) {
				t.Run("it should scale its alpha to them", func(t *testing.T) {
					assert.Equal(t, c.want, dimLight(light, c.k).A)
				})
			})
		}
	})
}

// Item 155: a building lights floor by floor as it rises, not all at
// once at the end of its build.
func TestWindowRows(t *testing.T) {
	const storey, plinth = 30.0, 2.0
	wallTop := func(n int) float64 { return plinth + float64(n)*storey }

	t.Run("when a four-storey building is two and a half storeys built", func(t *testing.T) {
		built := plinth + 2.5*storey
		rows := windowRows(4, storey, plinth, built)

		t.Run("it should light exactly its two finished storeys", func(t *testing.T) {
			assert.Len(t, rows, 2)
		})

		t.Run("it should leave the storey under construction dark", func(t *testing.T) {
			for _, y := range rows {
				require.False(t, y > wallTop(2) && y < wallTop(3), "a row at %v is inside the third storey", y)
			}
		})

		t.Run("it should draw no row above the built height", func(t *testing.T) {
			for _, y := range rows {
				require.LessOrEqual(t, y, built)
			}
		})
	})

	t.Run("when a building is fully built", func(t *testing.T) {
		t.Run("it should light every storey", func(t *testing.T) {
			assert.Len(t, windowRows(4, storey, plinth, wallTop(4)), 4)
		})
	})

	t.Run("when a building has not risen at all", func(t *testing.T) {
		t.Run("it should light nothing", func(t *testing.T) {
			assert.Empty(t, windowRows(4, storey, plinth, 0))
		})
	})

	t.Run("when a building's session has grown a storey", func(t *testing.T) {
		t.Run("it should light the new storey too", func(t *testing.T) {
			assert.Len(t, windowRows(4, storey, plinth, wallTop(4)), len(windowRows(3, storey, plinth, wallTop(3)))+1)
		})
	})

	t.Run("when the built height is exactly a storey's top", func(t *testing.T) {
		t.Run("it should light that storey", func(t *testing.T) {
			assert.Len(t, windowRows(4, storey, plinth, wallTop(1)), 1)
		})
	})
}
