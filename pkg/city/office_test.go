package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 90: the session coordinating a group is its line's site office.
func TestSiteOffice(t *testing.T) {
	// grouped is a group of f in botropolis and l in lanterns, with a
	// loose session d in cinders; visits are where f's subagents worked.
	grouped := func(t *testing.T, foreman state.State, visits map[string]int) *city.Scene {
		t.Helper()
		f := session("f", botropolis, foreman)
		f.Group, f.StartedAt, f.Visits = "g1", now.Add(-3*time.Hour), visits
		l := session("l", lanterns, state.Working)
		l.Group, l.StartedAt = "g1", now.Add(-2*time.Hour)
		snap := snapshot(f, l, session("d", cinders, state.Working))
		snap.Groups = []state.Group{{ID: "g1", Name: "PROJ-1", Short: "PROJ-1", Members: []string{"f", "l"}}}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snap)
		return s
	}
	building := func(s *city.Scene, id string) *city.Building {
		for _, b := range s.City().Buildings() {
			if b.Session.ID == id {
				return b
			}
		}
		return nil
	}
	reaching := map[string]int{lanterns: 2, cinders: 1}

	for _, foreman := range []struct {
		name  string
		state state.State
	}{
		{"is working", state.Working},
		{"has stopped", state.Resting},
	} {
		t.Run("when a group's session whose subagents worked in its other districts "+foreman.name, func(t *testing.T) {
			s := grouped(t, foreman.state, reaching)
			line := s.City().Lines[0]

			t.Run("the line should keep its site office at its head", func(t *testing.T) {
				require.NotNil(t, line.Office)
				assert.Equal(t, "f", line.Office.Building.Session.ID)
			})

			t.Run("the site office should stand on the coordinator's building", func(t *testing.T) {
				require.NotNil(t, line.Office)
				assert.True(t, building(s, "f").Rect.Contains(line.Office.At))
			})

			t.Run("the districts only it reached should stay stations", func(t *testing.T) {
				var names []string
				for _, st := range line.Stations {
					names = append(names, st.Name())
				}
				assert.Contains(t, names, "cinders")
			})
		})
	}

	t.Run("when a district only the coordinator's subagents worked in is a station", func(t *testing.T) {
		s := grouped(t, state.Working, reaching)
		require.True(t, s.FocusGroup("g1"))
		var row city.DiveRow
		for _, r := range s.Dive() {
			if r.Site == "cinders" {
				row = r
			}
		}

		t.Run("its dive row should lead with the coordinator", func(t *testing.T) {
			assert.Equal(t, "f", row.Session)
		})

		t.Run("its stand-up phrase should be the coordinator's", func(t *testing.T) {
			assert.Contains(t, s.Standup("g1"), "cinders in progress")
		})
	})

	t.Run("when the only district its subagents visited is outside the group", func(t *testing.T) {
		line := grouped(t, state.Working, map[string]int{cinders: 1}).City().Lines[0]

		t.Run("the line should have no site office", func(t *testing.T) {
			assert.Nil(t, line.Office)
		})
	})

	t.Run("when no session coordinates", func(t *testing.T) {
		line := grouped(t, state.Working, nil).City().Lines[0]

		t.Run("the line should have no site office", func(t *testing.T) {
			assert.Nil(t, line.Office)
		})
	})
}
