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

// grouped is served with the sessions' groups in the snapshot, each
// group's members taken from the sessions that name it.
func grouped(t *testing.T, sessions ...state.Session) *city.Scene {
	t.Helper()
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return now }, time.UTC)
	s.Resize(800, 600)
	var groups []state.Group
	index := map[string]int{}
	for _, sess := range sessions {
		if sess.Group == "" {
			continue
		}
		i, ok := index[sess.Group]
		if !ok {
			i = len(groups)
			index[sess.Group] = i
			groups = append(groups, state.Group{ID: sess.Group})
		}
		groups[i].Members = append(groups[i].Members, sess.ID)
	}
	s.SetSnapshot(state.Snapshot{At: now, Sessions: sessions, Groups: groups, Servers: state.Servers(sessions, claude.MCPConfig{})})
	return s
}

func member(id, cwd, group string, calls map[string]int) state.Session {
	s := caller(id, cwd, calls)
	s.Group = group
	return s
}

// Item 359: the pointer on a grouped building lights its whole group.
func TestHighlightGroup(t *testing.T) {
	a := member("a", cinders, "g1", map[string]int{"playwright": 3})
	a2 := member("a2", cinders, "g1", map[string]int{"atlassian": 1, "playwright": 2})
	b := caller("b", botropolis, map[string]int{"atlassian": 4})

	t.Run("when the pointer is on a grouped building", func(t *testing.T) {
		s := grouped(t, a, a2, b)
		s.SetHover(city.Hit{Building: buildingOf(t, s, "a")})
		h := s.Highlight()

		t.Run("it should be active", func(t *testing.T) {
			assert.True(t, h.Active)
		})

		t.Run("it should light every building of its group", func(t *testing.T) {
			assert.True(t, h.HasBuilding(buildingOf(t, s, "a2")))
		})

		t.Run("it should leave out a building outside its group", func(t *testing.T) {
			assert.False(t, h.HasBuilding(buildingOf(t, s, "b")))
		})

		t.Run("it should light the towers it calls", func(t *testing.T) {
			assert.True(t, h.HasTower(towerNamed(t, s, "playwright")))
		})

		t.Run("it should leave out a tower only another member calls", func(t *testing.T) {
			assert.False(t, h.HasTower(towerNamed(t, s, "atlassian")))
		})

		own, _ := beamsOf(t, s, "playwright")
		lit := map[string]bool{}
		for _, beam := range own {
			lit[beam.Building.Session.ID] = h.HasBeam(beam)
		}

		t.Run("it should light its line to a tower it calls", func(t *testing.T) {
			assert.True(t, lit["a"])
		})

		t.Run("it should leave out that tower's line to another member", func(t *testing.T) {
			assert.False(t, lit["a2"])
		})
	})

	t.Run("when the pointer is on a grouped building that calls nothing", func(t *testing.T) {
		s := grouped(t, member("a", cinders, "g1", nil), a2, b)
		s.SetHover(city.Hit{Building: buildingOf(t, s, "a")})

		t.Run("it should still light its group", func(t *testing.T) {
			assert.True(t, s.Highlight().HasBuilding(buildingOf(t, s, "a2")))
		})
	})

	t.Run("when the pointer is on the building of a one-member group that calls nothing", func(t *testing.T) {
		s := grouped(t, member("a", cinders, "g1", nil), b)
		s.SetHover(city.Hit{Building: buildingOf(t, s, "a")})

		t.Run("it should not be active, so a lone building never fades the city", func(t *testing.T) {
			assert.False(t, s.Highlight().Active)
		})
	})

	t.Run("when the pointer is on a building in no group", func(t *testing.T) {
		s := grouped(t, a, a2, b)
		s.SetHover(city.Hit{Building: buildingOf(t, s, "b")})
		h := s.Highlight()

		t.Run("it should leave out the grouped buildings", func(t *testing.T) {
			assert.False(t, h.HasBuilding(buildingOf(t, s, "a")))
		})

		t.Run("it should light the towers it calls", func(t *testing.T) {
			assert.True(t, h.HasTower(towerNamed(t, s, "atlassian")))
		})
	})

	t.Run("when a grouped building is selected and the pointer is on nothing", func(t *testing.T) {
		s := grouped(t, a, a2, b)
		require.True(t, s.Select("a"))
		h := s.Highlight()

		t.Run("it should light it", func(t *testing.T) {
			assert.True(t, h.HasBuilding(buildingOf(t, s, "a")))
		})

		t.Run("it should not light the rest of its group", func(t *testing.T) {
			assert.False(t, h.HasBuilding(buildingOf(t, s, "a2")))
		})
	})
}
