package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

const (
	lanterns = "/home/avesta/workspaces/github/mCedar/lanterns"
	orchard  = "/home/avesta/workspaces/github/mCedar/orchard"
)

func TestLines(t *testing.T) {
	member := func(id, cwd, group string, st state.State, started time.Duration) state.Session {
		s := session(id, cwd, st)
		s.Group, s.StartedAt = group, now.Add(started)
		return s
	}
	linedOn := func(t *testing.T, layout *city.Layout, sessions []state.Session, groups []state.Group) *city.Scene {
		t.Helper()
		snap := snapshot(sessions...)
		snap.Groups = groups
		s := city.NewScene(layout)
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snap)
		return s
	}
	lined := func(t *testing.T, sessions []state.Session, groups []state.Group) *city.Scene {
		t.Helper()
		return linedOn(t, city.NewLayout(), sessions, groups)
	}
	dirs := func(l city.Line) []string {
		var out []string
		for _, st := range l.Stations {
			out = append(out, st.District.Root)
		}
		return out
	}

	t.Run("when a group runs through three districts", func(t *testing.T) {
		guessed := member("c", lanterns, "g1", state.Working, -time.Hour)
		guessed.GroupLikely = true
		s := lined(t, []state.Session{
			member("a", cinders, "g1", state.NeedsYou, -3*time.Hour),
			member("b", botropolis, "g1", state.Working, -2*time.Hour),
			guessed,
			member("d", orchard, "", state.Working, 0),
		}, []state.Group{{ID: "g1", Name: "PROJ-1", Short: "PROJ-1", Members: []string{"a", "b", "c"}}})
		lines := s.Lines()
		require.Len(t, lines, 1)

		t.Run("it should have one station per district, in the order its sites were started", func(t *testing.T) {
			assert.Equal(t, []string{cinders, botropolis, lanterns}, dirs(lines[0]))
		})

		t.Run("each station should carry the move of the sessions at its site", func(t *testing.T) {
			var moves []state.Move
			for _, st := range lines[0].Stations {
				moves = append(moves, st.Move)
			}
			assert.Equal(t, []state.Move{state.MoveYours, state.MoveAgents, state.MoveAgents}, moves)
		})

		t.Run("each station should stand on its member's building, not the middle of the lot", func(t *testing.T) {
			var on *city.Building
			for _, b := range s.City().Buildings() {
				if b.Session.ID == "b" {
					on = b
				}
			}
			require.NotNil(t, on)
			assert.Equal(t, on.Rect.Center(), lines[0].Stations[1].At)
		})

		t.Run("a site joined only by a guess should have a dashed tie", func(t *testing.T) {
			var dashed []bool
			for _, st := range lines[0].Stations {
				dashed = append(dashed, st.Dashed)
			}
			assert.Equal(t, []bool{false, false, true}, dashed)
		})

		t.Run("when a station is hovered, its card should name the group, whose move it is and its stations", func(t *testing.T) {
			card := city.StationCard(lines[0], 1)
			assert.Equal(t, city.Card{Title: "PROJ-1", Lines: []string{
				"station  botropolis, the agents'",
				"line     cinders → botropolis → lanterns",
			}}, card)
		})
	})

	t.Run("when two groups share a district", func(t *testing.T) {
		s := lined(t, []state.Session{
			member("a", cinders, "g1", state.Working, 0), member("b", botropolis, "g1", state.Working, time.Hour),
			member("c", cinders, "g2", state.Working, 0), member("d", lanterns, "g2", state.Working, time.Hour),
		}, []state.Group{{ID: "g1", Short: "A", Members: []string{"a", "b"}}, {ID: "g2", Short: "B", Members: []string{"c", "d"}}})
		lines := s.Lines()
		require.Len(t, lines, 2)

		t.Run("their stations there should not overlap", func(t *testing.T) {
			assert.NotEqual(t, lines[0].Stations[0].At, lines[1].Stations[0].At)
		})
	})

	// Item 281: a line with one station leads nowhere, so a group whose
	// stations stand in one drawn district draws none.
	pair := []state.Group{{ID: "g1", Members: []string{"a", "b"}}}
	t.Run("when a group runs through one district only", func(t *testing.T) {
		s := lined(t, []state.Session{member("a", cinders, "g1", state.Working, 0), member("b", cinders, "g1", state.Working, time.Hour)}, pair)

		t.Run("it should draw no line", func(t *testing.T) {
			assert.Empty(t, s.Lines())
		})
	})

	t.Run("when a group runs through two districts", func(t *testing.T) {
		two := func() []state.Session {
			return []state.Session{member("a", cinders, "g1", state.Working, 0), member("b", botropolis, "g1", state.Working, time.Hour)}
		}

		t.Run("and both are drawn", func(t *testing.T) {
			s := lined(t, two(), pair)

			t.Run("it should draw its line through both", func(t *testing.T) {
				require.Len(t, s.Lines(), 1)
				assert.Equal(t, []string{cinders, botropolis}, dirs(s.Lines()[0]))
			})
		})

		t.Run("and one of them is a hidden project", func(t *testing.T) {
			layout := city.NewLayout()
			layout.SetHidden(botropolis, true)
			s := linedOn(t, layout, two(), pair)

			t.Run("it should draw no line", func(t *testing.T) {
				assert.Empty(t, s.Lines())
			})
		})

		t.Run("and one of its members is packed up", func(t *testing.T) {
			sessions := two()
			sessions[1].Packed = true
			s := lined(t, sessions, pair)

			t.Run("it should draw no line", func(t *testing.T) {
				assert.Empty(t, s.Lines())
			})
		})
	})

	t.Run("when the lines are hidden", func(t *testing.T) {
		s := lined(t, []state.Session{member("a", cinders, "g1", state.Working, 0), member("b", botropolis, "g1", state.Working, time.Hour)},
			[]state.Group{{ID: "g1", Members: []string{"a", "b"}}})
		s.SetGroupLines(false)

		t.Run("it should lay out none", func(t *testing.T) {
			assert.Empty(t, s.Lines())
		})
	})
}
