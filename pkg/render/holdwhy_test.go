package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 212: at fit the standing key broke on most frames and nothing
// said why. A debug build names, beside each frame's hold, the first
// input of the standing key that changed.
func TestWhyTheHoldBroke(t *testing.T) {
	settled := func(t *testing.T) *recordCity {
		rc := newRecordCity(t)
		rc.g.verifyBudget = 0
		rc.g.whyHold = true
		rc.scene.SetView(city.ViewAttention)
		rc.frame()
		rc.now = rc.now.Add(10 * time.Minute)
		for range 4 {
			rc.frame()
		}
		return rc
	}

	t.Run("when the standing key holds", func(t *testing.T) {
		rc := settled(t)
		rc.frame()

		t.Run("it should name nothing", func(t *testing.T) {
			assert.Empty(t, rc.g.holdBroke())
		})
	})

	for _, c := range []struct {
		name, input string
		change      func(rc *recordCity)
	}{
		{"night falls", "night", func(rc *recordCity) {
			for night := rc.scene.City().Night; rc.scene.City().Night == night; {
				rc.scene.CycleLight()
			}
		}},
		{"the labels are hidden", "labels", func(rc *recordCity) { rc.scene.SetLabels(false) }},
		{"a building is selected", "selected", func(rc *recordCity) { rc.scene.Select("s01") }},
		{"the camera pans by part of a pixel", "origin", func(rc *recordCity) { rc.scene.Pan(city.Point{X: 0.4}) }},
		{"a snapshot is taken again", "generation", func(rc *recordCity) { rc.snap(func(*state.Snapshot) {}) }},
	} {
		t.Run("when "+c.name, func(t *testing.T) {
			rc := settled(t)
			c.change(rc)
			rc.frame()

			t.Run("it should name "+c.input, func(t *testing.T) {
				assert.Equal(t, c.input, rc.g.holdBroke())
			})
			t.Run("it should have changed the key", func(t *testing.T) {
				assert.NotEqual(t, rc.g.still.lastStanding, rc.g.still.standing)
			})
		})
	}

	// The key folds the buildings in the city's order, so the first
	// building the search dims is the first of them it does not find.
	t.Run("when a search dims every building but s01", func(t *testing.T) {
		rc := settled(t)
		first := ""
		for _, b := range rc.scene.City().Buildings() {
			if b.Session.ID != "s01" {
				first = b.Session.ID
				break
			}
		}
		rc.scene.SetFilter("s01")
		rc.frame()

		t.Run("it should name the first of them dimmed", func(t *testing.T) {
			assert.Equal(t, first+" dimmed", rc.g.holdBroke())
		})
	})

	// A tool call's load stirs its session; the frame the load ends,
	// only the stirring sessions change.
	t.Run("when a session's load ends", func(t *testing.T) {
		rc := settled(t)
		rc.snap(func(s *state.Snapshot) {
			s.At = rc.now
			s.Sessions[0].ToolCalls++
		})
		for range 3 {
			rc.frame()
		}
		require.Positive(t, len(rc.g.stirring), "precondition: a session stirs")
		for range 300 {
			rc.now = rc.now.Add(100 * time.Millisecond)
			before := len(rc.g.stirring)
			rc.frame()
			if len(rc.g.stirring) != before {
				break
			}
		}

		t.Run("it should name stirring", func(t *testing.T) {
			assert.Equal(t, "stirring", rc.g.holdBroke())
		})
	})

	t.Run("when two inputs change at once", func(t *testing.T) {
		rc := settled(t)
		rc.scene.SetLabels(false)
		rc.scene.Select("s01")
		rc.frame()

		t.Run("it should name the first in the key's order", func(t *testing.T) {
			assert.Equal(t, "labels", rc.g.holdBroke())
		})
	})
}

// Item 212: what a frame held and ran, and why its hold broke, is one
// line of the debug frame log.
func TestTheHoldLine(t *testing.T) {
	var st stillStats
	st.held, st.ran, st.ranKept, st.agedHeld = 40, 3, 1, 12
	st.freshBy[kindHouse], st.freshBy[kindTree] = 2, 1

	t.Run("it should write held, ran, ran-kept, aged-held, fresh by kind and what broke", func(t *testing.T) {
		assert.Equal(t, "hold: held=40 ran=3 ran-kept=1 aged-held=12 fresh=house:2,tree:1 broke=s05 lights", holdLine(st, "s05 lights"))
	})
	t.Run("and nothing broke, it should say so", func(t *testing.T) {
		assert.Equal(t, "hold: held=0 ran=0 ran-kept=0 aged-held=0 fresh= broke=nothing", holdLine(stillStats{}, ""))
	})
}
