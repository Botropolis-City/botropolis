package stats_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const schemaV1 = `
CREATE TABLE dirs (id INTEGER PRIMARY KEY, path TEXT NOT NULL UNIQUE);
CREATE TABLE sessions (id TEXT PRIMARY KEY, started_at INTEGER, transcript_gone_at INTEGER);
CREATE TABLE messages (
  id TEXT PRIMARY KEY, at INTEGER NOT NULL, model TEXT NOT NULL, speed TEXT,
  dir_id INTEGER NOT NULL REFERENCES dirs, thread TEXT NOT NULL,
  input INTEGER NOT NULL, output INTEGER NOT NULL, cache_create INTEGER NOT NULL, cache_read INTEGER NOT NULL
);
CREATE TABLE message_sessions (
  message_id TEXT NOT NULL REFERENCES messages, session_id TEXT NOT NULL REFERENCES sessions,
  PRIMARY KEY (message_id, session_id)
) WITHOUT ROWID;
CREATE INDEX message_sessions_by_session ON message_sessions (session_id, message_id);
CREATE INDEX messages_by_at ON messages (at);
CREATE INDEX messages_by_dir_at ON messages (dir_id, at);
CREATE TABLE transcripts (path TEXT PRIMARY KEY, session_id TEXT NOT NULL REFERENCES sessions, size INTEGER NOT NULL, mtime INTEGER NOT NULL, read_to INTEGER NOT NULL);
CREATE TABLE gaps (off_at INTEGER NOT NULL, on_at INTEGER NOT NULL);
PRAGMA user_version = 1;
`

const (
	firstAt  = "2026-10-18T07:59:00.000Z"
	replyAt  = "2026-10-18T08:00:00.000Z"
	goneAt   = int64(1760000000000)
	writes1h = int64(300)
)

// v1 is a store written by schema version 1: sessionA's transcript is on
// disk and its message was stored without the split, and sessionB's
// transcript is gone.
type v1 struct {
	*fixture
	path string
}

func newV1(t *testing.T) *v1 {
	t.Helper()
	f := &fixture{t: t, projects: filepath.Join(t.TempDir(), "projects")}
	transcript := f.transcript(sessionA,
		prompt(sessionA, lib, firstAt),
		reply{session: sessionA, id: "msg_1", at: replyAt, cwd: lib, usage: usage{Input: 1, Output: 2, CacheCreate: writes1h, CacheRead: 4, Creation: &split{OneHour: writes1h}, Geo: "us"}}.line(),
	)
	info, err := os.Stat(transcript)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "stats.db")
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	_, err = db.Exec(schemaV1)
	require.NoError(t, err)
	for _, stmt := range []string{
		"INSERT INTO dirs (id, path) VALUES (1, '" + lib + "')",
		fmt.Sprintf("INSERT INTO sessions (id, started_at) VALUES ('%s', %d)", sessionA, ms(t, firstAt)),
		fmt.Sprintf("INSERT INTO sessions (id, started_at, transcript_gone_at) VALUES ('%s', %d, %d)", sessionB, ms(t, firstAt), goneAt),
		fmt.Sprintf("INSERT INTO messages VALUES ('msg_1', %d, 'claude-opus-5', NULL, 1, 'main', 1, 2, %d, 4)", ms(t, replyAt), writes1h),
		fmt.Sprintf("INSERT INTO messages VALUES ('msg_9', %d, 'claude-opus-5', NULL, 1, 'main', 1, 2, 50, 4)", ms(t, replyAt)),
		"INSERT INTO message_sessions VALUES ('msg_1', '" + sessionA + "'), ('msg_9', '" + sessionB + "')",
		fmt.Sprintf("INSERT INTO transcripts VALUES ('%s', '%s', %d, %d, %d)", transcript, sessionA, info.Size(), info.ModTime().UnixMilli(), info.Size()),
		fmt.Sprintf("INSERT INTO transcripts VALUES ('%s', '%s', 10, 1, 10)", f.transcriptPath(sessionB), sessionB),
		"INSERT INTO gaps VALUES (1, 2)",
	} {
		_, err := db.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	return &v1{fixture: f, path: path}
}

func (v *v1) open() {
	v.t.Helper()
	v.store = open(v.t, v.path)
}

func (v *v1) others() string {
	v.t.Helper()
	var out []string
	for _, q := range []string{
		"SELECT id || ' ' || path FROM dirs ORDER BY id",
		"SELECT id || ' ' || coalesce(started_at, '-') || ' ' || coalesce(transcript_gone_at, '-') FROM sessions ORDER BY id",
		"SELECT message_id || ' ' || session_id FROM message_sessions ORDER BY 1",
		"SELECT off_at || ' ' || on_at FROM gaps ORDER BY 1",
	} {
		rows, err := v.store.DB().Query(q)
		require.NoError(v.t, err)
		for rows.Next() {
			var line string
			require.NoError(v.t, rows.Scan(&line))
			out = append(out, line)
		}
		require.NoError(v.t, rows.Close())
	}
	return strings.Join(out, "\n")
}

func TestMigrateFromVersion1(t *testing.T) {
	v := newV1(t)
	v.open()

	t.Run("it should bring it to version 2", func(t *testing.T) {
		assert.Equal(t, 2, v.count("PRAGMA user_version"))
	})

	t.Run("it should add the split and geography columns to messages", func(t *testing.T) {
		assert.Equal(t, []string{"cache_create_5m", "cache_create_1h", "geo"}, columns(t, v.store.DB(), "messages")[10:])
	})

	t.Run("it should keep its messages", func(t *testing.T) {
		assert.Equal(t, 2, v.messages())
	})

	t.Run("it should leave the split unknown on its existing messages", func(t *testing.T) {
		assert.Equal(t, 2, v.count("SELECT count(*) FROM messages WHERE cache_create_5m IS NULL AND cache_create_1h IS NULL"))
	})

	t.Run("it should set every transcript to be read again from the start", func(t *testing.T) {
		assert.Equal(t, 0, v.count("SELECT count(*) FROM transcripts WHERE read_to <> 0"))
	})
}

func TestBackfill(t *testing.T) {
	v := newV1(t)
	v.open()
	before := v.others()
	v.ingest()

	t.Run("when a message's transcript is still on disk", func(t *testing.T) {
		t.Run("it should fill its 1 h writes from the transcript", func(t *testing.T) {
			assert.Equal(t, writes1h, v.number("SELECT cache_create_1h FROM messages WHERE id = 'msg_1'"))
		})

		t.Run("it should fill its 5 m writes from the transcript", func(t *testing.T) {
			assert.Equal(t, int64(0), v.number("SELECT cache_create_5m FROM messages WHERE id = 'msg_1'"))
		})

		t.Run("it should fill its geography", func(t *testing.T) {
			assert.Equal(t, "us", v.text("SELECT geo FROM messages WHERE id = 'msg_1'").String)
		})
	})

	t.Run("when a message's transcript is gone", func(t *testing.T) {
		t.Run("it should keep the message", func(t *testing.T) {
			assert.True(t, v.has("msg_9"))
		})

		t.Run("it should keep its split unknown", func(t *testing.T) {
			assert.Equal(t, 1, v.count("SELECT count(*) FROM messages WHERE id = 'msg_9' AND cache_create_1h IS NULL"))
		})
	})

	t.Run("it should still count each message once", func(t *testing.T) {
		assert.Equal(t, 2, v.messages())
	})

	t.Run("it should leave every other table as it was", func(t *testing.T) {
		assert.Equal(t, before, v.others())
	})
}

func TestIngestSplit(t *testing.T) {
	cases := []struct {
		name    string
		u       usage
		column  string
		want    any
		unknown bool
	}{
		{"when a message records its cache writes split by lifetime, it should store the 5 m writes",
			usage{CacheCreate: 30, Creation: &split{FiveMinutes: 10, OneHour: 20}}, "cache_create_5m", int64(10), false},
		{"when a message records its cache writes split by lifetime, it should store the 1 h writes",
			usage{CacheCreate: 30, Creation: &split{FiveMinutes: 10, OneHour: 20}}, "cache_create_1h", int64(20), false},
		{"when a message records no split, and it wrote to the cache, it should leave the split unknown",
			usage{CacheCreate: 30}, "cache_create_1h", nil, true},
		{"when a message records no split, and it wrote nothing to the cache, it should store no 5 m writes",
			usage{}, "cache_create_5m", int64(0), false},
		{"when a message records no split, and it wrote nothing to the cache, it should store no 1 h writes",
			usage{}, "cache_create_1h", int64(0), false},
		{"when a message records its inference geography, it should store it",
			usage{Geo: "global"}, "geo", "global", false},
		{"when a message records no inference geography, it should leave it unrecorded",
			usage{}, "geo", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.transcript(sessionA, reply{session: sessionA, id: "msg_1", at: replyAt, cwd: lib, usage: c.u}.line())
			f.ingest()
			var got any
			require.NoError(t, f.store.DB().QueryRow("SELECT "+c.column+" FROM messages WHERE id = 'msg_1'").Scan(&got))
			if c.unknown {
				assert.Nil(t, got)
				return
			}
			assert.Equal(t, c.want, got)
		})
	}
}
