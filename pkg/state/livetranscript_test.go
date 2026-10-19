package state_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 141. The loader read a live transcript again from its first byte
// on every change; it now reads what was appended, and must build the
// session a loader reading it whole builds.
func TestLiveTranscriptGrowth(t *testing.T) {
	longHome := func(t *testing.T) (*helpers.Home, string) {
		t.Helper()
		home := helpers.NewHome(t).Session(4242, sidA, loaderCWD, "interactive", "idle")
		var lines []string
		for i := range 2000 {
			ts := now.Add(-2*time.Hour + time.Duration(i)*time.Second).Format(time.RFC3339Nano)
			lines = append(lines,
				helpers.UserPrompt(sidA, loaderCWD, ts),
				helpers.AssistantReply(sidA, fmt.Sprintf("msg_%05d", i), ts))
		}
		return home, home.Transcript(sidA, loaderCWD, lines...)
	}
	freshSession := func(t *testing.T, home *helpers.Home, at time.Time) state.Session {
		t.Helper()
		snapshot, err := state.NewLoader(home.Path, alive).Load(at)
		require.NoError(t, err)
		return sessionByID(t, snapshot, sidA)
	}

	t.Run("when a live transcript grows by a line", func(t *testing.T) {
		home, _ := longHome(t)
		loader := state.NewLoader(home.Path, alive)
		_, err := loader.Load(now)
		require.NoError(t, err)
		line := helpers.UserPrompt(sidA, loaderCWD, now.Format(time.RFC3339Nano))
		home.AppendTranscript(sidA, loaderCWD, line)
		before := measure.Read(measure.BytesRead)
		snapshot, err := loader.Load(now.Add(time.Second))
		require.NoError(t, err)
		read := measure.Read(measure.BytesRead) - before

		t.Run("it should read about the bytes appended", func(t *testing.T) {
			assert.LessOrEqual(t, read, int64(len(line)+1+8<<10))
		})

		t.Run("it should build the session a fresh loader builds", func(t *testing.T) {
			assert.Equal(t, freshSession(t, home, now.Add(time.Second)), sessionByID(t, snapshot, sidA))
		})
	})

	t.Run("when a live transcript is replaced by a shorter one", func(t *testing.T) {
		home, path := longHome(t)
		loader := state.NewLoader(home.Path, alive)
		_, err := loader.Load(now)
		require.NoError(t, err)
		next := path + ".next"
		require.NoError(t, os.WriteFile(next, []byte(helpers.UserPrompt(sidA, loaderCWD, now.Format(time.RFC3339Nano))+"\n"), 0o600))
		require.NoError(t, os.Rename(next, path))
		snapshot, err := loader.Load(now.Add(time.Second))
		require.NoError(t, err)

		t.Run("it should build the session a fresh loader builds", func(t *testing.T) {
			assert.Equal(t, freshSession(t, home, now.Add(time.Second)), sessionByID(t, snapshot, sidA))
		})
	})
}

// Item 141, slice 2: the same for a live session's subagents.
func TestLiveSubagentGrowth(t *testing.T) {
	const agent = "a2f854e70"
	t.Run("when a live session's subagent grows by a line", func(t *testing.T) {
		home := liveHome(t)
		var lines []string
		for i := range 2000 {
			ts := now.Add(-time.Hour + time.Duration(i)*time.Second).Format(time.RFC3339Nano)
			lines = append(lines,
				helpers.UserPrompt(sidA, loaderCWD, ts),
				helpers.AssistantReply(sidA, fmt.Sprintf("msg_sub_%05d", i), ts))
		}
		home.Subagent(sidA, loaderCWD, agent, `{"agentType":"Explore","toolUseId":"toolu_01"}`, lines...)
		loader := state.NewLoader(home.Path, alive)
		_, err := loader.Load(now)
		require.NoError(t, err)
		line := helpers.AssistantReply(sidA, "msg_sub_last", now.Format(time.RFC3339Nano))
		home.AppendSubagent(sidA, loaderCWD, agent, line)
		before := measure.Read(measure.BytesRead)
		snapshot, err := loader.Load(now.Add(time.Second))
		require.NoError(t, err)
		read := measure.Read(measure.BytesRead) - before
		fresh, err := state.NewLoader(home.Path, alive).Load(now.Add(time.Second))
		require.NoError(t, err)

		t.Run("it should read about the bytes appended", func(t *testing.T) {
			assert.LessOrEqual(t, read, int64(len(line)+1+8<<10))
		})

		t.Run("it should build the session a fresh loader builds", func(t *testing.T) {
			assert.Equal(t, sessionByID(t, fresh, sidA), sessionByID(t, snapshot, sidA))
		})
	})
}
