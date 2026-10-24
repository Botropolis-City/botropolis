package targets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapDir(t *testing.T) {
	for _, c := range []struct {
		when, cache, want string
	}{
		{"when XDG_CACHE_HOME is set", "/c", "/c/botropolis/acceptance-map"},
		{"when XDG_CACHE_HOME is not set", "", "/home/me/.cache/botropolis/acceptance-map"},
	} {
		t.Run(c.when, func(t *testing.T) {
			got := MapDir(func(string) string { return c.cache }, "/home/me")

			t.Run("it should keep the maps where every checkout and worktree finds them", func(t *testing.T) {
				assert.Equal(t, c.want, got)
			})
		})
	}
}

func TestSharedMap(t *testing.T) {
	behind := map[string]int{"aaa": 12, "bbb": 3, "ccc": -1}
	distance := func(rev string) int {
		if n, ok := behind[rev]; ok {
			return n
		}
		return -1
	}

	t.Run("when a worktree has no map of its own and the shared directory holds one at an ancestor", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, SaveMap(dir, &Map{Rev: "aaa", Tests: map[string][]string{"TestA": {"p"}}}))
		m, n := NearestMap(dir, distance)

		t.Run("it should use the shared map", func(t *testing.T) {
			require.NotNil(t, m)
			assert.Equal(t, "aaa", m.Rev)
		})

		t.Run("it should say how far behind HEAD it is", func(t *testing.T) {
			assert.Equal(t, 12, n)
		})
	})

	t.Run("when the shared directory holds several", func(t *testing.T) {
		dir := t.TempDir()
		for _, rev := range []string{"aaa", "bbb", "ccc"} {
			require.NoError(t, SaveMap(dir, &Map{Rev: rev, Tests: map[string][]string{}}))
		}
		m, n := NearestMap(dir, distance)

		t.Run("it should use the nearest ancestor", func(t *testing.T) {
			require.NotNil(t, m)
			assert.Equal(t, []any{"bbb", 3}, []any{m.Rev, n})
		})
	})

	t.Run("when it holds only maps that are not ancestors", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, SaveMap(dir, &Map{Rev: "ccc", Tests: map[string][]string{}}))
		m, n := NearestMap(dir, distance)

		t.Run("it should hand back one, marked as not an ancestor", func(t *testing.T) {
			require.NotNil(t, m)
			assert.Equal(t, -1, n)
		})
	})

	t.Run("when it holds none", func(t *testing.T) {
		m, _ := NearestMap(filepath.Join(t.TempDir(), "absent"), distance)

		t.Run("it should hand back no map", func(t *testing.T) {
			assert.Nil(t, m)
		})
	})

	t.Run("when a full run passes", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "maps")
		require.NoError(t, SaveMap(dir, &Map{Rev: "ddd", Tests: map[string][]string{}}))

		t.Run("it should write the map under its revision", func(t *testing.T) {
			assert.FileExists(t, filepath.Join(dir, "ddd.json"))
		})
	})
}

func TestReadCI(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct{ when, record, want string }{
		{"when ci-status recorded green", "green abc1234 2026-10-24T12:00:00Z\n", "green"},
		{"when ci-status recorded red", "red abc1234 2026-10-24T12:00:00Z\n", "red"},
		{"when ci-status recorded nothing it knows", "maybe - now\n", "unknown"},
	} {
		t.Run(c.when, func(t *testing.T) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "botropolis-ci"), []byte(c.record), 0o644))

			t.Run("it should read "+c.want, func(t *testing.T) {
				assert.Equal(t, c.want, ReadCI(dir))
			})
		})
	}

	t.Run("when nothing was recorded", func(t *testing.T) {
		t.Run("it should read unknown", func(t *testing.T) {
			assert.Equal(t, "unknown", ReadCI(t.TempDir()))
		})
	})
}
