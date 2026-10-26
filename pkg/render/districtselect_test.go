package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/plan"
	"botropolis.city/botropolis/pkg/state"
)

// Item 283: the left button selects districts with Ctrl and moves them
// together; and, from item 282, a drag let go is not a click.

func selectionGame(t *testing.T) (*Game, *city.Layout) {
	t.Helper()
	layout := city.NewLayout()
	s := city.NewScene(layout)
	s.SetInstant(true)
	s.Resize(1600, 1200)
	var sessions []state.Session
	for _, name := range []string{"a", "b", "c", "d"} {
		sessions = append(sessions, state.Session{ID: name, CWD: "/p/" + name, State: state.Working})
	}
	s.SetSnapshot(state.Snapshot{Sessions: sessions})
	return &Game{scene: s, scripted: -1, clicked: -1}, layout
}

func districtNamed(t *testing.T, g *Game, root string) *city.District {
	t.Helper()
	for _, d := range g.scene.City().Districts {
		if d.Root == root {
			return d
		}
	}
	t.Fatalf("no district %s", root)
	return nil
}

// floorOf is a screen point on a district's own floor, clear of buildings.
func floorOf(t *testing.T, g *Game, root string) city.Point {
	d := districtNamed(t, g, root)
	p := city.Point{X: d.Rect.Min.X + 1, Y: d.Rect.Min.Y + 1}
	for _, b := range d.Buildings {
		if b.Rect.Contains(p) {
			p = city.Point{X: d.Rect.Max.X - 1, Y: d.Rect.Max.Y - 1}
		}
	}
	return g.scene.Camera().WorldToScreen(p)
}

// openGround is a screen point on the plaza, where no district stands.
func openGround(g *Game) city.Point {
	return g.scene.Camera().WorldToScreen(g.scene.City().Plaza.Center())
}

// gesture presses at the first point, holds through the rest and lets go
// at the last, a frame each, the pointer moving to each as a hand's would.
func gesture(g *Game, ctrl bool, points ...city.Point) {
	for i, p := range points {
		g.scene.PointerMove(p)
		g.leftButton(p, leftInput{press: i == 0, down: i < len(points)-1, release: i == len(points)-1, ctrl: ctrl})
	}
}

func TestLeftButton(t *testing.T) {
	t.Run("when the map is pressed and let go without moving", func(t *testing.T) {
		g, _ := selectionGame(t)
		b := districtNamed(t, g, "/p/a").Buildings[0]
		at := g.scene.Camera().WorldToScreen(b.Rect.Center())
		g.scene.PointerMove(at)
		require.Equal(t, b, g.scene.Hover().Building, "precondition: the pointer is on a building")
		gesture(g, false, at, at)

		t.Run("it should select what is under the pointer", func(t *testing.T) {
			assert.Equal(t, b, g.scene.Selected())
		})
	})

	t.Run("when the map is dragged and let go", func(t *testing.T) {
		g, _ := selectionGame(t)
		// Grabbed on a building, which the pan keeps under the pointer.
		b := districtNamed(t, g, "/p/a").Buildings[0]
		from := g.scene.Camera().WorldToScreen(b.Rect.Center())
		before := g.scene.Camera().Offset
		end := from.Add(city.Point{X: 200, Y: 120})
		mid := from.Add(city.Point{X: 100, Y: 60})
		gesture(g, false, from, mid, end, end)

		t.Run("it should pan the camera", func(t *testing.T) {
			assert.NotEqual(t, before, g.scene.Camera().Offset)
		})

		t.Run("it should select nothing", func(t *testing.T) {
			assert.Nil(t, g.scene.Selected())
		})
	})

	t.Run("when a plain click lands on open ground with districts selected", func(t *testing.T) {
		g, _ := selectionGame(t)
		require.True(t, g.scene.ToggleDistrict(floorOf(t, g, "/p/a")))
		gesture(g, false, openGround(g), openGround(g))

		t.Run("it should clear the district selection", func(t *testing.T) {
			assert.Empty(t, g.scene.SelectedDistricts())
		})
	})

	t.Run("when Ctrl is held and a district is clicked", func(t *testing.T) {
		g, _ := selectionGame(t)
		b := districtNamed(t, g, "/p/a").Buildings[0]
		at := g.scene.Camera().WorldToScreen(b.Rect.Center())
		gesture(g, true, at, at)

		t.Run("it should add the district to the selection", func(t *testing.T) {
			assert.Equal(t, []string{"/p/a"}, g.scene.SelectedDistricts())
		})

		t.Run("it should select no building", func(t *testing.T) {
			assert.Nil(t, g.scene.Selected())
		})
	})

	t.Run("when Ctrl is held and the press is dragged from open ground", func(t *testing.T) {
		g, _ := selectionGame(t)
		from, to := openGround(g), floorOf(t, g, "/p/a")
		want := g.scene.DistrictsInBox(from, to)
		require.NotEmpty(t, want, "precondition: the box touches a district")
		before := g.scene.Camera().Offset
		gesture(g, true, from, to, to)

		t.Run("it should select every district the box touches", func(t *testing.T) {
			assert.Equal(t, want, g.scene.SelectedDistricts())
		})

		t.Run("it should not pan", func(t *testing.T) {
			assert.Equal(t, before, g.scene.Camera().Offset)
		})
	})

	t.Run("when three districts are selected and one is dragged to a free slot and let go", func(t *testing.T) {
		g, layout := selectionGame(t)
		group := []string{"/p/b", "/p/c", "/p/d"}
		for _, root := range group {
			require.True(t, g.scene.ToggleDistrict(floorOf(t, g, root)))
		}
		before := map[string]plan.Slot{}
		for root, s := range layout.Plan.Slots {
			before[root] = s
		}
		by := plan.Slot{Col: -1, Row: -1}
		to := before["/p/b"]
		to = plan.Slot{Col: to.Col + by.Col, Row: to.Row + by.Row}
		drop := g.scene.Camera().WorldToScreen(g.scene.City().SlotRect(to).Center())
		start := floorOf(t, g, "/p/b")
		gesture(g, false, start, drop, drop)

		t.Run("it should move all three", func(t *testing.T) {
			got := map[string]plan.Slot{}
			want := map[string]plan.Slot{}
			for _, root := range group {
				got[root] = layout.Plan.Slots[root]
				want[root] = plan.Slot{Col: before[root].Col + by.Col, Row: before[root].Row + by.Row}
			}
			assert.Equal(t, want, got)
		})

		t.Run("it should select nothing on the drop target", func(t *testing.T) {
			assert.Nil(t, g.scene.Selected())
		})
	})
}

func TestEscapeClearsDistricts(t *testing.T) {
	g, _ := selectionGame(t)
	require.True(t, g.scene.ToggleDistrict(floorOf(t, g, "/p/a")))

	t.Run("when Esc is pressed with districts selected", func(t *testing.T) {
		pressKey(t, g, ebiten.KeyEscape, false)

		t.Run("it should clear the selection", func(t *testing.T) {
			assert.Empty(t, g.scene.SelectedDistricts())
		})

		t.Run("it should go back no further", func(t *testing.T) {
			assert.False(t, g.settingsOpen)
		})

		t.Run("and pressed again", func(t *testing.T) {
			pressKey(t, g, ebiten.KeyEscape, false)

			t.Run("it should fall through to the next rung", func(t *testing.T) {
				assert.True(t, g.settingsOpen)
			})
		})
	})
}

func TestScriptedCtrl(t *testing.T) {
	for _, tc := range []struct {
		hold string
		want bool
	}{
		{"Control", true},
		{"ControlLeft", true},
		{"Meta", true},
		{"ArrowLeft", false},
		{"", false},
	} {
		t.Run("when a script holds "+tc.hold, func(t *testing.T) {
			g := &Game{scripted: -1}
			g.play.hold = tc.hold

			t.Run("it should count as Ctrl or not", func(t *testing.T) {
				assert.Equal(t, tc.want, g.ctrlHeld())
			})
		})
	}
}

func TestDistrictGhosts(t *testing.T) {
	size := func(x, y float64) *city.District {
		return &city.District{Rect: city.RectAt(x, y, 100, 60)}
	}
	held, other := size(0, 0), size(200, 100)
	targets := []city.Rect{city.RectAt(10, 10, 100, 60), city.RectAt(210, 110, 100, 60)}
	drag := city.Drag{District: held, At: city.Point{X: 500, Y: 400}, Target: targets[0], Droppable: true,
		Members: []*city.District{held, other}, Targets: targets}

	t.Run("when a group is dragged", func(t *testing.T) {
		gs := districtGhosts(drag)

		t.Run("it should draw a ghost per member, shaped as the group is", func(t *testing.T) {
			require.Len(t, gs, 2)
			assert.Equal(t, other.Rect.Min.Sub(held.Rect.Min), gs[1].block.Min.Sub(gs[0].block.Min))
		})

		t.Run("it should outline a target per member", func(t *testing.T) {
			assert.Equal(t, targets, []city.Rect{gs[0].target, gs[1].target})
		})

		t.Run("and the drop is refused", func(t *testing.T) {
			refused := drag
			refused.Droppable = false
			gs := districtGhosts(refused)

			t.Run("it should mark every target refused", func(t *testing.T) {
				assert.Equal(t, []bool{true, true}, []bool{gs[0].refused, gs[1].refused})
			})
		})
	})
}
