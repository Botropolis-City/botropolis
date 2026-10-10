package city_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestDoor(t *testing.T) {
	t.Run("when a building stands clear of its block's edges", func(t *testing.T) {
		c := build(t, city.NewLayout(), sessionsIn(cinders, 9)...)
		d := c.Districts[0]
		b := d.Buildings[0]
		door := d.Door(b)

		t.Run("it should stand on the middle of the front face", func(t *testing.T) {
			assert.Equal(t, b.Rect.Center().X, door.X)
		})

		t.Run("it should stand half a tile inside the footprint", func(t *testing.T) {
			assert.Equal(t, b.Rect.Max.Y-city.DoorInset, door.Y)
		})
	})

	t.Run("when a building is flush with the south edge of its block", func(t *testing.T) {
		c := build(t, city.NewLayout(), sessionsIn(cinders, 9)...)
		d := c.Districts[0]
		b := d.Buildings[0]
		b.Rect = city.RectAt(d.Rect.Min.X, d.Rect.Max.Y-city.BuildingSize, city.BuildingSize, city.BuildingSize)
		door := d.Door(b)

		t.Run("it should stay half a tile inside the block", func(t *testing.T) {
			assert.Equal(t, d.Rect.Max.Y-city.DoorInset, door.Y)
		})
	})

	t.Run("when a building hangs over the south edge of its block", func(t *testing.T) {
		c := build(t, city.NewLayout(), sessionsIn(cinders, 9)...)
		d := c.Districts[0]
		b := d.Buildings[0]
		b.Rect = city.RectAt(d.Rect.Min.X, d.Rect.Max.Y, city.BuildingSize, city.BuildingSize)
		door := d.Door(b)

		t.Run("it should be pulled back inside the block", func(t *testing.T) {
			assert.Equal(t, d.Rect.Max.Y-city.DoorInset, door.Y)
		})
	})

	t.Run("when a building belongs to no district", func(t *testing.T) {
		c := build(t, city.NewLayout(), sessionsIn(cinders, 1)...)
		b := c.Districts[0].Buildings[0]
		var loose *city.District
		door := loose.Door(b)

		t.Run("it should still be the front-face centre", func(t *testing.T) {
			assert.Equal(t, city.Point{X: b.Rect.Center().X, Y: b.Rect.Max.Y - city.DoorInset}, door)
		})
	})

	t.Run("when every building in a three-project city is asked for its door", func(t *testing.T) {
		var sessions []state.Session
		for _, root := range []string{cinders, botropolis, mullet} {
			for i := 0; i < 11; i++ {
				sessions = append(sessions, session(fmt.Sprintf("%s-%02d", root, i), root, state.Working))
			}
		}
		c := build(t, city.NewLayout(), sessions...)
		streets := map[city.Cell]bool{}
		for _, s := range c.StreetCells {
			streets[s.Cell] = true
		}
		var onStreet []string
		for _, d := range c.Districts {
			for _, b := range d.Buildings {
				if streets[city.CellOf(d.Door(b))] {
					onStreet = append(onStreet, b.Session.ID)
				}
			}
		}
		require.NotEmpty(t, c.StreetCells)

		t.Run("it should never put a worker on an avenue", func(t *testing.T) {
			assert.Empty(t, onStreet)
		})
	})
}
