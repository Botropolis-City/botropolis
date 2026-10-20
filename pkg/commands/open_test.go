package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/control"
	"botropolis.city/botropolis/testing/helpers"
)

func TestSessionsOpen(t *testing.T) {
	live := []control.Agent{{ID: "0898d7e4", SessionID: "0898d7e4-f2cb-45fe-833e-1921c1f4bcaa", Kind: "background"}}
	open := func(t *testing.T, ctl *fakeController, id, dir string, here bool) error {
		t.Helper()
		home := helpers.NewHome(t)
		home.Transcript(sid, cwd, helpers.UserPrompt(sid, cwd, "2026-09-16T19:50:00.000Z"))
		return commands.NewSessions(home.Path, ctl).Open(context.Background(), id, dir, here)
	}

	for _, c := range []struct {
		scenario string
		id       string
	}{
		{"when the id is a live job's", "0898d7e4"},
		{"when the id is a live job's session", "0898d7e4-f2cb-45fe-833e-1921c1f4bcaa"},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			t.Run("and it is opened here", func(t *testing.T) {
				ctl := &fakeController{agents: live}
				_ = open(t, ctl, c.id, "", true)

				t.Run("it should attach in this terminal without resuming", func(t *testing.T) {
					assert.Equal(t, []string{"agents all=false", "attach here 0898d7e4"}, ctl.calls)
				})
			})

			t.Run("and it is opened elsewhere", func(t *testing.T) {
				ctl := &fakeController{agents: live}
				_ = open(t, ctl, c.id, "", false)

				t.Run("it should open a terminal on it without resuming", func(t *testing.T) {
					assert.Equal(t, []string{"agents all=false", "attach 0898d7e4"}, ctl.calls)
				})
			})
		})
	}

	t.Run("when the id is a resting session", func(t *testing.T) {
		t.Run("and no directory is given", func(t *testing.T) {
			ctl := &fakeController{agents: live, id: "16daefa1"}
			_ = open(t, ctl, sid, "", true)

			t.Run("it should resume it in its own directory and attach the new job", func(t *testing.T) {
				assert.Equal(t, []string{"agents all=false", "resume " + cwd + " " + sid, "attach here 16daefa1"}, ctl.calls)
			})
		})

		t.Run("and a directory is given", func(t *testing.T) {
			ctl := &fakeController{agents: live, id: "16daefa1"}
			_ = open(t, ctl, sid, "/tmp/elsewhere", false)

			t.Run("it should resume it there", func(t *testing.T) {
				assert.Equal(t, []string{"agents all=false", "resume /tmp/elsewhere " + sid, "attach 16daefa1"}, ctl.calls)
			})
		})
	})

	t.Run("when the id is a stopped job's", func(t *testing.T) {
		ctl := &fakeController{agents: []control.Agent{{ID: "08b1d655", SessionID: sid, Kind: "background"}}, liveAgents: []control.Agent{}, id: "16daefa1"}
		_ = open(t, ctl, "08b1d655", "", true)

		t.Run("it should resume its session", func(t *testing.T) {
			assert.Contains(t, ctl.calls, "resume "+cwd+" "+sid)
		})
	})

	t.Run("when a job id is not listed yet, as a job started a moment ago may not be", func(t *testing.T) {
		ctl := &fakeController{agents: live}
		_ = open(t, ctl, "deadbeef", "", true)

		t.Run("it should attach it anyway, as claude attach always did", func(t *testing.T) {
			assert.Equal(t, "attach here deadbeef", ctl.calls[len(ctl.calls)-1])
		})
	})

	t.Run("when a session id has no live job and no transcript", func(t *testing.T) {
		ctl := &fakeController{agents: live}
		err := open(t, ctl, "deadbeef-0000-0000-0000-000000000000", "", true)

		t.Run("it should say there is nothing to resume", func(t *testing.T) {
			assert.ErrorContains(t, err, "no transcript for session deadbeef-0000-0000-0000-000000000000")
		})
	})
}
