package stats_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
)

func (f *fixture) messageSeq(id string) int64 {
	f.t.Helper()
	return f.number("SELECT seq FROM changed_messages WHERE message_id = ?", id)
}

func (f *fixture) sessionSeq(id string) int64 {
	f.t.Helper()
	return f.number("SELECT seq FROM changed_sessions WHERE session_id = ?", id)
}

func (f *fixture) changeSeq() int64 {
	f.t.Helper()
	return f.number("SELECT value FROM meta WHERE key = 'change_seq'")
}

// Item 215: a reader brings what it read up to date from the messages
// and sessions that changed since, which the store logs as it is
// written, one row each, whichever build writes.
func TestChangeLog(t *testing.T) {
	first := reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Output: 1}}

	t.Run("when a message is ingested after another", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, first.line())
		f.ingest()
		before := f.messageSeq("msg_1")
		f.transcript(sessionA, first.line(), reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: lib, usage: usage{Output: 2}}.line())
		f.ingest()

		t.Run("it should log it with a later seq", func(t *testing.T) {
			assert.Greater(t, f.messageSeq("msg_2"), before)
		})
	})

	t.Run("when a transcript is read again and its messages are rewritten with the same values", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, first.line())
		f.ingest()
		before := f.changeSeq()
		_, err := f.store.DB().Exec("UPDATE transcripts SET read_to = 0")
		require.NoError(t, err)
		f.ingest()

		t.Run("it should log nothing", func(t *testing.T) {
			assert.Equal(t, before, f.changeSeq())
		})
	})

	t.Run("when a message's counters change", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, first.line())
		f.ingest()
		before := f.sessionSeq(sessionA)
		grown := first
		grown.usage.Output = 7
		f.transcript(sessionA, first.line(), grown.line())
		f.ingest()

		t.Run("it should log every session holding it", func(t *testing.T) {
			assert.Greater(t, f.sessionSeq(sessionA), before)
		})
	})

	t.Run("when a session is forgotten", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, first.line())
		f.ingest()
		message, session := f.messageSeq("msg_1"), f.sessionSeq(sessionA)
		require.NoError(t, f.store.Forget(sessionA))

		t.Run("it should log the message it held", func(t *testing.T) {
			assert.Greater(t, f.messageSeq("msg_1"), message)
		})

		t.Run("it should log the session", func(t *testing.T) {
			assert.Greater(t, f.sessionSeq(sessionA), session)
		})
	})

	t.Run("when an older writer, which never reads the log, rewrites the same messages 50 times", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, first.line(), reply{session: sessionA, id: "msg_2", at: "2026-10-18T08:01:00.000Z", cwd: lib, usage: usage{Output: 2}}.line())
		f.ingest()
		for range 50 {
			_, err := f.store.DB().Exec("UPDATE messages SET output = output + 1")
			require.NoError(t, err)
		}

		t.Run("it should hold at most one row per message", func(t *testing.T) {
			assert.Equal(t, f.messages(), f.count("SELECT count(*) FROM changed_messages"))
		})
	})

	t.Run("when a build that knows only the schema's version writes the store and opens it again", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, first.line())
		f.ingest()
		older, err := sql.Open("sqlite", f.db)
		require.NoError(t, err)
		t.Cleanup(func() { _ = older.Close() })
		_, err = older.Exec(`INSERT INTO messages (id, at, model, dir_id, thread, input, output, cache_create, cache_read)
			VALUES ('msg_old', 1, 'claude-opus-5', (SELECT id FROM dirs LIMIT 1), 'thread', 1, 1, 0, 0)
			ON CONFLICT (id) DO UPDATE SET output = excluded.output`)
		require.NoError(t, err)
		again, err := stats.Open(f.db)
		if err == nil {
			_ = again.Close()
		}

		t.Run("its write should be logged", func(t *testing.T) {
			assert.Positive(t, f.messageSeq("msg_old"))
		})

		t.Run("it should open again", func(t *testing.T) {
			assert.NoError(t, err)
		})
	})
}
