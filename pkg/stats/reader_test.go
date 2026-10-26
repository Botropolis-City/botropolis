package stats_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
)

// written is a store at this version, written and closed, with its
// version and readable_from set as given; zero leaves them as written,
// and a negative readable_from removes it.
func written(t *testing.T, version, readableFrom int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stats.db")
	s, err := stats.Open(path)
	require.NoError(t, err)
	_, err = s.DB().Exec("INSERT INTO dirs (path) VALUES ('/w/lib')")
	require.NoError(t, err)
	if version != 0 {
		_, err = s.DB().Exec(fmt.Sprintf("PRAGMA user_version = %d", version))
		require.NoError(t, err)
	}
	if readableFrom < 0 {
		_, err = s.DB().Exec("DELETE FROM meta WHERE key = 'readable_from'")
		require.NoError(t, err)
	}
	if readableFrom > 0 {
		_, err = s.DB().Exec(fmt.Sprintf("UPDATE meta SET value = %d WHERE key = 'readable_from'", readableFrom))
		require.NoError(t, err)
	}
	require.NoError(t, s.Close())
	return path
}

func openReader(t *testing.T, path string) error {
	t.Helper()
	r, err := stats.OpenReader(path)
	if r != nil {
		t.Cleanup(func() { _ = r.Close() })
	}
	return err
}

func TestOpenReader(t *testing.T) {
	t.Run("when the store is at this version, it should open it", func(t *testing.T) {
		assert.NoError(t, openReader(t, written(t, 0, 0)))
	})

	t.Run("when the store is newer and says this version can read it, it should open it", func(t *testing.T) {
		assert.NoError(t, openReader(t, written(t, stats.Version+1, stats.Version)))
	})

	cases := []struct {
		name, path, reason string
	}{
		{"when the store is newer and says this version cannot read it, it should be paused because it is newer",
			"newer", "this store is from a newer Botropolis — update to read it"},
		{"when the store is newer and does not say who can read it, it should be paused because it is newer",
			"silent", "this store is from a newer Botropolis — update to read it"},
		{"when the store is older than this version, it should be paused while it is upgraded",
			"older", "the store is being upgraded"},
	}
	paths := map[string]func(t *testing.T) string{
		"newer":  func(t *testing.T) string { return written(t, stats.Version+1, stats.Version+1) },
		"silent": func(t *testing.T) string { return written(t, stats.Version+1, -1) },
		"older":  func(t *testing.T) string { return written(t, stats.Version-1, 0) },
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.reason, stats.Reason(openReader(t, paths[c.path](t))))
		})
	}

	t.Run("when the store is at this version but has no meta table, it should refuse it as damaged", func(t *testing.T) {
		assert.ErrorIs(t, openReader(t, damaged(t)), stats.ErrDamaged)
	})

	t.Run("when the file is not a database, it should be paused with the error in plain words", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "stats.db")
		require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("not sqlite ", 200)), 0o600))
		assert.True(t, strings.HasPrefix(stats.Reason(openReader(t, path)), "the store could not be read: "))
	})

	t.Run("when there is no store yet, it should be paused saying so", func(t *testing.T) {
		assert.Equal(t, "there are no statistics yet", stats.Reason(openReader(t, filepath.Join(t.TempDir(), "stats.db"))))
	})

	t.Run("when there is no error, it should give no reason", func(t *testing.T) {
		assert.Equal(t, "", stats.Reason(nil))
	})

	t.Run("when a writer holds the store open and has just written", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "stats.db")
		w := open(t, path)
		_, err := w.DB().Exec("INSERT INTO dirs (path) VALUES ('/w/lib'), ('/w/app')")
		require.NoError(t, err)
		_, err = w.DB().Exec("INSERT INTO messages (id, at, model, dir_id, thread, input, output, cache_create, cache_read) VALUES ('m1', 0, 'x', 1, 'main', 0, 0, 0, 0), ('m2', 0, 'x', 2, 'main', 0, 0, 0, 0)")
		require.NoError(t, err)
		r, err := stats.OpenReader(path)
		require.NoError(t, err)
		t.Cleanup(func() { _ = r.Close() })

		t.Run("it should read what the writer wrote", func(t *testing.T) {
			districts, err := r.Districts(nil)
			require.NoError(t, err)
			assert.Equal(t, []string{"/w/app", "/w/lib"}, districts)
		})
	})

	t.Run("when the store is opened and read", func(t *testing.T) {
		path := written(t, 0, 0)
		before, err := os.ReadFile(path)
		require.NoError(t, err)
		info, err := os.Stat(path)
		require.NoError(t, err)
		r, err := stats.OpenReader(path)
		require.NoError(t, err)
		_, err = r.Districts(nil)
		require.NoError(t, err)
		require.NoError(t, r.Close())
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		again, err := os.Stat(path)
		require.NoError(t, err)

		t.Run("it should leave the file's bytes as they were", func(t *testing.T) {
			assert.Equal(t, before, after)
		})

		t.Run("it should leave the file's modification time as it was", func(t *testing.T) {
			assert.Equal(t, info.ModTime(), again.ModTime())
		})

		t.Run("it should refuse to write", func(t *testing.T) {
			r, err := stats.OpenReader(path)
			require.NoError(t, err)
			t.Cleanup(func() { _ = r.Close() })
			_, err = r.DB().Exec("INSERT INTO dirs (path) VALUES ('/w/app')")
			assert.Error(t, err)
		})
	})
}
