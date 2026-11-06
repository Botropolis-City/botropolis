package city_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 348, slice 1: a group's buildings in a district sit in adjacent
// lots, a neighbourhood. A session with no lot yet takes one beside its
// group; one that has a lot keeps it unless Aria changed its group.
// inOrder has the sessions arrive in the order given.
func inOrder(sessions ...state.Session) []state.Session {
	for i := range sessions {
		sessions[i].StartedAt = now.Add(-time.Hour + time.Duration(i)*time.Minute)
	}
	return sessions
}

// in is a working session in cinders in a group, or in none.
func in(id, group string) state.Session {
	s := session(id, cinders, state.Working)
	s.Group = group
	return s
}

func loose(id string) state.Session { return in(id, "") }

// withGroups is a snapshot of the sessions with a group for each group
// id they name.
func withGroups(sessions []state.Session) state.Snapshot {
	snapshot := snapshot(sessions...)
	seen := map[string]bool{}
	for _, s := range sessions {
		if s.Group != "" && !seen[s.Group] {
			seen[s.Group] = true
			snapshot.Groups = append(snapshot.Groups, state.Group{ID: s.Group, Name: s.Group})
		}
	}
	return snapshot
}

func TestNeighbourhoods(t *testing.T) {
	pitch := city.BuildingSize + city.BuildingGap
	build := func(t *testing.T, layout *city.Layout, sessions []state.Session) *city.City {
		t.Helper()
		c := city.Build(withGroups(sessions), layout)
		require.NotNil(t, c)
		return c
	}
	// lot is where a building stands in its district, from the district's
	// corner, so lots compare whatever the plan did with the district.
	lot := func(t *testing.T, c *city.City, id string) city.Point {
		t.Helper()
		b := buildingIn(t, c, id)
		return b.Rect.Min.Sub(c.DistrictOf(b).Rect.Min)
	}
	// lotsApart is how many lots two buildings are from each other in
	// their district's grid; neighbours are one apart.
	lotsApart := func(a, b city.Point) float64 {
		return (math.Abs(a.X-b.X) + math.Abs(a.Y-b.Y)) / pitch
	}
	neighbours := func(a, b city.Point) bool { return math.Abs(lotsApart(a, b)-1) < 1e-9 }
	// besideItsGroup is whether a building shares an edge with another of
	// its own group.
	besideItsGroup := func(t *testing.T, c *city.City, id string) bool {
		t.Helper()
		b := buildingIn(t, c, id)
		for _, o := range c.Buildings() {
			if o != b && o.Session.Group == b.Session.Group && neighbours(b.Rect.Min, o.Rect.Min) {
				return true
			}
		}
		return false
	}
	slotAt := func(column, row int) city.Point {
		return city.Point{X: city.DistrictPadding + float64(column)*pitch, Y: city.DistrictPadding + float64(row)*pitch}
	}

	t.Run("when a session arrives in a district where its group has a building", func(t *testing.T) {
		layout := city.NewLayout()
		build(t, layout, inOrder(in("a", "g1"), loose("b")))
		c := build(t, layout, inOrder(in("a", "g1"), loose("b"), in("e", "g1")))

		t.Run("it should take a lot beside it", func(t *testing.T) {
			assert.True(t, neighbours(lot(t, c, "a"), lot(t, c, "e")), "a at %v, e at %v", lot(t, c, "a"), lot(t, c, "e"))
		})
	})

	t.Run("when sessions of two groups arrive interleaved", func(t *testing.T) {
		c := build(t, city.NewLayout(), inOrder(in("a", "g1"), in("x", "g2"), in("b", "g1"), in("y", "g2"), in("c", "g1"), in("z", "g2")))

		for _, id := range []string{"a", "b", "c", "x", "y", "z"} {
			t.Run("it should sit "+id+" beside another of its group", func(t *testing.T) {
				assert.True(t, besideItsGroup(t, c, id))
			})
		}
	})

	t.Run("when a session in no group arrives", func(t *testing.T) {
		layout := city.NewLayout()
		build(t, layout, inOrder(in("a", "g1"), in("b", "g1")))
		c := build(t, layout, inOrder(in("a", "g1"), in("b", "g1"), loose("n")))

		t.Run("it should take the first free lot", func(t *testing.T) {
			assert.Equal(t, slotAt(2, 0), lot(t, c, "n"))
		})
	})

	t.Run("when every lot beside the group is taken", func(t *testing.T) {
		layout := city.NewLayout()
		build(t, layout, inOrder(in("a", "g1"), loose("b"), loose("c"), loose("d")))
		c := build(t, layout, inOrder(in("a", "g1"), loose("b"), loose("c"), loose("d"), in("e", "g1")))

		t.Run("it should take the nearest free lot", func(t *testing.T) {
			assert.InDelta(t, 2, lotsApart(lot(t, c, "a"), lot(t, c, "e")), 1e-9)
		})
	})

	// c, x, a, y, d take lots 0 to 4 of a three-wide grid: a stands at
	// (2,0) with the lot under it, (2,1), free, and d at (1,1), two lots
	// from a.
	settled := func(t *testing.T) (*city.Layout, []state.Session) {
		t.Helper()
		layout := city.NewLayout()
		sessions := inOrder(loose("c"), loose("x"), in("a", "g1"), loose("y"), loose("d"))
		c := build(t, layout, sessions)
		require.Equal(t, slotAt(1, 1), lot(t, c, "d"), "the fixture should put d two lots from a")
		return layout, sessions
	}

	t.Run("when the detector guesses a session into a group, and it already has a lot elsewhere", func(t *testing.T) {
		layout, sessions := settled(t)
		sessions[4].Group = "g1"
		c := build(t, layout, sessions)

		t.Run("it should keep its lot", func(t *testing.T) {
			assert.Equal(t, slotAt(1, 1), lot(t, c, "d"))
		})
	})

	t.Run("when a session is moved into a group by hand, and it already has a lot elsewhere", func(t *testing.T) {
		layout, sessions := settled(t)
		sessions[4].Group, sessions[4].GroupMoved = "g1", true
		c := build(t, layout, sessions)

		t.Run("it should move to a lot beside the group", func(t *testing.T) {
			assert.True(t, neighbours(lot(t, c, "a"), lot(t, c, "d")), "a at %v, d at %v", lot(t, c, "a"), lot(t, c, "d"))
		})

		t.Run("it should leave the group's building on its lot", func(t *testing.T) {
			assert.Equal(t, slotAt(2, 0), lot(t, c, "a"))
		})

		for i, id := range []string{"c", "x"} {
			t.Run("it should leave the unrelated session "+id+" on its lot", func(t *testing.T) {
				assert.Equal(t, slotAt(i, 0), lot(t, c, id))
			})
		}

		t.Run("and the city is built again", func(t *testing.T) {
			again := build(t, layout, sessions)

			t.Run("it should keep its new lot", func(t *testing.T) {
				assert.Equal(t, lot(t, c, "d"), lot(t, again, "d"))
			})
		})
	})

	t.Run("when a session leaves its group by hand", func(t *testing.T) {
		// A session moved to none sits out detection for good, so only a
		// taken-out one can be guessed into another group afterwards.
		ways := []struct {
			scenario  string
			leave     func(s *state.Session)
			guessable bool
		}{
			{"moved to none", func(s *state.Session) { s.Group, s.GroupMoved = "", true }, false},
			{"taken out", func(s *state.Session) { s.Group, s.GroupLeft = "", []string{"g1"} }, true},
		}
		for _, w := range ways {
			t.Run("and it was "+w.scenario, func(t *testing.T) {
				layout := city.NewLayout()
				sessions := inOrder(in("a", "g1"), in("b", "g1"))
				before := build(t, layout, sessions)
				require.True(t, neighbours(lot(t, before, "a"), lot(t, before, "b")), "the fixture should start as neighbours")
				w.leave(&sessions[1])
				c := build(t, layout, sessions)

				t.Run("it should leave the neighbourhood", func(t *testing.T) {
					assert.False(t, neighbours(lot(t, c, "a"), lot(t, c, "b")))
				})

				t.Run("and the detector then guesses it into another group", func(t *testing.T) {
					if !w.guessable {
						t.Skip("a session moved to none is never guessed anywhere")
					}
					was := lot(t, c, "b")
					sessions[1].Group = "g2"
					c := build(t, layout, sessions)

					t.Run("it should keep its lot", func(t *testing.T) {
						assert.Equal(t, was, lot(t, c, "b"))
					})
				})
			})
		}
	})

	t.Run("when a lead is moved into a group by hand, and its teammates follow", func(t *testing.T) {
		layout := city.NewLayout()
		var sessions []state.Session
		for i := 0; i < 7; i++ {
			sessions = append(sessions, loose(fmt.Sprintf("f%d", i)))
		}
		lead := loose("lead")
		lead.Leads = "crew"
		team := []state.Session{lead}
		for _, id := range []string{"t1", "t2"} {
			s := loose(id)
			s.Team, s.Agent = "crew", id
			team = append(team, s)
		}
		sessions = inOrder(append(append(sessions, in("a", "g1")), team...)...)
		before := build(t, layout, sessions)
		for i := range sessions[8:] {
			sessions[8+i].Group, sessions[8+i].GroupMoved = "g1", true
		}
		c := build(t, layout, sessions)

		for _, id := range []string{"lead", "t1", "t2"} {
			t.Run("it should move "+id+" off its lot", func(t *testing.T) {
				assert.NotEqual(t, lot(t, before, id), lot(t, c, id))
			})

			t.Run("it should sit "+id+" beside the group", func(t *testing.T) {
				assert.True(t, besideItsGroup(t, c, id))
			})
		}
	})

	t.Run("when a layout written before this build is read", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "layout.json")
		old := fmt.Sprintf(`{"version":1,"readableFrom":1,"districts":{%q:{"slots":{"a":0,"b":1,"c":2}}}}`, cinders)
		require.NoError(t, os.WriteFile(path, []byte(old), 0o600))
		layout, err := city.LoadLayout(path)
		require.NoError(t, err)
		c := build(t, layout, inOrder(in("a", "g1"), loose("b"), in("c", "g1")))

		t.Run("it should keep every lot as it was", func(t *testing.T) {
			assert.Equal(t, map[string]city.Point{"a": slotAt(0, 0), "b": slotAt(1, 0), "c": slotAt(2, 0)},
				map[string]city.Point{"a": lot(t, c, "a"), "b": lot(t, c, "b"), "c": lot(t, c, "c")})
		})
	})
}
