package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 350: a right-click on a district opens its own verbs, which act
// on the district's directory with no session selected.
func TestDistrictVerbs(t *testing.T) {
	district := func(t *testing.T, s *city.Scene, root string) *city.District {
		t.Helper()
		for _, d := range s.City().Districts {
			if d.Root == root {
				return d
			}
		}
		require.Failf(t, "no district", "%s", root)
		return nil
	}

	t.Run("when a district is asked for its verbs", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working))

		t.Run("it should offer the project's verbs in the columns' order", func(t *testing.T) {
			assert.Equal(t, []city.ActionKind{city.ActionNew, city.ActionReveal, city.ActionCopyPath, city.ActionStar, city.ActionHide, city.ActionDistrictType}, s.DistrictVerbs(district(t, s, cinders)))
		})

		t.Run("and it is starred", func(t *testing.T) {
			s.ActOnDistrict(district(t, s, cinders), city.ActionStar)
			kinds := s.DistrictVerbs(district(t, s, cinders))

			t.Run("it should offer unstar, not star", func(t *testing.T) {
				assert.True(t, contains(kinds, city.ActionUnstar) && !contains(kinds, city.ActionStar))
			})
		})
	})

	for _, c := range []struct {
		kind city.ActionKind
		want city.Action
	}{
		{city.ActionCopyPath, city.Action{Kind: city.ActionCopyPath, Dir: cinders}},
		{city.ActionReveal, city.Action{Kind: city.ActionReveal, Dir: cinders}},
		{city.ActionNew, city.Action{Kind: city.ActionNew, Dir: cinders}},
	} {
		t.Run("when "+string(c.kind)+" is acted on a district", func(t *testing.T) {
			s := scene(t, session("a", cinders+"/pkg", state.Working))
			action, _ := s.ActOnDistrict(district(t, s, cinders+"/pkg"), c.kind)

			t.Run("it should act on the district's root", func(t *testing.T) {
				want := c.want
				want.Dir = cinders + "/pkg"
				assert.Equal(t, want, action)
			})
		})
	}

	t.Run("when hide is acted on a district", func(t *testing.T) {
		s := scene(t, session("a", cinders, state.Working), session("b", botropolis, state.Working))
		s.ActOnDistrict(district(t, s, cinders), city.ActionHide)

		t.Run("it should hide it in the layout", func(t *testing.T) {
			assert.True(t, s.Layout().IsHidden(cinders))
		})
	})

	t.Run("when the depot is asked for its verbs", func(t *testing.T) {
		crated := session("b", cinders, state.Resting)
		crated.Packed = true
		s := scene(t, session("a", cinders, state.Working), crated)
		var depot *city.District
		for _, d := range s.City().Districts {
			if d.Depot {
				depot = d
			}
		}
		require.NotNil(t, depot)

		t.Run("it should offer nothing", func(t *testing.T) {
			assert.Empty(t, s.DistrictVerbs(depot))
		})
	})
}

func contains(kinds []city.ActionKind, k city.ActionKind) bool {
	for _, x := range kinds {
		if x == k {
			return true
		}
	}
	return false
}
