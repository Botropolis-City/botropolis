package state_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

const replyHeft = 2 + 5 + 250_000/10

// Item 109. A stopped session's heft needs its whole transcript, which
// the catalogue's window does not hold, so it is read whole once, in
// the background, after the first load.
func TestLoaderWholeRead(t *testing.T) {
	const sid = "dddddddd-0000-0000-0000-000000000004"
	bigHome := func(t *testing.T) *helpers.Home {
		t.Helper()
		start := now.Add(-7 * 24 * time.Hour)
		lines := []string{helpers.UserPrompt(sid, loaderCWD, start.Format(time.RFC3339Nano))}
		for i := range 1000 {
			at := start.Add(time.Duration(i) * 30 * time.Second).Format(time.RFC3339Nano)
			lines = append(lines, helpers.AssistantReply(sid, fmt.Sprintf("msg_%04d", i), at))
		}
		home := helpers.NewHome(t)
		home.Transcript(sid, loaderCWD, lines...)
		home.Subagent(sid, loaderCWD, "a1", "",
			helpers.UserPrompt(sid, loaderCWD, start.Format(time.RFC3339Nano)),
			helpers.AssistantReply(sid, "sub_01", start.Add(time.Second).Format(time.RFC3339Nano)),
			helpers.AssistantReply(sid, "sub_02", start.Add(2*time.Second).Format(time.RFC3339Nano)))
		return home
	}
	waitFor := func(t *testing.T, read <-chan struct{}) {
		t.Helper()
		select {
		case <-read:
		case <-time.After(30 * time.Second):
			require.FailNow(t, "the whole read never finished")
		}
	}

	t.Run("when a stopped transcript is larger than the window", func(t *testing.T) {
		t.Run("and the whole read has not run", func(t *testing.T) {
			snapshot, err := state.NewLoader(bigHome(t).Path, alive).Load(now)
			require.NoError(t, err)

			t.Run("it should report its heft as a lower bound", func(t *testing.T) {
				assert.True(t, sessionByID(t, snapshot, sid).HeftPartial)
			})

			t.Run("it should not report the history read", func(t *testing.T) {
				assert.False(t, snapshot.HeftsRead)
			})
		})

		t.Run("and the whole read has run", func(t *testing.T) {
			home := bigHome(t)
			read := make(chan struct{}, 8)
			calls := 0
			loader := state.NewLoader(home.Path, alive).WithWholeRead(func() { calls++; read <- struct{}{} })
			_, err := loader.Load(now)
			require.NoError(t, err)
			waitFor(t, read)
			snapshot, err := loader.Load(now.Add(time.Second))
			require.NoError(t, err)

			t.Run("it should report its exact heft, subagents included", func(t *testing.T) {
				assert.Equal(t, int64(1002*replyHeft), sessionByID(t, snapshot, sid).Heft)
			})

			t.Run("it should not mark it a lower bound", func(t *testing.T) {
				assert.False(t, sessionByID(t, snapshot, sid).HeftPartial)
			})

			t.Run("it should report the history read", func(t *testing.T) {
				assert.True(t, snapshot.HeftsRead)
			})

			t.Run("it should tell the daemon to rescan once", func(t *testing.T) {
				assert.Equal(t, 1, calls)
			})

			t.Run("and the file has not changed since", func(t *testing.T) {
				_, err := loader.Load(now.Add(2 * time.Second))
				require.NoError(t, err)

				t.Run("it should not read it again", func(t *testing.T) {
					assert.Equal(t, 1, loader.WholeReads())
				})
			})

			t.Run("and the file has changed", func(t *testing.T) {
				home.AppendTranscript(sid, loaderCWD,
					helpers.AssistantReply(sid, "msg_late", now.Add(-6*24*time.Hour).Format(time.RFC3339Nano)))
				_, err := loader.Load(now.Add(3 * time.Second))
				require.NoError(t, err)
				waitFor(t, read)

				t.Run("it should read it again", func(t *testing.T) {
					assert.Equal(t, 2, loader.WholeReads())
				})
			})
		})
	})

	t.Run("when a session is live", func(t *testing.T) {
		snapshot, err := state.NewLoader(liveHome(t).Path, alive).Load(now)
		require.NoError(t, err)

		t.Run("it should report its exact heft without waiting for the whole read", func(t *testing.T) {
			assert.Equal(t, int64(replyHeft), sessionByID(t, snapshot, sidA).Heft)
		})
	})
}
