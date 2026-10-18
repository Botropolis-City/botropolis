package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 89: a working building is scaffolded on its roof, and each tool
// call runs a load up a hoist on its outside front corner to it.
func TestBuildingWork(t *testing.T) {
	// A footprint on screen, north, east, south, west, and a roof 60 px up.
	lot := [4]city.Point{{X: 100, Y: 80}, {X: 160, Y: 110}, {X: 100, Y: 140}, {X: 40, Y: 110}}
	const roofLift = 60.0
	front := lot[2]

	for _, level := range city.LivelinessLevels {
		t.Run("when the level is "+level.String(), func(t *testing.T) {
			t.Run("a mid-turn building's scaffold should stand on its roof, inside its footprint, and rise above it", func(t *testing.T) {
				var wrong int
				for _, s := range roofScaffold(lot, roofLift, 1) {
					if !s.pole {
						continue
					}
					onRoof := false
					for _, c := range lot {
						if s.a.Y <= c.Y-roofLift+1e-9 && s.a.Y >= lot[0].Y-roofLift-1e-9 {
							onRoof = true
						}
					}
					inside := s.a.X >= lot[3].X && s.a.X <= lot[1].X
					if !onRoof || !inside || s.b.Y >= s.a.Y {
						wrong++
					}
				}
				assert.Zero(t, wrong)
			})
		})
	}

	// Aria, 2026-10-17: "Heavier scaffold".
	t.Run("when a scaffold is drawn", func(t *testing.T) {
		lines := roofScaffold(lot, roofLift, 1)
		var planks, thin int
		for _, s := range lines {
			if s.pole && s.width < 2 {
				thin++
			}
			if s.plank {
				planks++
			}
		}

		t.Run("its poles should be at least 2 px thick", func(t *testing.T) {
			assert.Zero(t, thin)
		})

		t.Run("it should have two plank levels round it", func(t *testing.T) {
			assert.Equal(t, 2*len(lot), planks)
		})
	})

	t.Run("when a load is drawn at the opening zoom", func(t *testing.T) {
		t.Run("its crate should be at least 6 px", func(t *testing.T) {
			assert.GreaterOrEqual(t, crateSize(20), 6.0)
		})
	})

	t.Run("when a load starts", func(t *testing.T) {
		t.Run("it should be at the foot of the outside front corner", func(t *testing.T) {
			at := hoistAt(lot, roofLift, 0)
			assert.InDelta(t, front.Y, at.Y, 1e-9)
		})
	})

	t.Run("when a load arrives", func(t *testing.T) {
		t.Run("it should be at the scaffold, on the roof", func(t *testing.T) {
			at := hoistAt(lot, roofLift, 1)
			assert.InDelta(t, front.Y-roofLift, at.Y, 1e-9)
		})
	})

	t.Run("while a load climbs", func(t *testing.T) {
		t.Run("it should keep to one line up the corner", func(t *testing.T) {
			assert.InDelta(t, hoistAt(lot, roofLift, 0).X, hoistAt(lot, roofLift, 0.6).X, 1e-9)
		})
	})

	t.Run("when the scaffold and a load are drawn with their building and a nearer building", func(t *testing.T) {
		cam := city.NewCamera()
		own := city.RectAt(5*city.Tile, 5*city.Tile, city.Tile, city.Tile)
		nearer := city.RectAt(6*city.Tile, 6*city.Tile, city.Tile, city.Tile)
		got := namedOrder(cam, func(rec func(string) func()) []drawable {
			return []drawable{
				footprintAt(cam, nearer, rec("nearer")),
				buildingItem(cam, own, rec("building"), rec("work")),
			}
		})

		t.Run("they should be drawn after their building and before the nearer one", func(t *testing.T) {
			assert.Equal(t, []string{"building", "work", "nearer"}, got)
		})
	})

	t.Run("when a drone is drawn at the fit zoom on a small building", func(t *testing.T) {
		t.Run("it should be at least 12 px wide", func(t *testing.T) {
			assert.GreaterOrEqual(t, droneWidth(20), 12.0)
		})
	})

	t.Run("when a drone is drawn at a normal zoom", func(t *testing.T) {
		t.Run("it should still be about one window wide", func(t *testing.T) {
			assert.InDelta(t, droneWindow*200, droneWidth(200), 1e-9)
		})
	})
}
