package city_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

func TestDive(t *testing.T) {
	member := func(id, cwd string, st state.State, started, active time.Duration, recap string) state.Session {
		s := session(id, cwd, st)
		s.Group, s.StartedAt, s.LastActivity = "g1", now.Add(started), now.Add(active)
		s.Recap = claude.Recap{Text: recap, At: now.Add(active)}
		return s
	}
	scene := func(t *testing.T) *city.Scene {
		t.Helper()
		a := member("a", cinders, state.NeedsYou, -3*time.Hour, -2*time.Hour, "the client is done")
		a.PRs = []claude.PR{{Number: 41, Repository: "cinders", State: claude.PROpen}}
		snap := snapshot(
			a,
			member("b", botropolis, state.Working, -2*time.Hour, -50*time.Minute, "wiring the headers"),
			member("b2", botropolis, state.Working, -90*time.Minute, -10*time.Minute, "the headers pass"),
			member("c", lanterns, state.Resting, -time.Hour, -30*time.Minute, "applied the change"),
			session("d", orchard, state.Working),
		)
		snap.Groups = []state.Group{{ID: "g1", Name: "PROJ-1", Short: "PROJ-1", Members: []string{"a", "b", "b2", "c"}}}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snap)
		return s
	}
	focused := func(t *testing.T) *city.Scene {
		t.Helper()
		s := scene(t)
		require.True(t, s.FocusGroup("g1"))
		return s
	}
	column := func(rows []city.DiveRow, of func(city.DiveRow) string) []string {
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, of(r))
		}
		return out
	}

	t.Run("when no group is focused", func(t *testing.T) {
		t.Run("it should have no dive rows", func(t *testing.T) {
			assert.Empty(t, scene(t).Dive())
		})
	})

	t.Run("when a group with three sites is focused", func(t *testing.T) {
		rows := focused(t).Dive()
		require.Len(t, rows, 3)

		t.Run("each row should carry its site's district name, in line order", func(t *testing.T) {
			assert.Equal(t, []string{"cinders", "botropolis", "lanterns"}, column(rows, func(r city.DiveRow) string { return r.Site }))
		})

		t.Run("each row should carry the sentence of its site's most urgent session", func(t *testing.T) {
			assert.Equal(t, "the client is done", rows[0].Sentence)
		})

		t.Run("and two sessions share a site with the same move", func(t *testing.T) {
			t.Run("the row should lead with the newer one", func(t *testing.T) {
				assert.Equal(t, "b2", rows[1].Session)
			})
		})

		t.Run("a row whose site saw a pull request should carry its number and last state", func(t *testing.T) {
			assert.Equal(t, "PR #41 open", rows[0].PR)
		})

		t.Run("a row whose site saw none should carry no pull request", func(t *testing.T) {
			assert.Empty(t, rows[2].PR)
		})

		t.Run("each row should carry the site's move", func(t *testing.T) {
			assert.Equal(t, []string{string(state.MoveYours), string(state.MoveAgents), string(state.MoveNobody)},
				column(rows, func(r city.DiveRow) string { return string(r.Move) }))
		})

		t.Run("each row should carry how long its site has waited", func(t *testing.T) {
			assert.Equal(t, []time.Duration{2 * time.Hour, 10 * time.Minute, 30 * time.Minute},
				[]time.Duration{rows[0].Waited, rows[1].Waited, rows[2].Waited})
		})

		t.Run("it should start with no row stepped to", func(t *testing.T) {
			assert.Equal(t, -1, focused(t).DiveStep())
		})

		t.Run("and it steps down once", func(t *testing.T) {
			s := focused(t)
			s.StepDive(1)

			t.Run("it should step to the first row", func(t *testing.T) {
				assert.Equal(t, 0, s.DiveStep())
			})

			t.Run("it should select that site's session", func(t *testing.T) {
				require.NotNil(t, s.Selected())
				assert.Equal(t, "a", s.Selected().Session.ID)
			})

			t.Run("it should centre the camera on that station", func(t *testing.T) {
				in := s.Insets()
				centre := city.Point{X: in.Left + (800-in.Left-in.Right)/2, Y: in.Top + (600-in.Top-in.Bottom)/2}
				at := s.Camera().WorldToScreen(s.Lines()[0].Stations[0].At)
				assert.InDelta(t, 0, math.Hypot(at.X-centre.X, at.Y-centre.Y), 1)
			})
		})

		t.Run("and it steps up from the first row", func(t *testing.T) {
			s := focused(t)
			s.StepDive(1)
			s.StepDive(-1)

			t.Run("it should stay on the first row", func(t *testing.T) {
				assert.Equal(t, 0, s.DiveStep())
			})
		})

		t.Run("and it steps down past the last row", func(t *testing.T) {
			s := focused(t)
			for range 5 {
				s.StepDive(1)
			}

			t.Run("it should stay on the last row", func(t *testing.T) {
				assert.Equal(t, 2, s.DiveStep())
			})
		})

		t.Run("and the dive is left", func(t *testing.T) {
			s := focused(t)
			s.StepDive(1)
			s.LeaveDive()

			t.Run("it should clear the focus", func(t *testing.T) {
				assert.Empty(t, s.FocusedGroup())
			})

			t.Run("it should clear the selection", func(t *testing.T) {
				assert.Nil(t, s.Selected())
			})

			t.Run("it should have no dive rows", func(t *testing.T) {
				assert.Empty(t, s.Dive())
			})

			t.Run("and the group is focused again", func(t *testing.T) {
				require.True(t, s.FocusGroup("g1"))

				t.Run("it should forget the row it was on", func(t *testing.T) {
					assert.Equal(t, -1, s.DiveStep())
				})
			})
		})
	})

	t.Run("when the dive reserves room", func(t *testing.T) {
		for _, side := range []city.Side{city.SideLeft, city.SideRight} {
			t.Run("on the "+side.String(), func(t *testing.T) {
				s := scene(t)
				s.SetDiveRoom(side, 300)
				require.True(t, s.FocusGroup("g1"))
				var framed city.Rect
				for i, st := range s.Lines()[0].Stations {
					r := screenRect(s.Camera(), st.District.Rect)
					if i == 0 {
						framed = r
					} else {
						framed = framed.Union(r)
					}
				}

				t.Run("the framed districts should lie outside the reserved room", func(t *testing.T) {
					if side == city.SideLeft {
						assert.GreaterOrEqual(t, framed.Min.X, 300.0-1)
					} else {
						assert.LessOrEqual(t, framed.Max.X, 800.0-300+1)
					}
				})
			})
		}

		t.Run("and no group is focused", func(t *testing.T) {
			s := scene(t)
			s.SetDiveRoom(city.SideLeft, 300)

			t.Run("it should reserve nothing", func(t *testing.T) {
				assert.Equal(t, scene(t).Insets(), s.Insets())
			})
		})
	})

	t.Run("when the river is drawn", func(t *testing.T) {
		for _, c := range []struct {
			name  string
			turns int
			want  city.Side
		}{
			{"on the east, facing north", 0, city.SideLeft},
			{"on the west, facing south", 2, city.SideRight},
		} {
			t.Run(c.name, func(t *testing.T) {
				s := scene(t)
				for range c.turns {
					s.Turn()
				}

				t.Run("the dive should dock on the side away from it", func(t *testing.T) {
					assert.Equal(t, c.want, s.DiveSide())
				})
			})
		}
	})
}

// screenRect is the screen box around a world rectangle.
func screenRect(c *city.Camera, r city.Rect) city.Rect {
	corners := []city.Point{r.Min, {X: r.Max.X, Y: r.Min.Y}, r.Max, {X: r.Min.X, Y: r.Max.Y}}
	first := c.WorldToScreen(corners[0])
	out := city.Rect{Min: first, Max: first}
	for _, p := range corners[1:] {
		out = out.Union(city.Rect{Min: c.WorldToScreen(p), Max: c.WorldToScreen(p)})
	}
	return out
}
