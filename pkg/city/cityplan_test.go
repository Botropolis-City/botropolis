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

		t.Run("and the wheel turns out", func(t *testing.T) {
			s := opened(t)
			s.Wheel(centre(s), -1)

			t.Run("it should stay open", func(t *testing.T) {
				assert.True(t, s.CityPlanOpen())
			})
		})

		t.Run("and the wheel turns in with the pointer over a district", func(t *testing.T) {
			s := opened(t)
			target := s.CityPlan().Badges[0]
			s.Wheel(target.At, 1)

			t.Run("it should close the plan", func(t *testing.T) {
				assert.False(t, s.CityPlanOpen())
			})

			t.Run("it should return at Skyline's zoom", func(t *testing.T) {
				assert.InDelta(t, s.SkylineZoom(), s.Camera().Zoom, 1e-9)
			})

			t.Run("it should put that district under the pointer on the map", func(t *testing.T) {
				under := s.Camera().ScreenToWorld(target.At)
				d := target.District.Rect.Center().Sub(under)
				assert.InDelta(t, 0, math.Hypot(d.X, d.Y), city.CellSize)
			})
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

// Item 106: the top-down projection is no longer offered; a setting
// that still asks for it opens the isometric map.
func TestProjectionTopDeprecated(t *testing.T) {
	p, warning, _ := city.UserProjection("top")

	t.Run("when a setting asks for top", func(t *testing.T) {
		t.Run("it should open isometric", func(t *testing.T) {
			assert.Equal(t, city.Isometric, p)
		})

		t.Run("it should say it is deprecated", func(t *testing.T) {
			assert.NotEmpty(t, warning)
		})
	})

	t.Run("when a setting asks for iso", func(t *testing.T) {
		_, warning, _ := city.UserProjection("iso")

		t.Run("it should not warn", func(t *testing.T) {
			assert.Empty(t, warning)
		})
	})
}
