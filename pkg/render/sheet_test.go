package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 166: the building sheet shows only what the city draws.
func TestSheetBuilding(t *testing.T) {
	t.Run("when a cell is a kind at a tier, lived in", func(t *testing.T) {
		b := sheetBuilding(city.OfficeDistrict, 3, false)

		t.Run("it should be that tier's piece", func(t *testing.T) {
			assert.Equal(t, "botropolis/office-3", buildingPiece(b))
		})

		t.Run("it should be lit", func(t *testing.T) {
			assert.True(t, b.Lit && !b.Blinds)
		})
	})

	t.Run("when a cell is boarded up", func(t *testing.T) {
		b := sheetBuilding(city.OfficeDistrict, 3, true)

		t.Run("it should be shuttered, dark and boarded as a shuttered session is", func(t *testing.T) {
			assert.Equal(t, []any{state.RungShuttered, false, true, true}, []any{b.Rung, b.Lit, b.Blinds, b.BoardedUp})
		})
	})

	t.Run("when a cell is the ruins", func(t *testing.T) {
		for _, boarded := range []bool{false, true} {
			b := sheetBuilding(city.ShoppingDistrict, 0, boarded)

			t.Run("it should be the kind's ruins", func(t *testing.T) {
				assert.Equal(t, "botropolis/shopping-ruins", buildingPiece(b))
			})

			t.Run("it should never be lit, since only a resting session is ruins", func(t *testing.T) {
				assert.False(t, b.Lit)
			})
		}
	})
}
