package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
