package render

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// groundCity is a city with two roads and two servers' beams, the
// snapshot it was built from, and what composes its ground.
type groundCity struct {
	scene    *city.Scene
	c        *city.City
	snapshot state.Snapshot
}

func newGroundCity(t testing.TB) *groundCity {
	t.Helper()
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for _, name := range []string{"a", "b", "c", "d"} {
		sessions = append(sessions, state.Session{ID: name, CWD: "/p/" + name, State: state.Working, StartedAt: at,
			LastActivity: at.Add(-time.Minute), MCPCalls: map[string]int{"files": 1, "search": 2}})
	}
	// a2 shares a's district, so pointing from one to the other moves
	// no district under the pointer.
	sessions = append(sessions, state.Session{ID: "a2", CWD: "/p/a", State: state.Working, StartedAt: at, LastActivity: at.Add(-time.Minute)})
	scene := city.NewScene(city.NewLayout())
	scene.SetClock(func() time.Time { return at }, time.UTC)
	scene.Resize(1600, 1000)
	gc := &groundCity{scene: scene}
	gc.snap(t, func(s *state.Snapshot) {
		*s = state.Snapshot{At: at, Sessions: sessions,
			Servers: []state.Server{{Name: "files", Configured: true, Calls: 12, Sessions: 4}, {Name: "search", Configured: true, Calls: 20, Sessions: 4}},
			Roads: []state.Road{
				{From: "/p/a", To: "/p/b", Files: 1, Sessions: 1, Active: 1},
				{From: "/p/c", To: "/p/d", Files: 1, Sessions: 1, Active: 1},
			}}
	})
	require.Len(t, gc.c.Roads, 2, "precondition: two roads")
	require.GreaterOrEqual(t, len(gc.c.Beams()), 2, "precondition: two beams")
	return gc
}

// snap hands the scene the snapshot change makes of the last one, as
// the daemon does, and rebuilds the city from it.
func (gc *groundCity) snap(t testing.TB, change func(s *state.Snapshot)) {
	t.Helper()
	s := gc.snapshot
	s.Sessions = slices.Clone(s.Sessions)
	for i := range s.Sessions {
		s.Sessions[i].MCPCalls = maps.Clone(s.Sessions[i].MCPCalls)
	}
	s.Roads, s.Servers, s.Teams = slices.Clone(s.Roads), slices.Clone(s.Servers), slices.Clone(s.Teams)
	change(&s)
	gc.snapshot = s
	gc.scene.SetSnapshot(s)
	gc.c = gc.scene.City()
}

// ground composes the ground for hover, as the frame does, and returns
// what its draws are and where.
func (gc *groundCity) ground(g *Game, hover city.Hit) map[uint64]stillMark {
	gc.scene.SetHover(hover)
	g.lit = gc.scene.Highlight()
	g.staticCity(gc.c, gc.scene.Camera(), hover, 1600, 1000)
	return maps.Clone(g.static.set.marks)
}

// fresh is the same ground composed again with nothing kept.
func (gc *groundCity) fresh(g *Game, hover city.Hit) map[uint64]stillMark {
	g.static.ok = false
	return gc.ground(g, hover)
}

// groundKits is the kits with every level up, loaded once: the ground
// draws its streets and their traffic's tint from them, and without
// them only plain road lines.
var groundKits = sync.OnceValues(func() (*kits, error) {
	k, err := loadKitsLazily()
	if err != nil {
		return nil, err
	}
	for i := range k.atlases {
		for !k.complete(i) {
			if err := k.uploadPage(i); err != nil {
				return nil, err
			}
		}
	}
	k.keepAll = true
	return k, nil
})

// game is a game drawing gc's city, from the kits or without them.
func (gc *groundCity) game(t testing.TB, kitted bool) *Game {
	t.Helper()
	g := &Game{scene: gc.scene, theme: ui.NewTheme(1)}
	if kitted {
		k, err := groundKits()
		require.NoError(t, err)
		g.kits = k
	}
	return g
}

// groundViews are the views that draw the ground's networks differently.
var groundViews = []city.View{city.ViewAttention, city.ViewServers, city.ViewTraffic}

// session is the snapshot's session id.
func session(s *state.Snapshot, id string) *state.Session {
	for i := range s.Sessions {
		if s.Sessions[i].ID == id {
			return &s.Sessions[i]
		}
	}
	return nil
}

// Item 207: the ground is keyed on what it draws from the city, item
// 206's GroundGeneration, not on every snapshot the city is built from.
// A snapshot that changes nothing it draws records nothing; one that
// changes what it draws, or a city whose ground noted a hover target
// that would point into the old city, composes it again.
func TestTheGroundOnASnapshot(t *testing.T) {
	t.Run("when a snapshot changes only activity times", func(t *testing.T) {
		gc := newGroundCity(t)
		gc.scene.SetView(city.ViewTraffic)
		g := gc.game(t, true)
		gc.ground(g, city.Hit{})
		before := g.static.records
		gc.snap(t, func(s *state.Snapshot) {
			s.At = s.At.Add(time.Second)
			session(s, "a").LastActivity = s.At
		})
		gc.ground(g, city.Hit{})

		t.Run("it should record the ground on no frame", func(t *testing.T) {
			assert.Equal(t, before, g.static.records)
		})
	})

	for _, c := range []struct {
		name   string
		change func(s *state.Snapshot)
		turn   bool
	}{
		{"when a session calls a server again", func(s *state.Snapshot) { session(s, "b").MCPCalls["files"]++ }, false},
		{"when a road carries another file", func(s *state.Snapshot) { s.Roads[0].Files++ }, false},
		{"when a new project starts", func(s *state.Snapshot) {
			s.Sessions = append(s.Sessions, state.Session{ID: "e", CWD: "/p/e", State: state.Working, StartedAt: s.At, LastActivity: s.At})
		}, false},
		{"when two sessions form a team", func(s *state.Snapshot) {
			session(s, "b").Team, session(s, "c").Team = "crew", "crew"
			s.Teams = append(s.Teams, claude.Team{Name: "crew", LeadSessionID: "b"})
		}, false},
		{"when the camera turns", func(*state.Snapshot) {}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			gc := newGroundCity(t)
			games := make([]*Game, len(groundViews))
			for i, v := range groundViews {
				gc.scene.SetView(v)
				games[i] = gc.game(t, true)
				gc.ground(games[i], city.Hit{})
			}
			gc.snap(t, c.change)
			if c.turn {
				gc.scene.Turn()
				gc.c = gc.scene.City()
			}
			for i, v := range groundViews {
				gc.scene.SetView(v)
				kept := gc.ground(games[i], city.Hit{})

				t.Run("and it is seen in the "+v.Name()+" view", func(t *testing.T) {
					t.Run("it should keep the ground composed afresh", func(t *testing.T) {
						assert.Equal(t, gc.fresh(games[i], city.Hit{}), kept)
					})
				})
			}
		})
	}

	t.Run("when the ground noted a hover target, and a snapshot rebuilds the city", func(t *testing.T) {
		gc := newGroundCity(t)
		g := &Game{scene: gc.scene, theme: ui.NewTheme(1)}
		gc.ground(g, city.Hit{})
		g.static.hits = append(g.static.hits, spriteHit{hit: city.Hit{District: gc.c.Districts[0]}})
		before := g.static.records
		gc.snap(t, func(s *state.Snapshot) {
			s.At = s.At.Add(time.Second)
			session(s, "a").LastActivity = s.At
		})
		gc.ground(g, city.Hit{})

		t.Run("it should record it again, for the new city's targets", func(t *testing.T) {
			assert.Equal(t, before+1, g.static.records)
		})
	})
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
			// Without kits the roads are lines; with them, streets.
			for _, drawn := range []struct {
				name   string
				kitted bool
			}{{"as plain lines", false}, {"from the kits", true}} {
				t.Run("and the city is drawn "+drawn.name, func(t *testing.T) {
					g := gc.game(t, drawn.kitted)
					gc.ground(g, move.from)
					kept := gc.ground(g, move.to)

					t.Run("it should keep the ground composed afresh for where it is", func(t *testing.T) {
						assert.Equal(t, gc.fresh(g, move.to), kept)
					})
				})
			}
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
		// hits is everything the pointer can be on in the city as built
		// now: a snapshot rebuilds it, and its targets with it.
		hits := func() []city.Hit {
			out := []city.Hit{{}}
			for i := range gc.c.Roads {
				out = append(out, city.Hit{Road: &gc.c.Roads[i]})
			}
			beams := gc.c.Beams()
			for i := range beams {
				out = append(out, city.Hit{Beam: &beams[i]})
			}
			for _, b := range gc.c.Buildings() {
				out = append(out, city.Hit{Building: b})
			}
			for _, d := range gc.c.Districts {
				out = append(out, city.Hit{District: d})
			}
			return out
		}
		// changes are the small things a snapshot moves, what the ground
		// draws and what it does not, each always changing something:
		// a change that changes nothing exercises nothing.
		changes := []func(s *state.Snapshot){
			func(s *state.Snapshot) {
				s.At = s.At.Add(time.Second)
				s.Sessions[rnd.IntN(len(s.Sessions))].LastActivity = s.At
			},
			func(s *state.Snapshot) {
				// Calls move to another count below where a beam's width
				// stops growing; a count of nought takes the beam away.
				sess := &s.Sessions[rnd.IntN(len(s.Sessions))]
				if sess.MCPCalls == nil {
					sess.MCPCalls = map[string]int{}
				}
				server := []string{"files", "search"}[rnd.IntN(2)]
				if n := (sess.MCPCalls[server] + 1 + rnd.IntN(3)) % 4; n == 0 {
					delete(sess.MCPCalls, server)
				} else {
					sess.MCPCalls[server] = n
				}
			},
			func(s *state.Snapshot) { s.Roads[rnd.IntN(len(s.Roads))].Files += []int{-1, 1, 2}[rnd.IntN(3)] },
			func(s *state.Snapshot) { s.Roads[rnd.IntN(len(s.Roads))].Messages++ },
			func(s *state.Snapshot) {
				sess := &s.Sessions[rnd.IntN(len(s.Sessions))]
				states := []state.State{state.Working, state.Resting, state.NeedsYou}
				states = slices.DeleteFunc(states, func(st state.State) bool { return st == sess.State })
				sess.State = states[rnd.IntN(len(states))]
			},
			func(s *state.Snapshot) {
				name := fmt.Sprintf("crew%d", len(s.Teams))
				lead, member := &s.Sessions[rnd.IntN(len(s.Sessions))], &s.Sessions[rnd.IntN(len(s.Sessions))]
				lead.Team, member.Team = name, name
				s.Teams = append(s.Teams, claude.Team{Name: name, LeadSessionID: lead.ID})
			},
			func(s *state.Snapshot) {
				if len(s.Sessions) > 5 && rnd.IntN(2) == 0 {
					s.Sessions = s.Sessions[:len(s.Sessions)-1]
					return
				}
				id := fmt.Sprintf("n%d", rnd.IntN(1000))
				s.Sessions = append(s.Sessions, state.Session{ID: id, CWD: "/p/" + []string{"a", "b", "new"}[rnd.IntN(3)], State: state.Working, StartedAt: s.At, LastActivity: s.At})
			},
		}
		ids := []string{"", "a", "a2", "c"}
		t.Cleanup(func() { gc.scene.SetView(city.ViewAttention); gc.scene.Select("") })
		// Each view keeps a ground of its own, as each draws different
		// networks, and every step asks each of them.
		games := make([]*Game, len(groundViews))
		for i := range games {
			games[i] = gc.game(t, true)
		}
		// Every kind of change comes once, in a shuffled order, before
		// the random steps, so each walk exercises all of them.
		first := rnd.Perm(len(changes))
		// each composes every view's ground with the pointer on hover.
		each := func(hover city.Hit) {
			for i, v := range groundViews {
				gc.scene.SetView(v)
				gc.ground(games[i], hover)
			}
		}
		stale := 0
		for step := range 150 {
			// The pointer's targets are pointers into the city, so one on
			// anything moves the key with every rebuild and hides a
			// snapshot's change. A snapshot is taken with it on bare
			// ground before and after, where only the snapshot can move
			// the ground.
			hover := city.Hit{}
			switch {
			case step < len(first):
				each(hover)
				gc.snap(t, changes[first[step]])
			case rnd.IntN(3) == 0:
				if id := ids[rnd.IntN(len(ids))]; gc.scene.Select(id) && rnd.IntN(2) == 0 {
					gc.scene.Act(city.ActionShowRoad)
				}
				all := hits()
				hover = all[rnd.IntN(len(all))]
			default:
				each(hover)
				gc.snap(t, changes[rnd.IntN(len(changes))])
			}
			for i, v := range groundViews {
				gc.scene.SetView(v)
				kept := gc.ground(games[i], hover)
				if !maps.Equal(gc.fresh(games[i], hover), kept) {
					stale++
					t.Logf("step %d: the kept ground is stale in view %v", step, v)
				}
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
