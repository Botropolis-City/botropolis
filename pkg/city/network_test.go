package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestNetworksPerView(t *testing.T) {
	// Bug 48 changed this. The base view used to draw no network at
	// all, on the subtractive principle that a view says something by
	// taking things away. Aria asked for the cars back, and traffic is
	// the one network that can answer for itself: a car's hover card
	// names the two projects and what passes between them, so it is not
	// the unexplained motion the principle was guarding against.
	t.Run("when Attention is up", func(t *testing.T) {
		t.Run("it should draw the traffic, which explains itself on hover", func(t *testing.T) {
			require.True(t, city.ViewAttention.Shows(city.NetworkTraffic))
		})

		t.Run("it should draw no other network, so the view stays quiet", func(t *testing.T) {
			for _, n := range city.Networks {
				if n == city.NetworkTraffic {
					continue
				}
				require.False(t, city.ViewAttention.Shows(n), n.String())
			}
		})
	})

	owns := map[city.View][]city.Network{
		city.ViewSpend:   {city.NetworkWires, city.NetworkFreight},
		city.ViewServers: {city.NetworkBeams},
		city.ViewTraffic: {city.NetworkTraffic},
		city.ViewFanout:  {city.NetworkCranes},
		city.ViewHealth:  {city.NetworkWires},
	}
	for v, wanted := range owns {
		t.Run("when "+v.Name()+" is up", func(t *testing.T) {
			for _, n := range wanted {
				t.Run("it should draw the "+n.String(), func(t *testing.T) {
					assert.True(t, v.Shows(n))
				})
			}

			t.Run("it should draw nothing else", func(t *testing.T) {
				for _, n := range city.Networks {
					want := false
					for _, w := range wanted {
						if w == n {
							want = true
						}
					}
					require.Equal(t, want, v.Shows(n), n.String())
				}
			})
		})
	}

	t.Run("when a view counts something with no network of its own", func(t *testing.T) {
		t.Run("it should draw none, so the map is only as busy as the question", func(t *testing.T) {
			for _, v := range []city.View{city.ViewPressure, city.ViewStaleness, city.ViewModels} {
				for _, n := range city.Networks {
					require.False(t, v.Shows(n), v.Name()+"/"+n.String())
				}
			}
		})
	})

	t.Run("when health is up and a session has never been heard from", func(t *testing.T) {
		t.Run("it should still draw the wires, because the missing one is the signal", func(t *testing.T) {
			assert.True(t, city.ViewHealth.Shows(city.NetworkWires))
		})
	})
}

func TestCarrierTint(t *testing.T) {
	busy := session("busy", cinders, state.Working)
	quiet := session("quiet", botropolis, state.Working)

	t.Run("when spend is up", func(t *testing.T) {
		s := viewed(t, city.ViewSpend, busy, quiet)
		hot := city.PowerLine{Fresh: 90, Cached: 10}
		cold := city.PowerLine{Fresh: 45, Cached: 5}

		t.Run("it should put the busiest wire at the top of the ramp", func(t *testing.T) {
			assert.InDelta(t, 1.0, s.WireTint(hot, []city.PowerLine{hot, cold}).Value, 1e-9)
		})

		t.Run("it should put a wire carrying half as much half way", func(t *testing.T) {
			assert.InDelta(t, 0.5, s.WireTint(cold, []city.PowerLine{hot, cold}).Value, 1e-9)
		})
	})

	t.Run("when servers is up", func(t *testing.T) {
		s := viewed(t, city.ViewServers, busy)
		hot := city.Beam{Calls: 20}
		cold := city.Beam{Calls: 5}

		t.Run("it should place a beam by what it carries", func(t *testing.T) {
			assert.InDelta(t, 0.25, s.BeamTint(cold, []city.Beam{hot, cold}).Value, 1e-9)
		})
	})

	t.Run("when traffic is up", func(t *testing.T) {
		s := viewed(t, city.ViewTraffic, busy)
		hot := city.RoadLine{Messages: 30, Files: 10}
		cold := city.RoadLine{Messages: 10, Files: 0}

		t.Run("it should place a road by its traffic", func(t *testing.T) {
			assert.InDelta(t, 0.25, s.RoadTint(cold, []city.RoadLine{hot, cold}).Value, 1e-9)
		})
	})

	t.Run("when the view does not own the carrier", func(t *testing.T) {
		s := viewed(t, city.ViewSpend, busy)
		beam := city.Beam{Calls: 20}

		t.Run("it should say it has nothing to say about it", func(t *testing.T) {
			assert.False(t, s.BeamTint(beam, []city.Beam{beam}).Known)
		})
	})

	t.Run("when nothing on a network is carrying anything", func(t *testing.T) {
		s := viewed(t, city.ViewServers, busy)
		empty := city.Beam{}

		t.Run("it should not divide by nothing", func(t *testing.T) {
			assert.InDelta(t, 0.0, s.BeamTint(empty, []city.Beam{empty}).Value, 1e-9)
		})
	})
}

// A road only exists when a session in one repo touches a file in
// another, which is rarer than it sounds — neither the unit fixtures nor
// the sample home has one. This builds the case on purpose so the
// Traffic view's carrier is exercised against a city rather than only
// against a literal.
func TestTrafficPaintsARealRoad(t *testing.T) {
	crosser := session("crosser", cinders, state.Working)
	crosser.Touches = map[string]int{botropolis + "/pkg/city/view.go": 7}
	neighbour := session("neighbour", botropolis, state.Working)

	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return now }, time.UTC)
	s.Resize(800, 600)
	s.SetSnapshot(state.Snapshot{
		At:       now,
		Sessions: []state.Session{crosser, neighbour},
		Roads:    state.Roads([]state.Session{crosser, neighbour}, nil),
	})
	s.SetView(city.ViewTraffic)

	t.Run("when one repo has been reaching into another", func(t *testing.T) {
		t.Run("it should have a road to paint", func(t *testing.T) {
			require.NotEmpty(t, s.City().Roads)
		})

		t.Run("it should put that road on the ramp", func(t *testing.T) {
			require.True(t, s.RoadTint(s.City().Roads[0], s.City().Roads).Known)
		})

		t.Run("it should hang that road on a street the renderer walks", func(t *testing.T) {
			found := false
			for i := range s.City().Streets {
				if s.City().Streets[i].Road != nil {
					found = true
				}
			}
			assert.True(t, found)
		})
	})
}

func TestLegendSaysWhenThereIsNothingToScale(t *testing.T) {
	// Fan-out with nobody spawning anything: the ramp's far end is zero,
	// so a gradient from 0 to 0 would be a scale that means nothing.
	t.Run("when a ramp view has a top of nothing", func(t *testing.T) {
		quiet := session("quiet", cinders, state.Working)
		quiet.SubagentsInFlight = 0
		l := viewed(t, city.ViewFanout, quiet).Legend()

		t.Run("it should say so in words", func(t *testing.T) {
			assert.Equal(t, "no subagents in flight", l.Note)
		})

		t.Run("it should not offer a bar to read it off", func(t *testing.T) {
			assert.False(t, l.Ramp)
		})
	})

	t.Run("when a ramp view has something to measure", func(t *testing.T) {
		busy := session("busy", cinders, state.Working)
		busy.SubagentsInFlight = 4
		l := viewed(t, city.ViewFanout, busy).Legend()

		t.Run("it should offer the bar", func(t *testing.T) {
			assert.True(t, l.Ramp)
		})

		t.Run("it should leave the note off", func(t *testing.T) {
			assert.Empty(t, l.Note)
		})
	})

	t.Run("when staleness names its far end", func(t *testing.T) {
		t.Run("it should say it the way a person would, not the way a Duration does", func(t *testing.T) {
			assert.Equal(t, "4h quiet", viewed(t, city.ViewStaleness, session("a", cinders, state.Waiting)).Legend().High)
		})
	})
}

func TestHealthCountsTheWires(t *testing.T) {
	// The wire is the daemon's live line to a session. Its absence means
	// the numbers beside that building came from files rather than from
	// hook events, and that is the datum Health inherited when the wires
	// left Attention. Drawing them is not enough on its own: a reader
	// has to be told that the missing ones mean something.
	hooked := session("hooked", cinders, state.Working)
	hooked.Hooked = true
	dark := session("dark", botropolis, state.Working)
	dark.Hooked = false

	t.Run("when health is up and some sessions are not on the wire", func(t *testing.T) {
		s := viewed(t, city.ViewHealth, hooked, dark)

		t.Run("it should say how many are", func(t *testing.T) {
			assert.Equal(t, "1 of 2 on the wire", s.Legend().Aside)
		})
	})

	t.Run("when every session is on the wire", func(t *testing.T) {
		other := session("other", botropolis, state.Working)
		other.Hooked = true
		s := viewed(t, city.ViewHealth, hooked, other)

		t.Run("it should say nothing, because there is nothing missing to point at", func(t *testing.T) {
			assert.Empty(t, s.Legend().Aside)
		})
	})

	t.Run("when a view other than health is up", func(t *testing.T) {
		s := viewed(t, city.ViewSpend, hooked, dark)

		t.Run("it should not borrow health's aside", func(t *testing.T) {
			assert.Empty(t, s.Legend().Aside)
		})
	})
}

func TestNothingToScaleTintsNothing(t *testing.T) {
	// A city where nobody has spawned a subagent: the ramp's far end is
	// zero, so painting every building the darkest stop says "all at the
	// bottom" of a scale that does not exist. Receding says the same
	// thing with less ink, and the legend says it in words.
	quiet := session("quiet", cinders, state.Working)
	quiet.SubagentsInFlight = 0

	t.Run("when a ramp view has nothing to measure", func(t *testing.T) {
		s := viewed(t, city.ViewFanout, quiet)

		t.Run("it should leave the city receded rather than paint it all at zero", func(t *testing.T) {
			assert.False(t, s.Tint(buildingOf(t, s, "quiet")).Known)
		})
	})
}
