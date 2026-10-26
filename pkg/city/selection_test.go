package city_test

import (
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 283: districts can be selected together and moved together.

var group = []string{"/p/b", "/p/c", "/p/d"}

func shifted(s, by plan.Slot) plan.Slot {
	return plan.Slot{Col: s.Col + by.Col, Row: s.Row + by.Row}
}

// groupOffset is the first offset that takes every root in the group to
// a slot that accepts it and that the rest of the layout matches: free
// when held is false, or one held by a district outside the group.
func groupOffset(t *testing.T, layout *city.Layout, roots []string, rings int, held bool) plan.Slot {
	t.Helper()
	in := map[string]bool{}
	for _, r := range roots {
		in[r] = true
	}
	holder := map[plan.Slot]string{}
	for root, s := range layout.Plan.Slots {
		holder[s] = root
	}
	for row := -2 * (rings + 1); row <= 2*(rings+1); row++ {
		for col := -2 * (rings + 1); col <= 2*(rings+1); col++ {
			by := plan.Slot{Col: col, Row: row}
			if by == (plan.Slot{}) {
				continue
			}
			ok, hits := true, 0
			for _, r := range roots {
				to := shifted(layout.Plan.Slots[r], by)
				if to.Ring() < 1 || to.Ring() > rings+1 {
					ok = false
				}
				if h, taken := holder[to]; taken && !in[h] {
					hits++
				}
			}
			if ok && (hits > 0) == held {
				return by
			}
		}
	}
	t.Fatalf("no offset with held=%v", held)
	return plan.Slot{}
}

func districtSlots(layout *city.Layout) map[string]plan.Slot {
	out := map[string]plan.Slot{}
	for root, s := range layout.Plan.Slots {
		out[root] = s
	}
	return out
}

func TestMoveDistricts(t *testing.T) {
	setup := func(t *testing.T) (*city.Layout, *city.City, map[string]plan.Slot) {
		layout := city.NewLayout()
		c := city.Build(snapshot(fourDistricts()...), layout)
		return layout, c, districtSlots(layout)
	}

	t.Run("when three districts are moved by an offset onto free slots", func(t *testing.T) {
		layout, c, before := setup(t)
		by := groupOffset(t, layout, group, c.Rings(), false)
		require.True(t, layout.MoveDistricts(group, by, c.Rings()))

		for _, root := range group {
			t.Run("it should move "+root+" by the offset", func(t *testing.T) {
				assert.Equal(t, shifted(before[root], by), layout.Plan.Slots[root])
			})
		}

		t.Run("it should leave the other districts alone", func(t *testing.T) {
			assert.Equal(t, before["/p/a"], layout.Plan.Slots["/p/a"])
		})

		t.Run("and the layout is saved and loaded again", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "layout.json")
			require.NoError(t, layout.Save(path))
			loaded, err := city.LoadLayout(path)
			require.NoError(t, err)

			t.Run("it should hold all three new slots", func(t *testing.T) {
				for _, root := range group {
					assert.Equal(t, shifted(before[root], by), loaded.Plan.Slots[root], root)
				}
			})
		})
	})

	t.Run("when one target is held by a district outside the group", func(t *testing.T) {
		layout, c, before := setup(t)
		by := groupOffset(t, layout, group, c.Rings(), true)
		ok := layout.MoveDistricts(group, by, c.Rings())

		t.Run("it should refuse the move", func(t *testing.T) {
			assert.False(t, ok)
		})

		t.Run("it should leave every slot as it was", func(t *testing.T) {
			assert.Equal(t, before, districtSlots(layout))
		})
	})

	t.Run("when the group moves onto its own slots", func(t *testing.T) {
		layout, c, before := setup(t)
		pair := []string{"/p/b", "/p/c"}
		by := plan.Slot{Col: before["/p/c"].Col - before["/p/b"].Col, Row: before["/p/c"].Row - before["/p/b"].Row}
		ok := layout.MoveDistricts(pair, by, c.Rings())

		t.Run("it should move them all", func(t *testing.T) {
			require.True(t, ok)
			assert.Equal(t, []plan.Slot{before["/p/c"], shifted(before["/p/c"], by)}, []plan.Slot{layout.Plan.Slots["/p/b"], layout.Plan.Slots["/p/c"]})
		})
	})

	for _, tc := range []struct {
		scenario string
		to       func(rings int) plan.Slot
	}{
		{"the plaza", func(int) plan.Slot { return plan.Slot{} }},
		{"past the outer ring", func(rings int) plan.Slot { return plan.Slot{Col: rings + 2, Row: 0} }},
	} {
		t.Run("when a target is "+tc.scenario, func(t *testing.T) {
			layout, c, before := setup(t)
			b := before["/p/b"]
			to := tc.to(c.Rings())
			ok := layout.MoveDistricts(group, plan.Slot{Col: to.Col - b.Col, Row: to.Row - b.Row}, c.Rings())

			t.Run("it should refuse the move", func(t *testing.T) {
				assert.False(t, ok)
			})

			t.Run("it should leave every slot as it was", func(t *testing.T) {
				assert.Equal(t, before, districtSlots(layout))
			})
		})
	}
}

func TestDistrictSelection(t *testing.T) {
	setup := func(t *testing.T) (*city.Scene, *city.Layout) {
		layout := city.NewLayout()
		s := city.NewScene(layout)
		s.Resize(1600, 1200)
		s.SetSnapshot(snapshot(fourDistricts()...))
		return s, layout
	}
	district := func(t *testing.T, s *city.Scene, root string) *city.District {
		t.Helper()
		for _, d := range s.City().Districts {
			if d.Root == root {
				return d
			}
		}
		t.Fatalf("no district %s", root)
		return nil
	}
	floor := func(t *testing.T, s *city.Scene, root string) city.Point {
		d := district(t, s, root)
		p := city.Point{X: d.Rect.Min.X + 1, Y: d.Rect.Min.Y + 1}
		for _, b := range d.Buildings {
			if b.Rect.Contains(p) {
				p = city.Point{X: d.Rect.Max.X - 1, Y: d.Rect.Max.Y - 1}
			}
		}
		return s.Camera().WorldToScreen(p)
	}
	// press moves the pointer onto a point first, as every frame does.
	press := func(s *city.Scene, p city.Point) bool {
		s.PointerMove(p)
		return s.PressDistrict(p, false)
	}
	selectGroup := func(t *testing.T, s *city.Scene) {
		for _, root := range group {
			require.True(t, s.ToggleDistrict(floor(t, s, root)))
		}
	}

	t.Run("when a district is toggled", func(t *testing.T) {
		s, _ := setup(t)
		s.ToggleDistrict(floor(t, s, "/p/a"))

		t.Run("it should be selected", func(t *testing.T) {
			assert.Equal(t, []string{"/p/a"}, s.SelectedDistricts())
		})

		t.Run("and it is toggled again", func(t *testing.T) {
			s.ToggleDistrict(floor(t, s, "/p/a"))

			t.Run("it should not be selected", func(t *testing.T) {
				assert.Empty(t, s.SelectedDistricts())
			})
		})
	})

	t.Run("when a toggle lands on open ground", func(t *testing.T) {
		s, _ := setup(t)

		t.Run("it should toggle nothing", func(t *testing.T) {
			assert.False(t, s.ToggleDistrict(s.Camera().WorldToScreen(s.City().Plaza.Center())))
		})
	})

	t.Run("when a box is drawn round three districts' floors", func(t *testing.T) {
		s, _ := setup(t)
		var lo, hi city.Point
		for i, root := range group {
			p := floor(t, s, root)
			if i == 0 {
				lo, hi = p, p
			}
			lo = city.Point{X: min(lo.X, p.X), Y: min(lo.Y, p.Y)}
			hi = city.Point{X: max(hi.X, p.X), Y: max(hi.Y, p.Y)}
		}
		touched := s.DistrictsInBox(lo, hi)
		s.SelectBox(lo, hi)

		t.Run("it should select every district the box touches", func(t *testing.T) {
			assert.Equal(t, touched, s.SelectedDistricts())
		})

		t.Run("it should select the three it was drawn round", func(t *testing.T) {
			assert.Subset(t, s.SelectedDistricts(), group)
		})
	})

	t.Run("when a box is drawn on one district's floor", func(t *testing.T) {
		s, _ := setup(t)
		p := floor(t, s, "/p/a")
		s.SelectBox(p, p.Add(city.Point{X: 1, Y: 1}))

		t.Run("it should select that district and nothing else", func(t *testing.T) {
			assert.Equal(t, []string{"/p/a"}, s.SelectedDistricts())
		})
	})

	t.Run("when the selection is cleared", func(t *testing.T) {
		s, _ := setup(t)
		selectGroup(t, s)
		s.ClearSelection()

		t.Run("it should be empty", func(t *testing.T) {
			assert.Empty(t, s.SelectedDistricts())
		})
	})

	t.Run("when a selected district is pressed and dropped at another slot", func(t *testing.T) {
		s, layout := setup(t)
		selectGroup(t, s)
		before := districtSlots(layout)
		by := groupOffset(t, layout, group, s.City().Rings(), false)
		target := s.Camera().WorldToScreen(s.City().SlotRect(shifted(before["/p/b"], by)).Center())
		require.True(t, press(s, floor(t, s, "/p/b")))
		s.DragDistrict(target)
		drag, _ := s.DistrictDrag()

		t.Run("it should carry the whole selection", func(t *testing.T) {
			assert.Len(t, drag.Members, 3)
		})

		t.Run("it should outline a target slot for each", func(t *testing.T) {
			assert.Len(t, drag.Targets, 3)
		})

		t.Run("it should take the drop", func(t *testing.T) {
			assert.True(t, drag.Droppable)
		})

		dropped := s.DropDistrict(target)

		t.Run("it should drop them", func(t *testing.T) {
			assert.True(t, dropped)
		})

		for _, root := range group {
			t.Run("it should move "+root+" keeping the group's shape", func(t *testing.T) {
				assert.Equal(t, shifted(before[root], by), layout.Plan.Slots[root])
			})
		}

		t.Run("it should keep the selection", func(t *testing.T) {
			assert.Equal(t, group, s.SelectedDistricts())
		})
	})

	t.Run("when a selected group is dropped where it would overlap", func(t *testing.T) {
		s, layout := setup(t)
		selectGroup(t, s)
		before := districtSlots(layout)
		by := groupOffset(t, layout, group, s.City().Rings(), true)
		target := s.Camera().WorldToScreen(s.City().SlotRect(shifted(before["/p/b"], by)).Center())
		require.True(t, press(s, floor(t, s, "/p/b")))
		s.DragDistrict(target)
		drag, _ := s.DistrictDrag()

		t.Run("it should report the drop refused", func(t *testing.T) {
			assert.False(t, drag.Droppable)
		})

		dropped := s.DropDistrict(target)

		t.Run("it should drop nothing", func(t *testing.T) {
			assert.False(t, dropped)
		})

		t.Run("it should move none of them", func(t *testing.T) {
			assert.Equal(t, before, districtSlots(layout))
		})
	})

	t.Run("when an unselected district is pressed and dropped", func(t *testing.T) {
		s, layout := setup(t)
		for _, root := range []string{"/p/c", "/p/d"} {
			require.True(t, s.ToggleDistrict(floor(t, s, root)))
		}
		before := districtSlots(layout)
		free := freeSlotOn(t, layout, 1)
		target := s.Camera().WorldToScreen(s.City().SlotRect(free).Center())
		require.True(t, press(s, floor(t, s, "/p/a")))
		s.DragDistrict(target)
		drag, _ := s.DistrictDrag()
		s.DropDistrict(target)

		t.Run("it should carry only that district", func(t *testing.T) {
			assert.Len(t, drag.Members, 1)
		})

		t.Run("it should move it", func(t *testing.T) {
			assert.Equal(t, free, layout.Plan.Slots["/p/a"])
		})

		t.Run("it should leave the selected districts where they were", func(t *testing.T) {
			assert.Equal(t, []plan.Slot{before["/p/c"], before["/p/d"]}, []plan.Slot{layout.Plan.Slots["/p/c"], layout.Plan.Slots["/p/d"]})
		})

		t.Run("it should leave the selection alone", func(t *testing.T) {
			assert.Equal(t, []string{"/p/c", "/p/d"}, s.SelectedDistricts())
		})
	})
}
