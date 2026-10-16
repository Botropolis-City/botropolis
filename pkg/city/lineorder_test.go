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

	teammate := "https://code.example.com/acme/lib/pull/7"
	// mentioning is a group whose api session names a pull request no
	// session in it opened, in words or, with dependsOn, as a footer.
	mentioningScene := func(t *testing.T, dependsOn bool) *city.Scene {
		t.Helper()
		lib := session("a", cinders, state.Resting)
		lib.Group, lib.StartedAt = "g1", now.Add(-3*time.Hour)
		api := session("b", botropolis, state.Resting)
		api.Group, api.StartedAt = "g1", now.Add(-2*time.Hour)
		api.OrderLines = []claude.OrderLine{{Text: "compare with " + teammate, At: now.Add(-48 * time.Hour)}}
		if dependsOn {
			api.OrderLines = []claude.OrderLine{{Text: "Depends-On: " + teammate, Command: true, At: now.Add(-48 * time.Hour)}}
		}
		snap := snapshot(lib, api)
		snap.Groups = []state.Group{{ID: "g1", Name: "PROJ-1", Short: "PROJ-1", Members: []string{"a", "b"}}}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snap)
		return s
	}
	mentioning := func(t *testing.T, dependsOn bool) city.Line {
		t.Helper()
		return mentioningScene(t, dependsOn).City().Lines[0]
	}
	teammateAt := func(l city.Line) int {
		for i, st := range l.Stations {
			if st.Teammate != nil {
				return i
			}
		}
		return -1
	}

	t.Run("when a group's session names a pull request opened outside it", func(t *testing.T) {
		line := mentioning(t, false)
		i := teammateAt(line)

		t.Run("the line should have a grey station for it", func(t *testing.T) {
			assert.GreaterOrEqual(t, i, 0)
		})

		t.Run("its card should say it was opened outside these sessions, its state, and where and when it was seen", func(t *testing.T) {
			require.GreaterOrEqual(t, i, 0)
			assert.Contains(t, city.StationCard(line, i).Lines, "pr #7 acme/lib, opened outside these sessions, open, seen 2d0h ago in botropolis")
		})

		t.Run("no building should take its station's number", func(t *testing.T) {
			var taken []int
			for _, st := range line.Stations {
				for _, b := range st.District.Buildings {
					taken = append(taken, b.Station)
				}
			}
			assert.NotContains(t, taken, i+1)
		})
	})

	t.Run("when the group is focused with a pull request opened outside it", func(t *testing.T) {
		s := mentioningScene(t, false)
		require.True(t, s.FocusGroup("g1"))
		i := teammateAt(s.City().Lines[0])
		require.GreaterOrEqual(t, i, 0)

		t.Run("its dive row should name the pull request, not its district's session", func(t *testing.T) {
			assert.Equal(t, city.DiveRow{Site: "acme/lib#7", Sentence: "opened outside these sessions, named in botropolis",
				PR: "PR #7 open", Move: state.MoveTheirs, Waited: 48 * time.Hour}, s.Dive()[i])
		})

		t.Run("the stand-up should give its own phrase", func(t *testing.T) {
			assert.Contains(t, s.Standup("g1"), "acme/lib#7 in review, seen 2 days ago")
		})
	})

	t.Run("when a site depends on a pull request opened outside the group", func(t *testing.T) {
		line := mentioning(t, true)
		i := teammateAt(line)
		var api int
		for j, st := range line.Stations {
			if st.Teammate == nil && st.District.Name == "botropolis" {
				api = j
			}
		}

		t.Run("its station should come before the site", func(t *testing.T) {
			assert.Less(t, i, api)
		})

		t.Run("the site should be unlit", func(t *testing.T) {
			assert.True(t, line.Stations[api].Unlit)
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
