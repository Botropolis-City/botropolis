package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 212: a building with a part that moves every frame, a beacon
// pulsing for its user, is noted in parts: what stands still is kept
// like any quiet building's record, and only what moves is drawn again.
func TestTheTwoPartBuilding(t *testing.T) {
	active := func(g *Game) int {
		n := 0
		for _, e := range g.still.entries {
			if e.active {
				n++
			}
		}
		return n
	}
	// before is how many entries show activity in the settled city
	// before a building pulses.
	var before int
	// waiting is a settled city with one building pulsing for its user.
	waiting := func(t *testing.T) *recordCity {
		rc := newRecordCity(t)
		rc.g.verifyBudget = 0
		rc.scene.SetView(city.ViewAttention)
		rc.frame()
		rc.now = rc.now.Add(10 * time.Minute)
		for range 3 {
			rc.frame()
		}
		before = active(rc.g)
		rc.snap(func(s *state.Snapshot) {
			s.At = rc.now
			s.Sessions[0].State = state.NeedsYou
		})
		// Past the turn's end: the scaffold comes down and it settles.
		for range 10 {
			rc.now = rc.now.Add(time.Second)
			rc.frame()
		}
		return rc
	}

	t.Run("when a building's beacon pulses between frames", func(t *testing.T) {
		rc := waiting(t)
		rc.now = rc.now.Add(100 * time.Millisecond)
		st := rc.frame()
		moving := active(rc.g)
		held := drawn(rc.g)

		t.Run("it should keep the records of what stands still in it", func(t *testing.T) {
			assert.Zero(t, st.freshBy[kindHouse])
		})
		t.Run("it should draw what moves in it afresh, as showing activity", func(t *testing.T) {
			assert.Greater(t, moving, before)
		})
		t.Run("it should hold the frame made afresh", func(t *testing.T) {
			assert.Equal(t, afresh(rc), held)
		})
	})

	t.Run("when a building's beacon pulses by night, over its lit windows", func(t *testing.T) {
		rc := waiting(t)
		for night := rc.scene.City().Night; rc.scene.City().Night == night; {
			rc.scene.CycleLight()
		}
		for range 3 {
			rc.frame()
		}
		lit := 0
		// Asked with the lights on, as the frame asks while it lays them.
		rc.g.lightsOn = true
		for _, b := range rc.scene.City().Buildings() {
			if f, ok := rc.g.buildingFrame(rc.scene.Camera(), b, false, true, 0); ok && b.Pulse && f.present[stepLights] {
				lit++
			}
		}
		rc.g.lightsOn = false
		require.Positive(t, lit, "precondition: the pulsing building's windows are lit")
		rc.now = rc.now.Add(100 * time.Millisecond)
		st := rc.frame()
		moving := active(rc.g)
		held := drawn(rc.g)

		t.Run("it should keep the records of what stands before its lights", func(t *testing.T) {
			assert.Zero(t, st.freshBy[kindHouse])
		})
		t.Run("it should draw its lights and what moves over them afresh, as showing activity", func(t *testing.T) {
			assert.Greater(t, moving, before)
		})
		t.Run("it should hold the frame made afresh", func(t *testing.T) {
			assert.Equal(t, afresh(rc), held)
		})
	})

	t.Run("when what stands still in it changes, a course poured", func(t *testing.T) {
		rc := waiting(t)
		rc.snap(func(s *state.Snapshot) {
			s.At = rc.now
			s.Sessions[0].Compactions++
		})
		st := rc.frame()

		t.Run("it should record it afresh", func(t *testing.T) {
			assert.Positive(t, st.freshBy[kindHouse])
		})
	})
}
