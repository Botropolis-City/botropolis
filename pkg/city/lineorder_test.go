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

// Item 90: a line reads the order its sites depend on.
func TestLineOrder(t *testing.T) {
	libPR := claude.PR{Number: 41, Repository: "acme/lib", URL: "https://code.example.com/acme/lib/pull/41", State: claude.PROpen}
	// grouped is lib, started last with its pull request in state,
	// and api, started first, whose commit depends on lib's.
	grouped := func(t *testing.T, prState string) *city.Scene {
		t.Helper()
		lib := session("a", cinders, state.Resting)
		lib.Group, lib.StartedAt = "g1", now.Add(-time.Hour)
		pr := libPR
		pr.State = prState
		lib.PRs = []claude.PR{pr}
		api := session("b", botropolis, state.Resting)
		api.Group, api.StartedAt = "g1", now.Add(-3*time.Hour)
		api.OrderLines = []claude.OrderLine{{Text: "Depends-On: " + libPR.URL, Command: true}}
		snap := snapshot(lib, api)
		snap.Groups = []state.Group{{ID: "g1", Name: "PROJ-1", Short: "PROJ-1", Members: []string{"a", "b"}}}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snap)
		return s
	}

	t.Run("when a station waits on another", func(t *testing.T) {
		s := grouped(t, claude.PROpen)
		line := s.City().Lines[0]
		require.Len(t, line.Stations, 2)

		t.Run("the stations should follow the order read", func(t *testing.T) {
			assert.Equal(t, []string{"cinders", "botropolis"}, []string{line.Stations[0].District.Name, line.Stations[1].District.Name})
		})

		t.Run("it should be unlit", func(t *testing.T) {
			assert.True(t, line.Stations[1].Unlit)
		})

		t.Run("its card should say what it waits on", func(t *testing.T) {
			assert.Contains(t, city.StationCard(line, 1).Lines, "waits on cinders until PR #41 merged")
		})

		t.Run("the station it waits on should be lit", func(t *testing.T) {
			assert.False(t, line.Stations[0].Unlit)
		})

		t.Run("and the group is focused, the dive should follow the order", func(t *testing.T) {
			require.True(t, s.FocusGroup("g1"))
			assert.Equal(t, "cinders", s.Dive()[0].Site)
		})

		t.Run("the stand-up should follow the order", func(t *testing.T) {
			assert.Equal(t, "cinders in review; botropolis stopped", s.Standup("g1"))
		})
	})

	t.Run("when its wait is done", func(t *testing.T) {
		line := grouped(t, claude.PRMerged).City().Lines[0]

		t.Run("it should be lit", func(t *testing.T) {
			assert.False(t, line.Stations[1].Unlit)
		})

		t.Run("its card should say what unblocked it", func(t *testing.T) {
			assert.Contains(t, city.StationCard(line, 1).Lines, "unblocked: cinders PR #41 merged")
		})
	})
}
