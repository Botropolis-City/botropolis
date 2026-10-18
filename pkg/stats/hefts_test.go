package stats_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
)

func (f *fixture) hefts() map[string]claude.Usage {
	f.t.Helper()
	hefts, err := f.store.Hefts()
	require.NoError(f.t, err)
	return hefts
}

func TestHefts(t *testing.T) {
	t.Run("when a session has messages", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA,
			reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Input: 1, Output: 2, CacheCreate: 3, CacheRead: 4}}.line(),
			reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: lib, usage: usage{Input: 10, Output: 20, CacheCreate: 30, CacheRead: 40}}.line(),
		)
		f.ingest()

		t.Run("it should sum its four counters", func(t *testing.T) {
			assert.Equal(t, claude.Usage{Input: 11, Output: 22, CacheCreate: 33, CacheRead: 44, Messages: 2}, f.hefts()[sessionA])
		})
	})

	t.Run("when a fork holds its parent's message", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Output: 5}}.line())
		f.transcript(sessionB, reply{session: sessionB, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Output: 5}}.line())
		f.ingest()

		t.Run("it should count it in both sessions", func(t *testing.T) {
			h := f.hefts()
			assert.Equal(t, [2]int64{5, 5}, [2]int64{h[sessionA].Output, h[sessionB].Output})
		})
	})

	t.Run("when a subagent answered", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Output: 5}}.line())
		f.subagent(sessionA, "a1b2c3", reply{session: sessionA, id: "msg_s", at: "2026-10-18T08:01:00.000Z", cwd: lib, usage: usage{Output: 7}}.line())
		f.ingest()

		t.Run("it should count its message in the session", func(t *testing.T) {
			assert.Equal(t, int64(12), f.hefts()[sessionA].Output)
		})
	})

	t.Run("when a session's transcript is gone", func(t *testing.T) {
		f := newFixture(t)
		path := f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Output: 5}}.line())
		f.ingest()
		require.NoError(t, os.Remove(path))
		f.ingest()

		t.Run("it should still give its heft", func(t *testing.T) {
			assert.Equal(t, int64(5), f.hefts()[sessionA].Output)
		})
	})
}
