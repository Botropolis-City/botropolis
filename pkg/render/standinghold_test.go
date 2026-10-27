package render

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 212: while the standing key holds, a frame notes every still thing
// as the last frame did without running its draw. Each case holds such a
// frame to the same moment drawn with nothing kept.
func TestHoldingTheStillCity(t *testing.T) {
	settled := func(t *testing.T) *recordCity {
		rc := newRecordCity(t)
		rc.g.verifyBudget = 0
		rc.scene.SetView(city.ViewAttention)
		rc.frame()
		// Past the flags' hoisting and the course's glow, which move.
		rc.now = rc.now.Add(10 * time.Minute)
		for range 4 {
			rc.frame()
		}
		return rc
	}

	t.Run("when a settled city is drawn again with nothing still changed", func(t *testing.T) {
		rc := settled(t)
		st := rc.frame()
		held := drawn(rc.g)

		t.Run("it should hold its still things", func(t *testing.T) {
			assert.Positive(t, st.held)
		})
		t.Run("it should run the draw of no still thing it keeps", func(t *testing.T) {
			assert.Zero(t, st.ranKept)
		})
		t.Run("it should hold the frame made afresh", func(t *testing.T) {
			assert.Equal(t, afresh(rc), held)
		})
	})

	for _, c := range []struct {
		name   string
		change func(rc *recordCity)
		holds  bool
	}{
		{"the snapshot is taken again", func(rc *recordCity) { rc.snap(func(*state.Snapshot) {}) }, false},
		{"the camera zooms", func(rc *recordCity) { rc.scene.Camera().Zoom *= 1.25 }, false},
		{"the camera turns", func(rc *recordCity) { rc.scene.Turn() }, false},
		{"the camera pans by part of a pixel", func(rc *recordCity) { rc.scene.Pan(city.Point{X: 0.4}) }, false},
		{"night falls", func(rc *recordCity) {
			for night := rc.scene.City().Night; rc.scene.City().Night == night; {
				rc.scene.CycleLight()
			}
		}, false},
		{"a search dims buildings", func(rc *recordCity) { rc.scene.SetFilter("s01") }, false},
		{"the labels are hidden", func(rc *recordCity) { rc.scene.SetLabels(false) }, false},
		{"a building is selected", func(rc *recordCity) { rc.scene.Select("s01") }, false},
		{"the camera pans by whole pixels", func(rc *recordCity) { rc.scene.Pan(city.Point{X: 9, Y: -7}) }, true},
		{"a course's glow ends", func(rc *recordCity) {
			rc.snap(func(s *state.Snapshot) {
				s.Sessions[3].Compactions++
				s.Sessions[3].LastCompactionAt = rc.now
			})
			for range 3 {
				rc.frame()
			}
			rc.now = rc.now.Add(courseGlow)
		}, false},
	} {
		t.Run("when "+c.name, func(t *testing.T) {
			rc := settled(t)
			c.change(rc)
			st := rc.frame()
			if c.holds {
				t.Run("it should hold its still things, moved", func(t *testing.T) {
					assert.Positive(t, st.held)
				})
				// A look the pan brings wholly on is baked that frame,
				// which changes what is counted and not what is drawn.
				t.Run("it should hold the frame made afresh", func(t *testing.T) {
					held := uncounted(drawn(rc.g))
					assert.Equal(t, uncounted(afresh(rc)), held)
				})
				return
			}
			t.Run("it should hold nothing that frame", func(t *testing.T) {
				assert.Zero(t, st.held)
			})
			// Settled again, it holds again, and what it holds is the
			// changed city.
			for range 3 {
				rc.frame()
			}
			again := rc.frame()
			held := drawn(rc.g)

			t.Run("and it has settled again", func(t *testing.T) {
				t.Run("it should hold its still things", func(t *testing.T) {
					assert.Positive(t, again.held)
				})
				t.Run("it should hold the frame made afresh", func(t *testing.T) {
					assert.Equal(t, afresh(rc), held)
				})
			})
		})
	}

	t.Run("when a course's glow is about to end", func(t *testing.T) {
		rc := settled(t)
		rc.snap(func(s *state.Snapshot) {
			s.Sessions[3].Compactions++
			s.Sessions[3].LastCompactionAt = rc.now
		})
		for range 3 {
			rc.frame()
		}
		rc.now = rc.now.Add(courseGlow - time.Second)

		t.Run("it should still hold the frame before", func(t *testing.T) {
			assert.Positive(t, rc.frame().held)
		})
	})

	t.Run("when the still things' order changes while the key holds", func(t *testing.T) {
		rc := settled(t)
		log := rc.g.still.log
		require.Greater(t, len(log), 2, "precondition: the frame logged its still things")
		i := len(log) / 2
		log[i], log[i+1] = log[i+1], log[i]
		require.NotEqual(t, log[i].at, log[i+1].at, "precondition: the two swapped are different things")
		st := rc.frame()
		held := drawn(rc.g)

		t.Run("it should count the order broken", func(t *testing.T) {
			assert.Equal(t, 1, st.orderBroke)
		})
		t.Run("it should hold nothing from where it broke", func(t *testing.T) {
			var after int
			for _, it := range rc.g.still.log[i:] {
				if it.held {
					after++
				}
			}
			assert.Zero(t, after)
		})
		t.Run("it should have held things before it", func(t *testing.T) {
			assert.Positive(t, st.held)
		})
		t.Run("it should hold the frame made afresh", func(t *testing.T) {
			assert.Equal(t, afresh(rc), held)
		})
	})

	t.Run("when the net checks eight a frame while the key holds", func(t *testing.T) {
		rc := settled(t)
		rc.g.verifyBudget = releaseVerifyBudget
		st := rc.frame()
		checked := map[uint64]bool{}
		rc.g.verified = func(key uint64) { checked[key] = true }
		for range len(rc.g.still.entries)/releaseVerifyBudget + 2 {
			rc.frame()
		}
		var unchecked int
		for _, it := range rc.g.still.log {
			if it.held && !checked[rc.g.still.entries[it.entry].key] {
				unchecked++
			}
		}

		t.Run("it should still check eight", func(t *testing.T) {
			assert.Equal(t, releaseVerifyBudget, st.checked)
		})
		t.Run("it should find none stale", func(t *testing.T) {
			assert.Zero(t, st.stale)
		})
		t.Run("and a round of frames passes", func(t *testing.T) {
			t.Run("it should have checked every record it holds", func(t *testing.T) {
				assert.Zero(t, unchecked)
			})
		})
	})
}

// drawn is everything a frame noted, and the records it made of its
// still things.
func drawn(g *Game) []string {
	out := frameNotes(g)
	for _, e := range g.still.entries {
		out = append(out, fmt.Sprintf("entry %v %x", e.box, e.hash))
	}
	return out
}

// uncounted is notes without the frame's counts.
func uncounted(notes []string) []string {
	return slices.DeleteFunc(notes, func(n string) bool { return strings.HasPrefix(n, "counts ") })
}

// afresh is the record city's moment drawn again with nothing kept, at
// the same tick, so what animates stands where it did.
func afresh(rc *recordCity) []string {
	rc.g.noReuse = true
	defer func() { rc.g.noReuse = false }()
	rc.g.ticks--
	rc.frame()
	return drawn(rc.g)
}

// Item 212: the frame test leaves out every entry that shows activity
// after the frame is noted, and the log names entries by their place.
func TestDroppingActiveEntries(t *testing.T) {
	s := stillFrame{
		entries: []entry{{key: 1}, {key: 2, active: true}, {key: 3}},
		log:     []heldItem{{entry: 0}, {entry: 1}, {entry: 2}, {entry: -1}},
	}
	s.entries = s.dropActive()
	var keys []uint64
	for _, it := range s.log {
		if it.entry < 0 {
			keys = append(keys, 0)
			continue
		}
		keys = append(keys, s.entries[it.entry].key)
	}

	t.Run("when the entries that show activity are left out", func(t *testing.T) {
		t.Run("it should point each logged item at its own entry, or at none", func(t *testing.T) {
			assert.Equal(t, []uint64{1, 0, 3, 0}, keys)
		})
	})
}
