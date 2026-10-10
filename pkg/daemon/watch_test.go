package daemon_test

import (
	"context"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	settle  = 3 * time.Second
	tick    = 20 * time.Millisecond
	sidNew  = "ffffffff-1111-2222-3333-444444444444"
	sidNewP = 4343
)

func watch(t *testing.T, d *daemon.Daemon) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = d.Watch(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	require.NoError(t, d.WaitWatching(ctx))
}

func session(d *daemon.Daemon, id string) (state.Session, bool) {
	for _, s := range d.Snapshot().Sessions {
		if s.ID == id {
			return s, true
		}
	}
	return state.Session{}, false
}

func TestWatch(t *testing.T) {
	t.Run("when a watched transcript grows", func(t *testing.T) {
		home := idleHome(t, "interactive")
		d := newDaemon(t, home)
		watch(t, d)
		home.AppendTranscript(sid, cwd, helpers.UserPrompt(sid, cwd, stamp(time.Minute)))

		t.Run("it should re-read it and update the tail", func(t *testing.T) {
			assert.Eventually(t, func() bool {
				s, _ := session(d, sid)
				return s.Turn == claude.TurnWorking
			}, settle, tick)
		})
	})

	t.Run("when a new session record appears", func(t *testing.T) {
		home := idleHome(t, "interactive")
		d := newDaemon(t, home)
		watch(t, d)
		home.Session(sidNewP, sidNew, cwd, "interactive", "busy")

		t.Run("it should add the session", func(t *testing.T) {
			assert.Eventually(t, func() bool {
				_, ok := session(d, sidNew)
				return ok
			}, settle, tick)
		})
	})

	t.Run("when a session record is removed", func(t *testing.T) {
		home := idleHome(t, "interactive")
		d := newDaemon(t, home)
		watch(t, d)
		home.RemoveSession(pid)

		t.Run("it should park the session", func(t *testing.T) {
			assert.Eventually(t, func() bool {
				s, ok := session(d, sid)
				return ok && s.State == state.Parked
			}, settle, tick)
		})
	})

	t.Run("when a subagent transcript appears under a live session", func(t *testing.T) {
		home := idleHome(t, "interactive")
		d := newDaemon(t, home)
		watch(t, d)
		home.Subagent(sid, cwd, "a2f854e70", `{"agentType":"Explore","toolUseId":"toolu_01"}`,
			helpers.UserPrompt(sid, cwd, stamp(20*time.Second)),
			helpers.AssistantReply(sid, "msg_sub", stamp(21*time.Second)))

		t.Run("it should count it", func(t *testing.T) {
			assert.Eventually(t, func() bool {
				s, _ := session(d, sid)
				return s.Subagents == 1
			}, settle, tick)
		})

		t.Run("and the subagent transcript then grows", func(t *testing.T) {
			require.Eventually(t, func() bool {
				s, _ := session(d, sid)
				return s.Subagents == 1
			}, settle, tick)
			before, _ := session(d, sid)
			home.AppendSubagent(sid, cwd, "a2f854e70",
				helpers.AssistantReply(sid, "msg_sub2", stamp(22*time.Second)))

			t.Run("it should add its usage", func(t *testing.T) {
				assert.Eventually(t, func() bool {
					s, _ := session(d, sid)
					return s.Usage.Messages == before.Usage.Messages+1
				}, settle, tick)
			})
		})
	})
}
