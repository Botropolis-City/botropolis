package city_test

import (
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/plan"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 85, slice 2: a district can be dragged to another slot, and only
// the remembered slots move.

func fourDistricts() []state.Session {
	return []state.Session{session("a", "/p/a", state.Working), session("b", "/p/b", state.Working),
		session("c", "/p/c", state.Working), session("d", "/p/d", state.Working)}
}

func freeSlotOn(t *testing.T, layout *city.Layout, ring int) plan.Slot {
	t.Helper()
	taken := map[plan.Slot]bool{}
	for _, s := range layout.Plan.Slots {
		taken[s] = true
	}
	for row := -ring; row <= ring; row++ {
		for col := -ring; col <= ring; col++ {
			s := plan.Slot{Col: col, Row: row}
			if s.Ring() == ring && !taken[s] {
				return s
			}
		}
	}
	t.Fatalf("no free slot on ring %d", ring)
	return plan.Slot{}
}

func TestMoveDistrict(t *testing.T) {
	setup := func(t *testing.T) (*city.Layout, *city.City, map[string]plan.Slot) {
		layout := city.NewLayout()
		c := city.Build(snapshot(fourDistricts()...), layout)
		before := map[string]plan.Slot{}
		for root, s := range layout.Plan.Slots {
			before[root] = s
		}
		return layout, c, before
	}

	t.Run("when a district is moved to a free slot", func(t *testing.T) {
		layout, c, before := setup(t)
		to := freeSlotOn(t, layout, 1)
		ok := layout.MoveDistrict("/p/a", to, c.Rings())
		require.True(t, ok)

		t.Run("it should remember the new slot for that district", func(t *testing.T) {
			assert.Equal(t, to, layout.Plan.Slots["/p/a"])
		})

		t.Run("it should leave every other district's slot alone", func(t *testing.T) {
			for _, root := range []string{"/p/b", "/p/c", "/p/d"} {
				assert.Equal(t, before[root], layout.Plan.Slots[root], root)
			}
		})
	})

	t.Run("when it is moved to a slot another district holds", func(t *testing.T) {
		layout, c, before := setup(t)
		ok := layout.MoveDistrict("/p/a", before["/p/b"], c.Rings())
		require.True(t, ok)

		t.Run("it should give the mover that slot", func(t *testing.T) {
			assert.Equal(t, before["/p/b"], layout.Plan.Slots["/p/a"])
		})

		t.Run("it should give the other district the mover's old slot", func(t *testing.T) {
			assert.Equal(t, before["/p/a"], layout.Plan.Slots["/p/b"])
		})
	})

	for _, tc := range []struct {
		scenario string
		to       func(rings int) plan.Slot
		ok       bool
	}{
		{"onto the plaza's slot", func(int) plan.Slot { return plan.Slot{} }, false},
		{"one ring past the city's outer ring", func(rings int) plan.Slot { return plan.Slot{Col: rings + 1, Row: 0} }, true},
		{"two rings past the city's outer ring", func(rings int) plan.Slot { return plan.Slot{Col: rings + 2, Row: 0} }, false},
	} {
		t.Run("when it is moved "+tc.scenario, func(t *testing.T) {
			layout, c, before := setup(t)
			ok := layout.MoveDistrict("/p/a", tc.to(c.Rings()), c.Rings())

			t.Run("it should accept or refuse it", func(t *testing.T) {
				assert.Equal(t, tc.ok, ok)
			})

			if !tc.ok {
				t.Run("it should change nothing", func(t *testing.T) {
					assert.Equal(t, before, layout.Plan.Slots)
				})
			}
		})
	}

	t.Run("when the layout is saved and loaded again", func(t *testing.T) {
		layout, c, _ := setup(t)
		to := freeSlotOn(t, layout, 1)
		require.True(t, layout.MoveDistrict("/p/a", to, c.Rings()))
		path := filepath.Join(t.TempDir(), "layout.json")
		require.NoError(t, layout.Save(path))
		loaded, err := city.LoadLayout(path)
		require.NoError(t, err)

		t.Run("it should keep the moved slot", func(t *testing.T) {
			assert.Equal(t, to, loaded.Plan.Slots["/p/a"])
		})
	})
}

func TestSlotAt(t *testing.T) {
	layout := city.NewLayout()
	c := city.Build(snapshot(fourDistricts()...), layout)
	var a *city.District
	for _, d := range c.Districts {
		if d.Root == "/p/a" {
			a = d
		}
	}
	require.NotNil(t, a)

	t.Run("when a point is inside a block's slot", func(t *testing.T) {
		slot, ok := c.DropSlot(a.Rect.Center())

		t.Run("it should name that slot", func(t *testing.T) {
			require.True(t, ok)
			assert.Equal(t, layout.Plan.Slots["/p/a"], slot)
		})
	})

	t.Run("when a point is on the plaza", func(t *testing.T) {
		_, ok := c.DropSlot(c.Plaza.Center())

		t.Run("it should report no droppable slot", func(t *testing.T) {
			assert.False(t, ok)
		})
	})

	t.Run("when a point is in the ring outside", func(t *testing.T) {
		east := c.SlotRect(plan.Slot{Col: c.Rings(), Row: 0})
		outside := city.Point{X: east.Max.X + city.CellSize*2.5, Y: east.Center().Y}
		slot, ok := c.DropSlot(outside)

		t.Run("it should name that outer slot", func(t *testing.T) {
			require.True(t, ok)
			assert.Equal(t, plan.Slot{Col: c.Rings() + 1, Row: 0}, slot)
		})
	})
}

func TestDistrictDrag(t *testing.T) {
	setup := func(t *testing.T) (*city.Scene, *city.Layout, *city.District) {
		layout := city.NewLayout()
		s := city.NewScene(layout)
		s.Resize(1600, 1200)
		s.SetSnapshot(snapshot(fourDistricts()...))
		for _, d := range s.City().Districts {
			if d.Root == "/p/a" {
				return s, layout, d
			}
		}
		t.Fatal("no district a")
		return nil, nil, nil
	}
	floor := func(s *city.Scene, d *city.District) city.Point {
		// The district's own floor: a corner of its block clear of buildings.
		p := city.Point{X: d.Rect.Min.X + 1, Y: d.Rect.Min.Y + 1}
		for _, b := range d.Buildings {
			if b.Rect.Contains(p) {
				p = city.Point{X: d.Rect.Max.X - 1, Y: d.Rect.Max.Y - 1}
			}
		}
		return s.Camera().WorldToScreen(p)
	}

	t.Run("when a press starts on a district's floor", func(t *testing.T) {
		s, layout, d := setup(t)
		started := s.PressDistrict(floor(s, d), false)
		free := freeSlotOn(t, layout, 1)
		target := s.Camera().WorldToScreen(s.City().SlotRect(free).Center())
		offset := s.Camera().Offset
		s.DragDistrict(target)
		drag, dragging := s.DistrictDrag()

		t.Run("it should pick the district up", func(t *testing.T) {
			assert.True(t, started)
		})

		t.Run("and the pointer moves, it should be dragging that district", func(t *testing.T) {
			require.True(t, dragging)
			assert.Equal(t, d.Root, drag.District.Root)
		})

		t.Run("and the pointer moves, it should not pan", func(t *testing.T) {
			assert.Equal(t, offset, s.Camera().Offset)
		})

		t.Run("it should outline the slot under the pointer as the target", func(t *testing.T) {
			assert.Equal(t, s.City().SlotRect(free), drag.Target)
		})

		t.Run("and it is released over a slot", func(t *testing.T) {
			others := map[string]city.Rect{}
			for _, o := range s.City().Districts {
				if o.Root != d.Root {
					others[o.Root] = o.Rect
				}
			}
			dropped := s.DropDistrict(target)

			t.Run("it should drop it", func(t *testing.T) {
				assert.True(t, dropped)
			})

			t.Run("it should move the district there", func(t *testing.T) {
				assert.Equal(t, free, layout.Plan.Slots[d.Root])
			})

			t.Run("it should rebuild the city with the district on its new slot", func(t *testing.T) {
				for _, o := range s.City().Districts {
					if o.Root == d.Root {
						assert.True(t, o.Rect.Contains(s.City().SlotRect(free).Center()))
					}
				}
			})

			t.Run("it should not move any other district's block", func(t *testing.T) {
				for _, o := range s.City().Districts {
					if want, ok := others[o.Root]; ok {
						assert.Equal(t, want, o.Rect, o.Root)
					}
				}
			})

			t.Run("it should stop dragging", func(t *testing.T) {
				_, still := s.DistrictDrag()
				assert.False(t, still)
			})
		})
	})

	t.Run("when a press starts on a building", func(t *testing.T) {
		s, _, d := setup(t)
		require.NotEmpty(t, d.Buildings)

		t.Run("it should not pick up the district", func(t *testing.T) {
			assert.False(t, s.PressDistrict(s.Camera().WorldToScreen(d.Buildings[0].Rect.Center()), false))
		})
	})

	t.Run("when a press starts on open ground", func(t *testing.T) {
		s, _, _ := setup(t)

		t.Run("it should not pick up a district", func(t *testing.T) {
			assert.False(t, s.PressDistrict(s.Camera().WorldToScreen(s.City().Plaza.Center()), false))
		})
	})

	for _, key := range []string{"Shift", "Alt"} {
		t.Run("when "+key+" is held", func(t *testing.T) {
			s, _, d := setup(t)

			t.Run("it should pan, wherever the press started", func(t *testing.T) {
				assert.False(t, s.PressDistrict(floor(s, d), true))
			})
		})
	}

	t.Run("when released over the plaza", func(t *testing.T) {
		s, layout, d := setup(t)
		was := layout.Plan.Slots[d.Root]
		require.True(t, s.PressDistrict(floor(s, d), false))
		plaza := s.Camera().WorldToScreen(s.City().Plaza.Center())
		s.DragDistrict(plaza)
		drag, _ := s.DistrictDrag()

		t.Run("it should mark the target refused", func(t *testing.T) {
			assert.False(t, drag.Droppable)
		})

		t.Run("it should drop nothing", func(t *testing.T) {
			assert.False(t, s.DropDistrict(plaza))
		})

		t.Run("it should leave the district where it was", func(t *testing.T) {
			assert.Equal(t, was, layout.Plan.Slots[d.Root])
		})
	})
}
