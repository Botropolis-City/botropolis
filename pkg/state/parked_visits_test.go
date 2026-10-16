package state_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 90: a stopped session keeps where its subagents worked, so a
// coordinator that stops keeps its site office, read once and cached.
func TestParkedVisits(t *testing.T) {
	const (
		foreman = "77777777-0000-4000-8000-000000000007"
		meta    = "/w/meta"
		lib     = "/w/lib"
	)
	stopped := func(t *testing.T) (*helpers.Home, *state.Loader, state.Snapshot) {
		t.Helper()
		home := helpers.NewHome(t)
		home.Transcript(foreman, meta, helpers.UserPrompt(foreman, meta, "2026-09-16T10:00:00.000Z"),
			helpers.AssistantReply(foreman, "msg_f1", "2026-09-16T12:00:00.000Z"))
		home.Subagent(foreman, meta, "a1", "", helpers.UserPrompt(foreman, lib, "2026-09-16T11:00:00.000Z"))
		loader := state.NewLoader(home.Path, alive)
		snapshot, err := loader.Load(now)
		require.NoError(t, err)
		return home, loader, snapshot
	}

	t.Run("when a session whose subagent worked elsewhere has stopped", func(t *testing.T) {
		_, _, snapshot := stopped(t)
		s := sessionByID(t, snapshot, foreman)
		require.Equal(t, state.Resting, s.State)

		t.Run("it should keep where its subagent worked", func(t *testing.T) {
			assert.Equal(t, map[string]int{lib: 1}, s.Visits)
		})

		t.Run("it should keep when", func(t *testing.T) {
			assert.Equal(t, state.Span{From: time.Date(2026, 9, 16, 11, 0, 0, 0, time.UTC), To: time.Date(2026, 9, 16, 11, 0, 0, 0, time.UTC)}, s.VisitSpans[lib])
		})
	})

	t.Run("when nothing in a stopped session's subagents changed", func(t *testing.T) {
		_, loader, _ := stopped(t)
		before := loader.Reads()
		_, err := loader.Load(now.Add(time.Second))
		require.NoError(t, err)

		t.Run("a reload should not read them again", func(t *testing.T) {
			assert.Equal(t, before, loader.Reads())
		})
	})

	t.Run("when a stopped session's subagent grows", func(t *testing.T) {
		home, loader, _ := stopped(t)
		before := loader.Reads()
		home.AppendSubagent(foreman, meta, "a1", helpers.UserPrompt(foreman, lib, "2026-09-16T11:30:00.000Z"))
		_, err := loader.Load(now.Add(time.Second))
		require.NoError(t, err)

		t.Run("a reload should read only it", func(t *testing.T) {
			assert.Equal(t, before+1, loader.Reads())
		})
	})
}
