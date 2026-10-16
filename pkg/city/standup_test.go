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

func TestStandup(t *testing.T) {
	member := func(id, cwd string, st state.State, started, idle time.Duration, prs ...claude.PR) state.Session {
		s := session(id, cwd, st)
		s.Group, s.StartedAt, s.LastActivity, s.PRs = "g1", now.Add(started), now.Add(-idle), prs
		return s
	}
	grouped := func(t *testing.T, sessions ...state.Session) *city.Scene {
		t.Helper()
		ids := []string{}
		for _, s := range sessions {
			if s.Group == "g1" {
				ids = append(ids, s.ID)
			}
		}
		snap := snapshot(sessions...)
		snap.Groups = []state.Group{{ID: "g1", Name: "PROJ-1", Short: "PROJ-1", Members: ids}}
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(snap)
		return s
	}
	pr := func(state string) claude.PR { return claude.PR{Number: 41, Repository: "lib", State: state} }

	for _, c := range []struct {
		when string
		site state.Session
		want string
	}{
		{"a site's every pull request merged", member("a", cinders, state.Resting, -time.Hour, time.Hour, pr(claude.PRMerged)), "cinders merged"},
		{"a site has an open pull request, waited under a day", member("a", cinders, state.Resting, -time.Hour, 5*time.Hour, pr(claude.PROpen)), "cinders in review"},
		{"a site has an open pull request, last seen three days ago", member("a", cinders, state.Resting, -80*time.Hour, 72*time.Hour, pr(claude.PROpen)), "cinders in review, seen 3 days ago"},
		{"a site needs you", member("a", cinders, state.NeedsYou, -time.Hour, 0), "cinders needs you"},
		{"a site is working", member("a", cinders, state.Working, -time.Hour, 0), "cinders in progress"},
		{"a site's pull requests all closed unmerged", member("a", cinders, state.Resting, -time.Hour, time.Hour, pr(claude.PRClosed)), "cinders closed"},
		{"a site stopped with no pull request two days ago", member("a", cinders, state.Resting, -50*time.Hour, 48*time.Hour), "cinders stopped, 2 days"},
	} {
		t.Run("when "+c.when, func(t *testing.T) {
			t.Run("it should say "+c.want, func(t *testing.T) {
				assert.Equal(t, c.want, grouped(t, c.site).Standup("g1"))
			})
		})
	}

	t.Run("when a group has three sites", func(t *testing.T) {
		s := grouped(t,
			member("a", cinders, state.Resting, -90*time.Hour, time.Hour, pr(claude.PRMerged)),
			member("b", botropolis, state.Working, -85*time.Hour, 0),
			member("c", lanterns, state.Resting, -80*time.Hour, 72*time.Hour, pr(claude.PROpen)),
		)

		t.Run("it should join them with semicolons in line order", func(t *testing.T) {
			assert.Equal(t, "cinders merged; botropolis in progress; lanterns in review, seen 3 days ago", s.Standup("g1"))
		})
	})

	t.Run("when a building in a group is selected", func(t *testing.T) {
		s := grouped(t, member("a", cinders, state.Working, -time.Hour, 0))
		require.True(t, s.Select("a"))

		t.Run("its verbs should include copy stand-up", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionCopyStandup)
		})

		t.Run("and copy stand-up is acted on", func(t *testing.T) {
			action, _ := s.Act(city.ActionCopyStandup)

			t.Run("it should copy the group's line", func(t *testing.T) {
				assert.Equal(t, city.Action{Kind: city.ActionCopy, Text: "cinders in progress"}, action)
			})
		})
	})

	t.Run("when a building in no group is selected", func(t *testing.T) {
		s := grouped(t, member("a", cinders, state.Working, -time.Hour, 0), session("d", orchard, state.Working))
		require.True(t, s.Select("d"))

		t.Run("its verbs should not include copy stand-up", func(t *testing.T) {
			assert.NotContains(t, s.Actions(), city.ActionCopyStandup)
		})
	})
}
