package render

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 212: the groups' lines cost 1.42 s of a 60 s fit profile at r1164,
// drawn afresh every frame. Each line's strokes are kept while what they
// are drawn from holds; its stations, office, hits and label are drawn
// afresh, in their places, as before.
func TestKeepingTheGroupLines(t *testing.T) {
	settled := func(t *testing.T) *recordCity {
		rc := newRecordCity(t)
		rc.g.verifyBudget = 0
		rc.scene.SetView(city.ViewAttention)
		rc.frame()
		rc.now = rc.now.Add(10 * time.Minute)
		for range 3 {
			rc.frame()
		}
		require.NotEmpty(t, rc.scene.Lines(), "precondition: the city has a group's line")
		return rc
	}
	// overlay is the overlay's commands, by where they fall and what
	// they draw.
	overlay := func(g *Game) []string {
		o := g.still.overlayRange
		// Its hash and box are what decides whether the overlay is drawn
		// again.
		out := []string{fmt.Sprintf("overlay %v %x", o.box, o.hash)}
		for _, c := range g.rec.cmds[o.from:o.to] {
			out = append(out, fmt.Sprintf("%v %x", c.box, c.hash))
		}
		return out
	}
	// afreshOverlay is the overlay of the same moment drawn with nothing
	// kept.
	afreshOverlay := func(rc *recordCity) []string {
		rc.g.noReuse = true
		defer func() { rc.g.noReuse = false }()
		rc.g.ticks--
		rc.frame()
		return overlay(rc.g)
	}

	t.Run("when a settled city with a group's line is drawn again", func(t *testing.T) {
		rc := settled(t)
		st := rc.frame()
		kept := overlay(rc.g)

		t.Run("it should keep the lines' strokes", func(t *testing.T) {
			assert.Positive(t, st.linesKept)
		})
		t.Run("it should draw no line's strokes again", func(t *testing.T) {
			assert.Zero(t, st.linesDrawn)
		})
		t.Run("it should draw the overlay as a frame made afresh does", func(t *testing.T) {
			assert.Equal(t, afreshOverlay(rc), kept)
		})
	})

	t.Run("when the snapshot is taken again, changing nothing a line is drawn from", func(t *testing.T) {
		rc := settled(t)
		rc.snap(func(*state.Snapshot) {})
		st := rc.frame()
		kept := overlay(rc.g)

		t.Run("it should keep the lines' strokes", func(t *testing.T) {
			assert.Zero(t, st.linesDrawn)
		})
		t.Run("it should draw the overlay as a frame made afresh does", func(t *testing.T) {
			assert.Equal(t, afreshOverlay(rc), kept)
		})
	})

	for _, c := range []struct {
		name   string
		change func(rc *recordCity)
	}{
		{"the camera zooms", func(rc *recordCity) { rc.scene.Camera().Zoom *= 1.25 }},
		{"the camera turns", func(rc *recordCity) { rc.scene.Turn() }},
		{"the camera pans by part of a pixel", func(rc *recordCity) { rc.scene.Pan(city.Point{X: 0.4}) }},
		{"the camera pans by whole pixels", func(rc *recordCity) { rc.scene.Pan(city.Point{X: 9, Y: -7}) }},
		{"a group is focused", func(rc *recordCity) { rc.scene.FocusGroup(rc.scene.Lines()[0].Group.ID) }},
	} {
		t.Run("when "+c.name, func(t *testing.T) {
			rc := settled(t)
			c.change(rc)
			st := rc.frame()
			t.Run("it should draw the lines' strokes afresh", func(t *testing.T) {
				assert.Positive(t, st.linesDrawn)
			})
			// Settled again, as the labels in the overlay fade in where
			// they now go, it keeps them again, and keeps what changed.
			for range 3 {
				rc.frame()
			}
			again := rc.frame()
			kept := overlay(rc.g)

			t.Run("and it has settled again", func(t *testing.T) {
				t.Run("it should keep them", func(t *testing.T) {
					assert.Positive(t, again.linesKept)
				})
				t.Run("it should draw the overlay as a frame made afresh does", func(t *testing.T) {
					assert.Equal(t, afreshOverlay(rc), kept)
				})
			})
		})
	}

	t.Run("when the net checks the lines", func(t *testing.T) {
		rc := settled(t)
		rc.g.verifyEvery = 1
		st := rc.frame()

		t.Run("it should find them as drawn afresh", func(t *testing.T) {
			assert.Zero(t, st.stale)
		})
	})

	t.Run("when the lines' key is made to miss the focus, and another group is focused", func(t *testing.T) {
		rc := settled(t)
		// A second group, so focusing it fades the first's line.
		rc.snap(func(s *state.Snapshot) {
			s.Groups = append(s.Groups, state.Group{ID: "g2", Name: "the docs", Short: "DOC"})
			s.Sessions[1].Group, s.Sessions[4].Group = "g2", "g2"
		})
		// Faulty from the start, so the last frame keyed its lines so.
		rc.g.faultyKeys = true
		for range 3 {
			rc.frame()
		}
		rc.g.verifyEvery = 1
		// Focusing it frames it; the camera is put back, so only the
		// first line's colours change.
		cam := *rc.scene.Camera()
		require.True(t, rc.scene.FocusGroup("g2"), "precondition: the second group can be focused")
		*rc.scene.Camera() = cam
		st := rc.frame()

		t.Run("it should find a line stale", func(t *testing.T) {
			assert.Positive(t, st.stale)
		})
	})
}
