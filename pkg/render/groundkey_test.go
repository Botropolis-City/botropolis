package render

import (
	"maps"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// groundCity is a city with two roads and two servers' beams, and
// what composes its ground.
type groundCity struct {
	scene *city.Scene
	c     *city.City
}

func newGroundCity(t testing.TB) groundCity {
	t.Helper()
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for _, name := range []string{"a", "b", "c", "d"} {
		sessions = append(sessions, state.Session{ID: name, CWD: "/p/" + name, State: state.Working, StartedAt: at,
			MCPCalls: map[string]int{"files": 3, "search": 5}})
	}
	// a2 shares a's district, so pointing from one to the other moves
	// no district under the pointer.
	sessions = append(sessions, state.Session{ID: "a2", CWD: "/p/a", State: state.Working, StartedAt: at})
	scene := city.NewScene(city.NewLayout())
	scene.SetClock(func() time.Time { return at }, time.UTC)
	scene.Resize(1600, 1000)
	scene.SetSnapshot(state.Snapshot{At: at, Sessions: sessions,
		Servers: []state.Server{{Name: "files", Configured: true, Calls: 12, Sessions: 4}, {Name: "search", Configured: true, Calls: 20, Sessions: 4}},
		Roads: []state.Road{
			{From: "/p/a", To: "/p/b", Files: 1, Sessions: 1, Active: 1},
			{From: "/p/c", To: "/p/d", Files: 1, Sessions: 1, Active: 1},
		}})
	c := scene.City()
	require.Len(t, c.Roads, 2, "precondition: two roads")
	require.GreaterOrEqual(t, len(c.Beams()), 2, "precondition: two beams")
	return groundCity{scene: scene, c: c}
}

// ground composes the ground for hover, as the frame does, and returns
// what its draws are and where.
func (gc groundCity) ground(g *Game, hover city.Hit) map[uint64]stillMark {
	gc.scene.SetHover(hover)
	g.lit = gc.scene.Highlight()
	g.staticCity(gc.c, gc.scene.Camera(), hover, 1600, 1000)
	return maps.Clone(g.static.set.marks)
}

// fresh is the same ground composed again with nothing kept.
func (gc groundCity) fresh(g *Game, hover city.Hit) map[uint64]stillMark {
	g.static.ok = false
	return gc.ground(g, hover)
}

// Item 207: the ground under the traffic is kept while its key holds,
// so whatever composeStatic reads must be in the key, or a change to it
// leaves the old picture up. A hovered road and a hovered beam were
// read and not keyed. The cached ground after each move must be the
// ground composed again for the same moment.
func TestTheGroundKey(t *testing.T) {
	gc := newGroundCity(t)
	road := func(i int) city.Hit { return city.Hit{Road: &gc.c.Roads[i]} }
	beam := func(i int) city.Hit { return city.Hit{Beam: &gc.c.Beams()[i]} }

	for _, move := range []struct {
		name     string
		view     city.View
		from, to city.Hit
	}{
		{"when the pointer stays on a road", city.ViewAttention, road(0), road(0)},
		{"when the pointer leaves a road for bare ground", city.ViewAttention, road(0), city.Hit{}},
		{"when the pointer moves from one road to another", city.ViewAttention, road(0), road(1)},
		{"when the pointer stays on a beam", city.ViewServers, beam(0), beam(0)},
		{"when the pointer leaves a beam for bare ground", city.ViewServers, beam(0), city.Hit{}},
		{"when the pointer moves from one beam to another", city.ViewServers, beam(0), beam(1)},
	} {
		t.Run(move.name, func(t *testing.T) {
			gc.scene.SetView(move.view)
			t.Cleanup(func() { gc.scene.SetView(city.ViewAttention) })
			g := &Game{scene: gc.scene, theme: ui.NewTheme(1)}
			gc.ground(g, move.from)
			kept := gc.ground(g, move.to)

			t.Run("it should keep the ground composed afresh for where it is", func(t *testing.T) {
				assert.Equal(t, gc.fresh(g, move.to), kept)
			})
		})
	}

	// A selected building with its roads shown lights them while the
	// pointer is on it, and not once it points at its neighbour.
	t.Run("when a building's roads are shown and the pointer moves to its neighbour", func(t *testing.T) {
		building := func(id string) *city.Building {
			for _, b := range gc.c.Buildings() {
				if b.Session.ID == id {
					return b
				}
			}
			t.Fatalf("no building %s", id)
			return nil
		}
		require.True(t, gc.scene.Select("a"))
		gc.scene.Act(city.ActionShowRoad)
		t.Cleanup(func() { gc.scene.Select("") })
		g := &Game{scene: gc.scene, theme: ui.NewTheme(1)}
		gc.ground(g, city.Hit{Building: building("a")})
		require.True(t, g.lit.HasRoad(&gc.c.Roads[0]), "precondition: the selected building's road is lit")
		kept := gc.ground(g, city.Hit{Building: building("a2")})

		t.Run("it should keep the ground composed afresh for where it is", func(t *testing.T) {
			assert.Equal(t, gc.fresh(g, city.Hit{Building: building("a2")}), kept)
		})
	})

	// Whatever composeStatic comes to read, a walk of the pointer, the
	// view and the selection through the city should never leave the
	// ground kept where the ground composed fresh differs. The seed is
	// logged, so a failure can be walked again.
	t.Run("when the pointer, the view and the selection wander at random", func(t *testing.T) {
		seed := time.Now().UnixNano()
		t.Logf("seed %d", seed)
		rnd := rand.New(rand.NewPCG(uint64(seed), 207))
		views := []city.View{city.ViewAttention, city.ViewServers, city.ViewTraffic}
		var hits []city.Hit
		hits = append(hits, city.Hit{})
		for i := range gc.c.Roads {
			hits = append(hits, city.Hit{Road: &gc.c.Roads[i]})
		}
		for i := range gc.c.Beams() {
			hits = append(hits, city.Hit{Beam: &gc.c.Beams()[i]})
		}
		for _, b := range gc.c.Buildings() {
			hits = append(hits, city.Hit{Building: b})
		}
		for _, d := range gc.c.Districts {
			hits = append(hits, city.Hit{District: d})
		}
		ids := []string{"", "a", "a2", "c"}
		t.Cleanup(func() { gc.scene.SetView(city.ViewAttention); gc.scene.Select("") })
		g := &Game{scene: gc.scene, theme: ui.NewTheme(1)}
		stale := 0
		for step := range 200 {
			switch rnd.IntN(4) {
			case 0:
				gc.scene.SetView(views[rnd.IntN(len(views))])
			case 1:
				if id := ids[rnd.IntN(len(ids))]; gc.scene.Select(id) && rnd.IntN(2) == 0 {
					gc.scene.Act(city.ActionShowRoad)
				}
			}
			hover := hits[rnd.IntN(len(hits))]
			kept := gc.ground(g, hover)
			if !maps.Equal(gc.fresh(g, hover), kept) {
				stale++
				t.Logf("step %d: the kept ground is stale", step)
			}
		}

		t.Run("it should keep the ground composed afresh after every step", func(t *testing.T) {
			assert.Zero(t, stale)
		})
	})

	// The city is drawn from the nearest level that is all up while the
	// one its zoom wants goes up, and from that one once it is.
	t.Run("when the level the zoom wants finishes going up", func(t *testing.T) {
		k, err := loadKitsLazily()
		require.NoError(t, err)
		upload := func(level int, zoom float64) {
			for range 32 {
				if k.complete(level) {
					return
				}
				require.NoError(t, k.view(zoom))
			}
			t.Fatalf("level %d never went up", level)
		}
		cam := *gc.scene.Camera()
		cam.Zoom = 1
		require.Zero(t, k.index(cam.Zoom), "precondition: zoom 1 wants the first level")
		upload(1, 2)
		g := &Game{scene: gc.scene, theme: ui.NewTheme(1), kits: k}
		compose := func() map[uint64]stillMark {
			gc.scene.SetHover(city.Hit{})
			g.lit = gc.scene.Highlight()
			g.staticCity(gc.c, &cam, city.Hit{}, 1600, 1000)
			return maps.Clone(g.static.set.marks)
		}
		require.Same(t, &k.atlases[1], k.pick(cam.Zoom), "precondition: drawn from the other level while its own goes up")
		compose()
		upload(0, 1)
		kept := compose()
		g.static.ok = false

		t.Run("it should keep the ground composed afresh from the level it wants", func(t *testing.T) {
			assert.Equal(t, compose(), kept)
		})
	})
}
