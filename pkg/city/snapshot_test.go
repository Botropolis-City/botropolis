package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// drawn is what of a city a snapshot lays out: its districts and their
// buildings, its lines and their stations, and its ground.
type drawn struct {
	Districts []drawnDistrict
	Lines     [][]drawnStation
	Parks     []city.Park
	Trees     []city.Tree
	Lamps     []city.Cell
	Extent    city.Rect
	// Streets are each road's path, and the cells laid and kept as
	// corridors (item 206).
	Streets     [][]city.Point
	StreetCells []city.StreetCell
	Corridors   []city.StreetCell
}

type drawnDistrict struct {
	Name      string
	Rect      city.Rect
	Buildings []drawnBuilding
}

type drawnBuilding struct {
	ID                string
	Rect              city.Rect
	State             state.State
	Lit               bool
	Station, Stations int
}

type drawnStation struct {
	Name  string
	At    city.Point
	Unlit bool
	Waits []string
}

func drawnOf(c *city.City) drawn {
	d := drawn{Parks: c.Parks, Trees: c.Trees, Lamps: c.Lamps, Extent: c.Extent(), StreetCells: c.StreetCells, Corridors: c.Corridors}
	for _, st := range c.Streets {
		d.Streets = append(d.Streets, st.Path)
	}
	for _, dist := range c.Districts {
		dd := drawnDistrict{Name: dist.Name, Rect: dist.Rect}
		for _, b := range dist.Buildings {
			dd.Buildings = append(dd.Buildings, drawnBuilding{ID: b.Session.ID, Rect: b.Rect, State: b.Session.State, Lit: b.Lit, Station: b.Station, Stations: b.Stations})
		}
		d.Districts = append(d.Districts, dd)
	}
	for _, l := range c.Lines {
		var stations []drawnStation
		for _, st := range l.Stations {
			stations = append(stations, drawnStation{Name: st.Name(), At: st.At, Unlit: st.Unlit, Waits: st.Waits})
		}
		d.Lines = append(d.Lines, stations)
	}
	return d
}

// Item 197: a snapshot lays out what changed in it, and nothing else.
func TestSnapshotLaysOutOnlyWhatChanged(t *testing.T) {
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	// shown is a scene showing a city the size of the real history,
	// then the snapshot change makes of it, with how many plans it made
	// and orders it read for that second snapshot.
	shown := func(t *testing.T, change func(*state.Snapshot)) (s *city.Scene, plans, orders int) {
		t.Helper()
		s, plans, orders, _ = shownRouting(t, at, change)
		return s, plans, orders
	}

	t.Run("when a city the size of the real history is shown, and the next snapshot moves only activity times (item 206)", func(t *testing.T) {
		s, plans, orders, routed := shownRouting(t, at, func(snap *state.Snapshot) {
			for i := range snap.Sessions {
				snap.Sessions[i].LastActivity = snap.Sessions[i].LastActivity.Add(time.Second)
			}
		})

		t.Run("it should not make the plan again", func(t *testing.T) {
			assert.Zero(t, plans)
		})

		t.Run("it should not route any street again", func(t *testing.T) {
			assert.Zero(t, routed)
		})

		t.Run("it should not read any group's order again", func(t *testing.T) {
			assert.Zero(t, orders)
		})

		t.Run("it should draw the city built from scratch", func(t *testing.T) {
			assert.Equal(t, drawnOf(s.FromScratch()), drawnOf(s.City()))
		})
	})

	t.Run("when a road joins two projects that had none between them (item 206)", func(t *testing.T) {
		from, to := "/home/user/src/project-001", "/home/user/src/project-002"
		for _, r := range realSized(at).Roads {
			require.False(t, (r.From == from && r.To == to) || (r.From == to && r.To == from), "the fixture already joins them")
		}
		s, _, _, routed := shownRouting(t, at, func(snap *state.Snapshot) {
			snap.Roads = append(snap.Roads, state.Road{From: from, To: to, Messages: 1, Sessions: 1})
		})

		t.Run("it should route that road and no other", func(t *testing.T) {
			assert.Equal(t, 1, routed)
		})

		t.Run("it should draw the city built from scratch", func(t *testing.T) {
			assert.Equal(t, drawnOf(s.FromScratch()), drawnOf(s.City()))
		})
	})

	t.Run("when the camera is turned a quarter (item 206)", func(t *testing.T) {
		s, _, _, _ := shownRouting(t, at, func(*state.Snapshot) {})
		s.Turn()
		s.SetSnapshot(realSized(at))

		t.Run("it should draw the city built from scratch facing that way", func(t *testing.T) {
			assert.Equal(t, drawnOf(s.FromScratch()), drawnOf(s.City()))
		})

		t.Run("and it has faced every way, and is turned again", func(t *testing.T) {
			s.Turn()
			s.Turn()
			s.Turn()
			before := s.StreetsRouted()
			s.Turn()

			t.Run("it should route no street again", func(t *testing.T) {
				assert.Equal(t, before, s.StreetsRouted())
			})
		})
	})

	t.Run("when a city the size of the real history is shown, and the next snapshot changes only one session's state", func(t *testing.T) {
		s, plans, orders, routed := shownRouting(t, at, func(snap *state.Snapshot) { snap.Sessions[4].State = state.Waiting })

		t.Run("it should not route any street again", func(t *testing.T) {
			assert.Zero(t, routed)
		})

		t.Run("it should not make the plan again", func(t *testing.T) {
			assert.Zero(t, plans)
		})

		t.Run("it should not read any group's order again", func(t *testing.T) {
			assert.Zero(t, orders)
		})

		t.Run("it should draw the city built from scratch", func(t *testing.T) {
			assert.Equal(t, drawnOf(s.FromScratch()), drawnOf(s.City()))
		})
	})

	t.Run("when the next snapshot shapes nothing new, as one carrying only the first read's progress does (item 159)", func(t *testing.T) {
		_, plans, orders, routed := shownRouting(t, at, func(*state.Snapshot) {})

		t.Run("it should not route any street again", func(t *testing.T) {
			assert.Zero(t, routed)
		})

		t.Run("it should not make the plan again", func(t *testing.T) {
			assert.Zero(t, plans)
		})

		t.Run("it should not read any group's order again", func(t *testing.T) {
			assert.Zero(t, orders)
		})
	})

	t.Run("when a session joins a new project", func(t *testing.T) {
		s, plans, _ := shown(t, func(snap *state.Snapshot) {
			joined := snap.Sessions[1]
			joined.ID, joined.CWD = "joined-0000-4000-8000-000000000000", "/home/user/src/a-new-project"
			snap.Sessions = append(snap.Sessions, joined)
		})

		t.Run("it should make the plan again", func(t *testing.T) {
			assert.Positive(t, plans)
		})

		t.Run("it should draw the city built from scratch", func(t *testing.T) {
			assert.Equal(t, drawnOf(s.FromScratch()), drawnOf(s.City()))
		})

		t.Run("it should hold no walk from the old plan", func(t *testing.T) {
			assert.Zero(t, s.StaleWalks())
		})
	})

	t.Run("when a session's order lines change", func(t *testing.T) {
		_, _, orders := shown(t, func(snap *state.Snapshot) {
			snap.Sessions[4].OrderLines = append(snap.Sessions[4].OrderLines, claude.OrderLine{Text: "a new line", At: at})
		})

		t.Run("it should read that group's order again, and no other", func(t *testing.T) {
			assert.Equal(t, 1, orders)
		})
	})

	t.Run("when a station waits on a site that is working", func(t *testing.T) {
		grouped := func(libState state.State) state.Snapshot {
			lib := session("a", cinders, libState)
			lib.Group, lib.StartedAt = "g1", now.Add(-time.Hour)
			api := session("b", botropolis, state.Resting)
			api.Group, api.StartedAt = "g1", now.Add(-3*time.Hour)
			api.Recap = claude.Recap{Text: "waits on cinders to finish", At: now}
			snap := snapshot(lib, api)
			snap.Groups = []state.Group{{ID: "g1", Name: "PROJ-1", Short: "PROJ-1", Members: []string{"a", "b"}}}
			return snap
		}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(grouped(state.Working))
		require.True(t, s.City().Lines[0].Stations[1].Unlit)

		t.Run("and the site it waits on stops", func(t *testing.T) {
			s.SetSnapshot(grouped(state.Resting))

			t.Run("the station should be lit again, as built from scratch", func(t *testing.T) {
				assert.Equal(t, drawnOf(s.FromScratch()), drawnOf(s.City()))
			})

			t.Run("the station should be lit", func(t *testing.T) {
				assert.False(t, s.City().Lines[0].Stations[1].Unlit)
			})
		})
	})
}

// shownRouting is a scene showing a city the size of the real history,
// then the snapshot change makes of it, with how many plans it made,
// orders it read and streets it routed for that last snapshot.
func shownRouting(t *testing.T, at time.Time, change func(*state.Snapshot)) (s *city.Scene, plans, orders, routed int) {
	t.Helper()
	s = city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return at }, time.UTC)
	s.Resize(1600, 900)
	s.SetSnapshot(realSized(at))
	s.SetSnapshot(realSized(at))
	plans0, orders0 := s.LayoutsDone()
	routed0 := s.StreetsRouted()
	next := realSized(at)
	change(&next)
	s.SetSnapshot(next)
	plans1, orders1 := s.LayoutsDone()
	return s, plans1 - plans0, orders1 - orders0, s.StreetsRouted() - routed0
}
