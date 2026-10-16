package state_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
)

func TestGroupJournal(t *testing.T) {
	older := state.Session{ID: "a", LastActivity: now.Add(-3 * time.Hour), LastPrompt: "older prompt",
		Recap: claude.Recap{Text: "Older recap. Next: older next"}}
	latest := func(recap, prompt string) state.Session {
		return state.Session{ID: "b", LastActivity: now.Add(-time.Hour), LastPrompt: prompt, Recap: claude.Recap{Text: recap}}
	}
	cases := []struct {
		scenario string
		latest   state.Session
		last     string
		next     string
	}{
		{"the latest member's recap has a Next:", latest("Finished the client; tests pass. Next: tag v1.4.0 once #41 merges.", "p"),
			"Finished the client; tests pass.", "tag v1.4.0 once #41 merges."},
		{"the recap has no Next:", latest("Finished the client; tests pass.", "p"), "Finished the client; tests pass.", "not recorded"},
		{"the latest member has no recap but an earlier one does", latest("", "wire the headers through"), "Older recap.", "older next"},
	}
	for _, c := range cases {
		t.Run("when "+c.scenario, func(t *testing.T) {
			j := state.GroupJournal([]state.Session{older, c.latest})

			t.Run("it should say what happened last time", func(t *testing.T) {
				assert.Equal(t, c.last, j.LastTime)
			})

			t.Run("it should say what is next", func(t *testing.T) {
				assert.Equal(t, c.next, j.Next)
			})

			t.Run("it should carry when the group was last active", func(t *testing.T) {
				assert.Equal(t, now.Add(-time.Hour), j.Active)
			})
		})
	}

	t.Run("when no member has a recap", func(t *testing.T) {
		j := state.GroupJournal([]state.Session{
			{ID: "a", LastActivity: now.Add(-3 * time.Hour), LastPrompt: "older prompt"},
			{ID: "b", LastActivity: now.Add(-time.Hour), LastPrompt: "wire the headers through"},
		})

		t.Run("it should take the latest member's last prompt as last time", func(t *testing.T) {
			assert.Equal(t, "wire the headers through", j.LastTime)
		})

		t.Run("it should say next is not recorded", func(t *testing.T) {
			assert.Equal(t, "not recorded", j.Next)
		})
	})
}

func TestLoaderGroupOrder(t *testing.T) {
	t.Run("when there are several groups", func(t *testing.T) {
		home := groupedHome(t)
		const sidC, sidD = "3333cccc-0000-0000-0000-000000000000", "4444dddd-0000-0000-0000-000000000000"
		for _, s := range []struct{ sid, dir, at string }{{sidC, libDir, "2026-09-16T20:00:00.000Z"}, {sidD, apiDir, "2026-09-16T23:00:00.000Z"}} {
			home.Transcript(s.sid, s.dir, helpers.UserPrompt(s.sid, s.dir, s.at), helpers.AssistantReply(s.sid, "msg_"+s.sid[:4], s.at))
			at, _ := time.Parse(time.RFC3339, s.at)
			home.Prompt(s.sid, s.dir, "LOG-88 ship the logs", at)
		}
		snapshot, err := state.NewLoader(home.Path, dead).Load(now)
		assert.NoError(t, err)
		var anchors []string
		for _, g := range snapshot.Groups {
			anchors = append(anchors, g.Anchor)
		}

		t.Run("it should list the most recently active first", func(t *testing.T) {
			assert.Equal(t, []string{"LOG-88", "PROJ-123"}, anchors)
		})
	})
}
