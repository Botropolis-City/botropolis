package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestFocusGroup(t *testing.T) {
	focused := func(t *testing.T) (*city.Scene, float64) {
		t.Helper()
		member := func(id, cwd string) state.Session {
			s := session(id, cwd, state.Working)
			s.Group = "g1"
			return s
		}
		snap := snapshot(
			member("a", cinders),
			member("b", botropolis),
			session("c", "/home/avesta/workspaces/github/mCedar/lanterns", state.Working),
			session("d", "/home/avesta/workspaces/github/mCedar/orchard", state.Working),
		)
		snap.Groups = []state.Group{{ID: "g1", Name: "PROJ-1", Members: []string{"a", "b"}}}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snap)
		before := s.Camera().Zoom
		require.True(t, s.FocusGroup("g1"))
		return s, before
	}
	building := func(s *city.Scene, id string) *city.Building {
		for _, b := range s.City().Buildings() {
			if b.Session.ID == id {
				return b
			}
		}
		return nil
	}

	t.Run("when a group is focused", func(t *testing.T) {
		s, before := focused(t)
		lit := s.Highlight()

		t.Run("it should light its members", func(t *testing.T) {
			assert.True(t, lit.HasBuilding(building(s, "a")) && lit.HasBuilding(building(s, "b")))
		})

		t.Run("it should recede the rest", func(t *testing.T) {
			assert.True(t, lit.Active && !lit.HasBuilding(building(s, "c")))
		})

		t.Run("it should frame the camera closer, on its districts", func(t *testing.T) {
			assert.Greater(t, s.Camera().Zoom, before)
		})

		t.Run("it should keep its members on screen", func(t *testing.T) {
			for _, id := range []string{"a", "b"} {
				p := centreOf(t, s, id)
				assert.True(t, p.X >= 0 && p.X <= 800 && p.Y >= 0 && p.Y <= 600, id)
			}
		})

		t.Run("and the focus is cleared", func(t *testing.T) {
			s, _ := focused(t)
			s.ClearFocus()

			t.Run("it should light nothing", func(t *testing.T) {
				assert.False(t, s.Highlight().Active)
			})
		})
	})

	t.Run("when the group runs through one small district", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working), session("b", botropolis, state.Working))
		snap := snapshot(session("a", cinders, state.Working), session("b", botropolis, state.Working))
		snap.Sessions[0].Group = "g1"
		snap.Groups = []state.Group{{ID: "g1", Members: []string{"a"}}}
		s.SetSnapshot(snap)
		require.True(t, s.FocusGroup("g1"))

		t.Run("it should not zoom in past the focus limit", func(t *testing.T) {
			assert.LessOrEqual(t, s.Camera().Zoom, city.FocusMaxZoom)
		})
	})

	t.Run("when the group does not exist", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))

		t.Run("it should not focus", func(t *testing.T) {
			assert.False(t, s.FocusGroup("missing"))
		})
	})
}
