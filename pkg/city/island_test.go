package city_test

import (
	"fmt"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 85, slice 3: the plan stands on an island in a sea, its coast
// growing with the city, and the river runs through it to the sea.

func districts(n int) []state.Session {
	var out []state.Session
	for i := 0; i < n; i++ {
		out = append(out, session(fmt.Sprint("s", i), fmt.Sprintf("/p/d%02d", i), state.Working))
	}
	return out
}

func onLand(g city.Ground) bool { return g == city.GroundLand || g == city.GroundSand }

func water(g city.Ground) bool {
	return g == city.GroundRiver || g == city.GroundShallows || g == city.GroundSea
}

func edge(r city.Rect, steps int) []city.Point {
	var out []city.Point
	for i := 0; i <= steps; i++ {
		f := float64(i) / float64(steps)
		x := r.Min.X + (r.Max.X-r.Min.X)*f
		y := r.Min.Y + (r.Max.Y-r.Min.Y)*f
		out = append(out, city.Point{X: x, Y: r.Min.Y + 0.01}, city.Point{X: x, Y: r.Max.Y - 0.01},
			city.Point{X: r.Min.X + 0.01, Y: y}, city.Point{X: r.Max.X - 0.01, Y: y})
	}
	return out
}

func TestIsland(t *testing.T) {
	for _, n := range []int{1, 9, 25, 49} {
		c := city.Build(snapshot(districts(n)...), city.NewLayout())
		island := c.Island()

		t.Run(fmt.Sprintf("when the city has %d rings", c.Rings()), func(t *testing.T) {
			t.Run("it should stand every district's block on land", func(t *testing.T) {
				for _, d := range c.Districts {
					for _, p := range edge(d.Rect, 4) {
						g, _ := island.At(p)
						assert.True(t, onLand(g), "%s at %v is %v", d.Name, p, g)
					}
				}
			})

			t.Run("it should stand the whole plan on land", func(t *testing.T) {
				for _, p := range edge(c.Bounds(), 40) {
					if g, _ := island.At(p); !onLand(g) && g != city.GroundRiver {
						t.Errorf("plan edge at %v is %v", p, g)
					}
				}
			})
		})
	}

	c := city.Build(snapshot(districts(9)...), city.NewLayout())
	island := c.Island()
	reach := island.Reach()

	t.Run("when the same plan is asked twice", func(t *testing.T) {
		again := city.Build(snapshot(districts(9)...), city.NewLayout()).Island()

		t.Run("it should draw the same coast", func(t *testing.T) {
			for _, p := range edge(reach, 60) {
				a, _ := island.At(p)
				b, _ := again.At(p)
				assert.Equal(t, a, b, "at %v", p)
			}
		})
	})

	t.Run("past the coast's furthest reach", func(t *testing.T) {
		out := city.Rect{Min: city.Point{X: reach.Min.X - 1, Y: reach.Min.Y - 1}, Max: city.Point{X: reach.Max.X + 1, Y: reach.Max.Y + 1}}

		t.Run("it should be sea", func(t *testing.T) {
			for _, p := range edge(out, 60) {
				g, _ := island.At(p)
				assert.Contains(t, []city.Ground{city.GroundSea, city.GroundFarShore}, g, "at %v", p)
			}
		})
	})

	t.Run("the river", func(t *testing.T) {
		from, to, _, ok := c.RiverBand()
		require.True(t, ok)
		b := c.Bounds()
		for _, end := range []struct {
			name   string
			x      float64
			y0, y1 float64
		}{
			{"north", from.X, b.Min.Y, reach.Min.Y - 1},
			{"south", to.X, b.Max.Y, reach.Max.Y + 1},
		} {
			t.Run("it should run as water from the plan's "+end.name+" edge out to the sea", func(t *testing.T) {
				for i := 0; i <= 50; i++ {
					y := end.y0 + (end.y1-end.y0)*float64(i)/50
					g, _ := island.At(city.Point{X: end.x, Y: y})
					assert.True(t, water(g), "at y %.0f it is %v", y, g)
				}
			})
		}
	})

	t.Run("when sessions arrive and leave", func(t *testing.T) {
		clock := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return clock }, time.UTC)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working)))
		clock = clock.Add(time.Minute)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working), session("b", botropolis, state.Working)))
		var arrival *city.Voyage
		for _, v := range s.Voyages() {
			if v.Kind == city.Arrival {
				arrival = v
			}
		}
		require.NotNil(t, arrival)
		north, south := s.City().Island().Mouths()

		t.Run("it should set an arriving tug out from the sea past the north mouth", func(t *testing.T) {
			assert.Equal(t, north, arrival.From)
		})

		t.Run("it should put the north mouth out at sea", func(t *testing.T) {
			g, _ := s.City().Island().At(north)
			assert.Contains(t, []city.Ground{city.GroundSea, city.GroundShallows}, g)
		})

		clock = clock.Add(time.Minute)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working)))
		var departure *city.Voyage
		for _, v := range s.Voyages() {
			if v.Kind == city.Departure {
				departure = v
			}
		}
		require.NotNil(t, departure)

		t.Run("it should sail a leaving tug out to the sea past the south mouth", func(t *testing.T) {
			assert.Equal(t, south, departure.To)
		})
	})

	t.Run("when the camera is panned as far as it goes", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(districts(9)...))
		s.Pan(city.Point{X: 1e6, Y: 1e6})
		centre := s.Camera().ScreenToWorld(city.Point{X: 400, Y: 300})

		t.Run("it should be able to show past the coast", func(t *testing.T) {
			g, _ := s.City().Island().At(centre)
			assert.Contains(t, []city.Ground{city.GroundSea, city.GroundShallows, city.GroundFarShore}, g, "centre at %v", centre)
		})
	})

	t.Run("the gauges", func(t *testing.T) {
		from, to, width, _ := c.RiverBand()

		t.Run("it should keep the river band inside the plan", func(t *testing.T) {
			b := c.Bounds()
			assert.True(t, from.Y >= b.Min.Y && to.Y <= b.Max.Y && width > 0)
		})
	})
}
