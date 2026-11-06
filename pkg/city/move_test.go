package city_test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 348, slice 1: a building moves when Aria changes its session's
// group, and the move is drawn, as packing's haul is, so she does not
// lose track of it; a guess by the detector moves nothing.
func TestMoves(t *testing.T) {
	at := now
	// settled is a scene on c, x, a in g1, y and d, d two lots from a.
	settled := func(t *testing.T, reduced bool) (*city.Scene, []state.Session) {
		t.Helper()
		at = now
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.SetReducedMotion(reduced)
		s.Resize(800, 600)
		sessions := inOrder(loose("c"), loose("x"), in("a", "g1"), loose("y"), loose("d"))
		s.SetSnapshot(withGroups(sessions))
		return s, sessions
	}
	byHand := func(sessions []state.Session, ids ...string) {
		for i := range sessions {
			for _, id := range ids {
				if sessions[i].ID == id {
					sessions[i].Group, sessions[i].GroupMoved = "g1", true
				}
			}
		}
	}

	t.Run("when a session is moved into a group by hand", func(t *testing.T) {
		s, sessions := settled(t, false)
		from := centreIn(t, s, "d")
		byHand(sessions, "d")
		s.SetSnapshot(withGroups(sessions))
		to := centreIn(t, s, "d")
		require.NotEqual(t, from, to, "the fixture should move d")
		hauls := s.Hauls()
		require.Len(t, hauls, 1)

		t.Run("it should start a move for the session", func(t *testing.T) {
			assert.Equal(t, city.Haul{SessionID: "d", Kind: city.HaulMove, From: from, To: to, Since: now, For: hauls[0].For, Hue: hauls[0].Hue}, hauls[0])
		})

		t.Run("it should take as long as the haul from lot to lot says", func(t *testing.T) {
			assert.Equal(t, city.HaulFor(from, to, false), hauls[0].For)
		})

		// Aria, 2026-11-06: "Use a crate, as packing does." The truck
		// carries a crate, and the building fades out where it stood and
		// in where it lands.
		t.Run("it should carry a crate, not the building", func(t *testing.T) {
			assert.False(t, s.Riding("d"))
		})

		fades := []struct {
			name     string
			after    time.Duration
			old, new float64
		}{
			{"it has just left", 0, 1, 0},
			{"its old lot has faded", city.PackFadeFor, 0, 0},
			{"it is half way", hauls[0].For / 2, 0, 0},
			{"it has arrived", hauls[0].For, 0, 1},
		}
		for _, f := range fades {
			t.Run("and "+f.name, func(t *testing.T) {
				old, new := hauls[0].Fades(now.Add(f.after))

				t.Run("it should show the building at its old lot as much as the fade says", func(t *testing.T) {
					assert.InDelta(t, f.old, old, 1e-9)
				})

				t.Run("it should show it at its new lot as much as the fade says", func(t *testing.T) {
					assert.InDelta(t, f.new, new, 1e-9)
				})
			})
		}

		t.Run("and it is half way", func(t *testing.T) {
			at = now.Add(hauls[0].For / 2)

			t.Run("it should not show the building on its new lot yet", func(t *testing.T) {
				assert.Zero(t, s.HaulProgress("d"))
			})

			// A working building's roof scaffolding is the building's,
			// so it does not stand on an empty lot while the crate rides.
			t.Run("it should put up no scaffolding", func(t *testing.T) {
				assert.Zero(t, s.Scaffold(s.City().BuildingOf("d")))
			})
		})

		t.Run("and the clock passes its span", func(t *testing.T) {
			at = now.Add(hauls[0].For)

			t.Run("it should have arrived", func(t *testing.T) {
				assert.Empty(t, s.Hauls())
			})
		})
	})

	t.Run("when the detector guesses a session into a group", func(t *testing.T) {
		s, sessions := settled(t, false)
		sessions[4].Group = "g1"
		s.SetSnapshot(withGroups(sessions))

		t.Run("it should start no move", func(t *testing.T) {
			assert.Empty(t, s.Hauls())
		})
	})

	t.Run("when a lead and its teammates are moved into a group by hand", func(t *testing.T) {
		at = now
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Resize(800, 600)
		var sessions []state.Session
		for i := 0; i < 7; i++ {
			sessions = append(sessions, loose(fmt.Sprintf("f%d", i)))
		}
		lead := loose("lead")
		lead.Leads = "crew"
		sessions = append(sessions, in("a", "g1"), lead)
		for _, id := range []string{"t1", "t2"} {
			m := loose(id)
			m.Team, m.Agent = "crew", id
			sessions = append(sessions, m)
		}
		sessions = inOrder(sessions...)
		s.SetSnapshot(withGroups(sessions))
		byHand(sessions, "lead", "t1", "t2")
		s.SetSnapshot(withGroups(sessions))
		hauls := s.Hauls()
		require.Len(t, hauls, 3)

		t.Run("it should leave as a convoy, a gap between each", func(t *testing.T) {
			var offsets []time.Duration
			for _, h := range hauls {
				offsets = append(offsets, h.Since.Sub(now))
			}
			sort.Slice(offsets, func(i, j int) bool { return offsets[i] < offsets[j] })
			assert.Equal(t, []time.Duration{0, city.ConvoyGap, 2 * city.ConvoyGap}, offsets)
		})
	})

	t.Run("when reduced motion is on, and a session is moved by hand", func(t *testing.T) {
		s, sessions := settled(t, true)
		byHand(sessions, "d")
		s.SetSnapshot(withGroups(sessions))
		require.Len(t, s.Hauls(), 1)

		t.Run("and the fade is half way", func(t *testing.T) {
			at = now.Add(city.PackFadeFor / 2)

			t.Run("it should have the building half faded in on its new lot", func(t *testing.T) {
				assert.InDelta(t, 0.5, s.HaulProgress("d"), 1e-9)
			})
		})
	})
}
