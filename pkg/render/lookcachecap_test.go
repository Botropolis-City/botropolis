//go:build !ebitenginedebug && !devtools

package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 212: the performance tests run the look cache at half its size
// from the same tree, by BOTROPOLIS_LOOK_CACHE_CAP in the debug build.
func TestTheLookCachesCap(t *testing.T) {
	for _, c := range []struct {
		name, env string
		want      int
	}{
		{"is not given", "", lookCacheCap},
		{"is a number of MiB", "16", 16 << 20},
		{"is not a number", "half", lookCacheCap},
		{"is no room at all", "0", lookCacheCap},
	} {
		t.Run("when the cap the debug build is given "+c.name, func(t *testing.T) {
			t.Run("it should be that many bytes, or else the cap", func(t *testing.T) {
				assert.Equal(t, c.want, lookCacheCapFrom(c.env))
			})
		})
	}

	t.Run("when a release build is given one", func(t *testing.T) {
		t.Setenv("BOTROPOLIS_LOOK_CACHE_CAP", "16")

		t.Run("it should keep the cap", func(t *testing.T) {
			assert.Equal(t, lookCacheCap, lookCacheBytes())
		})
	})
}
