package render

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// painted is every draw of the frame's ground and standing city in the
// order it is painted: the ground layer's, then each still entry's.
func painted(g *Game) []string {
	var out []string
	for _, m := range g.groundLayer().marks {
		out = append(out, fmt.Sprintf("%v %x", m.box, m.hash))
	}
	for _, e := range g.still.entries {
		for _, c := range g.rec.cmds[e.from:e.to] {
			out = append(out, fmt.Sprintf("%v %x", c.box, c.hash))
		}
	}
	return out
}

// apart is the record city's moment drawn again with the aged lots
// noted after the ground, as they were before they joined it.
func apart(rc *recordCity) []string {
	rc.g.agedApart = true
	defer func() { rc.g.agedApart = false }()
	rc.g.ticks--
	rc.g.lookFrame--
	rc.frame()
	return painted(rc.g)
}

// agedNoted is how many aged lots' drawables the frame noted.
func agedNoted(st stillStats) int {
	return st.freshBy[kindLotFloor] + st.keptBy[kindLotFloor] + st.freshBy[kindDistrictFloor] + st.keptBy[kindDistrictFloor]
}

// Item 212: the aged lots are painted into the ground layer, which is
// composed once and blitted, rather than noted in every frame's record;
// while roads overgrow, whose pads are painted before them, and for a
// lot being mowed, which moves, they are noted as before.
func TestTheAgedLotsInTheGround(t *testing.T) {
	settled := func(t *testing.T) *recordCity {
		rc := newRecordCity(t)
		rc.g.verifyBudget = 0
		rc.scene.SetView(city.ViewAttention)
		rc.frame()
		rc.now = rc.now.Add(10 * time.Minute)
		for range 4 {
			rc.frame()
		}
		return rc
	}
	// still asserts the frame is painted as the aged lots drawn apart
	// paint it.
	still := func(t *testing.T, rc *recordCity) {
		t.Helper()
		got := painted(rc.g)
		t.Run("it should paint what the aged lots noted apart paint", func(t *testing.T) {
			assert.Equal(t, apart(rc), got)
		})
	}

	t.Run("when the city is still, and nothing overgrows or mows", func(t *testing.T) {
		rc := settled(t)
		require.Positive(t, agedNoted(func() stillStats { rc.g.agedApart = true; defer func() { rc.g.agedApart = false }(); return rc.frame() }()),
			"precondition: the city has aged lots")
		rc.frame()
		st := rc.frame()
		records := rc.g.static.records
		rc.frame()

		t.Run("it should note no aged lot in the frame's record", func(t *testing.T) {
			assert.Zero(t, agedNoted(st))
		})
		t.Run("and the next frame", func(t *testing.T) {
			t.Run("it should compose the ground layer no more", func(t *testing.T) {
				assert.Equal(t, records, rc.g.static.records)
			})
		})
		still(t, rc)
	})

	t.Run("when a road overgrows", func(t *testing.T) {
		rc := settled(t)
		rc.snap(func(s *state.Snapshot) {
			s.Roads = []state.Road{{From: "/src/p0", To: "/src/p1", Files: 1, Sessions: 1, Active: 1}}
		})
		for range 3 {
			rc.frame()
		}
		rc.snap(func(s *state.Snapshot) { s.Roads = nil })
		st := rc.frame()
		require.NotEmpty(t, rc.scene.Overgrowth(), "precondition: a road overgrows")

		t.Run("it should note the aged lots in the frame's record, as before", func(t *testing.T) {
			assert.Positive(t, agedNoted(st))
		})
		still(t, rc)

		t.Run("and the overgrowth ends", func(t *testing.T) {
			rc.now = rc.now.Add(city.OvergrowFor)
			rc.frame()
			st := rc.frame()

			t.Run("it should note no aged lot in the frame's record again", func(t *testing.T) {
				assert.Zero(t, agedNoted(st))
			})
		})
	})

	// A view drains the ground layer as it is put down, and the aged lots
	// were painted over it in their own colours.
	t.Run("when a view is up", func(t *testing.T) {
		rc := settled(t)
		rc.scene.SetView(city.ViewSpend)
		rc.frame()
		st := rc.frame()

		t.Run("it should note the aged lots in the frame's record, as before", func(t *testing.T) {
			assert.Positive(t, agedNoted(st))
		})
		still(t, rc)
	})

	t.Run("when a building mows", func(t *testing.T) {
		rc := settled(t)
		rc.snap(func(s *state.Snapshot) {
			s.At = rc.now
			s.Sessions[1].State, s.Sessions[1].LastActivity = state.Working, rc.now
		})
		rc.frame()
		rc.now = rc.now.Add(city.MowFor / 3)
		require.GreaterOrEqual(t, rc.scene.Mowing("s01"), 0.0, "precondition: the lot is being mowed")
		st := rc.frame()

		t.Run("it should note the aged lots in the frame's record, as before", func(t *testing.T) {
			assert.Positive(t, agedNoted(st))
		})
		still(t, rc)

		t.Run("and the mowing ends", func(t *testing.T) {
			rc.now = rc.now.Add(city.MowFor)
			rc.frame()
			st := rc.frame()

			t.Run("it should note no aged lot in the frame's record again", func(t *testing.T) {
				assert.Zero(t, agedNoted(st))
			})
		})
	})

	for _, c := range []struct {
		name   string
		change func(rc *recordCity)
	}{
		{"a search dims a building", func(rc *recordCity) { rc.scene.SetFilter("s01") }},
		{"night falls", func(rc *recordCity) {
			for night := rc.scene.City().Night; rc.scene.City().Night == night; {
				rc.scene.CycleLight()
			}
		}},
		{"a snapshot ages a lot", func(rc *recordCity) {
			rc.snap(func(s *state.Snapshot) {
				s.Sessions[2].LastActivity = rc.now.Add(-60 * 24 * time.Hour)
			})
		}},
		// Its district's freshest lot is younger, so only the lot's own
		// dressing moves.
		{"a snapshot ages a lot behind its district's freshest", func(rc *recordCity) {
			rc.snap(func(s *state.Snapshot) {
				s.Sessions[20].LastActivity = rc.now.Add(-60 * 24 * time.Hour)
			})
		}},
	} {
		t.Run("when "+c.name, func(t *testing.T) {
			rc := settled(t)
			records := rc.g.static.records
			c.change(rc)
			rc.frame()

			t.Run("it should compose the ground layer again", func(t *testing.T) {
				assert.Greater(t, rc.g.static.records, records)
			})
			still(t, rc)
		})
	}

	t.Run("when a snapshot ages hundreds of lots at once", func(t *testing.T) {
		rc := settled(t)
		rc.snap(func(s *state.Snapshot) {
			for i := range 300 {
				s.Sessions = append(s.Sessions, state.Session{ID: fmt.Sprintf("r%03d", i), CWD: fmt.Sprintf("/src/r%d", i%40),
					State: state.Working, StartedAt: rc.now.Add(-time.Hour), LastActivity: rc.now})
			}
		})
		// Their districts lay new streets, and the old ones overgrow.
		rc.frame()
		rc.now = rc.now.Add(city.OvergrowFor)
		for range 3 {
			rc.frame()
		}
		require.Empty(t, rc.scene.Overgrowth(), "precondition: nothing overgrows")
		records := rc.g.static.records
		rc.snap(func(s *state.Snapshot) {
			for i := range s.Sessions {
				if strings.HasPrefix(s.Sessions[i].ID, "r") {
					s.Sessions[i].State, s.Sessions[i].LastActivity = state.Resting, rc.now.Add(-50*24*time.Hour)
				}
			}
		})
		rc.frame()

		t.Run("it should compose the ground once, not once a lot", func(t *testing.T) {
			assert.Equal(t, records+1, rc.g.static.records)
		})
		still(t, rc)
	})
}
