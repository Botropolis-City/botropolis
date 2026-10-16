package city_test

import (
	"math"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bug 54a. Aria: "Even with one boat, it still makes things refresh
// weird and the boat teleports off the river into the black."
//
// A voyage captured From and To as absolute world points when it was
// created and interpolated between them forever. The river sits at the
// map's east edge and the map's width follows the block layout, so the
// river moves whenever the city is re-planned — and measurably it moves
// for far less than a new district: a SECOND SESSION IN A PROJECT THAT
// ALREADY EXISTS shifts it one cell, 48 units. Any tug in flight was
// then sailing coordinates belonging to a river that no longer existed,
// and when the map shrank those coordinates were past its east edge,
// which is the black she saw.
//
// The endpoints are resolved from the live RiverBand on every read now,
// so a re-layout carries its boats instead of stranding them. Progress
// is time-based, so a tug keeps its place along the run and re-anchors.

func voyageScene(t *testing.T, clock *time.Time) *city.Scene {
	t.Helper()
	s := city.NewScene(city.NewLayout())
	s.Resize(800, 600)
	s.SetClock(func() time.Time { return *clock }, time.UTC)
	return s
}

// afloat reports how far a point is outside the river's water, in world
// units; zero when it is on the water.
func aground(c *city.City, p city.Point) float64 {
	from, _, width, ok := c.RiverBand()
	if !ok {
		return math.Inf(1)
	}
	return math.Max(0, math.Abs(p.X-from.X)-width/2)
}

func TestVoyagesSurviveTheMapMovingUnderThem(t *testing.T) {
	clock := time.Date(2026, 9, 18, 14, 0, 0, 0, time.UTC)
	s := voyageScene(t, &clock)

	// Three sessions in one project put the river at its widest offset;
	// dropping back to one pulls it two cells west.
	three := []state.Session{
		session("a", cinders, state.Working),
		session("b", cinders, state.Working),
		session("c", cinders, state.Working),
	}
	s.SetSnapshot(snapshot(three[0]))
	clock = clock.Add(time.Minute)
	s.SetSnapshot(snapshot(three...))
	wide, _, _, ok := s.City().RiverBand()
	require.True(t, ok)
	require.NotEmpty(t, s.Voyages(), "two sessions arriving should launch tugs")

	t.Run("when the city re-plans while a tug is still sailing", func(t *testing.T) {
		clock = clock.Add(city.SailFor / 3)
		s.SetSnapshot(snapshot(three[0]))
		narrow, _, _, _ := s.City().RiverBand()
		require.NotEqual(t, wide.X, narrow.X, "this test is pointless unless the river actually moves")

		t.Run("it should still have every tug on the water", func(t *testing.T) {
			for _, v := range s.Voyages() {
				assert.Zero(t, aground(s.City(), v.At(clock)),
					"%s tug for %s is %.0f units past the bank", v.Kind, v.SessionID,
					aground(s.City(), v.At(clock)))
			}
		})
	})
}

func TestVoyagesKeepTheirPlaceAlongTheRun(t *testing.T) {
	clock := time.Date(2026, 9, 18, 14, 0, 0, 0, time.UTC)
	s := voyageScene(t, &clock)
	s.SetSnapshot(snapshot(session("a", cinders, state.Working)))
	clock = clock.Add(time.Minute)
	s.SetSnapshot(snapshot(session("a", cinders, state.Working), session("b", cinders, state.Working)))
	require.NotEmpty(t, s.Voyages())

	t.Run("when the map moves mid-voyage", func(t *testing.T) {
		clock = clock.Add(city.SailFor / 2)
		before := s.Voyages()[0].Progress(clock)
		// A THIRD session, so the city re-plans and the river moves
		// while b is still on its way in. Removing b would move the
		// river too, but it would also end b's arrival, and then this
		// would be measuring the turn-round rather than the re-anchor.
		s.SetSnapshot(snapshot(
			session("a", cinders, state.Working),
			session("b", cinders, state.Working),
			session("c", cinders, state.Working)))

		t.Run("it should not send the tug back to the start", func(t *testing.T) {
			// Re-anchoring must not reset the clock: the tug belongs
			// half way down the new river, not at the top of it.
			var b *city.Voyage
			for _, v := range s.Voyages() {
				if v.SessionID == "b" {
					b = v
				}
			}
			require.NotNil(t, b)
			assert.InDelta(t, before, b.Progress(clock), 1e-9)
		})
	})
}

// Bug 54. Aria: "it shouldn't just despawn, it should run the whole
// length of the water or turn off the map or something."
//
// An arrival was dropped at SailFor+BuildFor wherever it floated. For a
// session whose building rises, the rising covers it. For one that goes
// away before it docks — every probe, and any short-lived session — the
// tug reached mid-river and evaporated, and a *second* tug was launched
// for the departure. Turning the arrival round instead is both one boat
// fewer and the honest animation: the session left, so its tug leaves.

func TestAVoyageWhoseSessionGoesSailsOnRatherThanVanishing(t *testing.T) {
	clock := time.Date(2026, 9, 18, 14, 0, 0, 0, time.UTC)
	s := voyageScene(t, &clock)
	s.SetSnapshot(snapshot(session("a", cinders, state.Working)))
	clock = clock.Add(time.Minute)
	s.SetSnapshot(snapshot(session("a", cinders, state.Working), session("b", cinders, state.Working)))
	require.Len(t, s.Voyages(), 1)

	t.Run("when the session goes while its tug is still arriving", func(t *testing.T) {
		clock = clock.Add(city.SailFor / 2)
		s.SetSnapshot(snapshot(session("a", cinders, state.Working)))
		tugs := s.Voyages()

		t.Run("it should turn that tug round rather than launch a second", func(t *testing.T) {
			require.Len(t, tugs, 1)
			assert.Equal(t, city.Departure, tugs[0].Kind)
		})

		t.Run("it should set out from where the tug had got to, not from the river's head", func(t *testing.T) {
			north, _, _, _ := s.City().RiverBand()
			assert.Greater(t, math.Abs(tugs[0].From.Y-north.Y), 1.0)
		})

		t.Run("and it has sailed its time", func(t *testing.T) {
			clock = clock.Add(city.SailFor)

			t.Run("it should have left by the south end rather than vanished mid-river", func(t *testing.T) {
				_, south := s.City().Island().Mouths()
				last := tugs[0].At(clock)
				assert.InDelta(t, south.Y, last.Y, 1.0)
			})
		})
	})
}
