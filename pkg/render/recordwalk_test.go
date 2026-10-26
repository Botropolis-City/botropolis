package render

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// recordCity is a city with aged lots beside working ones, the snapshot
// it was built from, its clock, and a game drawing it.
type recordCity struct {
	scene    *city.Scene
	snapshot state.Snapshot
	now      time.Time
	g        *Game
	screen   *ebiten.Image
}

const recordWidth, recordHeight = 1200, 800

func newRecordCity(t testing.TB) *recordCity {
	t.Helper()
	at := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	rc := &recordCity{now: at}
	var sessions []state.Session
	for i := range 24 {
		s := state.Session{ID: fmt.Sprintf("s%02d", i), CWD: fmt.Sprintf("/src/p%d", i%6), StartedAt: at.Add(-time.Hour)}
		switch i % 3 {
		case 0:
			s.State, s.LastActivity = state.Working, at.Add(-time.Minute)
		case 1:
			s.State, s.LastActivity = state.Resting, at.Add(-time.Duration(40+i)*24*time.Hour)
		default:
			s.State, s.LastActivity = state.Resting, at.Add(-time.Duration(3+i)*24*time.Hour)
		}
		if i == 3 {
			// Its top course glows until the clock moves on.
			s.Compactions, s.LastCompactionAt = 2, at.Add(-courseGlow+20*time.Second)
		}
		if i == 6 {
			s.PRs = []claude.PR{{Number: 1, URL: "https://example.com/pr/1", State: claude.PROpen}, {Number: 2, URL: "https://example.com/pr/2", State: claude.PRMerged}}
		}
		sessions = append(sessions, s)
	}
	rc.snapshot = state.Snapshot{At: at, Sessions: sessions}
	rc.scene = city.NewScene(city.NewLayout())
	rc.scene.SetClock(func() time.Time { return rc.now }, time.UTC)
	rc.scene.SetProjection(city.Isometric)
	rc.scene.Resize(recordWidth, recordHeight)
	rc.scene.SetSnapshot(rc.snapshot)
	th := ui.NewTheme(1)
	f, err := newFaces(th)
	require.NoError(t, err)
	k, err := shareKits()
	require.NoError(t, err)
	rc.g = NewGame(rc.scene, &recordingActor{}, th, f, nil)
	rc.g.kits = k
	rc.g.driven = true
	rc.g.scripted, rc.g.clicked = -1, -1
	rc.screen = ebiten.NewImage(recordWidth, recordHeight)
	t.Cleanup(rc.screen.Deallocate)
	return rc
}

// snap hands the scene the snapshot change makes of the last one.
func (rc *recordCity) snap(change func(s *state.Snapshot)) {
	s := rc.snapshot
	s.Sessions = slices.Clone(s.Sessions)
	for i := range s.Sessions {
		s.Sessions[i].MCPCalls = maps.Clone(s.Sessions[i].MCPCalls)
	}
	change(&s)
	rc.snapshot = s
	rc.scene.SetSnapshot(s)
}

// frame draws the world once, as a frame does, and returns what its
// still drawables cost.
func (rc *recordCity) frame() stillStats {
	g := rc.g
	g.ticks++
	g.lit = rc.scene.Highlight()
	g.drawIso(rc.screen, rc.scene.City(), rc.scene.Camera(), rc.scene.Hover(), rc.scene.Selected(),
		recordWidth, recordHeight, g.clock())
	return g.still.stats
}

// recordWalk is a seeded walk over what a frame is drawn from: every
// kind of change once in a shuffled order, then random ones, each
// drawn with every kept record checked against one made afresh. It
// returns how many kept records differed from theirs, and how many
// were kept.
func recordWalk(t *testing.T, seed uint64, steps int) (stale, reused int) {
	t.Helper()
	rc := newRecordCity(t)
	rc.g.verifyEvery = 1
	rnd := rand.New(rand.NewPCG(seed, 212))
	ids := func() []string {
		var out []string
		for _, s := range rc.snapshot.Sessions {
			out = append(out, s.ID)
		}
		return out
	}
	hits := func() []city.Hit {
		out := []city.Hit{{}}
		c := rc.scene.City()
		for _, b := range c.Buildings() {
			out = append(out, city.Hit{Building: b})
		}
		for _, d := range c.Districts {
			out = append(out, city.Hit{District: d})
		}
		return out
	}
	zooms := []float64{0.5, 0.75, 1, 1.5, 2}
	views := []city.View{city.ViewAttention, city.ViewServers, city.ViewTraffic}
	changes := []struct {
		name string
		do   func()
	}{
		{"a session's activity moves", func() {
			rc.snap(func(s *state.Snapshot) {
				s.At = rc.now
				s.Sessions[rnd.IntN(len(s.Sessions))].LastActivity = rc.now
			})
		}},
		{"a session's state changes", func() {
			rc.snap(func(s *state.Snapshot) {
				sess := &s.Sessions[rnd.IntN(len(s.Sessions))]
				states := slices.DeleteFunc([]state.State{state.Working, state.Resting, state.NeedsYou}, func(st state.State) bool { return st == sess.State })
				sess.State = states[rnd.IntN(len(states))]
			})
		}},
		{"a session comes or goes", func() {
			rc.snap(func(s *state.Snapshot) {
				if len(s.Sessions) > 20 && rnd.IntN(2) == 0 {
					i := rnd.IntN(len(s.Sessions))
					s.Sessions = slices.Delete(s.Sessions, i, i+1)
					return
				}
				s.Sessions = append(s.Sessions, state.Session{ID: fmt.Sprintf("n%03d", rnd.IntN(1000)), CWD: fmt.Sprintf("/src/p%d", rnd.IntN(7)),
					State: state.Resting, StartedAt: rc.now.Add(-time.Hour), LastActivity: rc.now.Add(-time.Duration(rnd.IntN(60)) * 24 * time.Hour)})
			})
		}},
		{"a pull request opens or merges", func() {
			rc.snap(func(s *state.Snapshot) {
				sess := &s.Sessions[rnd.IntN(len(s.Sessions))]
				sess.PRs = slices.Clone(sess.PRs)
				if len(sess.PRs) > 0 && rnd.IntN(2) == 0 {
					sess.PRs[0].State = claude.PRMerged
					return
				}
				n := 10 + len(sess.PRs)
				sess.PRs = append(sess.PRs, claude.PR{Number: n, URL: fmt.Sprintf("https://example.com/%s/pr/%d", sess.ID, n), State: claude.PROpen})
			})
		}},
		{"the pointer moves", func() {
			all := hits()
			rc.scene.SetHover(all[rnd.IntN(len(all))])
		}},
		{"the view changes", func() { rc.scene.SetView(views[rnd.IntN(len(views))]) }},
		{"a search narrows the map or clears", func() {
			all := append(ids(), "")
			rc.scene.SetFilter(all[rnd.IntN(len(all))])
		}},
		{"the selection changes", func() {
			all := append(ids(), "")
			rc.scene.Select(all[rnd.IntN(len(all))])
		}},
		{"the camera pans by whole pixels", func() {
			rc.scene.Pan(city.Point{X: float64(rnd.IntN(41) - 20), Y: float64(rnd.IntN(41) - 20)})
		}},
		{"the camera pans by part of a pixel", func() {
			rc.scene.Pan(city.Point{X: rnd.Float64()*4 - 2, Y: rnd.Float64()*4 - 2})
		}},
		{"the camera zooms", func() {
			cam := rc.scene.Camera()
			zs := slices.DeleteFunc(slices.Clone(zooms), func(z float64) bool { return z == cam.Zoom })
			cam.Zoom = zs[rnd.IntN(len(zs))]
		}},
		{"night falls or lifts", func() { rc.scene.CycleLight() }},
		{"the camera turns", func() { rc.scene.Turn() }},
		{"the look pages fill and start again", func() {
			// Pages a few looks wide, so looks find no room, are drawn
			// straight, and pages start again under kept records.
			if rc.g.lookPages != nil {
				rc.g.lookPages.release()
			}
			rc.g.lookPages = newLookPages(2, 96+rnd.IntN(4)*64)
			for _, k := range rc.g.looks.keys() {
				rc.g.looks.remove(k)
			}
		}},
		{"the clock moves a moment", func() {
			// Less than a scaffold takes to come down or a flag to go up.
			rc.now = rc.now.Add(time.Duration(50+rnd.IntN(650)) * time.Millisecond)
		}},
		{"the clock moves on", func() {
			rc.now = rc.now.Add(time.Duration(1+rnd.IntN(3600)) * time.Second)
		}},
	}
	// The city settles first, so there is something to keep.
	for range 3 {
		rc.frame()
	}
	first := rnd.Perm(len(changes))
	for step := range steps {
		k := rnd.IntN(len(changes))
		if step < len(first) {
			k = first[step]
		}
		// A snapshot is taken with the pointer on bare ground, so only
		// the snapshot can move a key (item 207).
		if k < 4 {
			rc.scene.SetHover(city.Hit{})
		}
		changes[k].do()
		// Twice: the frame after a change keeps what the change left.
		for range 2 {
			st := rc.frame()
			if st.stale > 0 {
				t.Logf("seed %d, step %d (%s): %d kept records differ from the ones made afresh", seed, step, changes[k].name, st.stale)
			}
			stale += st.stale
			reused += st.reused
		}
	}
	return stale, reused
}

// Item 212: a still drawable whose key holds keeps last frame's record
// rather than being drawn again, and a key that misses an input leaves
// the city stale. The walk checks every kept record against one made
// afresh in the same frame, after every step.
func TestTheRecordKeyWalk(t *testing.T) {
	t.Run("when snapshots, the pointer, the view, the selection, the camera, night and the clock wander at random", func(t *testing.T) {
		seed := uint64(time.Now().UnixNano())
		if s, err := strconv.ParseUint(os.Getenv("RECORD_WALK_SEED"), 10, 64); err == nil {
			seed = s
		}
		t.Logf("seed %d (RECORD_WALK_SEED replays it)", seed)
		stale, reused := recordWalk(t, seed, 60)

		t.Run("it should find no kept record that differs from the one made afresh, after every step", func(t *testing.T) {
			assert.Zero(t, stale)
		})
		t.Run("it should keep records on some step", func(t *testing.T) {
			assert.Positive(t, reused)
		})
	})

	t.Run("when a key is made to miss an input, the aged lots keyed without whether a view drains their colour", func(t *testing.T) {
		rc := newRecordCity(t)
		rc.g.verifyEvery = 1
		rc.g.faultyKeys = true
		rc.scene.SetView(city.ViewAttention)
		for range 3 {
			rc.frame()
		}
		require.False(t, rc.g.viewing(), "precondition: the attention view drains no colour")
		rc.scene.SetView(city.ViewServers)
		require.True(t, rc.g.viewing(), "precondition: the servers view drains colour")
		stale := rc.frame().stale

		t.Run("it should find a kept record that differs from the one made afresh", func(t *testing.T) {
			assert.Positive(t, stale)
		})
	})
}
