package city_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 89: a pull request opening hoists a flag and parks an inspector
// outside; merging it sends a crate away on a truck over the bridge, and
// the merge that leaves a group with nothing open cuts its ribbon, once.
func TestPullRequests(t *testing.T) {
	start := time.Date(2026, 10, 17, 15, 0, 0, 0, time.UTC)
	pr := func(n int, st string) claude.PR {
		return claude.PR{Number: n, URL: fmt.Sprintf("https://github.com/o/r/pull/%d", n), Repository: "o/r", State: st}
	}
	with := func(id, cwd, group string, prs ...claude.PR) state.Session {
		s := session(id, cwd, state.Working)
		s.SubagentsInFlight, s.Group, s.PRs = 0, group, prs
		return s
	}
	type scene struct {
		s     *city.Scene
		clock *time.Time
	}
	// open builds a scene that has seen sessions before, then shows them.
	open := func(t *testing.T, level city.Liveliness, before, after []state.Session) scene {
		t.Helper()
		clock := start
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return clock }, time.UTC)
		s.SetReducedMotion(level.CrossFades())
		s.SetSnapshot(snapshot(before...))
		s.SetSnapshot(snapshot(after...))
		return scene{s: s, clock: &clock}
	}
	at := func(sc scene, d time.Duration) { *sc.clock = start.Add(d) }

	t.Run("when a PR opens", func(t *testing.T) {
		sc := open(t, city.Lively, []state.Session{with("a", cinders, "")}, []state.Session{with("a", cinders, "", pr(1, claude.PROpen))})
		at(sc, city.FlagFor/2)

		t.Run("it should hoist a flag over FlagFor", func(t *testing.T) {
			assert.InDelta(t, 0.5, sc.s.Hoist("a", 1), 1e-9)
		})

		t.Run("it should park an inspector's car outside, arriving over ParkFor", func(t *testing.T) {
			at(sc, city.ParkFor)
			inspector, ok := sc.s.Inspector("a")
			require.True(t, ok)
			assert.Equal(t, 1.0, inspector.Parked)
		})
	})

	t.Run("while the session has any open PR", func(t *testing.T) {
		sc := open(t, city.Lively,
			[]state.Session{with("a", cinders, "", pr(1, claude.PROpen), pr(2, claude.PROpen))},
			[]state.Session{with("a", cinders, "", pr(1, claude.PRMerged), pr(2, claude.PROpen))})
		at(sc, time.Minute)
		inspector, ok := sc.s.Inspector("a")

		t.Run("it should keep the inspector parked", func(t *testing.T) {
			assert.Equal(t, [2]any{true, 1.0}, [2]any{ok, inspector.Parked})
		})
	})

	for _, end := range []string{claude.PRMerged, claude.PRClosed} {
		t.Run("when the last open PR is "+end, func(t *testing.T) {
			sc := open(t, city.Lively, []state.Session{with("a", cinders, "", pr(1, claude.PROpen))}, []state.Session{with("a", cinders, "", pr(1, end))})
			at(sc, city.ParkFor/2)
			inspector, ok := sc.s.Inspector("a")

			t.Run("the inspector should drive off", func(t *testing.T) {
				assert.Equal(t, [2]any{true, true}, [2]any{ok, inspector.Leaving})
			})

			t.Run("and once it has left, it should be gone", func(t *testing.T) {
				at(sc, city.ParkFor)
				_, ok := sc.s.Inspector("a")
				assert.False(t, ok)
			})
		})
	}

	t.Run("when a PR merges", func(t *testing.T) {
		sc := open(t, city.Lively,
			[]state.Session{with("a", cinders, "", pr(1, claude.PROpen), pr(2, claude.PROpen))},
			[]state.Session{with("a", cinders, "", pr(1, claude.PRMerged), pr(2, claude.PROpen))})
		trucks := sc.s.Deliveries()

		t.Run("it should load a crate onto a delivery truck", func(t *testing.T) {
			require.Len(t, trucks, 1)
			assert.Equal(t, [2]any{"a", 1}, [2]any{trucks[0].SessionID, trucks[0].PR.Number})
		})

		t.Run("the truck's path should end on the bridge's far side", func(t *testing.T) {
			require.Len(t, trucks, 1)
			path := trucks[0].Path
			require.NotEmpty(t, path)
			assert.Equal(t, sc.s.City().Bridge().Far, path[len(path)-1])
		})

		t.Run("the truck should leave its lot by its driveway", func(t *testing.T) {
			require.Len(t, trucks, 1)
			c := sc.s.City()
			dw, ok := c.Driveway(c.Districts[0])
			assert.True(t, ok && byDriveway(trucks[0].Path, dw) >= 0, "path %v, driveway %+v", trucks[0].Path, dw)
		})

		t.Run("the truck's path should never cross the river band", func(t *testing.T) {
			require.Len(t, trucks, 1)
			from, _, width, ok := sc.s.City().RiverBand()
			require.True(t, ok)
			var wet int
			for _, p := range trucks[0].Path {
				if p.X > from.X-width/2 && p.X < from.X+width/2 {
					wet++
				}
			}
			assert.Zero(t, wet)
		})

		t.Run("and another PR in the group is still open", func(t *testing.T) {
			t.Run("it should cut no ribbon", func(t *testing.T) {
				assert.Empty(t, sc.s.Ribbons())
			})
		})
	})

	for _, c := range []struct {
		name          string
		before, after []state.Session
	}{
		{"a group of three",
			[]state.Session{with("a", cinders, "g", pr(1, claude.PROpen)), with("b", cinders, "g", pr(2, claude.PRMerged)), with("c", mullet, "g")},
			[]state.Session{with("a", cinders, "g", pr(1, claude.PRMerged)), with("b", cinders, "g", pr(2, claude.PRMerged)), with("c", mullet, "g")}},
		{"a group of one",
			[]state.Session{with("a", cinders, "g", pr(1, claude.PROpen))},
			[]state.Session{with("a", cinders, "g", pr(1, claude.PRMerged))}},
		{"a session with no group",
			[]state.Session{with("a", cinders, "", pr(1, claude.PROpen))},
			[]state.Session{with("a", cinders, "", pr(1, claude.PRMerged))}},
	} {
		t.Run("when a merge is the last open PR of "+c.name, func(t *testing.T) {
			sc := open(t, city.Lively, c.before, c.after)

			t.Run("it should cut the ribbon, once", func(t *testing.T) {
				assert.Len(t, sc.s.Ribbons(), 1)
			})

			t.Run("and a later snapshot shows the same", func(t *testing.T) {
				at(sc, city.RibbonFor+time.Second)
				sc.s.SetSnapshot(snapshot(c.after...))

				t.Run("it should not cut it again", func(t *testing.T) {
					assert.Empty(t, sc.s.Ribbons())
				})
			})
		})
	}

	t.Run("when a PR is closed without merging", func(t *testing.T) {
		sc := open(t, city.Lively, []state.Session{with("a", cinders, "", pr(1, claude.PROpen))}, []state.Session{with("a", cinders, "", pr(1, claude.PRClosed))})
		at(sc, city.FlagFor/2)

		t.Run("the flag should come down", func(t *testing.T) {
			assert.InDelta(t, 0.5, sc.s.Hoist("a", 1), 1e-9)
		})

		t.Run("it should send no truck", func(t *testing.T) {
			assert.Empty(t, sc.s.Deliveries())
		})

		t.Run("it should cut no ribbon", func(t *testing.T) {
			assert.Empty(t, sc.s.Ribbons())
		})
	})

	t.Run("when the level is still", func(t *testing.T) {
		t.Run("and a PR opens", func(t *testing.T) {
			sc := open(t, city.Still, []state.Session{with("a", cinders, "")}, []state.Session{with("a", cinders, "", pr(1, claude.PROpen))})

			t.Run("the flag should stand at once", func(t *testing.T) {
				assert.Equal(t, 1.0, sc.s.Hoist("a", 1))
			})

			t.Run("the inspector should fade in over the short fade", func(t *testing.T) {
				at(sc, city.PRFadeFor/2)
				inspector, ok := sc.s.Inspector("a")
				require.True(t, ok)
				assert.InDelta(t, 0.5, inspector.Fade, 1e-9)
			})
		})

		t.Run("and a merge cuts a ribbon", func(t *testing.T) {
			sc := open(t, city.Still, []state.Session{with("a", cinders, "", pr(1, claude.PROpen))}, []state.Session{with("a", cinders, "", pr(1, claude.PRMerged))})
			ribbons := sc.s.Ribbons()

			t.Run("it should be a cross-fade with no fireworks", func(t *testing.T) {
				require.Len(t, ribbons, 1)
				assert.False(t, ribbons[0].Fireworks)
			})

			t.Run("its truck should fade rather than drive", func(t *testing.T) {
				trucks := sc.s.Deliveries()
				require.Len(t, trucks, 1)
				assert.True(t, trucks[0].Fades)
			})
		})
	})

	for _, c := range []struct {
		name  string
		after time.Duration
	}{{"a flag rises", city.FlagFor / 2}, {"a truck drives", city.FlagFor + time.Second}} {
		t.Run("while "+c.name, func(t *testing.T) {
			sc := open(t, city.Lively,
				[]state.Session{with("a", cinders, "", pr(1, claude.PROpen), pr(2, claude.PROpen))},
				[]state.Session{with("a", cinders, "", pr(1, claude.PRMerged), pr(2, claude.PROpen), pr(3, claude.PROpen))})
			at(sc, c.after)

			t.Run("the scene should be animating", func(t *testing.T) {
				assert.True(t, sc.s.Animating())
			})
		})
	}
}

// Item 89: one bridge leaves the island, where the streets reach the
// coast, and it is always there.
func TestBridge(t *testing.T) {
	c := build(t, city.NewLayout(), session("a", cinders, state.Working), session("b", mullet, state.Resting))
	bridge := c.Bridge()
	island := c.Island()

	t.Run("it should start on land and end out at sea", func(t *testing.T) {
		land, _ := island.At(bridge.Land)
		sea, _ := island.At(bridge.Far)
		assert.Equal(t, [2]city.Ground{city.GroundLand, city.GroundSea}, [2]city.Ground{land, sea})
	})

	t.Run("it should not stand on the river", func(t *testing.T) {
		from, _, width, ok := c.RiverBand()
		require.True(t, ok)
		west, east := from.X-width/2, from.X+width/2
		crosses := (bridge.Land.X-west)*(bridge.Far.X-west) <= 0 || (bridge.Land.X-east)*(bridge.Far.X-east) <= 0 ||
			(bridge.Land.X > west && bridge.Land.X < east)
		assert.False(t, crosses)
	})

	t.Run("it should start on a street", func(t *testing.T) {
		assert.True(t, c.IsStreet(city.CellOf(bridge.Land)))
	})
}
