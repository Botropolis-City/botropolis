package stats_test

import (
	"database/sql"
	"fmt"
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

// damaged is a store at this version with no meta table, left in
// rollback-journal mode, as one was found on a real machine (item 190).
func damaged(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stats.db")
	s, err := stats.Open(path)
	require.NoError(t, err)
	for _, stmt := range []string{"DROP TABLE meta", "PRAGMA journal_mode = DELETE"} {
		_, err = s.DB().Exec(stmt)
		require.NoError(t, err)
	}
	require.NoError(t, s.Close())
	return path
}

func metaValue(t *testing.T, db *sql.DB, key string) int {
	t.Helper()
	var v int
	require.NoError(t, db.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&v))
	return v
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
	t.Run("when a store is opened", func(t *testing.T) {
		t.Run("and the pool opens three connections at once", func(t *testing.T) {
			db := open(t, filepath.Join(t.TempDir(), "stats.db")).DB()
			caches := make([]int, 3)
			for i := range caches {
				conn, err := db.Conn(t.Context())
				require.NoError(t, err)
				t.Cleanup(func() { _ = conn.Close() })
				require.NoError(t, conn.QueryRowContext(t.Context(), "PRAGMA cache_size").Scan(&caches[i]))
			}

			for i, cache := range caches {
				t.Run(fmt.Sprintf("it should give connection %d a cache of CacheKiB", i+1), func(t *testing.T) {
					assert.Equal(t, -stats.CacheKiB, cache)
				})
			}
		})
	})

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

		t.Run("it should set the schema version to 3", func(t *testing.T) {
			var version int
			require.NoError(t, db.QueryRow("PRAGMA user_version").Scan(&version))
			assert.Equal(t, 3, version)
		})

		t.Run("it should record the oldest version that can read and write it", func(t *testing.T) {
			assert.Equal(t, stats.ReadableFrom, metaValue(t, db, "readable_from"))
		})

		t.Run("it should record this version as the last to write it", func(t *testing.T) {
			assert.Equal(t, stats.Version, metaValue(t, db, "written_by"))
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
			{"messages", []string{"id", "at", "model", "speed", "dir_id", "thread", "input", "output", "cache_create", "cache_read", "cache_create_5m", "cache_create_1h", "geo", "web_search_requests", "web_fetch_requests", "service_tier"}},
			{"message_sessions", []string{"message_id", "session_id"}},
			{"transcripts", []string{"path", "session_id", "size", "mtime", "read_to"}},
			{"gaps", []string{"off_at", "on_at"}},
			{"meta", []string{"key", "value"}},
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
		newer := func(t *testing.T, readableFrom string) (*stats.Store, error) {
			t.Helper()
			path := filepath.Join(t.TempDir(), "stats.db")
			first, err := stats.Open(path)
			require.NoError(t, err)
			_, err = first.DB().Exec(fmt.Sprintf("PRAGMA user_version = %d", stats.Version+1))
			require.NoError(t, err)
			if readableFrom == "" {
				_, err = first.DB().Exec("DELETE FROM meta WHERE key = 'readable_from'")
			} else {
				_, err = first.DB().Exec("UPDATE meta SET value = " + readableFrom + " WHERE key = 'readable_from'")
			}
			require.NoError(t, err)
			_, err = first.DB().Exec(fmt.Sprintf("UPDATE meta SET value = %d WHERE key = 'written_by'", stats.Version+1))
			require.NoError(t, err)
			require.NoError(t, first.Close())
			s, err := stats.Open(path)
			if s != nil {
				t.Cleanup(func() { _ = s.Close() })
			}
			return s, err
		}

		t.Run("and it says this version can still read and write it", func(t *testing.T) {
			s, err := newer(t, fmt.Sprint(stats.Version))

			t.Run("it should open it", func(t *testing.T) {
				assert.NoError(t, err)
			})

			t.Run("it should leave its version alone", func(t *testing.T) {
				var version int
				require.NoError(t, s.DB().QueryRow("PRAGMA user_version").Scan(&version))
				assert.Equal(t, stats.Version+1, version)
			})

			t.Run("it should record this version as the last to write it", func(t *testing.T) {
				assert.Equal(t, stats.Version, metaValue(t, s.DB(), "written_by"))
			})
		})

		t.Run("and it says this version cannot", func(t *testing.T) {
			_, err := newer(t, fmt.Sprint(stats.Version+1))

			t.Run("it should refuse to open it", func(t *testing.T) {
				assert.ErrorContains(t, err, "update Botropolis")
			})

			t.Run("it should refuse it as newer", func(t *testing.T) {
				assert.ErrorIs(t, err, stats.ErrNewer)
			})
		})

		t.Run("and it does not say", func(t *testing.T) {
			_, err := newer(t, "")

			t.Run("it should refuse to open it", func(t *testing.T) {
				assert.ErrorContains(t, err, "update Botropolis")
			})
		})
	})

	t.Run("when the database is at this version but has no meta table", func(t *testing.T) {
		path := damaged(t)
		s, err := stats.Open(path)
		if s != nil {
			t.Cleanup(func() { _ = s.Close() })
		}

		t.Run("it should refuse it as damaged", func(t *testing.T) {
			assert.ErrorIs(t, err, stats.ErrDamaged)
		})

		t.Run("it should name the file", func(t *testing.T) {
			assert.ErrorContains(t, err, path)
		})

		t.Run("it should leave the file as it was", func(t *testing.T) {
			db, err := sql.Open("sqlite", path)
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			var mode string
			require.NoError(t, db.QueryRow("PRAGMA journal_mode").Scan(&mode))
			assert.Equal(t, "delete", mode)
		})
	})

	t.Run("when an older version wrote to it since this one last did", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "stats.db")
		first, err := stats.Open(path)
		require.NoError(t, err)
		for _, stmt := range []string{
			"INSERT INTO sessions (id) VALUES ('s')",
			"INSERT INTO transcripts VALUES ('/p/s.jsonl', 's', 10, 1, 10)",
			fmt.Sprintf("UPDATE meta SET value = %d WHERE key = 'written_by'", stats.Version-1),
		} {
			_, err = first.DB().Exec(stmt)
			require.NoError(t, err)
		}
		require.NoError(t, first.Close())
		again := open(t, path)

		t.Run("it should set every transcript to be read again from the start", func(t *testing.T) {
			var readTo int64
			require.NoError(t, again.DB().QueryRow("SELECT read_to FROM transcripts").Scan(&readTo))
			assert.Equal(t, int64(0), readTo)
		})

		t.Run("it should record this version as the last to write it", func(t *testing.T) {
			assert.Equal(t, stats.Version, metaValue(t, again.DB(), "written_by"))
		})
	})

	t.Run("when this version wrote it last", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "stats.db")
		first, err := stats.Open(path)
		require.NoError(t, err)
		for _, stmt := range []string{
			"INSERT INTO sessions (id) VALUES ('s')",
			"INSERT INTO transcripts VALUES ('/p/s.jsonl', 's', 10, 1, 10)",
		} {
			_, err = first.DB().Exec(stmt)
			require.NoError(t, err)
		}
		require.NoError(t, first.Close())
		again := open(t, path)

		t.Run("it should leave every transcript where it was read to", func(t *testing.T) {
			var readTo int64
			require.NoError(t, again.DB().QueryRow("SELECT read_to FROM transcripts").Scan(&readTo))
			assert.Equal(t, int64(10), readTo)
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
