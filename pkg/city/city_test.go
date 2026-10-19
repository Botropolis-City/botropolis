package city_test

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	cinders    = "/home/avesta/workspaces/github/mCedar/cinders"
	botropolis = "/home/avesta/workspaces/github/auroq/botropolis"
	mullet     = "/home/avesta/workspaces/github/mCedar/mullet"
	worktree   = botropolis + "/.claude/worktrees/milestone-0-scaffold"
)

var now = time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC)

func session(id, cwd string, s state.State) state.Session {
	return state.Session{
		ID: id, CWD: cwd, State: s, Title: "Fix the CI queue", Branch: "main",
		Model: "claude-opus-5[1m]", ContextPercent: 25, ContextTokens: 250_000, ContextWindow: 1_000_000,
		FreshTokensPerHour: 152_000, CacheReadPerHour: 8_900_000, OutputPerHour: 41_000,
		Usage:     claude.Usage{Input: 1000, Output: 600, CacheRead: 2000, Messages: 3},
		Subagents: 4, SubagentsInFlight: 1, StartedAt: now.Add(-5*time.Hour - 6*time.Minute), LastActivity: now.Add(-6 * time.Minute),
		Hooked: true,
	}
}

func snapshot(sessions ...state.Session) state.Snapshot {
	return state.Snapshot{At: now, Sessions: sessions}
}

func build(t *testing.T, layout *city.Layout, sessions ...state.Session) *city.City {
	t.Helper()
	c := city.Build(snapshot(sessions...), layout)
	require.NotNil(t, c)
	return c
}

func TestProjectRoot(t *testing.T) {
	cases := []struct {
		name, cwd, want string
	}{
		{"a plain repository", botropolis, botropolis},
		{"a worktree under .claude/worktrees", worktree, botropolis},
		{"a path inside a worktree", worktree + "/pkg/city", botropolis},
		{"a path with no worktree segment", cinders + "/pkg", cinders + "/pkg"},
		{"an empty cwd", "", ""},
	}
	for _, c := range cases {
		t.Run("when the cwd is "+c.name, func(t *testing.T) {
			t.Run("it should resolve the project root", func(t *testing.T) {
				assert.Equal(t, c.want, city.ProjectRoot(c.cwd))
			})
		})
	}
}

func TestBuild(t *testing.T) {
	t.Run("when two sessions share a project and one is elsewhere", func(t *testing.T) {
		c := build(t, city.NewLayout(),
			session("a", cinders, state.NeedsYou),
			session("b", cinders, state.Working),
			session("c", botropolis, state.Unattended))

		t.Run("it should make one district per project", func(t *testing.T) {
			assert.Len(t, c.Districts, 2)
		})

		t.Run("it should name a district after its directory", func(t *testing.T) {
			assert.Equal(t, []string{"botropolis", "cinders"}, []string{c.Districts[0].Name, c.Districts[1].Name})
		})

		t.Run("it should put both sessions of a project in its district", func(t *testing.T) {
			assert.Len(t, c.Districts[1].Buildings, 2)
		})

		t.Run("it should give every session a building", func(t *testing.T) {
			assert.Len(t, c.Buildings(), 3)
		})

		t.Run("it should not overlap two districts", func(t *testing.T) {
			assert.False(t, c.Districts[0].Rect.Overlaps(c.Districts[1].Rect))
		})

		t.Run("it should keep every building inside its district", func(t *testing.T) {
			for _, d := range c.Districts {
				for _, b := range d.Buildings {
					assert.True(t, d.Rect.Contains(b.Rect.Min) && d.Rect.Contains(b.Rect.Max), b.Session.ID)
				}
			}
		})

		t.Run("it should not overlap two buildings in one district", func(t *testing.T) {
			assert.False(t, c.Districts[1].Buildings[0].Rect.Overlaps(c.Districts[1].Buildings[1].Rect))
		})
	})

	t.Run("when a session runs in a worktree of another session's project", func(t *testing.T) {
		c := build(t, city.NewLayout(),
			session("a", botropolis, state.Working),
			session("b", worktree, state.Unattended))

		t.Run("it should fold them into one district", func(t *testing.T) {
			require.Len(t, c.Districts, 1)
			assert.Len(t, c.Districts[0].Buildings, 2)
		})
	})

	t.Run("when a building's session needs you", func(t *testing.T) {
		c := build(t, city.NewLayout(), session("a", cinders, state.NeedsYou))

		t.Run("it should pulse", func(t *testing.T) {
			assert.True(t, c.Buildings()[0].Pulse)
		})

		t.Run("it should be lit", func(t *testing.T) {
			assert.True(t, c.Buildings()[0].Lit)
		})
	})

	t.Run("when a building's session is parked", func(t *testing.T) {
		c := build(t, city.NewLayout(), session("a", cinders, state.Resting))

		t.Run("it should not pulse", func(t *testing.T) {
			assert.False(t, c.Buildings()[0].Pulse)
		})

		t.Run("it should not be lit", func(t *testing.T) {
			assert.False(t, c.Buildings()[0].Lit)
		})

		t.Run("it should have its blinds down", func(t *testing.T) {
			assert.True(t, c.Buildings()[0].Blinds)
		})
	})

	t.Run("when a building's session is empty", func(t *testing.T) {
		empty := session("a", cinders, state.Empty)
		empty.Title, empty.ContextPercent, empty.ContextTokens, empty.ContextWindow = "", 0, 0, 0
		c := build(t, city.NewLayout(), empty, session("b", cinders, state.Working))
		require.Len(t, c.Districts, 1)
		require.Len(t, c.Districts[0].Buildings, 2)
		plot := c.Districts[0].Buildings[0]
		if plot.Session.ID != "a" {
			plot = c.Districts[0].Buildings[1]
		}

		t.Run("it should stand on a plot in its district", func(t *testing.T) {
			assert.Equal(t, "a", plot.Session.ID)
		})

		t.Run("it should be vacant", func(t *testing.T) {
			assert.True(t, plot.Vacant)
		})

		t.Run("it should not be lit", func(t *testing.T) {
			assert.False(t, plot.Lit)
		})

		t.Run("it should not count in the summary", func(t *testing.T) {
			assert.Equal(t, 1, c.Summary().Live())
		})

		t.Run("it should not count as live in its project row", func(t *testing.T) {
			assert.Equal(t, 1, c.Projects(nil)[0].Live)
		})

		t.Run("its card should say nothing was typed", func(t *testing.T) {
			assert.Contains(t, plot.Card(now).Details, "empty    nothing typed yet; prune clears it after an hour")
		})
	})

	t.Run("when a building's session is working", func(t *testing.T) {
		c := build(t, city.NewLayout(), session("a", cinders, state.Working))

		t.Run("it should be lit without pulsing", func(t *testing.T) {
			assert.True(t, c.Buildings()[0].Lit)
			assert.False(t, c.Buildings()[0].Pulse)
		})
	})

	t.Run("when a session has used a quarter of its context", func(t *testing.T) {
		c := build(t, city.NewLayout(), session("a", cinders, state.Working))

		t.Run("it should fill the building to a quarter", func(t *testing.T) {
			assert.InDelta(t, 0.25, c.Buildings()[0].Fill, 1e-9)
		})
	})

	t.Run("when a session has subagents in flight", func(t *testing.T) {
		c := build(t, city.NewLayout(), session("a", cinders, state.Working))

		t.Run("it should put that many cranes on the roof", func(t *testing.T) {
			assert.Equal(t, 1, c.Buildings()[0].Cranes)
		})
	})

	t.Run("when the snapshot is empty", func(t *testing.T) {
		c := build(t, city.NewLayout())

		t.Run("it should have no districts", func(t *testing.T) {
			assert.Empty(t, c.Districts)
		})
	})
}

func TestStickyLayout(t *testing.T) {
	t.Run("when a city is built twice from the same layout", func(t *testing.T) {
		layout := city.NewLayout()
		first := build(t, layout, session("a", cinders, state.Working), session("b", botropolis, state.Working))
		second := build(t, layout, session("a", cinders, state.Working), session("b", botropolis, state.Working))

		t.Run("it should place every district in the same place", func(t *testing.T) {
			assert.Equal(t, first.Districts[0].Rect, second.Districts[0].Rect)
		})

		t.Run("it should place every building in the same place", func(t *testing.T) {
			assert.Equal(t, first.Buildings()[0].Rect, second.Buildings()[0].Rect)
		})
	})

	t.Run("when a session disappears and a new one arrives", func(t *testing.T) {
		layout := city.NewLayout()
		before := build(t, layout, session("a", cinders, state.Working), session("b", cinders, state.Working))
		aRect := before.Districts[0].Buildings[0].Rect
		bRect := before.Districts[0].Buildings[1].Rect
		after := build(t, layout, session("a", cinders, state.Working), session("c", cinders, state.Working))

		t.Run("it should keep the surviving session where it was", func(t *testing.T) {
			assert.Equal(t, aRect, after.Districts[0].Buildings[0].Rect)
		})

		t.Run("it should not put the new session on top of the old one", func(t *testing.T) {
			assert.NotEqual(t, aRect, after.Districts[0].Buildings[1].Rect)
		})

		t.Run("and the old session comes back", func(t *testing.T) {
			back := build(t, layout,
				session("a", cinders, state.Working), session("b", cinders, state.Working), session("c", cinders, state.Working))

			t.Run("it should get its old slot back", func(t *testing.T) {
				for _, b := range back.Districts[0].Buildings {
					if b.Session.ID == "b" {
						assert.Equal(t, bRect, b.Rect)
						return
					}
				}
				t.Fatal("session b is not in the city")
			})
		})
	})

	t.Run("when one district is much taller than the others", func(t *testing.T) {
		layout := city.NewLayout()
		var sessions []state.Session
		for _, id := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
			sessions = append(sessions, session(id, cinders, state.Working))
		}
		for i, root := range []string{"/p/one", "/p/two", "/p/three", "/p/four", "/p/five"} {
			sessions = append(sessions, session("x"+string(rune('a'+i)), root, state.Working))
		}
		c := build(t, layout, sessions...)

		t.Run("it should not overlap any two districts", func(t *testing.T) {
			for i, a := range c.Districts {
				for _, b := range c.Districts[i+1:] {
					assert.False(t, a.Rect.Overlaps(b.Rect), a.Name+" overlaps "+b.Name)
				}
			}
		})
	})

	t.Run("when a district is added later", func(t *testing.T) {
		layout := city.NewLayout()
		first := build(t, layout, session("a", cinders, state.Working))
		second := build(t, layout, session("a", cinders, state.Working), session("b", botropolis, state.Working))

		t.Run("it should not move the district that was already placed", func(t *testing.T) {
			for _, d := range second.Districts {
				if d.Name == "cinders" {
					assert.Equal(t, first.Districts[0].Rect.Min, d.Rect.Min)
					return
				}
			}
			t.Fatal("cinders is not in the city")
		})
	})

	t.Run("when a district grows past its first size", func(t *testing.T) {
		layout := city.NewLayout()
		small := build(t, layout, session("a", cinders, state.Working))
		many := []state.Session{}
		for _, id := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
			many = append(many, session(id, cinders, state.Working))
		}
		big := build(t, layout, many...)

		t.Run("it should grow the district rather than spill outside it", func(t *testing.T) {
			assert.Greater(t, big.Districts[0].Rect.Area(), small.Districts[0].Rect.Area())
			for _, b := range big.Districts[0].Buildings {
				assert.True(t, big.Districts[0].Rect.Contains(b.Rect.Max), b.Session.ID)
			}
		})
	})
}

func TestHitTest(t *testing.T) {
	c := build(t, city.NewLayout(), session("a", cinders, state.Working), session("b", botropolis, state.Working))
	building := c.Buildings()[0]

	t.Run("when a point is inside a building", func(t *testing.T) {
		hit := c.At(building.Rect.Center())

		t.Run("it should find that building", func(t *testing.T) {
			require.NotNil(t, hit.Building)
			assert.Equal(t, building.Session.ID, hit.Building.Session.ID)
		})

		t.Run("it should also name the district", func(t *testing.T) {
			require.NotNil(t, hit.District)
			assert.Equal(t, c.Districts[0].Name, hit.District.Name)
		})
	})

	t.Run("when a point is inside a district but not a building", func(t *testing.T) {
		corner := c.Districts[0].Rect.Min.Add(city.Point{X: 1, Y: 1})
		hit := c.At(corner)

		t.Run("it should find the district", func(t *testing.T) {
			require.NotNil(t, hit.District)
		})

		t.Run("it should find no building", func(t *testing.T) {
			assert.Nil(t, hit.Building)
		})
	})

	t.Run("when a point is outside everything", func(t *testing.T) {
		hit := c.At(city.Point{X: -10_000, Y: -10_000})

		t.Run("it should find nothing", func(t *testing.T) {
			assert.Nil(t, hit.District)
			assert.Nil(t, hit.Building)
		})
	})
}

func TestHoverCard(t *testing.T) {
	c := build(t, city.NewLayout(), session("a", cinders, state.NeedsYou))

	t.Run("when a building is hovered", func(t *testing.T) {
		card := c.Buildings()[0].Card(now)

		t.Run("it should lead with the title", func(t *testing.T) {
			assert.Equal(t, "Fix the CI queue", card.Title)
		})

		t.Run("it should show the same state as the table", func(t *testing.T) {
			assert.Contains(t, card.Details, "state    needs-you")
		})

		t.Run("it should show the same context percentage as the table, with the token count", func(t *testing.T) {
			assert.Contains(t, card.Details, "context  25% of 1.0M (250k tokens)")
		})

		t.Run("it should show the same token rates as the table", func(t *testing.T) {
			assert.Contains(t, card.Details, "tokens   152k/h fresh, 8.9M/h cached")
		})

		t.Run("it should show the age and how long it has been idle, the table's number", func(t *testing.T) {
			assert.Contains(t, card.Details, "age      5h06m, idle 6m")
		})

		t.Run("it should show the branch and model", func(t *testing.T) {
			assert.Contains(t, card.Details, "branch   main")
			assert.Contains(t, card.Details, "model    claude-opus-5[1m]")
		})

		t.Run("it should show the subagents in flight", func(t *testing.T) {
			assert.Contains(t, card.Details, "subs     1 of 4 in flight")
		})

		t.Run("it should carry the day's tokens per hour for a sparkline", func(t *testing.T) {
			assert.Len(t, card.Series, 24)
		})
	})

	t.Run("when a district is hovered", func(t *testing.T) {
		card := c.Districts[0].Card()

		t.Run("it should lead with the project name", func(t *testing.T) {
			assert.Equal(t, "cinders", card.Title)
		})

		t.Run("it should count its sessions", func(t *testing.T) {
			assert.Contains(t, card.Details, "sessions 1")
		})

		t.Run("it should count its PRs", func(t *testing.T) {
			assert.Contains(t, card.Details, "prs      0")
		})
	})
}

func TestBuildingCardDetails(t *testing.T) {
	s := session("a", cinders, state.Working)
	s.Tool, s.Subject = "Edit", cinders+"/pkg/city/city.go"
	s.Compactions, s.LastCompactionAt = 2, now.Add(-30*time.Minute)
	s.SubagentNames = []string{"Explore"}
	s.PRs = []claude.PR{{Number: 1181, Repository: "mCedar/mullet"}}
	s.APIErrors, s.LastErrorAt = 1, now.Add(-10*time.Minute)
	s.Note = "Claude needs your permission to use Bash"
	c := build(t, city.NewLayout(), s)
	card := c.Buildings()[0].Card(now)

	t.Run("when the worker is mid-tool", func(t *testing.T) {
		t.Run("it should say the tool and the file relative to the project", func(t *testing.T) {
			assert.Contains(t, card.Details, "doing    Edit pkg/city/city.go")
		})
	})

	t.Run("when the session was compacted", func(t *testing.T) {
		t.Run("it should say how often and when", func(t *testing.T) {
			assert.Contains(t, card.Details, "context  25% of 1.0M (250k tokens), compacted 2x, last "+now.Add(-30*time.Minute).Local().Format("15:04"))
		})
	})

	t.Run("when subagents are in flight", func(t *testing.T) {
		t.Run("it should name them", func(t *testing.T) {
			assert.Contains(t, card.Details, "subs     1 of 4 in flight: Explore")
		})
	})

	t.Run("when a PR is linked", func(t *testing.T) {
		t.Run("it should list it", func(t *testing.T) {
			assert.Contains(t, card.Details, "prs      #1181 mCedar/mullet")
		})

		t.Run("it should say when a PR merged", func(t *testing.T) {
			done := s
			done.PRs = []claude.PR{{Number: 1181, Repository: "mCedar/mullet", State: claude.PRMerged}}
			assert.Contains(t, city.Build(snapshot(done), city.NewLayout()).Buildings()[0].Card(now).Details, "prs      #1181 mCedar/mullet (merged)")
		})
	})

	t.Run("when there were API errors", func(t *testing.T) {
		t.Run("it should count them with the last time", func(t *testing.T) {
			assert.Contains(t, card.Details, "errors   1 api, last "+now.Add(-10*time.Minute).Local().Format("15:04"))
		})
	})

	t.Run("when a hook left a note", func(t *testing.T) {
		t.Run("it should show it", func(t *testing.T) {
			assert.Contains(t, card.Details, "note     Claude needs your permission to use Bash")
		})
	})
}

func TestDistrictShape(t *testing.T) {

	t.Run("when a district holds fifty buildings", func(t *testing.T) {
		c := build(t, city.NewLayout(), sessionsIn(cinders, 50)...)
		d := c.Districts[0]

		t.Run("it should be roughly square rather than a tower", func(t *testing.T) {
			ratio := d.Rect.Height() / d.Rect.Width()
			assert.Less(t, ratio, 1.5)
			assert.Greater(t, ratio, 0.5)
		})

		t.Run("it should still keep every building inside", func(t *testing.T) {
			for _, b := range d.Buildings {
				assert.True(t, d.Rect.Contains(b.Rect.Max), b.Session.ID)
			}
		})

		t.Run("it should not overlap any two buildings", func(t *testing.T) {
			for i, a := range d.Buildings {
				for _, b := range d.Buildings[i+1:] {
					assert.False(t, a.Rect.Overlaps(b.Rect), a.Session.ID+" overlaps "+b.Session.ID)
				}
			}
		})
	})

	t.Run("when a district holds three buildings", func(t *testing.T) {
		c := build(t, city.NewLayout(), sessionsIn(cinders, 3)...)

		t.Run("it should stay one row of three", func(t *testing.T) {
			for _, b := range c.Districts[0].Buildings {
				assert.Equal(t, c.Districts[0].Buildings[0].Rect.Min.Y, b.Rect.Min.Y, b.Session.ID)
			}
		})
	})
}

func sessionsIn(root string, n int) []state.Session {
	var out []state.Session
	for i := 0; i < n; i++ {
		out = append(out, session(fmt.Sprintf("%s-%02d", filepath.Base(root), i), root, state.Working))
	}
	return out
}

func restingIn(cwd string, n int) []state.Session {
	var out []state.Session
	for i := 0; i < n; i++ {
		out = append(out, session(fmt.Sprintf("%s-r%02d", filepath.Base(cwd), i), cwd, state.Resting))
	}
	return out
}

func buildingIn(t *testing.T, c *city.City, id string) *city.Building {
	t.Helper()
	for _, b := range c.Buildings() {
		if b.Session.ID == id {
			return b
		}
	}
	t.Fatalf("session %s is not in the city", id)
	return nil
}

func quietFor(id string, quiet time.Duration) state.Session {
	s := session(id, cinders, state.Resting)
	s.LastActivity = now.Add(-quiet)
	return s
}

func TestLadder(t *testing.T) {
	day := 24 * time.Hour

	t.Run("when a project holds only resting sessions", func(t *testing.T) {
		c := build(t, city.NewLayout(), restingIn(cinders, 3)...)

		t.Run("it should keep its district on the map", func(t *testing.T) {
			require.Len(t, c.Districts, 1)
			assert.Equal(t, cinders, c.Districts[0].Root)
		})

		t.Run("it should keep every session on a lot in it", func(t *testing.T) {
			assert.Len(t, c.Districts[0].Buildings, 3)
		})
	})

	t.Run("when a project has live and resting sessions", func(t *testing.T) {
		c := build(t, city.NewLayout(), append(sessionsIn(cinders, 2), restingIn(cinders, 5)...)...)

		t.Run("it should build no other district for the resting ones", func(t *testing.T) {
			assert.Len(t, c.Districts, 1)
		})

		t.Run("it should give each a full-size lot", func(t *testing.T) {
			for _, b := range c.Buildings() {
				assert.InDelta(t, city.BuildingSize, b.Rect.Width(), 1e-9, b.Session.ID)
			}
		})
	})

	t.Run("when a live session stops", func(t *testing.T) {
		layout := city.NewLayout()
		a, b := session("a", cinders, state.Working), session("b", cinders, state.Working)
		before := buildingIn(t, build(t, layout, a, b), "a").Rect
		a.State = state.Resting
		c := build(t, layout, a, b)

		t.Run("it should keep its lot", func(t *testing.T) {
			assert.Equal(t, before, buildingIn(t, c, "a").Rect)
		})

		t.Run("it should stay in its own district", func(t *testing.T) {
			assert.Equal(t, cinders, c.DistrictOf(buildingIn(t, c, "a")).Root)
		})

		t.Run("and a new session arrives", func(t *testing.T) {
			c := build(t, layout, a, b, session("c", cinders, state.Working))

			t.Run("it should not take the stopped session's lot", func(t *testing.T) {
				assert.NotEqual(t, buildingIn(t, c, "a").Rect, buildingIn(t, c, "c").Rect)
			})
		})
	})

	t.Run("when each rung is built", func(t *testing.T) {
		live := session("live", cinders, state.Working)
		rungs := []struct {
			name                            string
			session                         state.Session
			lit, blinds, boarded, overgrown bool
		}{
			{"live", live, true, false, false, false},
			{"resting", quietFor("resting", day), false, true, false, false},
			{"shuttered", quietFor("shuttered", 10*day), false, true, true, false},
			{"overgrown", quietFor("overgrown", 40*day), false, true, true, true},
		}
		for _, r := range rungs {
			t.Run("and the session is "+r.name, func(t *testing.T) {
				b := build(t, city.NewLayout(), r.session).Buildings()[0]

				t.Run(fmt.Sprintf("it should be lit: %v", r.lit), func(t *testing.T) {
					assert.Equal(t, r.lit, b.Lit)
				})

				t.Run(fmt.Sprintf("it should have its blinds down: %v", r.blinds), func(t *testing.T) {
					assert.Equal(t, r.blinds, b.Blinds)
				})

				t.Run(fmt.Sprintf("it should be boarded up: %v", r.boarded), func(t *testing.T) {
					assert.Equal(t, r.boarded, b.BoardedUp)
				})

				t.Run(fmt.Sprintf("it should be overgrown: %v", r.overgrown), func(t *testing.T) {
					assert.Equal(t, r.overgrown, b.Overgrown)
				})

				t.Run("it should carry its rung", func(t *testing.T) {
					assert.Equal(t, state.Rung(r.name), b.Rung)
				})
			})
		}
	})

	t.Run("when the thresholds are not the defaults", func(t *testing.T) {
		c := city.BuildWith(snapshot(quietFor("a", 4*day)), city.NewLayout(), state.Thresholds{Shuttered: 3 * day, Overgrown: 10 * day})

		t.Run("it should climb by them", func(t *testing.T) {
			assert.Equal(t, state.RungShuttered, c.Buildings()[0].Rung)
		})
	})

	t.Run("when a stopped session's card is read", func(t *testing.T) {
		cases := []struct {
			name, state, quiet string
			for_               time.Duration
		}{
			{"and it has been quiet for hours", "state    resting", "quiet    for 5 hours", 5 * time.Hour},
			{"and it has been quiet for days", "state    shuttered", "quiet    for 12 days", 12 * day},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				lines := build(t, city.NewLayout(), quietFor("a", c.for_)).Buildings()[0].Card(now).Details

				t.Run("it should name its rung", func(t *testing.T) {
					assert.Contains(t, lines, c.state)
				})

				t.Run("it should say how long it has been quiet", func(t *testing.T) {
					assert.Contains(t, lines, c.quiet)
				})
			})
		}
	})

	t.Run("when a live session's card is read", func(t *testing.T) {
		lines := build(t, city.NewLayout(), session("a", cinders, state.Working)).Buildings()[0].Card(now).Details

		t.Run("it should not say it is quiet", func(t *testing.T) {
			for _, l := range lines {
				assert.NotContains(t, l, "quiet")
			}
		})
	})

	t.Run("when a session's conversation is gone", func(t *testing.T) {
		s := quietFor("a", 45*day)
		s.ConversationGone = true
		lines := build(t, city.NewLayout(), s).Buildings()[0].Card(now).Details

		t.Run("it should say Claude Code deleted it, naming the setting", func(t *testing.T) {
			assert.Contains(t, lines, "gone     Claude Code deleted the conversation (cleanupPeriodDays)")
		})
	})
}

func TestDistanceToSegment(t *testing.T) {
	a, b := city.Point{X: 0, Y: 0}, city.Point{X: 10, Y: 0}

	t.Run("when the point is beside the segment", func(t *testing.T) {
		t.Run("it should measure the perpendicular", func(t *testing.T) {
			assert.InDelta(t, 3, city.Point{X: 5, Y: 3}.DistanceToSegment(a, b), 1e-9)
		})
	})

	t.Run("when the point is past an end", func(t *testing.T) {
		t.Run("it should measure to that end", func(t *testing.T) {
			assert.InDelta(t, 5, city.Point{X: 13, Y: 4}.DistanceToSegment(a, b), 1e-9)
		})
	})

	t.Run("when the segment has no length", func(t *testing.T) {
		t.Run("it should measure to the point", func(t *testing.T) {
			assert.InDelta(t, 5, city.Point{X: 3, Y: 4}.DistanceToSegment(a, a), 1e-9)
		})
	})
}

func TestTrains(t *testing.T) {
	t.Run("when the store priced two models over the last day", func(t *testing.T) {
		c := city.BuildWithLedger(snapshot(spender("a", cinders, "claude-opus-5", map[int]int64{1: 300})), city.NewLayout(), &fakeLedger{window: stored})
		require.Len(t, c.Trains, 2)

		t.Run("it should run one train per model, the dearest first", func(t *testing.T) {
			assert.Equal(t, []string{"claude-opus-5-5", "claude-haiku-4-5"}, []string{c.Trains[0].Model, c.Trains[1].Model})
		})

		t.Run("it should size the wagon in dollars to keep the dearest train to six", func(t *testing.T) {
			assert.Equal(t, 0.5, c.Trains[0].Unit)
		})

		t.Run("it should give the dearest train a wagon per unit", func(t *testing.T) {
			assert.Equal(t, 4, c.Trains[0].Wagons)
		})

		t.Run("it should give the small train a wagon per unit, never none", func(t *testing.T) {
			assert.Equal(t, 1, c.Trains[1].Wagons)
		})

		card := c.Trains[0].Card()

		t.Run("it should title the card with the model", func(t *testing.T) {
			assert.Equal(t, "claude-opus-5-5", card.Title)
		})

		t.Run("it should give the cost and messages in the one wording", func(t *testing.T) {
			assert.Contains(t, card.Lines, stats.Headline(cost(2), 3)+" in 24 h")
		})

		t.Run("it should give the counters apart", func(t *testing.T) {
			assert.Contains(t, card.Lines, stats.CountersLine(stats.Counters{Output: 30, Messages: 3}))
		})

		t.Run("it should say what a wagon stands for", func(t *testing.T) {
			assert.Contains(t, card.Lines, "wagons   4, one per $0.50")
		})

		t.Run("it should lay the rails as a closed loop", func(t *testing.T) {
			require.GreaterOrEqual(t, len(c.Rails), 4)
			assert.Equal(t, c.Rails[0], c.Rails[len(c.Rails)-1])
		})
	})

	t.Run("when statistics cannot be read", func(t *testing.T) {
		c := city.BuildWithLedger(snapshot(spender("a", cinders, "claude-opus-5", map[int]int64{1: 300})), city.NewLayout(), &fakeLedger{reason: "the store is being upgraded"})

		t.Run("it should run no trains, since nothing says which model spent what", func(t *testing.T) {
			assert.Empty(t, c.Trains)
		})
	})

	t.Run("when statistics are off, it should run no trains", func(t *testing.T) {
		assert.Empty(t, build(t, city.NewLayout(), spender("a", cinders, "claude-opus-5", map[int]int64{1: 300})).Trains)
	})

	t.Run("when a wagon unit is chosen", func(t *testing.T) {
		cases := []struct {
			max, unit float64
		}{{0, 0.01}, {0.06, 0.01}, {0.061, 0.02}, {2, 0.5}, {300, 50}, {12, 2}}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("it should keep $%g to six wagons of $%g", tc.max, tc.unit), func(t *testing.T) {
				assert.InDelta(t, tc.unit, city.TrainUnit(tc.max), 1e-12)
			})
		}
	})
}
func TestTrees(t *testing.T) {
	t.Run("when the city plants the plan's trees", func(t *testing.T) {
		c := build(t, city.NewLayout(), session("a", cinders, state.Working))
		require.NotEmpty(t, c.Trees)

		t.Run("it should set each tree at its cell's centre plus its offset", func(t *testing.T) {
			tree := c.Trees[0]
			centre := tree.Cell.Center()
			assert.NotEqual(t, centre, tree.At)
			assert.LessOrEqual(t, math.Abs(tree.At.X-centre.X), 0.5*city.CellSize+1e-9)
			assert.LessOrEqual(t, math.Abs(tree.At.Y-centre.Y), 0.5*city.CellSize+1e-9)
		})
	})
}

func TestUnhookedSession(t *testing.T) {
	t.Run("when a session has sent no hook events", func(t *testing.T) {
		quiet := session("a", cinders, state.Working)
		quiet.Hooked = false
		c := build(t, city.NewLayout(), quiet, session("b", cinders, state.Working))

		t.Run("it should get no power line", func(t *testing.T) {
			lines := c.PowerLines()
			require.Len(t, lines, 1)
			assert.Equal(t, "b", lines[0].Building.Session.ID)
		})

		t.Run("its card should say the daemon is reading files", func(t *testing.T) {
			var card city.Card
			for _, b := range c.Buildings() {
				if b.Session.ID == "a" {
					card = b.Card(now)
				}
			}
			assert.Contains(t, card.Details, "telemetry files only; no hook events seen (no wire)")
		})

		t.Run("a hooked session's card should say so", func(t *testing.T) {
			var card city.Card
			for _, b := range c.Buildings() {
				if b.Session.ID == "b" {
					card = b.Card(now)
				}
			}
			assert.Contains(t, card.Details, "telemetry hooks")
		})
	})
}
