package appd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
)

// Item 110: hefts come from the statistics store while statistics are
// on and nothing about them is owed, and from a whole read otherwise.
// Item 190: whichever it is, the daemon says so, and says why a store
// it would have used was refused.
func TestStatsWiring(t *testing.T) {
	stateDir := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		t.Setenv("XDG_STATE_HOME", root)
		return filepath.Join(root, "botropolis")
	}
	source := func(t *testing.T, cfg *config.Config) (*stats.Store, state.StatsStore, string) {
		t.Helper()
		var log bytes.Buffer
		src, st := statsSource(cfg, &log)
		if src != nil {
			t.Cleanup(func() { _ = src.Close() })
		}
		return src, st, log.String()
	}
	// written is a store at dir's stats.db, changed by stmts once closed.
	written := func(t *testing.T, dir string, stmts ...string) string {
		t.Helper()
		path := filepath.Join(dir, "stats.db")
		s, err := stats.Open(path)
		require.NoError(t, err)
		for _, stmt := range stmts {
			_, err = s.DB().Exec(stmt)
			require.NoError(t, err)
		}
		require.NoError(t, s.Close())
		return path
	}

	t.Run("when stats are on and nothing is owed", func(t *testing.T) {
		stateDir(t)
		src, st, _ := source(t, &config.Config{Stats: true})

		t.Run("it should take hefts from the store", func(t *testing.T) {
			assert.NotNil(t, src)
		})

		t.Run("it should report the store open", func(t *testing.T) {
			assert.Equal(t, state.StoreOpen, st.State)
		})

		t.Run("and it is asked again in the same process", func(t *testing.T) {
			again, _ := statsSource(&config.Config{Stats: true}, &bytes.Buffer{})

			t.Run("it should give the same store", func(t *testing.T) {
				assert.Same(t, src, again)
			})
		})
	})

	t.Run("when the store will not open because it is damaged", func(t *testing.T) {
		path := written(t, stateDir(t), "DROP TABLE meta")
		src, st, log := source(t, &config.Config{Stats: true})

		t.Run("it should read whole", func(t *testing.T) {
			assert.Nil(t, src)
		})

		t.Run("it should report the store refused", func(t *testing.T) {
			assert.Equal(t, state.StoreRefused, st.State)
		})

		t.Run("it should name the file", func(t *testing.T) {
			assert.Equal(t, path, st.Path)
		})

		t.Run("it should give the reason as damaged", func(t *testing.T) {
			assert.Contains(t, st.Reason, "damaged")
		})

		t.Run("it should say why: damaged", func(t *testing.T) {
			assert.Equal(t, state.WhyDamaged, st.Why)
		})

		t.Run("it should log the error, naming the file", func(t *testing.T) {
			assert.Contains(t, log, path+" is ")
		})
	})

	t.Run("when the store is from a newer build that this one cannot read", func(t *testing.T) {
		written(t, stateDir(t),
			fmt.Sprintf("PRAGMA user_version = %d", stats.Version+1),
			fmt.Sprintf("UPDATE meta SET value = %d WHERE key = 'readable_from'", stats.Version+1))
		_, st, _ := source(t, &config.Config{Stats: true})

		t.Run("it should report the store refused", func(t *testing.T) {
			assert.Equal(t, state.StoreRefused, st.State)
		})

		t.Run("it should say to update Botropolis", func(t *testing.T) {
			assert.Contains(t, st.Reason, "update Botropolis")
		})

		t.Run("it should say why: newer", func(t *testing.T) {
			assert.Equal(t, state.WhyNewer, st.Why)
		})
	})

	t.Run("when stats are off", func(t *testing.T) {
		dir := stateDir(t)
		src, st, _ := source(t, &config.Config{Stats: false})

		t.Run("it should read whole, as before", func(t *testing.T) {
			assert.Nil(t, src)
		})

		t.Run("it should report statistics off", func(t *testing.T) {
			assert.Equal(t, state.StoreOff, st.State)
		})

		t.Run("it should not create a database", func(t *testing.T) {
			assert.NoFileExists(t, filepath.Join(dir, "stats.db"))
		})
	})

	t.Run("when the daemon was pointed at a home", func(t *testing.T) {
		dir := stateDir(t)
		src, st, _ := source(t, &config.Config{Stats: true, HomeSet: true})

		t.Run("it should read whole", func(t *testing.T) {
			assert.Nil(t, src)
		})

		t.Run("it should report statistics off", func(t *testing.T) {
			assert.Equal(t, state.StoreOff, st.State)
		})

		t.Run("it should not create a database", func(t *testing.T) {
			assert.NoFileExists(t, filepath.Join(dir, "stats.db"))
		})
	})

	t.Run("when the on prompt is owed", func(t *testing.T) {
		dir := stateDir(t)
		written(t, dir)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "stats-off"), nil, 0o600))
		src, st, _ := source(t, &config.Config{Stats: true})

		t.Run("it should read whole", func(t *testing.T) {
			assert.Nil(t, src)
		})

		t.Run("it should report the store waiting on the prompt", func(t *testing.T) {
			assert.Equal(t, state.StoreOwed, st.State)
		})

		t.Run("it should not open the store", func(t *testing.T) {
			assert.NoFileExists(t, filepath.Join(dir, "stats.db-wal"))
		})
	})
}
