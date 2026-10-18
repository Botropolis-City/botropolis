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

		t.Run("it should have two plank levels spanning the grid", func(t *testing.T) {
			assert.Equal(t, 2*scaffoldSpans, planks)
		})
	})

	// Aria, 2026-10-17: eight supports, four corners and four middles, and
	// those at the back drawn behind the wood.
	t.Run("when a scaffold stands on a roof", func(t *testing.T) {
		lines := roofScaffold(lot, roofLift, 1)
		var poles, corners int
		for _, s := range lines {
			if s.pole {
				poles++
				if s.corner {
					corners++
				}
			}
		}

		t.Run("it should have eight poles, four of them at the corners", func(t *testing.T) {
			assert.Equal(t, [2]int{8, 4}, [2]int{poles, corners})
		})

		t.Run("no pole should stand in the middle of the roof", func(t *testing.T) {
			mid := onRoof(lot, roofLift, 0.5, 0.5)
			var central int
			for _, s := range lines {
				if s.pole && s.a == mid {
					central++
				}
			}
			assert.Zero(t, central)
		})
	})

	t.Run("when a scaffold is drawn in order", func(t *testing.T) {
		drawn := scaffoldOrder(roofScaffold(lot, roofLift, 1))
		lastBackPole, firstPlank := -1, len(drawn)
		for i, s := range drawn {
			if s.pole && s.back && i > lastBackPole {
				lastBackPole = i
			}
			if s.plank && i < firstPlank {
				firstPlank = i
			}
		}

		t.Run("no back pole should be drawn after a plank it passes behind", func(t *testing.T) {
			assert.Less(t, lastBackPole, firstPlank)
		})
	})

	// Aria, 2026-10-17: braced like scaffolding, lit at its corners.
	t.Run("when the scaffold is braced", func(t *testing.T) {
		lines := roofScaffold(lot, roofLift, 1)
		var braces []segment
		for _, s := range lines {
			if s.brace {
				braces = append(braces, s)
			}
		}
		roofMiddle := (lot[0].Y+lot[2].Y)/2 - roofLift

		t.Run("it should brace each bay of the two faces toward the camera", func(t *testing.T) {
			assert.Len(t, braces, 2*(len(scaffoldGrid)-1))
		})

		t.Run("every brace should be on a face toward the camera", func(t *testing.T) {
			var behind int
			for _, b := range braces {
				if (b.a.Y+b.b.Y)/2 < roofMiddle-storeyOf(roofLift) {
					behind++
				}
			}
			assert.Zero(t, behind)
		})
	})

	t.Run("when the work lights are placed", func(t *testing.T) {
		lights := workLights(roofScaffold(lot, roofLift, 1))

		t.Run("they should stand on the corner poles only, but the back one", func(t *testing.T) {
			assert.Len(t, lights, 3)
		})
	})

	t.Run("when the roof is scaffolded", func(t *testing.T) {
		items := roofItems(lot, roofLift, 1)

		// Aria, 2026-10-17: the column only; the stack and the pallet meant
		// nothing.
		t.Run("it should carry the next storey's column, and nothing else", func(t *testing.T) {
			var kinds []string
			for _, it := range items {
				kinds = append(kinds, it.kind)
			}
			assert.Equal(t, []string{"column"}, kinds)
		})

		t.Run("they should stand inside the footprint", func(t *testing.T) {
			var outside int
			for _, it := range items {
				if it.at.X < lot[3].X || it.at.X > lot[1].X {
					outside++
				}
			}
			assert.Zero(t, outside)
		})
	})

	t.Run("when the scaffold has come down", func(t *testing.T) {
		t.Run("the roof items should be gone", func(t *testing.T) {
			assert.Empty(t, roofItems(lot, roofLift, 0))
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

	// Aria, 2026-10-17: beside the pole, so it does not cover it.
	t.Run("when the hoist is drawn", func(t *testing.T) {
		var front segment
		for _, s := range roofScaffold(lot, roofLift, 1) {
			if s.pole && s.corner && s.a.Y > front.a.Y {
				front = s
			}
		}

		t.Run("it should run beside the front corner pole, not over it", func(t *testing.T) {
			gap := hoistAt(lot, roofLift, 0.5).X - front.a.X
			assert.Greater(t, max(gap, -gap), front.width)
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
