package city_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 108: a road runs from one lot's driveway to the other's, and each
// lot's driveway is on the side of its block facing the camera.

// sixDistricts is three districts across the first row of blocks, two
// on the second with a gap between them, and one on the third:
//
//	a b c
//	d   e
//	f
func sixDistricts() []state.Session {
	var out []state.Session
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		out = append(out, session(name, "/p/"+name, state.Working))
	}
	return out
}

func roadsBetween(pairs ...[2]string) []state.Road {
	var out []state.Road
	for _, p := range pairs {
		out = append(out, state.Road{From: "/p/" + p[0], To: "/p/" + p[1], Messages: 1, Sessions: 1, Active: 1})
	}
	return out
}

func districtNamed(c *city.City, name string) *city.District {
	for _, d := range c.Districts {
		if d.Name == name {
			return d
		}
	}
	return nil
}

func streetOf(c *city.City, from, to string) (city.Street, bool) {
	for _, st := range c.Streets {
		if st.Road.From.Name == from && st.Road.To.Name == to {
			return st, true
		}
	}
	return city.Street{}, false
}

// toward is the side of a cell a point lies on, as a direction bit.
func toward(cell city.Cell, p city.Point) int {
	d := p.Sub(cell.Center())
	switch {
	case math.Abs(d.X) >= math.Abs(d.Y) && d.X > 0:
		return city.DirE
	case math.Abs(d.X) >= math.Abs(d.Y):
		return city.DirW
	case d.Y > 0:
		return city.DirS
	}
	return city.DirN
}

// byDriveway is where in a path it goes through a driveway: the lot end
// and the driveway cell next to each other, in either order, or -1.
func byDriveway(path []city.Point, dw city.Driveway) int {
	for i := 1; i < len(path); i++ {
		out := path[i-1] == dw.Lot && city.CellOf(path[i]) == dw.Cell
		in := city.CellOf(path[i-1]) == dw.Cell && path[i] == dw.Lot
		if out || in {
			return i
		}
	}
	return -1
}

func streetCellAt(c *city.City, cell city.Cell) (city.StreetCell, bool) {
	for _, sc := range c.StreetCells {
		if sc.Cell == cell {
			return sc, true
		}
	}
	return city.StreetCell{}, false
}

func TestDriveways(t *testing.T) {
	for _, quarters := range []int{0, 1, 2, 3} {
		heading := quarters * 90
		t.Run(fmt.Sprintf("when a city of six districts is seen at heading %d", heading), func(t *testing.T) {
			s := city.NewScene(city.NewLayout())
			s.SetClock(func() time.Time { return now }, time.UTC)
			s.Resize(1600, 1000)
			s.SetSnapshot(snapshot(sixDistricts()...))
			for k := 0; k < quarters; k++ {
				s.Turn()
			}
			c := s.City()
			cam := city.Camera{Heading: heading}
			require.Len(t, c.Districts, 6, "precondition: six districts")

			for _, d := range c.Districts {
				t.Run("and the district is "+d.Name, func(t *testing.T) {
					dw, ok := c.Driveway(d)
					block := cam.TurnRect(d.Rect)
					cell := cam.TurnPoint(dw.Cell.Center())

					t.Run("it should have a driveway", func(t *testing.T) {
						assert.True(t, ok)
					})

					t.Run("it should put its driveway on the avenue along the side facing the camera", func(t *testing.T) {
						assert.True(t, c.IsStreet(dw.Cell) && cell.Y > block.Max.Y && cell.Y < block.Max.Y+city.BuildingSize,
							"driveway %v at %v, block %v", dw.Cell, cell, block)
					})

					t.Run("it should put its driveway under the middle of that side", func(t *testing.T) {
						assert.LessOrEqual(t, math.Abs(cell.X-block.Center().X), city.BuildingSize/2+1e-9)
					})

					t.Run("it should put the lot end inside the block, between the driveway and the buildings", func(t *testing.T) {
						lot := cam.TurnPoint(dw.Lot)
						assert.True(t, d.Rect.Contains(dw.Lot) && block.Max.Y-lot.Y <= city.DistrictPadding,
							"lot end %v, block %v", lot, block)
					})
				})
			}
		})
	}

	t.Run("when a road joins two districts", func(t *testing.T) {
		with := city.Build(state.Snapshot{At: now, Sessions: sixDistricts(), Roads: roadsBetween([2]string{"a", "b"})}, city.NewLayout())
		without := city.Build(snapshot(sixDistricts()...), city.NewLayout())
		dw, _ := with.Driveway(districtNamed(with, "a"))

		t.Run("it should pave the driveway", func(t *testing.T) {
			_, paved := streetCellAt(with, dw.Cell)
			assert.True(t, paved)
		})

		t.Run("and the road is gone, it should not pave the driveway", func(t *testing.T) {
			_, paved := streetCellAt(without, dw.Cell)
			assert.False(t, paved)
		})

		pads := with.Pads(with.StreetCells)
		into := func(lot city.Point) bool {
			for _, p := range pads {
				if p.Contains(lot) {
					return true
				}
			}
			return false
		}
		end, _ := with.Driveway(districtNamed(with, "b"))

		t.Run("it should pave a pad from each driveway into its lot", func(t *testing.T) {
			assert.Equal(t, [2]bool{true, true}, [2]bool{into(dw.Lot), into(end.Lot)})
		})

		t.Run("it should pave no pad off any other street cell", func(t *testing.T) {
			assert.Len(t, pads, 2)
		})

		t.Run("it should keep each pad inside its lot's padding", func(t *testing.T) {
			var out []city.Rect
			for _, p := range pads {
				a, b := districtNamed(with, "a").Rect, districtNamed(with, "b").Rect
				inA := a.Contains(p.Min) && a.Contains(p.Max) && p.Min.Y >= a.Max.Y-city.DistrictPadding
				inB := b.Contains(p.Min) && b.Contains(p.Max) && p.Min.Y >= b.Max.Y-city.DistrictPadding
				if !inA && !inB {
					out = append(out, p)
				}
			}
			assert.Empty(t, out)
		})

		t.Run("and the road has just gone", func(t *testing.T) {
			s := city.NewScene(city.NewLayout())
			s.SetClock(func() time.Time { return now }, time.UTC)
			s.Resize(1600, 1000)
			s.SetSnapshot(state.Snapshot{At: now, Sessions: sixDistricts(), Roads: roadsBetween([2]string{"a", "b"})})
			s.SetSnapshot(snapshot(sixDistricts()...))
			var growing []city.StreetCell
			for _, o := range s.Overgrowth() {
				growing = append(growing, city.StreetCell{Cell: o.Cell, Mask: o.Mask})
			}

			t.Run("it should grow the pads over with the driveways", func(t *testing.T) {
				assert.Len(t, s.City().Pads(growing), 2)
			})
		})
	})
}

func TestRoadsRunLotToLot(t *testing.T) {
	pairs := []struct {
		name     string
		from, to string
	}{
		{"neighbours across one avenue", "a", "b"},
		{"neighbours across the avenue below", "a", "d"},
		{"neighbours across a corner", "b", "e"},
		{"far apart", "c", "f"},
	}
	for _, pair := range pairs {
		t.Run("when a road joins "+pair.name, func(t *testing.T) {
			snap := snapshot(sixDistricts()...)
			snap.Roads = roadsBetween([2]string{pair.from, pair.to})
			c := city.Build(snap, city.NewLayout())
			street, ok := streetOf(c, pair.from, pair.to)
			require.True(t, ok, "precondition: the road is routed")
			from, to := districtNamed(c, pair.from), districtNamed(c, pair.to)
			start, _ := c.Driveway(from)
			end, _ := c.Driveway(to)
			path := street.Path
			require.GreaterOrEqual(t, len(path), 3, "precondition: lot, driveway, ..., driveway, lot")

			t.Run("it should start in the first lot and leave by its driveway", func(t *testing.T) {
				assert.Equal(t, [2]any{start.Lot, start.Cell}, [2]any{path[0], city.CellOf(path[1])})
			})

			t.Run("it should arrive by the second lot's driveway and end in that lot", func(t *testing.T) {
				assert.Equal(t, [2]any{end.Cell, end.Lot}, [2]any{city.CellOf(path[len(path)-2]), path[len(path)-1]})
			})

			t.Run("it should stay on the streets, cell to cell, between the two driveways", func(t *testing.T) {
				var strays []city.Point
				for i := 1; i < len(path)-1; i++ {
					if !c.IsStreet(city.CellOf(path[i])) || (i > 1 && math.Abs(math.Hypot(path[i].X-path[i-1].X, path[i].Y-path[i-1].Y)-city.BuildingSize) > 1e-9) {
						strays = append(strays, path[i])
					}
				}
				assert.Empty(t, strays)
			})

			t.Run("it should not run along the avenue beside both lots, only across it", func(t *testing.T) {
				beside := func(d *city.District, p city.Point) bool {
					grown := d.Rect.Inset(-city.BuildingSize)
					corner := (p.X < d.Rect.Min.X || p.X > d.Rect.Max.X) && (p.Y < d.Rect.Min.Y || p.Y > d.Rect.Max.Y)
					return grown.Contains(p) && !d.Rect.Contains(p) && !corner
				}
				both := 0
				for _, p := range path[1 : len(path)-1] {
					if beside(from, p) && beside(to, p) {
						both++
					}
				}
				assert.LessOrEqual(t, both, 1)
			})

			t.Run("it should lay both driveways with a stub into their lots", func(t *testing.T) {
				a, _ := streetCellAt(c, start.Cell)
				b, _ := streetCellAt(c, end.Cell)
				assert.Equal(t, [2]bool{true, true}, [2]bool{
					a.Mask&toward(start.Cell, start.Lot) != 0,
					b.Mask&toward(end.Cell, end.Lot) != 0,
				})
			})
		})
	}

	// Aria, 2026-10-18: a driveway goes on the side that keeps its lot's
	// roads shortest, the camera-facing side when it ties or is within a
	// cell or two.
	t.Run("when one lot stands directly behind the other", func(t *testing.T) {
		// a's front faces d's back across one avenue: with both driveways
		// facing the camera, the road went round d's whole depth.
		snap := snapshot(sixDistricts()...)
		snap.Roads = roadsBetween([2]string{"a", "d"})
		c := city.Build(snap, city.NewLayout())
		street, ok := streetOf(c, "a", "d")
		require.True(t, ok, "precondition: the road is routed")
		a, d := districtNamed(c, "a"), districtNamed(c, "d")
		require.Less(t, a.Rect.Max.Y, d.Rect.Min.Y, "precondition: a is north of d")

		t.Run("it should join them by the avenue between them", func(t *testing.T) {
			var off []city.Point
			for _, p := range street.Path[1 : len(street.Path)-1] {
				if p.Y < a.Rect.Max.Y || p.Y > d.Rect.Min.Y {
					off = append(off, p)
				}
			}
			assert.Empty(t, off)
		})

		t.Run("it should keep the front lot's driveway on the side facing the camera", func(t *testing.T) {
			dw, _ := c.Driveway(a)
			assert.Greater(t, dw.Cell.Center().Y, a.Rect.Max.Y)
		})

		t.Run("and a lot has no roads, it should keep the side facing the camera", func(t *testing.T) {
			other := districtNamed(c, "c")
			dw, _ := c.Driveway(other)
			assert.Greater(t, dw.Cell.Center().Y, other.Rect.Max.Y)
		})
	})

	t.Run("when another side is shorter by only a cell or two", func(t *testing.T) {
		// b and e meet across a corner: b's east side or e's west would
		// save a step or two, which is not worth a driveway out of view.
		snap := snapshot(sixDistricts()...)
		snap.Roads = roadsBetween([2]string{"b", "e"})
		c := city.Build(snap, city.NewLayout())
		require.Len(t, c.Streets, 1, "precondition: the road is routed")
		b := districtNamed(c, "b")

		t.Run("it should keep the side facing the camera", func(t *testing.T) {
			dw, _ := c.Driveway(b)
			assert.Greater(t, dw.Cell.Center().Y, b.Rect.Max.Y)
		})
	})

	t.Run("when two roads share a stretch of street", func(t *testing.T) {
		snap := snapshot(sixDistricts()...)
		snap.Roads = roadsBetween([2]string{"a", "b"}, [2]string{"a", "c"})
		c := city.Build(snap, city.NewLayout())
		seen := map[city.Cell]int{}
		var twice []city.Cell
		for _, sc := range c.StreetCells {
			seen[sc.Cell]++
			if seen[sc.Cell] == 2 {
				twice = append(twice, sc.Cell)
			}
		}

		t.Run("it should lay that stretch once", func(t *testing.T) {
			assert.Empty(t, twice)
		})
	})
}
