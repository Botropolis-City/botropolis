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

// The city is drawn from --home, so what the map shows about a session
// has to come from --home too. A home pointed at demo data must not
// show the real account's limits, and a demo session's Generate must
// not summarise a real transcript that happens to share its id.

func TestReadsFollowTheConfiguredHome(t *testing.T) {
	write := func(t *testing.T, path, content string) {
		t.Helper()
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	t.Run("when --home differs from the real home", func(t *testing.T) {
		configured, real := t.TempDir(), t.TempDir()
		t.Setenv("HOME", real)

		t.Run("and both hold a usage cache", func(t *testing.T) {
			write(t, filepath.Join(configured, ".claude.json"), `{"cachedUsageUtilization":{"fetchedAtMs":1000}}`)
			write(t, filepath.Join(real, ".claude.json"), `{"cachedUsageUtilization":{"fetchedAtMs":2000}}`)
			g := &Game{scene: city.NewScene(city.NewLayout()), home: configured}
			g.readUsage()

			t.Run("it should read the configured home's", func(t *testing.T) {
				assert.Equal(t, time.UnixMilli(1000), g.usage.FetchedAt)
			})
		})

		t.Run("and both hold a transcript for the same session", func(t *testing.T) {
			write(t, filepath.Join(configured, ".claude", "projects", "-demo", "s1.jsonl"), "{}\n")
			write(t, filepath.Join(real, ".claude", "projects", "-real", "s1.jsonl"), "{}\n")
			path, _ := transcriptFor(configured, "s1")

			t.Run("it should find the configured home's", func(t *testing.T) {
				assert.Equal(t, filepath.Join(configured, ".claude", "projects", "-demo", "s1.jsonl"), path)
			})
		})
	})
}
