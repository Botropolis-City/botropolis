package render

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 212: a scaffold comes down on the scene's own tick, which the
// record city does not run; the Drive harness does. While only it moves,
// the frame it finishes on is drawn afresh, and drawn as a frame made
// with nothing kept would be.
func TestAScaffoldComingDownOnTheTick(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.Local)
	d, err := NewDrive(DriveOptions{Home: helpers.ParkedWeek(t, 24, now, "D"), Now: now, Width: 1200, Height: 800})
	require.NoError(t, err)
	g := d.game
	offer := func(change func(s *state.Snapshot)) {
		s := g.scene.Snapshot()
		s.Sessions = slices.Clone(s.Sessions)
		s.At = g.now()
		change(&s)
		g.Offer(s)
	}
	require.NoError(t, d.Run(5))
	id := g.scene.Snapshot().Sessions[0].ID
	offer(func(s *state.Snapshot) { s.Sessions[0].State, s.Sessions[0].LastActivity = state.Working, g.now() })
	require.NoError(t, d.Run(int(10*time.Second*liveTPS/time.Second)))
	building := func() *city.Building {
		for _, b := range g.scene.City().Buildings() {
			if b.Session.ID == id {
				return b
			}
		}
		return nil
	}
	require.Equal(t, 1.0, g.scene.Scaffold(building()), "precondition: the working building is scaffolded")

	// The turn ends: the session waits on its user, and its scaffold
	// comes down over ScaffoldDownFor.
	offer(func(s *state.Snapshot) { s.Sessions[0].State = state.NeedsYou })
	var finished, after stillStats
	var held, fresh []string
	ok := false
	for range int(2*city.ScaffoldDownFor*liveTPS/time.Second) + 10 {
		was := g.scene.Scaffold(building())
		require.NoError(t, d.Tick())
		if was > 0 && g.scene.Scaffold(building()) == 0 {
			finished, ok = g.still.stats, true
			// The frame it finishes on is drawn while the scene lets go of
			// the strike, part way through, so it draws the building
			// otherwise than the frames after (item 294); the frame after
			// is the first drawn from what the scaffold left, and the one
			// after that the first whose stirring holds too.
			require.NoError(t, d.Run(2))
			require.Positive(t, g.scene.PlantLights(), "precondition: the plant's lights still fade")
			after = g.still.stats
			held = drawn(g)
			g.noReuse = true
			g.ticks--
			g.Draw(d.screen)
			g.noReuse = false
			fresh = drawn(g)
			break
		}
	}
	require.True(t, ok, "precondition: the scaffold came down")
	// What else the session's stopping set going settles in its time.
	again := 0
	for range 20 * liveTPS {
		require.NoError(t, d.Tick())
		if again = g.still.stats.held; again > 0 {
			break
		}
	}

	t.Run("when the scaffold has come down", func(t *testing.T) {
		t.Run("it should not hold the still city the frame it finishes", func(t *testing.T) {
			assert.Zero(t, finished.held)
		})
		t.Run("it should not hold it two frames after, while the plant's lights still fade", func(t *testing.T) {
			assert.Zero(t, after.held)
		})
		t.Run("it should draw that frame as one made with nothing kept", func(t *testing.T) {
			assert.Equal(t, fresh, held)
		})
		t.Run("it should hold it again once it has settled", func(t *testing.T) {
			assert.Positive(t, again)
		})
	})
}
