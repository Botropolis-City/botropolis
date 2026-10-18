package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 108: pointing at a car keeps its path lit, driveway to driveway,
// and recedes every other road's ground.
func TestCarHoverGround(t *testing.T) {
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		sessions = append(sessions, state.Session{ID: name, CWD: "/p/" + name, State: state.Working, StartedAt: at})
	}
	scene := city.NewScene(city.NewLayout())
	scene.SetClock(func() time.Time { return at }, time.UTC)
	scene.Resize(1600, 1000)
	scene.SetSnapshot(state.Snapshot{At: at, Sessions: sessions, Roads: []state.Road{
		{From: "/p/a", To: "/p/b", Files: 1, Sessions: 1, Active: 1},
		{From: "/p/c", To: "/p/f", Files: 1, Sessions: 1, Active: 1},
	}})
	c := scene.City()
	require.Len(t, c.Streets, 2, "precondition: two roads")
	ours := c.Streets[0]
	path := map[city.Cell]bool{}
	for _, p := range ours.Path[1 : len(ours.Path)-1] {
		path[city.CellOf(p)] = true
	}
	g := &Game{scene: scene}

	t.Run("when the pointer is on a car", func(t *testing.T) {
		scene.SetHover(city.Hit{Car: &city.CarHit{Road: ours.Road}})
		g.lit = scene.Highlight()
		lit := g.litGround(c)

		t.Run("it should keep every cell of its path lit, driveway to driveway", func(t *testing.T) {
			var dark []city.Cell
			for cell := range path {
				if !lit[cell] {
					dark = append(dark, cell)
				}
			}
			assert.Empty(t, dark)
		})

		t.Run("it should recede every other laid cell", func(t *testing.T) {
			var kept []city.Cell
			for _, sc := range c.StreetCells {
				if lit[sc.Cell] && !path[sc.Cell] {
					kept = append(kept, sc.Cell)
				}
			}
			assert.Empty(t, kept)
		})
	})

	t.Run("when the pointer is on nothing", func(t *testing.T) {
		scene.SetHover(city.Hit{})
		g.lit = scene.Highlight()

		t.Run("it should recede no ground", func(t *testing.T) {
			assert.Nil(t, g.litGround(c))
		})
	})

	t.Run("when the hovered car changes", func(t *testing.T) {
		cam := city.NewCamera()
		one := g.keyFor(cam, city.Hit{Car: &city.CarHit{Road: c.Streets[0].Road}}, 100, 100)
		other := g.keyFor(cam, city.Hit{Car: &city.CarHit{Road: c.Streets[1].Road}}, 100, 100)
		again := g.keyFor(cam, city.Hit{Car: &city.CarHit{Road: c.Streets[0].Road}}, 100, 100)

		t.Run("it should compose the city under the traffic again", func(t *testing.T) {
			assert.NotEqual(t, one, other)
		})

		t.Run("and it is the same car, it should not", func(t *testing.T) {
			assert.Equal(t, one, again)
		})
	})
}

// Item 108: a script can point at a car, so a frame can show one hovered.
func TestCarScriptTarget(t *testing.T) {
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return at }, time.UTC)
	s.SetProjection(city.Isometric)
	s.SetInstant(true)
	s.Resize(1200, 800)
	s.SetSnapshot(state.Snapshot{At: at, Sessions: []state.Session{
		{ID: "a", CWD: "/p/kiln", State: state.Working, StartedAt: at},
		{ID: "b", CWD: "/p/tidepool", State: state.Working, StartedAt: at},
	}, Roads: []state.Road{{From: "/p/kiln", To: "/p/tidepool", Files: 1, Sessions: 1, Active: 1}}})
	g := &Game{scene: s, scripted: -1, clicked: -1}
	g.motion.ambient = 3.7
	cars := g.cars(s.City(), 3.7)
	require.Len(t, cars, 1, "precondition: one car")

	t.Run("when a script points at the car between two districts", func(t *testing.T) {
		got, ok := g.resolveTarget("car:kiln->tidepool")

		t.Run("it should aim at the car where it is now", func(t *testing.T) {
			assert.Equal(t, []any{true, s.Camera().WorldToScreen(cars[0].at)}, []any{ok, got})
		})
	})

	t.Run("when no car runs between them", func(t *testing.T) {
		_, ok := g.resolveTarget("car:tidepool->nowhere")

		t.Run("it should aim at nothing", func(t *testing.T) {
			assert.False(t, ok)
		})
	})
}
