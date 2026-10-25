package city_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
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

	// Item 217: from the landing out, a truck drives the deck as it is
	// drawn now, whatever the city did since the merge.
	offDeck := func(path []city.Point, b city.Bridge) []city.Point {
		var off []city.Point
		out := math.Copysign(1, b.Far.X-b.Land.X)
		for _, p := range path {
			if (p.X-b.Land.X)*out > 0 && math.Abs(p.Y-b.Land.Y) > city.CellSize/2 {
				off = append(off, p)
			}
		}
		return off
	}

	for _, root := range []string{cinders, mullet, botropolis} {
		t.Run("when a PR merges in the district "+filepath.Base(root), func(t *testing.T) {
			others := []state.Session{with("x", cinders, ""), with("y", mullet, ""), with("z", botropolis, "")}
			sc := open(t, city.Lively,
				append([]state.Session{with("a", root, "", pr(1, claude.PROpen))}, others...),
				append([]state.Session{with("a", root, "", pr(1, claude.PRMerged))}, others...))
			trucks := sc.s.Deliveries()

			t.Run("it should keep every point of the truck's path from the landing on within the deck's width", func(t *testing.T) {
				require.Len(t, trucks, 1)
				assert.Empty(t, offDeck(trucks[0].Path, sc.s.City().Bridge()))
			})
		})
	}

	t.Run("when the island grows a row while a truck is on its way, so the bridge moves", func(t *testing.T) {
		with4 := func(n int, prs ...claude.PR) []state.Session {
			out := []state.Session{with("a", mullet, "", prs...)}
			for i := 0; i < n; i++ {
				out = append(out, with(fmt.Sprintf("c%d", i), cinders, ""))
			}
			return out
		}
		grow := func(t *testing.T, until func(sc scene, at city.Point, b city.Bridge) bool) (scene, city.Point, city.Bridge) {
			t.Helper()
			sc := open(t, city.Lively, with4(3, pr(1, claude.PROpen)), with4(3, pr(1, claude.PRMerged)))
			was := sc.s.City().Bridge()
			var here city.Point
			for d := time.Duration(0); ; d += 50 * time.Millisecond {
				at(sc, d)
				trucks := sc.s.Deliveries()
				require.Len(t, trucks, 1)
				here = trucks[0].At(start.Add(d))
				if until(sc, here, was) {
					break
				}
			}
			sc.s.SetSnapshot(snapshot(with4(4, pr(1, claude.PRMerged))...))
			require.NotEqual(t, was.Land, sc.s.City().Bridge().Land, "the fixture should move the bridge")
			return sc, here, was
		}

		t.Run("and the truck has not reached the landing", func(t *testing.T) {
			sc, here, _ := grow(t, func(_ scene, p city.Point, b city.Bridge) bool {
				return p.Y <= b.Land.Y-2*city.CellSize && p.X <= b.Land.X+2*city.CellSize
			})
			trucks := sc.s.Deliveries()
			now := sc.s.City().Bridge()

			t.Run("it should carry on from where it was", func(t *testing.T) {
				require.Len(t, trucks, 1)
				assertPathNear(t, []city.Point{here}, trucks[0].Path[:1])
			})

			t.Run("it should end its path on the bridge as it now stands", func(t *testing.T) {
				require.Len(t, trucks, 1)
				path := trucks[0].Path
				assert.Equal(t, []city.Point{now.Land, now.Far}, path[len(path)-2:])
			})

			t.Run("it should keep every point of its path from the landing on within the deck's width", func(t *testing.T) {
				require.Len(t, trucks, 1)
				assert.Empty(t, offDeck(trucks[0].Path, now))
			})
		})

		t.Run("and the truck is already on the deck", func(t *testing.T) {
			sc, here, _ := grow(t, func(_ scene, p city.Point, b city.Bridge) bool {
				return p.Y == b.Land.Y && math.Abs(p.X-b.Land.X) > 2*city.CellSize
			})
			trucks := sc.s.Deliveries()
			now := sc.s.City().Bridge()

			t.Run("it should carry on along the deck as it now stands, as far out as it was", func(t *testing.T) {
				require.Len(t, trucks, 1)
				assertPathNear(t, []city.Point{{X: here.X, Y: now.Land.Y}, now.Far}, trucks[0].Path)
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

// Item 218: the bridge is placed once and kept, while the island grows
// and across a restart, and moves only when its row stops being a street.
func TestBridgeStays(t *testing.T) {
	cityOf := func(layout *city.Layout, sessions, packed int) *city.City {
		all := sessionsIn(cinders, sessions)
		all = append(all, session("m", mullet, state.Working))
		for i := 0; i < packed; i++ {
			s := session(fmt.Sprintf("p%02d", i), fmt.Sprintf("/home/avesta/packed/p%02d", i), state.Resting)
			s.Packed = true
			all = append(all, s)
		}
		return city.Build(snapshot(all...), layout)
	}
	landing := func(c *city.City) city.Point { return c.Bridge().Land }
	fromPlaza := func(c *city.City) float64 { return landing(c).Y - c.Plaza.Min.Y }

	t.Run("when the island grows rows south of the bridge, as a depot arrives", func(t *testing.T) {
		layout := city.NewLayout()
		was := cityOf(layout, 1, 0)
		now := cityOf(layout, 1, 12)
		require.NotEqual(t, landing(was).Y, landing(cityOf(city.NewLayout(), 1, 12)).Y, "the middle rule should move it")

		t.Run("it should keep the landing where it was", func(t *testing.T) {
			assert.Equal(t, landing(was).Y, landing(now).Y)
		})
	})

	t.Run("when the island grows rows north of the bridge, as a district grows", func(t *testing.T) {
		layout := city.NewLayout()
		was := cityOf(layout, 1, 0)
		now := cityOf(layout, 8, 0)
		require.NotEqual(t, fromPlaza(was), fromPlaza(cityOf(city.NewLayout(), 8, 0)), "the middle rule should move it")

		t.Run("it should keep the landing the same distance from the plaza", func(t *testing.T) {
			assert.Equal(t, fromPlaza(was), fromPlaza(now))
		})
	})

	t.Run("when the bridge's row stops being a street, as the depot it stood beside goes", func(t *testing.T) {
		layout := city.NewLayout()
		cityOf(layout, 1, 30)
		now := cityOf(layout, 1, 0)
		col := city.CellOf(landing(now)).Col
		southmost := -1
		for row := 0; float64(row)*city.CellSize < now.Bounds().Max.Y; row++ {
			if now.IsStreet(city.Cell{Col: col, Row: row}) {
				southmost = row
			}
		}
		require.NotEqual(t, landing(now), landing(cityOf(city.NewLayout(), 1, 0)), "the nearest street row should not be the middle")

		t.Run("it should land on the coastal street row nearest where it was", func(t *testing.T) {
			assert.Equal(t, southmost, city.CellOf(landing(now)).Row)
		})

		t.Run("it should start on a street", func(t *testing.T) {
			assert.True(t, now.IsStreet(city.CellOf(landing(now))))
		})
	})

	t.Run("when the layout is saved and loaded again, as on a restart", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "layout.json")
		layout := city.NewLayout()
		cityOf(layout, 1, 0)
		before := cityOf(layout, 1, 12)
		require.NoError(t, layout.Save(path))
		loaded, err := city.LoadLayout(path)
		require.NoError(t, err)
		require.NotEqual(t, landing(before), landing(cityOf(city.NewLayout(), 1, 12)), "the middle rule should put it elsewhere")

		t.Run("it should place the bridge where it was before the restart", func(t *testing.T) {
			assert.Equal(t, landing(before), landing(cityOf(loaded, 1, 12)))
		})
	})

	t.Run("when a layout from before the bridge mark is loaded", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "layout.json")
		layout := city.NewLayout()
		cityOf(layout, 1, 0)
		require.NoError(t, layout.Save(path))
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		var file map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &file))
		require.Contains(t, file, "bridge", "the fixture should have a mark to take out")
		delete(file, "bridge")
		data, err = json.Marshal(file)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, data, 0o600))
		older, err := city.LoadLayout(path)
		require.NoError(t, err)

		t.Run("it should place the bridge by the middle rule", func(t *testing.T) {
			assert.Equal(t, landing(cityOf(city.NewLayout(), 1, 12)), landing(cityOf(older, 1, 12)))
		})
	})
}

func assertPathNear(t *testing.T, want, got []city.Point) {
	t.Helper()
	require.Len(t, got, len(want))
	for i := range want {
		assert.InDeltaf(t, want[i].X, got[i].X, 1e-9, "point %d's X", i)
		assert.InDeltaf(t, want[i].Y, got[i].Y, 1e-9, "point %d's Y", i)
	}
}

// Item 220: once the island holds 16 districts a second bridge leaves by
// the coast opposite the first (Aria, 2026-10-25), anchored as the first
// is, and each truck takes whichever bridge is nearer its building.
func TestSecondBridge(t *testing.T) {
	roots := func(n int) []string {
		var out []string
		for i := 0; i < n; i++ {
			out = append(out, fmt.Sprintf("/home/avesta/projects/d%02d", i))
		}
		return out
	}
	sessionsOf := func(n, grown int) []state.Session {
		var all []state.Session
		for i, root := range roots(n) {
			all = append(all, session(fmt.Sprintf("s%02d", i), root, state.Working))
		}
		for i := 0; i < grown; i++ {
			all = append(all, session(fmt.Sprintf("g%02d", i), roots(1)[0], state.Working))
		}
		return all
	}
	cityOf := func(layout *city.Layout, n, grown int) *city.City {
		return city.Build(snapshot(sessionsOf(n, grown)...), layout)
	}
	second := func(c *city.City) city.Bridge { return c.Bridges()[1] }

	t.Run("when the island holds 15 districts", func(t *testing.T) {
		c := cityOf(city.NewLayout(), 15, 0)

		t.Run("it should have one bridge", func(t *testing.T) {
			assert.Len(t, c.Bridges(), 1)
		})
	})

	t.Run("when the island holds 16 districts", func(t *testing.T) {
		c := cityOf(city.NewLayout(), 16, 0)
		bridges := c.Bridges()
		require.Len(t, bridges, 2)
		first, other := bridges[0], bridges[1]

		t.Run("it should have two bridges", func(t *testing.T) {
			assert.Len(t, bridges, 2)
		})

		t.Run("the second should leave by the coast opposite the first", func(t *testing.T) {
			assert.Equal(t, first.Far.X < first.Land.X, other.Far.X > other.Land.X)
		})

		t.Run("the second should start on a street", func(t *testing.T) {
			assert.True(t, c.IsStreet(city.CellOf(other.Land)))
		})

		t.Run("the second should start on land and end out at sea", func(t *testing.T) {
			land, _ := c.Island().At(other.Land)
			sea, _ := c.Island().At(other.Far)
			assert.Equal(t, [2]city.Ground{city.GroundLand, city.GroundSea}, [2]city.Ground{land, sea})
		})
	})

	for i, name := range []string{"first", "second"} {
		t.Run("when a PR merges in a district nearer the "+name+" bridge", func(t *testing.T) {
			pr := func(st string) claude.PR {
				return claude.PR{Number: 1, URL: "https://github.com/o/r/pull/1", Repository: "o/r", State: st}
			}
			clock := now
			s := city.NewScene(city.NewLayout())
			s.Resize(800, 600)
			s.SetClock(func() time.Time { return clock }, time.UTC)
			plain := sessionsOf(16, 0)
			s.SetSnapshot(snapshot(plain...))
			bridges := s.City().Bridges()
			require.Len(t, bridges, 2)
			nearest, best := -1, math.Inf(1)
			for j, d := range s.City().Districts {
				if d.Depot {
					continue
				}
				c := d.Rect.Center()
				lead := math.Hypot(c.X-bridges[i].Land.X, c.Y-bridges[i].Land.Y) - math.Hypot(c.X-bridges[1-i].Land.X, c.Y-bridges[1-i].Land.Y)
				if lead < best {
					nearest, best = j, lead
				}
			}
			root := s.City().Districts[nearest].Root
			withPR := func(st string) []state.Session {
				out := append([]state.Session(nil), plain...)
				for k := range out {
					if out[k].CWD == root {
						out[k].SubagentsInFlight, out[k].PRs = 0, []claude.PR{pr(st)}
						break
					}
				}
				return out
			}
			s.SetSnapshot(snapshot(withPR(claude.PROpen)...))
			s.SetSnapshot(snapshot(withPR(claude.PRMerged)...))
			trucks := s.Deliveries()
			require.Len(t, trucks, 1)

			t.Run("the truck's path should end on the "+name+" bridge's far side", func(t *testing.T) {
				path := trucks[0].Path
				assert.Equal(t, s.City().Bridges()[i].Far, path[len(path)-1])
			})
		})
	}

	t.Run("when the island grows rows, as a district grows", func(t *testing.T) {
		layout := city.NewLayout()
		was := cityOf(layout, 16, 0)
		now := cityOf(layout, 16, 12)
		fromPlaza := func(c *city.City) float64 { return second(c).Land.Y - c.Plaza.Min.Y }
		require.NotEqual(t, fromPlaza(was), fromPlaza(cityOf(city.NewLayout(), 16, 12)), "the middle rule should move it")

		t.Run("it should keep the second landing the same distance from the plaza", func(t *testing.T) {
			assert.Equal(t, fromPlaza(was), fromPlaza(now))
		})
	})

	t.Run("when the layout is saved and loaded again, as on a restart", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "layout.json")
		layout := city.NewLayout()
		cityOf(layout, 16, 0)
		before := cityOf(layout, 16, 12)
		require.NoError(t, layout.Save(path))
		loaded, err := city.LoadLayout(path)
		require.NoError(t, err)
		require.NotEqual(t, second(before).Land, second(cityOf(city.NewLayout(), 16, 12)).Land, "the middle rule should put it elsewhere")

		t.Run("it should place the second bridge where it was", func(t *testing.T) {
			assert.Equal(t, second(before).Land, second(cityOf(loaded, 16, 12)).Land)
		})
	})

	// Aria, 2026-10-25: the east rail leg moves out one cell, toward the
	// river, only when the second bridge exists, so it has one ramp cell
	// as the first does, and every other city's east edge stays.
	eastRailOff := func(c *city.City) int {
		rail := math.Inf(-1)
		for _, p := range c.Rails {
			rail = math.Max(rail, p.X)
		}
		col, road := city.CellOf(city.Point{X: rail, Y: 0}).Col, -1
		for cl := 0; cl < col; cl++ {
			for row := 0; float64(row)*city.CellSize < c.Bounds().Max.Y; row++ {
				if c.IsStreet(city.Cell{Col: cl, Row: row}) {
					road = cl
				}
			}
		}
		return col - road
	}
	for _, c := range []struct {
		districts, off int
		where          string
	}{{15, 1, "beside the east road"}, {16, 2, "one cell further out"}} {
		t.Run(fmt.Sprintf("when the island holds %d districts", c.districts), func(t *testing.T) {
			t.Run("it should stand the east rail leg "+c.where, func(t *testing.T) {
				assert.Equal(t, c.off, eastRailOff(cityOf(city.NewLayout(), c.districts, 0)))
			})
		})
	}

	t.Run("when the second bridge leaves over the moved rail", func(t *testing.T) {
		c := cityOf(city.NewLayout(), 16, 0)

		t.Run("it should climb over one ramp cell, as the first does", func(t *testing.T) {
			assert.Len(t, second(c).Ramp, len(c.Bridge().Ramp))
		})
	})

	t.Run("when the count falls back below 16 after the second bridge was placed", func(t *testing.T) {
		layout := city.NewLayout()
		cityOf(layout, 16, 0)
		c := cityOf(layout, 15, 0)

		t.Run("it should keep the second bridge", func(t *testing.T) {
			assert.Len(t, c.Bridges(), 2)
		})
	})
}
