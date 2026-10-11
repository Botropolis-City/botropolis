package city_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 79. Aria: "If we set the option to only do usage manually, we
// should have the boats not there by default. We should have them drive
// up when you update manually and then drive off the screen", and "when
// things change, the boats should drive forward instead of teleporting."

func boatScene(clock *time.Time, manual bool) *city.Scene {
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return *clock }, time.UTC)
	s.SetGaugesManual(manual)
	return s
}

func reading(percent float64) []city.Gauge {
	return []city.Gauge{{Title: "this session", Percent: percent}}
}

func trackOf(t *testing.T, s *city.Scene) (float64, bool) {
	t.Helper()
	boats := s.GaugeBoats()
	if len(boats) == 0 {
		return 0, false
	}
	require.Len(t, boats, 1)
	return boats[0].Track, true
}

// settle runs the clock past anything a boat could still be doing.
func settle(clock *time.Time) {
	*clock = clock.Add(time.Minute)
}

func TestGaugeBoatsWhenRefreshingAutomatically(t *testing.T) {
	start := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

	t.Run("when the first reading lands", func(t *testing.T) {
		clock := start
		s := boatScene(&clock, false)
		s.SetGauges(reading(40))
		track, shown := trackOf(t, s)

		t.Run("it should be on the river", func(t *testing.T) {
			assert.True(t, shown)
		})

		// A film's first frame and a screenshot are taken as the first
		// snapshot lands; a boat still sailing in would be in neither.
		t.Run("it should already be at its reading", func(t *testing.T) {
			assert.InDelta(t, 0.4, track, 1e-9)
		})
	})

	t.Run("when a reading rises", func(t *testing.T) {
		clock := start
		s := boatScene(&clock, false)
		s.SetGauges(reading(40))
		s.SetGauges(reading(60))
		before, _ := trackOf(t, s)
		clock = clock.Add(city.GaugeDriveMin / 2)
		midway, _ := trackOf(t, s)
		settle(&clock)
		after, _ := trackOf(t, s)

		t.Run("it should start from the old reading rather than jump", func(t *testing.T) {
			assert.InDelta(t, 0.4, before, 1e-9)
		})

		t.Run("it should be driving forward on the way", func(t *testing.T) {
			assert.True(t, midway > 0.4 && midway < 0.6, "midway at %v", midway)
		})

		t.Run("it should arrive at the new reading", func(t *testing.T) {
			assert.InDelta(t, 0.6, after, 1e-9)
		})

		t.Run("it should count as motion while it drives", func(t *testing.T) {
			clock = start
			s := boatScene(&clock, false)
			s.SetGauges(reading(40))
			s.SetGauges(reading(60))
			assert.True(t, s.Animating())
		})
	})

	// A window that turns over drops its reading. Backing down the
	// river would be the one motion a boat never makes, so it carries
	// on forward, off the top, and comes round from the bottom.
	t.Run("when a reading falls", func(t *testing.T) {
		clock := start
		s := boatScene(&clock, false)
		s.SetGauges(reading(80))
		s.SetGauges(reading(10))
		var seen []float64
		for i := 0; i < 200; i++ {
			clock = clock.Add(50 * time.Millisecond)
			if track, ok := trackOf(t, s); ok {
				seen = append(seen, track)
			}
		}

		t.Run("it should go on past the limit first", func(t *testing.T) {
			assert.Greater(t, maxOf(seen), 1.0)
		})

		t.Run("it should come back in below the start", func(t *testing.T) {
			assert.Less(t, minOf(seen), 0.0)
		})

		t.Run("it should end at the new reading", func(t *testing.T) {
			assert.InDelta(t, 0.1, seen[len(seen)-1], 1e-9)
		})
	})

	t.Run("when motion is reduced", func(t *testing.T) {
		clock := start
		s := boatScene(&clock, false)
		s.SetReducedMotion(true)
		s.SetGauges(reading(40))
		s.SetGauges(reading(60))
		track, _ := trackOf(t, s)

		t.Run("it should be at the new reading at once", func(t *testing.T) {
			assert.InDelta(t, 0.6, track, 1e-9)
		})
	})
}

func TestGaugeBoatsWhenRefreshingByHand(t *testing.T) {
	start := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

	t.Run("when a reading lands without a visit", func(t *testing.T) {
		clock := start
		s := boatScene(&clock, true)
		s.SetGauges(reading(40))

		t.Run("it should keep the boats off the river", func(t *testing.T) {
			_, shown := trackOf(t, s)
			assert.False(t, shown)
		})
	})

	t.Run("when the boats are called up", func(t *testing.T) {
		clock := start
		s := boatScene(&clock, true)
		s.SetGauges(reading(40))
		s.VisitGauges()
		clock = clock.Add(10 * time.Millisecond)
		arriving, shown := trackOf(t, s)
		clock = clock.Add(city.GaugeRunFor * 2)
		onStation, _ := trackOf(t, s)
		clock = clock.Add(city.GaugeStayFor - city.GaugeRunFor)
		staying, stillShown := trackOf(t, s)

		t.Run("it should bring them onto the river", func(t *testing.T) {
			assert.True(t, shown)
		})

		t.Run("it should sail them in from below the start", func(t *testing.T) {
			assert.Less(t, arriving, 0.0)
		})

		t.Run("it should hold them at their reading", func(t *testing.T) {
			assert.InDelta(t, 0.4, onStation, 1e-9)
		})

		t.Run("it should keep them there for the stay", func(t *testing.T) {
			assert.True(t, stillShown && staying == onStation)
		})

		t.Run("and the stay is over", func(t *testing.T) {
			clock = clock.Add(city.GaugeRunFor/2 + city.GaugeRunFor)
			leaving, _ := trackOf(t, s)

			t.Run("it should drive them on past the limit", func(t *testing.T) {
				assert.Greater(t, leaving, 0.4)
			})

			t.Run("and they have gone", func(t *testing.T) {
				settle(&clock)

				t.Run("it should leave the river empty", func(t *testing.T) {
					_, shown := trackOf(t, s)
					assert.False(t, shown)
				})
			})
		})
	})

	t.Run("when fresh figures land during a visit", func(t *testing.T) {
		clock := start
		s := boatScene(&clock, true)
		s.SetGauges(reading(40))
		s.VisitGauges()
		clock = clock.Add(city.GaugeRunFor * 2)
		s.SetGauges(reading(55))
		clock = clock.Add(city.GaugeRunFor)
		track, _ := trackOf(t, s)

		t.Run("it should drive to the new reading", func(t *testing.T) {
			assert.InDelta(t, 0.55, track, 1e-9)
		})
	})

	t.Run("when the boats are called up again while on station", func(t *testing.T) {
		clock := start
		s := boatScene(&clock, true)
		s.SetGauges(reading(40))
		s.VisitGauges()
		clock = clock.Add(city.GaugeStayFor)
		s.VisitGauges()
		clock = clock.Add(city.GaugeStayFor - time.Second)
		track, shown := trackOf(t, s)

		t.Run("it should start the stay over", func(t *testing.T) {
			assert.True(t, shown && track == 0.4, "shown %v at %v", shown, track)
		})
	})

	// Turning round would be the one motion a boat never makes, so boats
	// called up on their way out finish leaving and come round again.
	t.Run("when the boats are called up again while sailing off", func(t *testing.T) {
		clock := start
		s := boatScene(&clock, true)
		s.SetGauges(reading(40))
		s.VisitGauges()
		clock = clock.Add(city.GaugeRunFor*2 + city.GaugeStayFor)
		leaving, _ := trackOf(t, s)
		require.Greater(t, leaving, 0.4)
		s.VisitGauges()
		var seen []float64
		for i := 0; i < 400; i++ {
			clock = clock.Add(50 * time.Millisecond)
			if track, ok := trackOf(t, s); ok {
				seen = append(seen, track)
			}
		}
		firstIn := len(seen)
		for i, track := range seen {
			if track < 0 {
				firstIn = i
				break
			}
		}

		t.Run("it should carry on forward rather than turn back", func(t *testing.T) {
			for _, track := range seen[:firstIn] {
				require.GreaterOrEqual(t, track, leaving)
			}
		})

		t.Run("it should come back in from below the start", func(t *testing.T) {
			assert.Less(t, firstIn, len(seen))
		})

		t.Run("it should end at its reading", func(t *testing.T) {
			assert.InDelta(t, 0.4, seen[len(seen)-1], 1e-9)
		})
	})

	t.Run("when the setting turns to manual while the boats are out", func(t *testing.T) {
		clock := start
		s := boatScene(&clock, false)
		s.SetGauges(reading(40))
		s.SetGaugesManual(true)
		clock = clock.Add(city.GaugeDriveMin)
		leaving, shown := trackOf(t, s)

		t.Run("it should sail them off rather than drop them", func(t *testing.T) {
			assert.True(t, shown && leaving > 0.4, "shown %v at %v", shown, leaving)
		})
	})

	t.Run("when the setting turns to automatic while the boats are away", func(t *testing.T) {
		clock := start
		s := boatScene(&clock, true)
		s.SetGauges(reading(40))
		s.SetGaugesManual(false)
		settle(&clock)
		track, shown := trackOf(t, s)

		t.Run("it should bring them in to stay", func(t *testing.T) {
			assert.True(t, shown && track == 0.4, "shown %v at %v", shown, track)
		})
	})
}

func maxOf(xs []float64) float64 {
	m := xs[0]
	for _, x := range xs {
		m = max(m, x)
	}
	return m
}

func minOf(xs []float64) float64 {
	m := xs[0]
	for _, x := range xs {
		m = min(m, x)
	}
	return m
}
