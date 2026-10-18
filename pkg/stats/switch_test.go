package stats_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
)

type stateDir struct {
	t   *testing.T
	dir string
}

func newStateDir(t *testing.T) stateDir { return stateDir{t: t, dir: t.TempDir()} }

func (d stateDir) db() string { return filepath.Join(d.dir, "stats.db") }

func (d stateDir) marker() string { return filepath.Join(d.dir, "stats-off") }

func (d stateDir) notes() string { return filepath.Join(d.dir, "stats-pruned") }

func (d stateDir) withDatabase() stateDir {
	d.t.Helper()
	s, err := stats.Open(d.db())
	require.NoError(d.t, err)
	_, err = s.DB().Exec("INSERT INTO dirs (path) VALUES ('/w/lib')")
	require.NoError(d.t, err)
	_, err = s.DB().Exec(`INSERT INTO messages (id, at, model, dir_id, thread, input, output, cache_create, cache_read)
		VALUES ('msg_1', 0, 'claude-opus-5', 1, 'main', 1, 1, 0, 0)`)
	require.NoError(d.t, err)
	require.NoError(d.t, s.Close())
	return d
}

func (d stateDir) withMarker() stateDir {
	d.t.Helper()
	require.NoError(d.t, os.WriteFile(d.marker(), nil, 0o600))
	return d
}

func (d stateDir) count(query string) int {
	d.t.Helper()
	s, err := stats.Open(d.db())
	require.NoError(d.t, err)
	defer func() { _ = s.Close() }()
	var n int
	require.NoError(d.t, s.DB().QueryRow(query).Scan(&n))
	return n
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestOwed(t *testing.T) {
	for _, c := range []struct {
		name  string
		on    bool
		setup func(stateDir) stateDir
		want  stats.Prompt
	}{
		{"when stats are on, and there is no marker, it should owe none", true, func(d stateDir) stateDir { return d.withDatabase() }, stats.NoPrompt},
		{"when stats are on, and the marker is there, it should owe the on prompt", true, func(d stateDir) stateDir { return d.withDatabase().withMarker() }, stats.OnPrompt},
		{"when stats are off, and a database exists with no marker, it should owe the off prompt", false, func(d stateDir) stateDir { return d.withDatabase() }, stats.OffPrompt},
		{"when stats are off, and the marker is there, it should owe none", false, func(d stateDir) stateDir { return d.withDatabase().withMarker() }, stats.NoPrompt},
		{"when stats are off, and there is no database, it should owe none", false, func(d stateDir) stateDir { return d }, stats.NoPrompt},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := c.setup(newStateDir(t))
			assert.Equal(t, c.want, stats.Owed(d.dir, c.on))
		})
	}
}

func TestTurnOff(t *testing.T) {
	t.Run("when keeping the data", func(t *testing.T) {
		d := newStateDir(t).withDatabase()
		require.NoError(t, stats.TurnOff(d.dir, true))

		t.Run("it should keep stats.db", func(t *testing.T) {
			assert.FileExists(t, d.db())
		})

		t.Run("it should write the marker", func(t *testing.T) {
			assert.FileExists(t, d.marker())
		})
	})

	t.Run("when deleting it", func(t *testing.T) {
		d := newStateDir(t).withDatabase()
		for _, suffix := range []string{"-wal", "-shm"} {
			require.NoError(t, os.WriteFile(d.db()+suffix, nil, 0o600))
		}
		require.NoError(t, stats.TurnOff(d.dir, false))

		for _, name := range []string{"stats.db", "stats.db-wal", "stats.db-shm"} {
			t.Run("it should delete "+name, func(t *testing.T) {
				assert.False(t, exists(filepath.Join(d.dir, name)))
			})
		}

		t.Run("it should leave no marker", func(t *testing.T) {
			assert.False(t, exists(d.marker()))
		})
	})
}

func TestTurnOn(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

	t.Run("when keeping the old data", func(t *testing.T) {
		d := newStateDir(t).withDatabase().withMarker()
		lastWritten := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(d.db(), lastWritten, lastWritten))
		if exists(d.db() + "-wal") {
			earlier := lastWritten.Add(-time.Hour)
			require.NoError(t, os.Chtimes(d.db()+"-wal", earlier, earlier))
		}
		require.NoError(t, stats.TurnOn(d.dir, true, now))

		t.Run("it should record a gap from when the database was last written to now", func(t *testing.T) {
			s, err := stats.Open(d.db())
			require.NoError(t, err)
			defer func() { _ = s.Close() }()
			var off, on int64
			require.NoError(t, s.DB().QueryRow("SELECT off_at, on_at FROM gaps").Scan(&off, &on))
			assert.Equal(t, [2]int64{lastWritten.UnixMilli(), now.UnixMilli()}, [2]int64{off, on})
		})

		t.Run("it should keep the old messages", func(t *testing.T) {
			assert.Equal(t, 1, d.count("SELECT count(*) FROM messages"))
		})

		t.Run("it should remove the marker", func(t *testing.T) {
			assert.False(t, exists(d.marker()))
		})
	})

	t.Run("when the -wal was written after the database file", func(t *testing.T) {
		d := newStateDir(t).withDatabase().withMarker()
		base := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
		later := base.Add(time.Hour)
		require.NoError(t, os.WriteFile(d.db()+"-wal", nil, 0o600))
		require.NoError(t, os.Chtimes(d.db(), base, base))
		require.NoError(t, os.Chtimes(d.db()+"-wal", later, later))
		require.NoError(t, stats.TurnOn(d.dir, true, now))

		t.Run("it should take the gap's start from the -wal", func(t *testing.T) {
			s, err := stats.Open(d.db())
			require.NoError(t, err)
			defer func() { _ = s.Close() }()
			var off int64
			require.NoError(t, s.DB().QueryRow("SELECT off_at FROM gaps").Scan(&off))
			assert.Equal(t, later.UnixMilli(), off)
		})
	})

	t.Run("when starting fresh", func(t *testing.T) {
		d := newStateDir(t).withDatabase().withMarker()
		require.NoError(t, stats.TurnOn(d.dir, false, now))

		t.Run("it should delete the old messages", func(t *testing.T) {
			assert.Equal(t, 0, d.count("SELECT count(*) FROM messages"))
		})

		t.Run("it should remove the marker", func(t *testing.T) {
			assert.False(t, exists(d.marker()))
		})
	})
}
