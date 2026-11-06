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

// served is a scene whose snapshot carries the towers the sessions' own
// calls imply, which is how the daemon builds it.
func served(t *testing.T, sessions ...state.Session) *city.Scene {
	t.Helper()
	return servedOn(t, city.NewScene(city.NewLayout()), sessions...)
}

// servedOn hands a scene the snapshot served would, so a setting can
// be made before the city is built.
func servedOn(t *testing.T, s *city.Scene, sessions ...state.Session) *city.Scene {
	t.Helper()
	s.SetClock(func() time.Time { return now }, time.UTC)
	s.Resize(800, 600)
	s.SetSnapshot(state.Snapshot{At: now, Sessions: sessions, Servers: state.Servers(sessions, claude.MCPConfig{})})
	return s
}

// caller builds a session that leans on the named servers.
func caller(id, cwd string, calls map[string]int) state.Session {
	s := session(id, cwd, state.Working)
	s.MCPCalls = calls
	return s
}

func towerNamed(t *testing.T, s *city.Scene, name string) *city.Tower {
	t.Helper()
	for _, tw := range s.City().Towers {
		if tw.Server.Name == name {
			return tw
		}
	}
	t.Fatalf("no tower for %s", name)
	return nil
}

func TestHighlight(t *testing.T) {
	// Two sessions on two servers, sharing one of them, so "what breaks
	// if this goes down" has a real answer either way.
	both := caller("both", cinders, map[string]int{"playwright": 9, "atlassian": 2})
	only := caller("only", botropolis, map[string]int{"atlassian": 5})

	t.Run("when the pointer is on nothing", func(t *testing.T) {
		s := served(t, both, only)

		t.Run("it should not be active, so the map is not dimmed for no reason", func(t *testing.T) {
			assert.False(t, s.Highlight().Active)
		})
	})

	t.Run("when a server's tower is under the pointer", func(t *testing.T) {
		s := served(t, both, only)
		tower := towerNamed(t, s, "playwright")
		s.SetHover(city.Hit{Tower: tower})
		h := s.Highlight()

		t.Run("it should be active", func(t *testing.T) {
			require.True(t, h.Active)
		})

		t.Run("it should light the session that calls that server", func(t *testing.T) {
			assert.True(t, h.HasBuilding(buildingOf(t, s, "both")))
		})

		t.Run("it should leave out the session that does not", func(t *testing.T) {
			assert.False(t, h.HasBuilding(buildingOf(t, s, "only")))
		})

		t.Run("it should light the tower itself, so the thing you asked about stays lit", func(t *testing.T) {
			assert.True(t, h.HasTower(tower))
		})
	})

	t.Run("when a session's building is under the pointer", func(t *testing.T) {
		s := served(t, both, only)
		b := buildingOf(t, s, "only")
		s.SetHover(city.Hit{Building: b})
		h := s.Highlight()

		t.Run("it should light that building", func(t *testing.T) {
			assert.True(t, h.HasBuilding(b))
		})

		t.Run("it should light the towers it calls", func(t *testing.T) {
			assert.True(t, h.HasTower(towerNamed(t, s, "atlassian")))
		})

		t.Run("it should leave out a tower it never calls", func(t *testing.T) {
			assert.False(t, h.HasTower(towerNamed(t, s, "playwright")))
		})
	})

	t.Run("when a session is selected and the pointer has moved away", func(t *testing.T) {
		s := served(t, both, only)
		b := buildingOf(t, s, "only")
		require.True(t, s.Select("only"))
		h := s.Highlight()

		t.Run("it should keep the answer on screen rather than lose it with the pointer", func(t *testing.T) {
			assert.True(t, h.HasBuilding(b))
		})
	})

	t.Run("when the pointer is on a district", func(t *testing.T) {
		s := served(t, both, only)
		b := buildingOf(t, s, "both")
		s.SetHover(city.Hit{District: s.City().DistrictOf(b)})
		h := s.Highlight()

		t.Run("it should light the buildings that sit in it", func(t *testing.T) {
			assert.True(t, h.HasBuilding(b))
		})
	})
}

func TestHighlightStaysOutOfTheWay(t *testing.T) {
	lonely := caller("lonely", cinders, nil)
	busy := caller("busy", botropolis, map[string]int{"playwright": 3})

	t.Run("when a session that calls nothing is selected", func(t *testing.T) {
		s := served(t, lonely, busy)
		require.True(t, s.Select("lonely"))

		t.Run("it should not dim the city for an answer it does not have", func(t *testing.T) {
			assert.False(t, s.Highlight().Active)
		})
	})

	t.Run("when a session that calls nothing is hovered", func(t *testing.T) {
		s := served(t, lonely, busy)
		s.SetHover(city.Hit{Building: buildingOf(t, s, "lonely")})

		t.Run("it should stay out of the way there too", func(t *testing.T) {
			assert.False(t, s.Highlight().Active)
		})
	})

	t.Run("when a tower is hovered", func(t *testing.T) {
		s := served(t, lonely, busy)
		s.SetHover(city.Hit{Tower: towerNamed(t, s, "playwright")})

		t.Run("it should answer even so, because what leans on a server is worth asking either way", func(t *testing.T) {
			assert.True(t, s.Highlight().Active)
		})
	})
}

// beamsOf are the beams from the named tower, and the rest.
func beamsOf(t *testing.T, s *city.Scene, name string) (own, others []city.Beam) {
	t.Helper()
	for _, b := range s.City().Beams() {
		if b.Tower.Server.Name == name {
			own = append(own, b)
		} else {
			others = append(others, b)
		}
	}
	require.NotEmpty(t, own, "no beams from %s", name)
	require.NotEmpty(t, others, "no beams from any other tower")
	return own, others
}

// Item 347: the lines light with the thing pointed at, not only the
// buildings at their ends.
func TestHighlightBeams(t *testing.T) {
	both := caller("both", cinders, map[string]int{"playwright": 9, "atlassian": 2})
	only := caller("only", botropolis, map[string]int{"atlassian": 5})

	t.Run("when a server's tower is under the pointer", func(t *testing.T) {
		s := served(t, both, only)
		s.SetHover(city.Hit{Tower: towerNamed(t, s, "atlassian")})
		h := s.Highlight()
		own, others := beamsOf(t, s, "atlassian")

		t.Run("it should put each of that tower's beams in the highlight", func(t *testing.T) {
			for _, b := range own {
				assert.True(t, h.HasBeam(b), "beam to %s", b.Building.Session.ID)
			}
		})

		t.Run("it should leave out a beam from another tower", func(t *testing.T) {
			for _, b := range others {
				assert.False(t, h.HasBeam(b), "beam from %s to %s", b.Tower.Server.Name, b.Building.Session.ID)
			}
		})
	})

	t.Run("when a session's building is under the pointer", func(t *testing.T) {
		s := served(t, both, only)
		s.SetHover(city.Hit{Building: buildingOf(t, s, "only")})
		h := s.Highlight()
		own, _ := beamsOf(t, s, "atlassian")
		var ours, theirs *city.Beam
		for i := range own {
			if own[i].Building.Session.ID == "only" {
				ours = &own[i]
			} else {
				theirs = &own[i]
			}
		}
		require.NotNil(t, ours)
		require.NotNil(t, theirs)

		t.Run("it should put its beam to a tower it calls in the highlight", func(t *testing.T) {
			assert.True(t, h.HasBeam(*ours))
		})

		t.Run("it should leave out that tower's beam to another session", func(t *testing.T) {
			assert.False(t, h.HasBeam(*theirs))
		})
	})

	t.Run("when a district is under the pointer", func(t *testing.T) {
		s := served(t, both, only)
		s.SetHover(city.Hit{District: s.City().DistrictOf(buildingOf(t, s, "both"))})
		h := s.Highlight()
		require.True(t, h.Active)

		t.Run("it should put no beam in the highlight, so the lines fade with the towers", func(t *testing.T) {
			for _, b := range s.City().Beams() {
				assert.False(t, h.HasBeam(b), "beam from %s to %s", b.Tower.Server.Name, b.Building.Session.ID)
			}
		})
	})
}
