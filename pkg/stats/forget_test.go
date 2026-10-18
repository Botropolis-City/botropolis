package stats_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForget(t *testing.T) {
	t.Run("when a pruned session's messages are held by no other session", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA,
			reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: "/w/gone"}.line(),
			reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: "/w/gone"}.line(),
		)
		f.transcript(sessionB, reply{session: sessionB, id: "msg_9", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
		f.ingest()
		require.NoError(t, f.store.Forget(sessionA))

		t.Run("it should delete its messages", func(t *testing.T) {
			assert.Equal(t, 1, f.messages())
		})

		t.Run("it should delete its links", func(t *testing.T) {
			assert.Equal(t, 0, f.count("SELECT count(*) FROM message_sessions WHERE session_id = ?", sessionA))
		})

		t.Run("it should delete the session", func(t *testing.T) {
			assert.Equal(t, 0, f.count("SELECT count(*) FROM sessions WHERE id = ?", sessionA))
		})

		t.Run("it should delete its transcripts rows", func(t *testing.T) {
			assert.Equal(t, 0, f.count("SELECT count(*) FROM transcripts WHERE session_id = ?", sessionA))
		})

		t.Run("it should delete a directory no message uses any more", func(t *testing.T) {
			assert.Equal(t, 0, f.count("SELECT count(*) FROM dirs WHERE path = '/w/gone'"))
		})
	})

	t.Run("when one of its messages is also held by a fork that stays", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
		f.transcript(sessionB, reply{session: sessionB, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
		f.ingest()
		require.NoError(t, f.store.Forget(sessionA))

		t.Run("it should keep the message", func(t *testing.T) {
			assert.True(t, f.has("msg_1"))
		})

		t.Run("it should keep the fork's link", func(t *testing.T) {
			assert.Equal(t, 1, f.count("SELECT count(*) FROM message_sessions WHERE session_id = ?", sessionB))
		})
	})
}
