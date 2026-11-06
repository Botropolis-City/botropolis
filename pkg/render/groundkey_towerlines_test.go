package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 347: the tower_lines setting changes which beams the ground
// paints, so it has to move the kept ground's key: the key names every
// input the drawing reads.
func TestTheGroundKeyFollowsTowerLines(t *testing.T) {
	gc := newGroundCity(t)
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	gc.snap(t, func(s *state.Snapshot) {
		s.Sessions = append(s.Sessions, state.Session{ID: "packed", CWD: "/p/b", State: state.Resting, StartedAt: at.Add(-2 * time.Hour),
			LastActivity: at.Add(-time.Hour), Packed: true, PackedAt: at.Add(-time.Hour), MCPCalls: map[string]int{"files": 4}})
	})
	gc.scene.SetView(city.ViewServers)
	t.Cleanup(func() { gc.scene.SetView(city.ViewAttention); gc.scene.SetDepotBeams(false) })
	before := len(gc.c.Beams())
	gc.scene.SetDepotBeams(true)
	require.Greater(t, len(gc.c.Beams()), before, "precondition: the depot's crate adds a beam")
	gc.scene.SetDepotBeams(false)

	t.Run("when the tower lines are switched to the depot too, with the pointer still", func(t *testing.T) {
		g := gc.game(t, false)
		gc.ground(g, city.Hit{})
		gc.scene.SetDepotBeams(true)
		kept := gc.ground(g, city.Hit{})

		t.Run("it should compose the ground afresh, with the crate's beam", func(t *testing.T) {
			assert.Equal(t, gc.fresh(g, city.Hit{}), kept)
		})
	})
}
