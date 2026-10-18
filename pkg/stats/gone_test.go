package stats_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f *fixture) goneAt(session string) (int64, bool) {
	v := f.text("SELECT transcript_gone_at FROM sessions WHERE id = ?", session)
	if !v.Valid {
		return 0, false
	}
	return f.number("SELECT transcript_gone_at FROM sessions WHERE id = ?", session), true
}

func TestGone(t *testing.T) {
	t.Run("when a session's transcript is deleted and ingest runs", func(t *testing.T) {
		f := newFixture(t)
		path := f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
		f.ingest()
		require.NoError(t, os.Remove(path))
		f.now = f.now.Add(time.Hour)
		f.ingest()
		goneAt, _ := f.goneAt(sessionA)

		t.Run("it should set its transcript_gone_at to that ingest's time", func(t *testing.T) {
			assert.Equal(t, f.now.UnixMilli(), goneAt)
		})

		t.Run("and when the transcript comes back", func(t *testing.T) {
			f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
			f.ingest()

			t.Run("it should clear it", func(t *testing.T) {
				_, gone := f.goneAt(sessionA)
				assert.False(t, gone)
			})
		})
	})

	t.Run("when only a subagent's transcript is deleted", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
		agent := f.subagent(sessionA, "a1b2c3", reply{session: sessionA, id: "msg_s", at: "2026-10-18T08:01:00.000Z", cwd: lib}.line())
		f.ingest()
		require.NoError(t, os.Remove(agent))
		f.ingest()

		t.Run("it should leave the session's transcript_gone_at empty", func(t *testing.T) {
			_, gone := f.goneAt(sessionA)
			assert.False(t, gone)
		})
	})

	t.Run("when the transcript is still there", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib}.line())
		f.ingest()
		f.ingest()

		t.Run("it should leave transcript_gone_at empty", func(t *testing.T) {
			_, gone := f.goneAt(sessionA)
			assert.False(t, gone)
		})
	})
}
