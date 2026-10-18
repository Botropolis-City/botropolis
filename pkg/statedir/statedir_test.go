package statedir_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/statedir"
)

func TestDefault(t *testing.T) {
	t.Run("when XDG_STATE_HOME is set", func(t *testing.T) {
		state := t.TempDir()
		t.Setenv("XDG_STATE_HOME", state)

		t.Run("it should put the state directory under it", func(t *testing.T) {
			assert.Equal(t, filepath.Join(state, "botropolis"), statedir.Default())
		})
	})

	t.Run("when XDG_STATE_HOME is unset", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_STATE_HOME", "")

		t.Run("it should put the state directory under HOME/.local/state", func(t *testing.T) {
			assert.Equal(t, filepath.Join(home, ".local", "state", "botropolis"), statedir.Default())
		})
	})
}

func TestDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("TMPDIR", t.TempDir())

	t.Run("when no private directory was made", func(t *testing.T) {
		t.Run("it should resolve to the default", func(t *testing.T) {
			assert.Equal(t, statedir.Default(), statedir.Dir())
		})

		t.Run("it should not say the state is private", func(t *testing.T) {
			assert.False(t, statedir.IsPrivate())
		})
	})

	t.Run("when a private directory is made", func(t *testing.T) {
		seed := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(seed, "ledger.json"), []byte(`{"seeded":{}}`), 0o600))
		cleanup, err := statedir.MakePrivate(seed)
		require.NoError(t, err)
		dir := statedir.Dir()

		t.Run("it should resolve outside the default", func(t *testing.T) {
			assert.False(t, strings.HasPrefix(dir, statedir.Default()), dir)
		})

		t.Run("it should say the state is private", func(t *testing.T) {
			assert.True(t, statedir.IsPrivate())
		})

		t.Run("and the seed holds a file", func(t *testing.T) {
			data, _ := os.ReadFile(filepath.Join(dir, "ledger.json"))

			t.Run("it should copy it in", func(t *testing.T) {
				assert.Equal(t, `{"seeded":{}}`, string(data))
			})
		})

		t.Run("and it is cleaned up", func(t *testing.T) {
			require.NoError(t, cleanup())

			t.Run("it should remove the directory", func(t *testing.T) {
				assert.NoDirExists(t, dir)
			})

			t.Run("it should resolve to the default again", func(t *testing.T) {
				assert.Equal(t, statedir.Default(), statedir.Dir())
			})
		})
	})

	t.Run("when a private directory another process made is adopted", func(t *testing.T) {
		made := t.TempDir()
		forget := statedir.Adopt(made)

		t.Run("it should resolve to it", func(t *testing.T) {
			assert.Equal(t, made, statedir.Dir())
		})

		t.Run("it should say the state is private", func(t *testing.T) {
			assert.True(t, statedir.IsPrivate())
		})

		t.Run("and it is forgotten", func(t *testing.T) {
			forget()

			t.Run("it should leave the directory for the process that made it", func(t *testing.T) {
				assert.DirExists(t, made)
			})

			t.Run("it should resolve to the default again", func(t *testing.T) {
				assert.Equal(t, statedir.Default(), statedir.Dir())
			})
		})
	})

	t.Run("when a private directory is made from a seed that does not exist", func(t *testing.T) {
		cleanup, err := statedir.MakePrivate(filepath.Join(t.TempDir(), "missing"))
		t.Cleanup(func() { _ = cleanup() })

		t.Run("it should not fail", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should make an empty directory", func(t *testing.T) {
			entries, _ := os.ReadDir(statedir.Dir())
			assert.Empty(t, entries)
		})
	})
}
