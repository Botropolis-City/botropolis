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

// Items 89 and 255: what runs on a road says which kind of traffic it
// is — a van for each message, a box truck for each file edited across,
// a motorbike for each subagent's visit — and nothing rides a group's
// line.
func TestRoadTraffic(t *testing.T) {
	start := time.Date(2026, 10, 17, 16, 0, 0, 0, time.UTC)
	member := func(id, cwd, group string) state.Session {
		s := session(id, cwd, state.Working)
		s.SubagentsInFlight, s.Group = 0, group
		return s
	}
	loads := []struct {
		load    city.Cargo
		kind    city.Kind
		counted func(r *state.Road, forward, back int)
	}{
		{city.CargoMessage, city.KindVan, func(r *state.Road, f, b int) { r.Messages, r.BackMessages = f, b }},
		{city.CargoFiles, city.KindBoxTruck, func(r *state.Road, f, b int) { r.Files, r.BackFiles = f, b }},
		{city.CargoVisit, city.KindMotorbike, func(r *state.Road, f, b int) { r.Visits, r.BackVisits = f, b }},
	}
	type scene struct {
		s     *city.Scene
		clock *time.Time
	}
	district := func(sc scene, root string) *city.District {
		for _, d := range sc.s.City().Districts {
			if d.Root == root {
				return d
			}
		}
		return nil
	}
	road := func(sc scene) []city.Point {
		for _, st := range sc.s.City().Streets {
			if st.Road.From.Root == cinders && st.Road.To.Root == mullet {
				return st.Path
			}
		}
		return nil
	}

	for _, l := range loads {
		grown := func(forward, back, extra int, group string) state.Snapshot {
			sessions := []state.Session{member("a", cinders, group), member("b", mullet, group)}
			for i := 0; i < extra; i++ {
				sessions = append(sessions, member(fmt.Sprintf("c%d", i), cinders, ""))
			}
			s := snapshot(sessions...)
			r := state.Road{From: cinders, To: mullet, Sessions: 2}
			l.counted(&r, forward, back)
			s.Roads = []state.Road{r}
			if group != "" {
				s.Groups = []state.Group{{ID: group, Short: "G", Members: []string{"a", "b"}}}
			}
			return s
		}
		snap := func(forward, back int, group string) state.Snapshot {
			return grown(forward, back, 0, group)
		}
		open := func(t *testing.T, level city.Liveliness, lines bool, before, after state.Snapshot) scene {
			t.Helper()
			clock := start
			s := city.NewScene(city.NewLayout())
			s.Resize(800, 600)
			s.SetClock(func() time.Time { return clock }, time.UTC)
			s.SetReducedMotion(level.CrossFades())
			s.SetGroupLines(lines)
			s.SetSnapshot(before)
			s.SetSnapshot(after)
			return scene{s: s, clock: &clock}
		}
		name := l.kind.Name()

		t.Run("when a road gains one "+l.kind.Is(), func(t *testing.T) {
			sc := open(t, city.Lively, true, snap(0, 0, ""), snap(1, 0, ""))
			on := sc.s.Traffic()

			t.Run("it should send one "+name+" from the first district to the second", func(t *testing.T) {
				require.Len(t, on, 1)
				assert.Equal(t, [3]any{l.load, cinders, mullet}, [3]any{on[0].Cargo, on[0].From.Root, on[0].To.Root})
			})

			t.Run("its path should be the road's street path, in that direction", func(t *testing.T) {
				require.Len(t, on, 1)
				require.NotEmpty(t, sc.s.City().Streets)
				assert.Equal(t, sc.s.City().Streets[0].Path, on[0].Path)
			})

			t.Run("it should leave by the first district's driveway and arrive by the second's", func(t *testing.T) {
				require.Len(t, on, 1)
				c := sc.s.City()
				from, _ := c.Driveway(district(sc, cinders))
				to, _ := c.Driveway(district(sc, mullet))
				out, in := byDriveway(on[0].Path, from), byDriveway(on[0].Path, to)
				assert.True(t, out >= 0 && in > out, "out at %d, in at %d", out, in)
			})

			t.Run("its hit should name its kind", func(t *testing.T) {
				require.Len(t, on, 1)
				assert.Equal(t, l.kind, city.KindOf(city.Hit{Traffic: &on[0]}))
			})

			t.Run("and its trip is over", func(t *testing.T) {
				require.Len(t, on, 1)
				*sc.clock = start.Add(on[0].For() + time.Millisecond)

				t.Run("it should be gone", func(t *testing.T) {
					assert.Empty(t, sc.s.Traffic())
				})
			})
		})

		t.Run("when a road gains three of "+name+"'s traffic in one snapshot", func(t *testing.T) {
			sc := open(t, city.Lively, true, snap(0, 0, ""), snap(3, 0, ""))
			on := sc.s.Traffic()

			t.Run("it should send three, spaced so they do not overlap", func(t *testing.T) {
				require.Len(t, on, 3)
				assert.Equal(t, [2]time.Duration{city.VanGap, city.VanGap},
					[2]time.Duration{on[1].Since.Sub(on[0].Since), on[2].Since.Sub(on[1].Since)})
			})
		})

		t.Run("when a road gains "+name+"'s traffic the other way", func(t *testing.T) {
			sc := open(t, city.Lively, true, snap(1, 0, ""), snap(1, 1, ""))
			on := sc.s.Traffic()

			t.Run("it should send a "+name+" the other way", func(t *testing.T) {
				require.Len(t, on, 1)
				assert.Equal(t, [2]string{mullet, cinders}, [2]string{on[0].From.Root, on[0].To.Root})
			})
		})

		t.Run("when both ends of a road gaining "+name+"'s traffic are stations on a drawn line", func(t *testing.T) {
			sc := open(t, city.Lively, true, snap(0, 0, "g1"), snap(1, 0, "g1"))
			require.NotEmpty(t, sc.s.City().Lines)
			on := sc.s.Traffic()

			t.Run("it should drive the road's street path, and nothing should ride the line", func(t *testing.T) {
				require.Len(t, on, 1)
				assert.Equal(t, road(sc), on[0].Path)
			})
		})

		t.Run("when the map first opens on "+name+"'s traffic already there", func(t *testing.T) {
			clock := start
			s := city.NewScene(city.NewLayout())
			s.Resize(800, 600)
			s.SetClock(func() time.Time { return clock }, time.UTC)
			s.SetSnapshot(snap(5, 2, ""))

			t.Run("it should send nothing", func(t *testing.T) {
				assert.Empty(t, s.Traffic())
			})
		})

		t.Run("when the level is still and a road gains one "+l.kind.Is(), func(t *testing.T) {
			sc := open(t, city.Still, true, snap(0, 0, ""), snap(1, 0, ""))
			on := sc.s.Traffic()

			t.Run("it should fade the "+name+" in at its end, with no drive", func(t *testing.T) {
				require.Len(t, on, 1)
				assert.Equal(t, [2]any{true, on[0].Path[len(on[0].Path)-1]}, [2]any{on[0].Fades, on[0].At(start.Add(time.Millisecond))})
			})
		})

		// Item 224: three more sessions in cinders re-lay the city, which
		// moves the road a row while the vehicle is on its way.
		relaid := func(t *testing.T, level city.Liveliness, group string) (scene, city.Point) {
			t.Helper()
			sc := open(t, level, true, snap(0, 0, group), snap(1, 0, group))
			on := sc.s.Traffic()
			require.Len(t, on, 1)
			*sc.clock = start.Add(on[0].For() / 2)
			here := on[0].At(*sc.clock)
			was := sc.s.City().Streets
			sc.s.SetSnapshot(grown(1, 0, 3, group))
			require.NotEqual(t, was[0].Path, sc.s.City().Streets[0].Path, "the fixture should move the road")
			return sc, here
		}

		for _, group := range []string{"", "g1"} {
			context := "when the city is re-laid out while a " + name + " drives, so its road moves"
			if group != "" {
				context += ", and both ends are stations on a drawn line"
			}
			t.Run(context, func(t *testing.T) {
				sc, here := relaid(t, city.Lively, group)
				on := sc.s.Traffic()
				now := road(sc)

				t.Run("it should carry on from where it was", func(t *testing.T) {
					require.Len(t, on, 1)
					assert.Equal(t, here, on[0].At(*sc.clock))
				})

				t.Run("it should end at the far kerb as it now stands, by its driveway", func(t *testing.T) {
					require.Len(t, on, 1)
					path := on[0].Path
					assert.Equal(t, now[len(now)-2:], path[len(path)-2:])
				})

				t.Run("it should drive only on the streets as they now stand on its way to the kerb", func(t *testing.T) {
					require.Len(t, on, 1)
					path := on[0].Path
					var off []city.Point
					for _, p := range path[1 : len(path)-1] {
						if !sc.s.City().IsStreet(city.CellOf(p)) {
							off = append(off, p)
						}
					}
					assert.Empty(t, off)
				})

				t.Run("it should name the districts as they now stand", func(t *testing.T) {
					require.Len(t, on, 1)
					assert.True(t, on[0].From == district(sc, cinders) && on[0].To == district(sc, mullet), "it names districts from an older city")
				})
			})
		}

		t.Run("when the level is still and the city is re-laid out under a "+name, func(t *testing.T) {
			sc, _ := relaid(t, city.Still, "")
			on := sc.s.Traffic()
			now := road(sc)

			t.Run("it should fade in at its end as it now stands", func(t *testing.T) {
				require.Len(t, on, 1)
				assert.Equal(t, now[len(now)-1], on[0].At(*sc.clock))
			})
		})

		t.Run("while a "+name+" drives", func(t *testing.T) {
			sc := open(t, city.Lively, true, snap(0, 0, ""), snap(1, 0, ""))
			*sc.clock = start.Add(200 * time.Millisecond)

			t.Run("the scene should be animating", func(t *testing.T) {
				assert.True(t, sc.s.Animating())
			})
		})
	}

	t.Run("when a road gains a message, a file and a visit one way in one snapshot", func(t *testing.T) {
		mixed := func(n int) state.Snapshot {
			s := snapshot(member("a", cinders, ""), member("b", mullet, ""))
			s.Roads = []state.Road{{From: cinders, To: mullet, Messages: n, Files: n, Visits: n, Sessions: 2}}
			return s
		}
		clock := start
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return clock }, time.UTC)
		s.SetSnapshot(mixed(0))
		s.SetSnapshot(mixed(1))

		t.Run("it should send the three spaced so they do not overlap", func(t *testing.T) {
			on := s.Traffic()
			require.Len(t, on, 3)
			assert.Equal(t, [2]time.Duration{city.VanGap, city.VanGap},
				[2]time.Duration{on[1].Since.Sub(on[0].Since), on[2].Since.Sub(on[1].Since)})
		})
	})

	// Slice 9 of item 89: a message between two ordinary sessions
	// reaches the road the same way, and sends one van.
	t.Run("when a peer message arrives between two districts", func(t *testing.T) {
		sender := member("a", cinders, "")
		sender.PID = 42
		peered := func(n int) state.Snapshot {
			recipient := member("b", mullet, "")
			for i := 0; i < n; i++ {
				recipient.PeerMessages = append(recipient.PeerMessages, claude.PeerMessage{ID: fmt.Sprint(i), FromPID: 42})
			}
			s := snapshot(sender, recipient)
			s.Roads = state.Roads(s.Sessions, nil)
			return s
		}
		clock := start
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return clock }, time.UTC)
		s.SetSnapshot(peered(0))
		s.SetSnapshot(peered(1))

		t.Run("it should send one van, as a team message does", func(t *testing.T) {
			on := s.Traffic()
			require.Len(t, on, 1)
			assert.Equal(t, [3]any{city.CargoMessage, cinders, mullet}, [3]any{on[0].Cargo, on[0].From.Root, on[0].To.Root})
		})
	})
}
