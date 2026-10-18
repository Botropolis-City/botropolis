package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 110: the statistics row.
func TestStatsSettingsRow(t *testing.T) {
	row := func(cfg *config.Config) ui.Setting {
		for _, s := range settingsFor(cfg).Items {
			if s.Key == config.KeyStats {
				return s
			}
		}
		return ui.Setting{}
	}
	for _, c := range []struct {
		on   bool
		want string
	}{{true, "on"}, {false, "off"}} {
		t.Run("when stats are "+c.want, func(t *testing.T) {
			t.Run("it should read "+c.want, func(t *testing.T) {
				assert.Equal(t, c.want, row(&config.Config{Stats: c.on}).Value)
			})
		})
	}
}

// Item 110: a statistics row turned on with nothing kept is saved as is.
func TestStatsRowSaved(t *testing.T) {
	t.Run("when the row is saved on over a file that says off", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "config.toml")
		require.NoError(t, os.WriteFile(file, []byte("stats = false\n"), 0o600))
		require.NoError(t, saveSetting(file, ui.Setting{Key: config.KeyStats, Value: "on"}))
		v := config.NewViper()
		v.Set(config.KeyConfig, file)
		cfg, err := config.New(v)
		require.NoError(t, err)

		t.Run("it should read back on", func(t *testing.T) {
			assert.True(t, cfg.Stats)
		})
	})
}

// Item 110: the app's answer to a statistics prompt: the store's own
// call, then the setting saved.
func TestStatsChoice(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	type world struct {
		state, file string
	}
	setup := func(t *testing.T, marker bool) world {
		t.Helper()
		w := world{state: filepath.Join(t.TempDir(), "botropolis"), file: filepath.Join(t.TempDir(), "config.toml")}
		s, err := stats.Open(filepath.Join(w.state, "stats.db"))
		require.NoError(t, err)
		_, err = s.DB().Exec("INSERT INTO sessions (id) VALUES ('s1')")
		require.NoError(t, err)
		require.NoError(t, s.Close())
		if marker {
			require.NoError(t, os.WriteFile(filepath.Join(w.state, "stats-off"), nil, 0o600))
		}
		return w
	}
	saved := func(t *testing.T, w world) bool {
		t.Helper()
		v := config.NewViper()
		v.Set(config.KeyConfig, w.file)
		cfg, err := config.New(v)
		require.NoError(t, err)
		return cfg.Stats
	}
	count := func(t *testing.T, w world, query string) int {
		t.Helper()
		s, err := stats.Open(filepath.Join(w.state, "stats.db"))
		require.NoError(t, err)
		defer func() { _ = s.Close() }()
		var n int
		require.NoError(t, s.DB().QueryRow(query).Scan(&n))
		return n
	}
	choose := func(t *testing.T, w world, p stats.Prompt, keep bool) {
		t.Helper()
		require.NoError(t, statsChoice(w.file, w.state, func() time.Time { return now })(p, keep))
	}

	t.Run("when off is chosen, keeping the data", func(t *testing.T) {
		w := setup(t, false)
		choose(t, w, stats.OffPrompt, true)

		t.Run("it should write the marker", func(t *testing.T) {
			assert.FileExists(t, filepath.Join(w.state, "stats-off"))
		})

		t.Run("it should save stats = false", func(t *testing.T) {
			assert.False(t, saved(t, w))
		})
	})

	t.Run("when off is chosen, deleting it", func(t *testing.T) {
		w := setup(t, false)
		choose(t, w, stats.OffPrompt, false)

		t.Run("it should delete stats.db", func(t *testing.T) {
			assert.NoFileExists(t, filepath.Join(w.state, "stats.db"))
		})

		t.Run("it should save stats = false", func(t *testing.T) {
			assert.False(t, saved(t, w))
		})
	})

	t.Run("when on is chosen, keeping the old data", func(t *testing.T) {
		w := setup(t, true)
		choose(t, w, stats.OnPrompt, true)

		t.Run("it should record a gap", func(t *testing.T) {
			assert.Equal(t, 1, count(t, w, "SELECT count(*) FROM gaps"))
		})

		t.Run("it should save stats = true", func(t *testing.T) {
			assert.True(t, saved(t, w))
		})
	})

	t.Run("when on is chosen, starting fresh", func(t *testing.T) {
		w := setup(t, true)
		choose(t, w, stats.OnPrompt, false)

		t.Run("it should delete the old data", func(t *testing.T) {
			assert.Equal(t, 0, count(t, w, "SELECT count(*) FROM sessions"))
		})

		t.Run("it should save stats = true", func(t *testing.T) {
			assert.True(t, saved(t, w))
		})
	})
}
