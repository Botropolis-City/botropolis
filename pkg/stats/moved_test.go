package stats_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/stats"
)

type movedHefts struct {
	hefts map[string]claude.Usage
	moved []string
}

func (f *fixture) movedHefts() movedHefts {
	f.t.Helper()
	hefts, moved, err := f.store.MovedHefts()
	require.NoError(f.t, err)
	return movedHefts{hefts: hefts, moved: moved}
}

func (f *fixture) dataVersion(db *sql.DB) int64 {
	f.t.Helper()
	var v int64
	require.NoError(f.t, db.QueryRow("PRAGMA data_version").Scan(&v))
	return v
}

// Item 196: a settled daemon read every session's counters after every
// rescan; the store now records which sessions' rows changed, in the
// database itself, so another process's writes are caught too.
func TestMovedHefts(t *testing.T) {
	t.Run("when every transcript was ingested and the moved sessions were read", func(t *testing.T) {
		newSettled := func(t *testing.T) (*fixture, string) {
			f := newFixture(t)
			a := f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Input: 1, Output: 2}}.line())
			f.transcript(sessionB, reply{session: sessionB, id: "msg_2", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Output: 3}}.line())
			f.ingest()
			f.movedHefts()
			return f, a
		}

		t.Run("and ingest runs again with nothing changed", func(t *testing.T) {
			f, _ := newSettled(t)
			other, err := sql.Open("sqlite", f.db)
			require.NoError(t, err)
			t.Cleanup(func() { _ = other.Close() })
			before := f.dataVersion(other)
			f.ingest()
			after := f.dataVersion(other)
			moved := f.movedHefts()

			t.Run("it should report no session moved", func(t *testing.T) {
				assert.Empty(t, moved.moved)
			})

			t.Run("it should commit nothing", func(t *testing.T) {
				assert.Equal(t, before, after)
			})
		})

		t.Run("and one of two sessions gains a reply", func(t *testing.T) {
			f, a := newSettled(t)
			f.appendRaw(a, reply{session: sessionA, id: "msg_3", at: "2026-10-18T08:05:00.000Z", cwd: lib, usage: usage{Output: 10}}.line()+"\n")
			f.ingest()
			moved := f.movedHefts()

			t.Run("it should report only that session moved", func(t *testing.T) {
				assert.Equal(t, []string{sessionA}, moved.moved)
			})

			t.Run("it should give that session's counters as Hefts would", func(t *testing.T) {
				assert.Equal(t, map[string]claude.Usage{sessionA: f.hefts()[sessionA]}, moved.hefts)
			})
		})

		t.Run("and ingest runs many times without the moved sessions being read", func(t *testing.T) {
			f, a := newSettled(t)
			for id, at := range map[string]string{"msg_4": "2026-10-18T08:10:00.000Z", "msg_5": "2026-10-18T08:11:00.000Z", "msg_6": "2026-10-18T08:12:00.000Z"} {
				f.appendRaw(a, reply{session: sessionA, id: id, at: at, cwd: lib, usage: usage{Output: 1}}.line()+"\n")
				f.ingest()
			}

			t.Run("it should hold at most one row per session", func(t *testing.T) {
				assert.Equal(t, 1, f.count("SELECT count(*) FROM moved"))
			})
		})

		t.Run("and a session is forgotten through another store on the same file", func(t *testing.T) {
			f, _ := newSettled(t)
			other := open(t, f.db)
			require.NoError(t, other.Forget(sessionB))
			moved := f.movedHefts()

			t.Run("it should report it moved", func(t *testing.T) {
				assert.Equal(t, []string{sessionB}, moved.moved)
			})

			t.Run("it should give it no counters", func(t *testing.T) {
				assert.NotContains(t, moved.hefts, sessionB)
			})
		})
	})

	t.Run("when a fork shares a message whose usage the original rewrites", func(t *testing.T) {
		f := newFixture(t)
		a := f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Output: 5}}.line())
		f.transcript(sessionB, reply{session: sessionB, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Output: 5}}.line())
		f.ingest()
		f.movedHefts()
		f.appendRaw(a, reply{session: sessionA, id: "msg_1", at: "2026-10-18T08:00:00.000Z", cwd: lib, usage: usage{Output: 9}}.line()+"\n")
		f.ingest()

		t.Run("it should report both sessions moved", func(t *testing.T) {
			assert.ElementsMatch(t, []string{sessionA, sessionB}, f.movedHefts().moved)
		})
	})

	t.Run("when a database this build made is opened", func(t *testing.T) {
		f := newFixture(t)

		t.Run("it should keep the schema version, so an older build opens it as before", func(t *testing.T) {
			assert.Equal(t, int64(stats.Version), f.number("PRAGMA user_version"))
		})
	})
}
