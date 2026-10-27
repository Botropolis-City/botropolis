package city_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 270: a frame asks every building for its inspector, so asking
// one must not cost a pass over every building.
const scaleBuildings, scaleDistricts = 600, 200

func scaleSessions(open string) []state.Session {
	sessions := make([]state.Session, 0, scaleBuildings)
	for i := range scaleBuildings {
		id := fmt.Sprintf("scale-%04d", i)
		s := session(id, fmt.Sprintf("/w/scale%03d", i%scaleDistricts), state.Working)
		s.SubagentsInFlight = 0
		if id == open {
			s.PRs = []claude.PR{{Number: 1, URL: "https://example.com/o/r/pull/1", Repository: "o/r", State: claude.PROpen}}
		}
		sessions = append(sessions, s)
	}
	return sessions
}

func TestBuildingsOfABuiltCity(t *testing.T) {
	t.Run("when a city of several districts is built", func(t *testing.T) {
		c := city.Build(snapshot(scaleSessions("")...), city.NewLayout())
		var want []string
		for _, d := range c.Districts {
			for _, b := range d.Buildings {
				want = append(want, b.Session.ID)
			}
		}
		var got []string
		for _, b := range c.Buildings() {
			got = append(got, b.Session.ID)
		}

		t.Run("it should list every district's buildings, in district order", func(t *testing.T) {
			assert.Equal(t, want, got)
		})

		t.Run("and Buildings is asked again", func(t *testing.T) {
			t.Run("it should allocate nothing", func(t *testing.T) {
				assert.Zero(t, testing.AllocsPerRun(10, func() { _ = c.Buildings() }))
			})
		})
	})
}

func TestInspectorAtScale(t *testing.T) {
	open := func(parked string) *city.Scene {
		start := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
		s := city.NewScene(city.NewLayout())
		s.Resize(800, 600)
		s.SetClock(func() time.Time { return start.Add(time.Minute) }, time.UTC)
		s.SetSnapshot(snapshot(scaleSessions(parked)...))
		return s
	}
	every := func(s *city.Scene) (found []string) {
		for _, b := range s.City().Buildings() {
			if _, ok := s.Inspector(b.Session.ID); ok {
				found = append(found, b.Session.ID)
			}
		}
		return found
	}

	t.Run("when a scene of many buildings has nothing parked", func(t *testing.T) {
		s := open("")

		t.Run("and every building's inspector is asked for, as a frame does", func(t *testing.T) {
			t.Run("it should allocate nothing", func(t *testing.T) {
				assert.Zero(t, testing.AllocsPerRun(5, func() { _ = every(s) }))
			})

			t.Run("it should find none", func(t *testing.T) {
				assert.Empty(t, every(s))
			})
		})
	})

	t.Run("when one building of many is parked", func(t *testing.T) {
		s := open("scale-0421")

		t.Run("it should find that building's inspector, and no other building's", func(t *testing.T) {
			assert.Equal(t, []string{"scale-0421"}, every(s))
		})
	})
}

func TestDistrictOfAtScale(t *testing.T) {
	t.Run("when a city of 600 buildings in 200 districts is built", func(t *testing.T) {
		c := city.Build(snapshot(scaleSessions("")...), city.NewLayout())
		var want, got []*city.District
		for _, d := range c.Districts {
			for range d.Buildings {
				want = append(want, d)
			}
		}
		before := c.DistrictsLookedAt()
		for _, d := range c.Districts {
			for _, b := range d.Buildings {
				got = append(got, c.DistrictOf(b))
			}
		}
		looked := c.DistrictsLookedAt() - before

		t.Run("it should find each building's district, as the district-by-district scan does", func(t *testing.T) {
			assert.Equal(t, want, got)
		})

		t.Run("it should look at one entry to find each", func(t *testing.T) {
			assert.LessOrEqual(t, looked, len(got))
		})
	})
}

func TestDistrictSignOnceBuilt(t *testing.T) {
	t.Run("when a built city's district is asked for its sign a second time", func(t *testing.T) {
		c := city.Build(snapshot(scaleSessions("")...), city.NewLayout())
		d := c.Districts[0]
		_ = d.Sign()

		t.Run("it should allocate nothing", func(t *testing.T) {
			assert.Zero(t, testing.AllocsPerRun(10, func() { _ = d.Sign() }))
		})
	})
}

// Item 270: the chrome asks for the triage walk every frame, and it
// looked at every building in the city to find the few waiting on you.
func TestTriageWalkAtScale(t *testing.T) {
	at := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	open := func(layout *city.Layout) *city.Scene {
		sessions := scaleSessions("")
		for k, i := range []int{17, 230, 511} {
			sessions[i].State = state.NeedsYou
			sessions[i].LastActivity = at.Add(-time.Duration(k+1) * time.Hour)
		}
		s := city.NewScene(layout)
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(sessions...))
		return s
	}
	ids := func(walk []*city.Building) []string {
		var out []string
		for _, b := range walk {
			out = append(out, b.Session.ID)
		}
		return out
	}

	t.Run("when a city of 600 buildings has three sessions waiting on you", func(t *testing.T) {
		s := open(city.NewLayout())
		before := s.TriageLooks()
		walk := ids(s.TriageWalk())
		looked := s.TriageLooks() - before

		t.Run("it should walk those three, the longest waiting first, as the walk over every building does", func(t *testing.T) {
			assert.Equal(t, ids(s.TriageWalkEveryBuilding()), walk)
		})

		t.Run("it should look at no building that is not waiting on you", func(t *testing.T) {
			assert.LessOrEqual(t, looked, 3)
		})
	})

	t.Run("when one of the three is set aside", func(t *testing.T) {
		layout := city.NewLayout()
		layout.Snoozed = map[string]time.Time{"scale-0230": at.Add(time.Hour)}
		s := open(layout)

		t.Run("it should walk the other two", func(t *testing.T) {
			assert.Equal(t, []string{"scale-0511", "scale-0017"}, ids(s.TriageWalk()))
		})
	})
}

// Item 280: the clinic's count is asked for every frame, by the standing
// key and its plate, so it looks only at the buildings the city listed
// as needing you or in trouble when it was built, as triage does, and
// agrees with a walk over every building.
func TestClinicCountAtScale(t *testing.T) {
	at := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	mixed := func() []state.Session {
		sessions := scaleSessions("")
		for k, i := range []int{17, 230, 511} {
			sessions[i].State = state.NeedsYou
			sessions[i].LastActivity = at.Add(-time.Duration(k+1) * time.Hour)
		}
		sessions[42].APIErrors, sessions[42].LastErrorAt = 1, sessions[42].LastActivity
		sessions[43].APIErrors, sessions[43].LastErrorAt = 1, sessions[43].LastActivity.Add(-time.Minute)
		sessions[99].FailedTools = city.FailedToolsInARow
		sessions[100].FailedTools = city.FailedToolsInARow - 1
		for i, ago := range map[int]time.Duration{300: time.Hour, 301: city.StoppedFor, 302: city.StoppedFor + time.Second} {
			sessions[i].State, sessions[i].StoppedMidTurn, sessions[i].LastActivity = state.Resting, true, at.Add(-ago)
		}
		sessions[303].State, sessions[303].StoppedMidTurn, sessions[303].Packed = state.Resting, true, true
		return sessions
	}
	open := func(layout *city.Layout, clock *time.Time) *city.Scene {
		s := city.NewScene(layout)
		s.SetClock(func() time.Time { return *clock }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snapshot(mixed()...))
		return s
	}

	t.Run("when a city of 600 buildings has three needing you and four in trouble, with the stopped sessions about the day's edge", func(t *testing.T) {
		clock := at
		s := open(city.NewLayout(), &clock)
		before := s.ClinicLooks()
		count := s.ClinicCount()
		looked := s.ClinicLooks() - before

		t.Run("it should count those seven", func(t *testing.T) {
			assert.Equal(t, 7, count)
		})

		t.Run("it should count what a walk over every building counts", func(t *testing.T) {
			assert.Equal(t, s.ClinicCountEveryBuilding(), count)
		})

		t.Run("it should count what its menu lists", func(t *testing.T) {
			assert.Len(t, s.Clinic(), count)
		})

		t.Run("it should look at no building that is neither", func(t *testing.T) {
			assert.LessOrEqual(t, looked, 9)
		})

		t.Run("and a second passes, so a stopped session leaves the day", func(t *testing.T) {
			clock = at.Add(time.Second)

			t.Run("it should still count what a walk over every building counts", func(t *testing.T) {
				assert.Equal(t, s.ClinicCountEveryBuilding(), s.ClinicCount())
			})

			t.Run("it should count one fewer", func(t *testing.T) {
				assert.Equal(t, 6, s.ClinicCount())
			})
		})
	})

	t.Run("when one needing you is snoozed", func(t *testing.T) {
		layout := city.NewLayout()
		layout.Snoozed = map[string]time.Time{"scale-0230": at.Add(time.Hour)}
		clock := at
		s := open(layout, &clock)

		t.Run("it should count what a walk over every building counts", func(t *testing.T) {
			assert.Equal(t, s.ClinicCountEveryBuilding(), s.ClinicCount())
		})
	})
}
