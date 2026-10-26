package city_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 106: one zoom step past Skyline, the city is a flat plan of
// district badges at their real places.
func TestCityPlan(t *testing.T) {
	spread := func(districts int) []state.Session {
		var out []state.Session
		for i := 0; i < districts; i++ {
			out = append(out, session(fmt.Sprintf("s%02d", i), fmt.Sprintf("/home/demo/src/p%02d", i), state.Working))
		}
		return out
	}
	open := func(t *testing.T, width, height float64, snap state.Snapshot) *city.Scene {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.SetProjection(city.Isometric)
		s.SetInstant(true)
		s.Resize(width, height)
		s.SetSnapshot(snap)
		return s
	}
	centre := func(s *city.Scene) city.Point { return s.Size().Scale(0.5) }
	badgeOf := func(p city.CityPlan, d *city.District) (city.PlanBadge, bool) {
		for _, b := range p.Badges {
			if b.District == d {
				return b, true
			}
		}
		return city.PlanBadge{}, false
	}

	t.Run("when the map opens", func(t *testing.T) {
		s := open(t, 1600, 1000, snapshot(spread(6)...))

		t.Run("it should open on the isometric map, not the plan", func(t *testing.T) {
			assert.False(t, s.CityPlanOpen())
		})
	})

	for _, c := range []struct {
		name          string
		districts     int
		width, height float64
	}{
		{"a city that reads whole at Skyline", 6, 1600, 1000},
		{"a city that overflows at the floor", 12, 400, 280},
	} {
		t.Run("for "+c.name+", when the wheel turns out one notch at Skyline", func(t *testing.T) {
			s := open(t, c.width, c.height, snapshot(spread(c.districts)...))
			s.Skyline()
			s.Wheel(centre(s), -1)

			t.Run("it should open the City Plan", func(t *testing.T) {
				assert.True(t, s.CityPlanOpen())
			})
		})
	}

	t.Run("when the wheel turns out one notch from a zoom above Skyline", func(t *testing.T) {
		s := open(t, 1600, 1000, snapshot(spread(2)...))
		s.Camera().Zoom = city.MaxZoom
		s.Wheel(centre(s), -1)

		t.Run("it should step the map out", func(t *testing.T) {
			assert.Less(t, s.Camera().Zoom, city.MaxZoom)
		})

		t.Run("it should leave the plan closed", func(t *testing.T) {
			assert.False(t, s.CityPlanOpen())
		})
	})

	// A quick zoom-out parks at Skyline; the plan opens on a notch that
	// comes after a pause, so a slow zoom flows into it.
	ticking := func(t *testing.T) (*city.Scene, func(time.Duration)) {
		t.Helper()
		at := now
		s := open(t, 1600, 1000, snapshot(spread(6)...))
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Camera().Zoom = city.MaxZoom
		return s, func(d time.Duration) { at = at.Add(d) }
	}
	quick := city.PlanDebounce / 6

	t.Run("when the wheel turns out several notches from above Skyline in one event", func(t *testing.T) {
		s, _ := ticking(t)
		s.Wheel(centre(s), -20)

		t.Run("it should stop at Skyline", func(t *testing.T) {
			assert.InDelta(t, s.SkylineZoom(), s.Camera().Zoom, 1e-9)
		})

		t.Run("it should leave the plan closed", func(t *testing.T) {
			assert.False(t, s.CityPlanOpen())
		})
	})

	t.Run("when the wheel turns out in quick events from above Skyline", func(t *testing.T) {
		s, tick := ticking(t)
		for i := 0; i < 20; i++ {
			s.Wheel(centre(s), -1)
			tick(quick)
		}
		stopped, opened := s.Camera().Zoom, s.CityPlanOpen()

		t.Run("it should stop at Skyline", func(t *testing.T) {
			assert.InDelta(t, s.SkylineZoom(), stopped, 1e-9)
		})

		t.Run("it should leave the plan closed", func(t *testing.T) {
			assert.False(t, opened)
		})

		t.Run("and one more notch comes after a pause", func(t *testing.T) {
			tick(city.PlanDebounce)
			s.Wheel(centre(s), -1)

			t.Run("it should open the plan", func(t *testing.T) {
				assert.True(t, s.CityPlanOpen())
			})
		})
	})

	t.Run("when the wheel turns out slowly from above Skyline", func(t *testing.T) {
		s, tick := ticking(t)
		for i := 0; i < len(city.ZoomSteps)+1; i++ {
			tick(city.PlanDebounce + quick)
			s.Wheel(centre(s), -1)
		}

		t.Run("it should flow into the plan", func(t *testing.T) {
			assert.True(t, s.CityPlanOpen())
		})
	})

	// Item 282: Skyline's zoom is computed from the island, and can lie
	// below the ladder's last step.
	crowd := func(districts int) []state.Session {
		var out []state.Session
		for i := 0; i < 3*districts; i++ {
			out = append(out, session(fmt.Sprintf("c%03d", i), fmt.Sprintf("/w/p%03d", i%districts), state.Working))
		}
		return out
	}
	for _, c := range []struct {
		districts     int
		width, height float64
		belowLadder   bool
	}{
		{2, 1600, 1000, false},
		{6, 1600, 1000, false},
		{60, 1600, 1000, true},
		{200, 2560, 1400, true},
		{6, 800, 600, true},
	} {
		t.Run(fmt.Sprintf("for %d districts in a %.0fx%.0f window, when the wheel turns out in quick notches from MaxZoom", c.districts, c.width, c.height), func(t *testing.T) {
			at := now
			s := open(t, c.width, c.height, snapshot(crowd(c.districts)...))
			s.SetClock(func() time.Time { return at }, time.UTC)
			require.Equal(t, c.belowLadder, s.SkylineZoom() < city.MinZoom, "precondition: whether Skyline lies below the ladder")
			s.Camera().Zoom = city.MaxZoom
			for i := 0; i < 20; i++ {
				s.Wheel(centre(s), -1)
				at = at.Add(quick)
			}
			stopped := s.Camera().Zoom

			t.Run("it should stop at Skyline", func(t *testing.T) {
				assert.InDelta(t, s.SkylineZoom(), stopped, 1e-9)
			})

			t.Run("and one more notch comes after a pause", func(t *testing.T) {
				at = at.Add(city.PlanDebounce)
				s.Wheel(centre(s), -1)

				t.Run("it should open the plan", func(t *testing.T) {
					assert.True(t, s.CityPlanOpen())
				})
			})
		})
	}

	t.Run("when the plan is open", func(t *testing.T) {
		opened := func(t *testing.T) *city.Scene {
			t.Helper()
			s := open(t, 1600, 1000, snapshot(spread(6)...))
			s.Skyline()
			s.Wheel(centre(s), -1)
			require.True(t, s.CityPlanOpen(), "precondition: the plan is open")
			return s
		}

		t.Run("and the pointer moves and clicks over the map hidden beneath", func(t *testing.T) {
			s := opened(t)
			b := s.City().Buildings()[0]
			at := s.Camera().WorldToScreen(b.Rect.Center())
			s.PointerMove(at)
			s.Click(at)

			t.Run("it should hover nothing", func(t *testing.T) {
				assert.Equal(t, city.Hit{}, s.Hover())
			})

			t.Run("it should select nothing", func(t *testing.T) {
				assert.Nil(t, s.Selected())
			})
		})

		t.Run("and the wheel turns out at the plan's widest", func(t *testing.T) {
			s := opened(t)
			s.Wheel(centre(s), -1)

			t.Run("it should stay open", func(t *testing.T) {
				assert.True(t, s.CityPlanOpen())
			})

			t.Run("it should stay at its widest", func(t *testing.T) {
				assert.Equal(t, city.PlanZoomSteps[0], s.PlanZoom())
			})
		})
	})

	// Item 282: the plan has its own zoom, about the pointer, from its
	// fitted widest to PlanZoomSteps' last; a rested notch in past the
	// closest returns to the map.
	zoomable := func(t *testing.T, sessions []state.Session, width, height float64) (*city.Scene, func(time.Duration)) {
		t.Helper()
		at := now
		s := open(t, width, height, snapshot(sessions...))
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Skyline()
		s.Wheel(centre(s), -1)
		require.True(t, s.CityPlanOpen(), "precondition: the plan is open")
		return s, func(d time.Duration) { at = at.Add(d) }
	}
	closest := city.PlanZoomSteps[len(city.PlanZoomSteps)-1]
	gap := func(a, b city.Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

	t.Run("when the plan is open and the wheel turns in with the pointer over a district", func(t *testing.T) {
		s, _ := zoomable(t, spread(6), 1600, 1000)
		target := s.CityPlan().Badges[0]
		under := s.CityPlan().ToWorld(target.At)
		s.Wheel(target.At, 1)

		t.Run("it should keep the plan open", func(t *testing.T) {
			assert.True(t, s.CityPlanOpen())
		})

		t.Run("it should zoom the plan in one step", func(t *testing.T) {
			assert.Equal(t, city.PlanZoomSteps[1], s.PlanZoom())
		})

		t.Run("it should keep the point under the pointer", func(t *testing.T) {
			assert.InDelta(t, 0, gap(target.At, s.CityPlan().ToScreen(under)), 1e-6)
		})
	})

	t.Run("when the plan is zoomed in and the wheel turns out", func(t *testing.T) {
		s, _ := zoomable(t, spread(6), 1600, 1000)
		pointer := s.CityPlan().Badges[0].At
		s.Wheel(pointer, 2)
		under := s.CityPlan().ToWorld(pointer)
		s.Wheel(pointer, -1)

		t.Run("it should zoom the plan out one step", func(t *testing.T) {
			assert.Equal(t, city.PlanZoomSteps[1], s.PlanZoom())
		})

		t.Run("it should keep the point under the pointer", func(t *testing.T) {
			assert.InDelta(t, 0, gap(pointer, s.CityPlan().ToScreen(under)), 1e-6)
		})
	})

	t.Run("when the wheel turns in quickly past the plan's closest zoom", func(t *testing.T) {
		s, tick := zoomable(t, spread(6), 1600, 1000)
		target := s.CityPlan().Badges[0]
		for i := 0; i < 20; i++ {
			tick(quick)
			s.Wheel(target.At, 1)
		}
		stopped, opened := s.PlanZoom(), s.CityPlanOpen()

		t.Run("it should stop at the closest zoom", func(t *testing.T) {
			assert.Equal(t, closest, stopped)
		})

		t.Run("it should keep the plan open", func(t *testing.T) {
			assert.True(t, opened)
		})

		t.Run("and one more notch comes after a pause", func(t *testing.T) {
			under := s.CityPlan().ToWorld(target.At)
			tick(city.PlanDebounce)
			s.Wheel(target.At, 1)

			t.Run("it should return to the map", func(t *testing.T) {
				assert.False(t, s.CityPlanOpen())
			})

			t.Run("it should return at Skyline's zoom", func(t *testing.T) {
				assert.InDelta(t, s.SkylineZoom(), s.Camera().Zoom, 1e-9)
			})

			t.Run("it should put the point under the pointer on the plan under it on the map", func(t *testing.T) {
				assert.InDelta(t, 0, gap(under, s.Camera().ScreenToWorld(target.At)), city.CellSize)
			})
		})
	})

	t.Run("when the plan is zoomed in until districts lie off the window", func(t *testing.T) {
		s, _ := zoomable(t, spread(6), 1600, 1000)
		s.Wheel(s.CityPlan().Badges[0].At, float64(len(city.PlanZoomSteps)))
		plan := s.CityPlan()
		var off int
		for _, d := range s.City().Districts {
			if !d.Depot && !plan.Area.Contains(plan.ToScreen(d.Rect.Center())) {
				off++
			}
		}
		require.Positive(t, off, "precondition: some district lies off the window")

		t.Run("it should leave their badges off the plan rather than pile them at its edge", func(t *testing.T) {
			var piled []string
			for _, b := range plan.Badges {
				if !plan.Area.Contains(b.Place) {
					piled = append(piled, b.Name)
				}
			}
			assert.Empty(t, piled)
		})
	})

	t.Run("when a crowded plan is zoomed in until few badges are in view", func(t *testing.T) {
		s, _ := zoomable(t, crowd(60), 800, 600)
		require.True(t, s.CityPlan().Badges[0].Compact, "precondition: the crowded plan's badges are compact")
		s.Wheel(s.CityPlan().Badges[0].At, float64(len(city.PlanZoomSteps)))

		t.Run("it should draw the badges in view full", func(t *testing.T) {
			var compact []string
			for _, b := range s.CityPlan().Badges {
				if b.Compact {
					compact = append(compact, b.Name)
				}
			}
			assert.Empty(t, compact)
		})
	})

	t.Run("when the plan is zoomed in and dragged", func(t *testing.T) {
		s, _ := zoomable(t, spread(6), 1600, 1000)
		s.Wheel(centre(s), 2)
		world := s.CityPlan().ToWorld(centre(s))
		s.Pan(city.Point{X: 40, Y: -30})

		t.Run("it should move the plan with the pointer", func(t *testing.T) {
			assert.InDelta(t, 0, gap(centre(s).Add(city.Point{X: 40, Y: -30}), s.CityPlan().ToScreen(world)), 1e-6)
		})
	})

	for _, drag := range []city.Point{{X: 1e5}, {X: -1e5}, {Y: 1e5}, {Y: -1e5}} {
		t.Run(fmt.Sprintf("when the plan is zoomed in and dragged far by %v", drag), func(t *testing.T) {
			s, _ := zoomable(t, spread(6), 1600, 1000)
			fitted := s.CityPlan()
			corners := [2]city.Point{fitted.ToWorld(fitted.Area.Min), fitted.ToWorld(fitted.Area.Max)}
			s.Wheel(centre(s), 2)
			s.Pan(drag)
			plan := s.CityPlan()
			lo, hi := plan.ToScreen(corners[0]), plan.ToScreen(corners[1])

			t.Run("it should keep the sheet across the whole window", func(t *testing.T) {
				assert.True(t, lo.X <= plan.Area.Min.X+1e-6 && lo.Y <= plan.Area.Min.Y+1e-6 &&
					hi.X >= plan.Area.Max.X-1e-6 && hi.Y >= plan.Area.Max.Y-1e-6,
					"sheet %v-%v, window %v", lo, hi, plan.Area)
			})
		})
	}

	t.Run("when the plan is at its widest and dragged", func(t *testing.T) {
		s, _ := zoomable(t, spread(6), 1600, 1000)
		world := s.CityPlan().ToWorld(centre(s))
		s.Pan(city.Point{X: 40, Y: -30})

		t.Run("it should not move", func(t *testing.T) {
			assert.InDelta(t, 0, gap(centre(s), s.CityPlan().ToScreen(world)), 1e-6)
		})
	})

	t.Run("when the plan is zoomed in, closed and opened again", func(t *testing.T) {
		s, _ := zoomable(t, spread(6), 1600, 1000)
		s.Wheel(centre(s), 2)
		s.ToggleCityPlan()
		s.ToggleCityPlan()

		t.Run("it should open at its widest", func(t *testing.T) {
			assert.Equal(t, city.PlanZoomSteps[0], s.PlanZoom())
		})
	})

	for _, heading := range []int{0, 90, 180, 270} {
		t.Run(fmt.Sprintf("when the plan is laid out facing %d", heading), func(t *testing.T) {
			s := open(t, 1600, 1000, snapshot(spread(6)...))
			s.Camera().Heading = heading
			s.Skyline()
			plan := s.CityPlan()
			var districts int
			for _, d := range s.City().Districts {
				if !d.Depot {
					districts++
				}
			}

			t.Run("it should outline each district's lot on the plan, around its badge", func(t *testing.T) {
				var off []string
				for _, b := range plan.Badges {
					var mid city.Point
					for _, p := range b.Lot {
						mid = mid.Add(p.Scale(0.25))
					}
					if math.Hypot(mid.X-b.At.X, mid.Y-b.At.Y) > 1 {
						off = append(off, b.Name)
					}
				}
				assert.Empty(t, off)
			})

			t.Run("it should give every district but the depot one badge", func(t *testing.T) {
				assert.Len(t, plan.Badges, districts)
			})

			t.Run("it should fit every badge inside the window, clear of the chrome", func(t *testing.T) {
				in := s.Insets()
				size := s.Size()
				area := city.Rect{Min: city.Point{X: in.Left, Y: in.Top}, Max: city.Point{X: size.X - in.Right, Y: size.Y - in.Bottom}}
				var outside []string
				for _, b := range plan.Badges {
					if b.Rect.Min.X < area.Min.X || b.Rect.Min.Y < area.Min.Y || b.Rect.Max.X > area.Max.X || b.Rect.Max.Y > area.Max.Y {
						outside = append(outside, b.Name)
					}
				}
				assert.Empty(t, outside)
			})

			t.Run("and one district lies left of another on the map, it should lie left of it on the plan", func(t *testing.T) {
				var disagree []string
				for _, a := range plan.Badges {
					for _, b := range plan.Badges {
						onMap := s.Camera().WorldToScreen(a.District.Rect.Center()).X - s.Camera().WorldToScreen(b.District.Rect.Center()).X
						onPlan := a.At.X - b.At.X
						if onMap < -1 && onPlan >= 0 {
							disagree = append(disagree, a.Name+" / "+b.Name)
						}
					}
				}
				assert.Empty(t, disagree)
			})
		})
	}

	t.Run("when a district has sessions", func(t *testing.T) {
		waiting := session("w", "/home/demo/src/alpha", state.NeedsYou)
		waiting.SubagentsInFlight = 2
		busy := session("b", "/home/demo/src/alpha", state.Working)
		busy.SubagentsInFlight = 1
		other := session("o", "/home/demo/src/beta", state.Working)
		s := open(t, 1600, 1000, snapshot(waiting, busy, other))
		var alpha *city.District
		for _, d := range s.City().Districts {
			if d.Root == "/home/demo/src/alpha" {
				alpha = d
			}
		}
		require.NotNil(t, alpha)
		badge, ok := badgeOf(s.CityPlan(), alpha)
		require.True(t, ok)
		sign := alpha.Sign()

		for _, f := range []struct {
			name      string
			got, want any
		}{
			{"name the district", badge.Name, sign.Name},
			{"say whose move it is", badge.Move, sign.Move},
			{"carry the move's tone", badge.Tone, sign.Tone},
			{"count its sessions", badge.Sessions, 2},
			{"count those needing you", badge.NeedsYou, 1},
			{"count subagents in flight", badge.Subagents, 3},
		} {
			t.Run("its badge should "+f.name, func(t *testing.T) {
				assert.Equal(t, f.want, f.got)
			})
		}
	})

	t.Run("when a group runs through districts", func(t *testing.T) {
		member := func(id, cwd string, started time.Duration) state.Session {
			m := session(id, cwd, state.Working)
			m.Group, m.StartedAt = "g1", now.Add(started)
			return m
		}
		snap := snapshot(
			member("a", "/home/demo/src/alpha", -3*time.Hour),
			member("b", "/home/demo/src/beta", -2*time.Hour),
			member("c", "/home/demo/src/gamma", -time.Hour),
			session("d", "/home/demo/src/delta", state.Working),
		)
		snap.Groups = []state.Group{{ID: "g1", Name: "PROJ-1", Short: "PROJ-1", Members: []string{"a", "b", "c"}}}
		s := open(t, 1600, 1000, snap)
		require.Len(t, s.Lines(), 1)
		plan := s.CityPlan()

		t.Run("it should draw one line, through the stations' badges in station order", func(t *testing.T) {
			var want []city.Point
			for _, st := range s.Lines()[0].Stations {
				b, _ := badgeOf(plan, st.District)
				want = append(want, b.At)
			}
			require.Len(t, plan.Lines, 1)
			assert.Equal(t, want, plan.Lines[0].Points)
		})

		t.Run("and group lines are switched off", func(t *testing.T) {
			s.SetGroupLines(false)

			t.Run("it should draw no lines", func(t *testing.T) {
				assert.Empty(t, s.CityPlan().Lines)
			})
		})
	})

	t.Run("when usage limits are known", func(t *testing.T) {
		s := open(t, 1600, 1000, snapshot(spread(2)...))
		gauges := []city.Gauge{{Title: "this week", Percent: 41}, {Title: "this session", Percent: 73}, {Title: "this week, Opus", Percent: 12}}
		s.SetGauges(gauges)
		var got []city.PlanReading
		for _, r := range s.CityPlan().Readings {
			got = append(got, city.PlanReading{Title: r.Title, Percent: r.Percent})
		}

		t.Run("the legend should carry one reading per gauge, in the gauges' order", func(t *testing.T) {
			assert.Equal(t, []city.PlanReading{{Title: "this week", Percent: 41}, {Title: "this session", Percent: 73}, {Title: "this week, Opus", Percent: 12}}, got)
		})

		t.Run("and none are known", func(t *testing.T) {
			s.SetGauges(nil)

			t.Run("the legend should carry no readings", func(t *testing.T) {
				assert.Empty(t, s.CityPlan().Readings)
			})
		})
	})

	t.Run("when badges carry moves", func(t *testing.T) {
		s := open(t, 1600, 1000, snapshot(
			session("a", "/home/demo/src/alpha", state.NeedsYou),
			session("b", "/home/demo/src/beta", state.Working),
			session("c", "/home/demo/src/gamma", state.Working),
		))
		plan := s.CityPlan()
		var tones []state.Move
		seen := map[state.Move]bool{}
		for _, b := range plan.Badges {
			if !seen[b.Tone] {
				seen[b.Tone] = true
				tones = append(tones, b.Tone)
			}
		}

		t.Run("the legend should key each move tone that appears on a badge", func(t *testing.T) {
			assert.ElementsMatch(t, tones, plan.Tones)
		})
	})

	t.Run("when liveliness is still", func(t *testing.T) {
		t.Run("the fade should be shorter than at steady", func(t *testing.T) {
			assert.Less(t, city.PlanFade(city.Still), city.PlanFade(city.Steady))
		})
	})
}

// planScene is a city of six districts on the isometric map, at Skyline,
// with a clock the test can move.
func planScene(t *testing.T) (*city.Scene, func(time.Duration)) {
	t.Helper()
	at := now
	var sessions []state.Session
	for i := 0; i < 6; i++ {
		sessions = append(sessions, session(fmt.Sprintf("s%02d", i), fmt.Sprintf("/home/demo/src/p%02d", i), state.Working))
	}
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return at }, time.UTC)
	s.SetProjection(city.Isometric)
	s.SetInstant(true)
	s.Resize(1600, 1000)
	s.SetSnapshot(snapshot(sessions...))
	s.Skyline()
	return s, func(d time.Duration) { at = at.Add(d) }
}

// Item 106: a key opens the plan from anywhere and closes it back to
// where the camera was.
func TestCityPlanToggle(t *testing.T) {
	t.Run("when the plan is toggled from a zoom above Skyline", func(t *testing.T) {
		s, _ := planScene(t)
		s.Wheel(s.Size().Scale(0.3), 2)
		zoom, offset := s.Camera().Zoom, s.Camera().Offset
		s.ToggleCityPlan()

		t.Run("it should open the plan", func(t *testing.T) {
			assert.True(t, s.CityPlanOpen())
		})

		t.Run("and toggled again", func(t *testing.T) {
			s.ToggleCityPlan()

			t.Run("it should close the plan", func(t *testing.T) {
				assert.False(t, s.CityPlanOpen())
			})

			t.Run("it should restore the zoom it had", func(t *testing.T) {
				assert.InDelta(t, zoom, s.Camera().Zoom, 1e-9)
			})

			t.Run("it should restore the offset it had", func(t *testing.T) {
				assert.Equal(t, offset, s.Camera().Offset)
			})
		})
	})

	t.Run("when the plan is toggled just after a wheel event", func(t *testing.T) {
		s, _ := planScene(t)
		s.Wheel(s.Size().Scale(0.5), 1)
		s.ToggleCityPlan()

		t.Run("it should open the plan", func(t *testing.T) {
			assert.True(t, s.CityPlanOpen())
		})
	})

	t.Run("when the plan was opened by the wheel and is closed by the key", func(t *testing.T) {
		s, tick := planScene(t)
		tick(city.PlanDebounce)
		s.Wheel(s.Size().Scale(0.5), -1)
		require.True(t, s.CityPlanOpen(), "precondition: the wheel opened the plan")
		s.ToggleCityPlan()

		t.Run("it should return at Skyline", func(t *testing.T) {
			assert.InDelta(t, s.SkylineZoom(), s.Camera().Zoom, 1e-9)
		})
	})
}

// Item 106: clicking a badge flies into its district on the map.
func TestCityPlanFlyIn(t *testing.T) {
	for _, pick := range []struct {
		name string
		at   func(n int) int
	}{
		{"the first", func(int) int { return 0 }},
		{"the last", func(n int) int { return n - 1 }},
	} {
		t.Run("when "+pick.name+" badge is clicked on the plan", func(t *testing.T) {
			s, _ := planScene(t)
			s.ToggleCityPlan()
			badges := s.CityPlan().Badges
			badge := badges[pick.at(len(badges))]
			s.Click(badge.At)
			open, selected := s.CityPlanOpen(), s.Selected()
			for i := 0; i < 200; i++ {
				s.Animate(1)
			}

			t.Run("it should close the plan", func(t *testing.T) {
				assert.False(t, open)
			})

			t.Run("it should select nothing", func(t *testing.T) {
				assert.Nil(t, selected)
			})

			t.Run("once the ease settles, the district should be in the middle of the view", func(t *testing.T) {
				in := s.Insets()
				size := s.Size()
				middle := city.Point{X: in.Left + (size.X-in.Left-in.Right)/2, Y: in.Top + (size.Y-in.Top-in.Bottom)/2}
				d := badge.District.Rect.Center().Sub(s.Camera().ScreenToWorld(middle))
				assert.InDelta(t, 0, math.Hypot(d.X, d.Y), city.CellSize)
			})

			t.Run("once the ease settles, the zoom should be past Skyline", func(t *testing.T) {
				assert.Greater(t, s.Camera().Zoom, s.SkylineZoom())
			})

			t.Run("once the ease settles, the zoom should be at most MaxZoom", func(t *testing.T) {
				assert.LessOrEqual(t, s.Camera().Zoom, city.MaxZoom)
			})
		})
	}

	t.Run("when a click on the plan lands between badges", func(t *testing.T) {
		s, _ := planScene(t)
		s.ToggleCityPlan()
		s.Click(city.Point{X: 2, Y: s.Size().Y / 2})

		t.Run("it should leave the plan open", func(t *testing.T) {
			assert.True(t, s.CityPlanOpen())
		})
	})
}
