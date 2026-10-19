package codex_test

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/harness/codex"
	"botropolis.city/botropolis/pkg/measure"
)

// Item 93, slice 2: rollouts are decoded in parallel, and the snapshot is
// the one a single reader would build.
func TestLoadingManyRollouts(t *testing.T) {
	t.Run("when a history holds more rollouts than there are workers", func(t *testing.T) {
		home := t.TempDir()
		count := 4*runtime.GOMAXPROCS(0) + 3
		var ids, paths []string
		for n := 0; n < count; n++ {
			id := fmt.Sprintf("019a3c2e-5a1b-7c3d-9e4f-%012d", n)
			ids = append(ids, id)
			paths = append(paths, writeRollout(t, home, fmt.Sprintf("rollout-%03d.jsonl", n),
				strings.Replace(rollout, "019a3c2e-5a1b-7c3d-9e4f-0a1b2c3d4e5f", id, 1), now.Add(-time.Hour)))
		}
		sort.Strings(ids)
		loader := codex.NewLoader(home)
		first, err := loader.Load(now)
		require.NoError(t, err)

		t.Run("it should list the sessions in the order of their paths", func(t *testing.T) {
			var got []string
			for _, s := range first.Sessions {
				got = append(got, s.ID)
			}
			assert.Equal(t, ids, got)
		})

		t.Run("it should count every rollout's usage once", func(t *testing.T) {
			var output int64
			for _, s := range first.Sessions {
				output += s.Usage.Output
			}
			assert.Equal(t, int64(800*count), output)
		})

		t.Run("and it is loaded again with nothing changed", func(t *testing.T) {
			before := measure.Read(measure.RolloutLinesDecoded)
			_, err := loader.Load(now)
			require.NoError(t, err)

			t.Run("it should decode nothing", func(t *testing.T) {
				assert.Equal(t, before, measure.Read(measure.RolloutLinesDecoded))
			})
		})

		t.Run("and one of them cannot be read", func(t *testing.T) {
			require.NoError(t, os.Chmod(paths[1], 0))
			t.Cleanup(func() { _ = os.Chmod(paths[1], 0o600) })
			require.NoError(t, os.Chtimes(paths[1], now, now))
			snapshot, err := codex.NewLoader(home).Load(now)
			require.NoError(t, err)

			t.Run("it should report it skipped", func(t *testing.T) {
				require.Len(t, snapshot.Skipped, 1)
				assert.Equal(t, paths[1], snapshot.Skipped[0].Path)
			})

			t.Run("it should load the rest", func(t *testing.T) {
				assert.Len(t, snapshot.Sessions, count-1)
			})
		})
	})
}
