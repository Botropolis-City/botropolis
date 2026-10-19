package stats_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/statedir"
	"botropolis.city/botropolis/pkg/stats"
)

func TestPath(t *testing.T) {
	t.Run("when the process works in a private state directory", func(t *testing.T) {
		cleanup, err := statedir.MakePrivate(filepath.Join(t.TempDir(), "no-seed"))
		require.NoError(t, err)
		t.Cleanup(func() { _ = cleanup() })
		private := statedir.Dir()

		t.Run("it should be stats.db inside it", func(t *testing.T) {
			assert.Equal(t, filepath.Join(private, "stats.db"), stats.Path())
		})
	})

	t.Run("when it does not", func(t *testing.T) {
		state := t.TempDir()
		t.Setenv("XDG_STATE_HOME", state)

		t.Run("it should be stats.db under $XDG_STATE_HOME/botropolis", func(t *testing.T) {
			assert.Equal(t, filepath.Join(state, "botropolis", "stats.db"), stats.Path())
		})
	})
}

func open(t *testing.T, path string) *stats.Store {
	t.Helper()
	s, err := stats.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func columns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query("SELECT name FROM pragma_table_info(?) ORDER BY cid", table)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		out = append(out, name)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestOpen(t *testing.T) {
	t.Run("when no database exists", func(t *testing.T) {
		t.Run("and its directory does not exist either", func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "state", "botropolis")
			open(t, filepath.Join(dir, "stats.db"))

			t.Run("it should create the directory", func(t *testing.T) {
				assert.DirExists(t, dir)
			})
		})

		path := filepath.Join(t.TempDir(), "stats.db")
		s := open(t, path)
		db := s.DB()

		t.Run("it should create the file", func(t *testing.T) {
			assert.FileExists(t, path)
		})

		t.Run("it should make the file readable by its owner only", func(t *testing.T) {
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		})

		t.Run("it should set the schema version to 2", func(t *testing.T) {
			var version int
			require.NoError(t, db.QueryRow("PRAGMA user_version").Scan(&version))
			assert.Equal(t, 2, version)
		})

		t.Run("it should use write-ahead logging", func(t *testing.T) {
			var mode string
			require.NoError(t, db.QueryRow("PRAGMA journal_mode").Scan(&mode))
			assert.Equal(t, "wal", mode)
		})

		for _, table := range []struct {
			name    string
			columns []string
		}{
			{"dirs", []string{"id", "path"}},
			{"sessions", []string{"id", "started_at", "transcript_gone_at"}},
			{"messages", []string{"id", "at", "model", "speed", "dir_id", "thread", "input", "output", "cache_create", "cache_read", "cache_create_5m", "cache_create_1h", "geo"}},
			{"message_sessions", []string{"message_id", "session_id"}},
			{"transcripts", []string{"path", "session_id", "size", "mtime", "read_to"}},
			{"gaps", []string{"off_at", "on_at"}},
		} {
			t.Run("it should give "+table.name+" exactly its columns, in order", func(t *testing.T) {
				assert.Equal(t, table.columns, columns(t, db, table.name))
			})
		}

		for _, index := range []string{"message_sessions_by_session", "messages_by_at", "messages_by_dir_at"} {
			t.Run("it should create "+index, func(t *testing.T) {
				var n int
				require.NoError(t, db.QueryRow("SELECT count(*) FROM sqlite_schema WHERE type = 'index' AND name = ?", index).Scan(&n))
				assert.Equal(t, 1, n)
			})
		}

		t.Run("it should refuse a message whose directory does not exist", func(t *testing.T) {
			_, err := db.Exec(`INSERT INTO messages (id, at, model, dir_id, thread, input, output, cache_create, cache_read)
				VALUES ('msg_1', 0, 'claude-sonnet', 42, 'main', 1, 1, 0, 0)`)
			assert.Error(t, err)
		})
	})

	t.Run("when the database already exists at this version", func(t *testing.T) {
		t.Run("and it holds rows", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "stats.db")
			first, err := stats.Open(path)
			require.NoError(t, err)
			_, err = first.DB().Exec("INSERT INTO dirs (path) VALUES ('/home/someone/project')")
			require.NoError(t, err)
			require.NoError(t, first.Close())
			again := open(t, path)

			t.Run("it should keep them", func(t *testing.T) {
				var n int
				require.NoError(t, again.DB().QueryRow("SELECT count(*) FROM dirs").Scan(&n))
				assert.Equal(t, 1, n)
			})
		})
	})

	t.Run("when the database is from a newer version", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "stats.db")
		first, err := stats.Open(path)
		require.NoError(t, err)
		_, err = first.DB().Exec("PRAGMA user_version = 3")
		require.NoError(t, err)
		require.NoError(t, first.Close())
		newer, err := stats.Open(path)
		if newer != nil {
			t.Cleanup(func() { _ = newer.Close() })
		}

		t.Run("it should refuse to open it", func(t *testing.T) {
			assert.Error(t, err)
		})
	})

	t.Run("when a second store holds a read open on the same file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "stats.db")
		writer := open(t, path)
		reader := open(t, path)
		tx, err := reader.DB().Begin()
		require.NoError(t, err)
		t.Cleanup(func() { _ = tx.Rollback() })
		var n int
		require.NoError(t, tx.QueryRow("SELECT count(*) FROM dirs").Scan(&n))

		t.Run("it should still let the first write", func(t *testing.T) {
			_, err := writer.DB().Exec("INSERT INTO dirs (path) VALUES ('/home/someone/project')")
			assert.NoError(t, err)
		})
	})
}
