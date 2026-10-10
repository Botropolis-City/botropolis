package render

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// The boats used to be read once, at startup, so a cache Claude Code
// refreshed later -- or a demo's usage climbing partway through a clip --
// never reached the river. A snapshot now re-reads the cache when the
// file has changed: a stat, and a read only when there is news.

func TestFollowUsage(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude.json")
	cache := func(t *testing.T, percent string, mod time.Time) {
		t.Helper()
		require.NoError(t, os.WriteFile(path, []byte(`{"cachedUsageUtilization":{"fetchedAtMs":1,"utilization":{"limits":[`+
			`{"kind":"session","group":"session","percent":`+percent+`,"severity":"normal"}]}}}`), 0o600))
		require.NoError(t, os.Chtimes(path, mod, mod))
	}
	g := &Game{scene: city.NewScene(city.NewLayout()), home: home}
	at := time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC)

	t.Run("when the cache changes after the city started", func(t *testing.T) {
		cache(t, "40", at)
		g.followUsage()
		cache(t, "85", at.Add(time.Minute))
		g.followUsage()

		t.Run("it should float the boat at the new reading", func(t *testing.T) {
			assert.Equal(t, 85.0, g.usage.Limits[0].Percent)
		})
	})
}
