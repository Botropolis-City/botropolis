package acceptance_test

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/demo"
)

// Item 232: the README and the demo captions, which the website quotes
// verbatim, described the city before 0.2.0 for two releases after it.
func TestCopyUsesTheCitysOwnWords(t *testing.T) {
	texts := map[string]string{
		"README.md":      readRepoFile(t, "README.md"),
		"demo/README.md": readRepoFile(t, "demo/README.md"),
	}
	paths, err := filepath.Glob(filepath.Join("..", "..", "demo", "scenarios", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		s, err := demo.LoadScenario(path)
		require.NoError(t, err)
		texts[s.Name+"'s description"] = s.Description
		for _, shot := range s.Shots {
			texts[s.Name+"'s shot "+shot.Name] = shot.Description
		}
	}

	stale := map[string]*regexp.Regexp{
		"storage yard":        regexp.MustCompile(`(?i)storage yard`),
		"parked":              regexp.MustCompile(`(?i)\bparked\b`),
		"context window used": regexp.MustCompile(`(?i)context window used`),
		"install-hooks":       regexp.MustCompile(`install-hooks`),
	}

	for name, text := range texts {
		t.Run("when "+name+" is read", func(t *testing.T) {
			for phrase, re := range stale {
				t.Run("it should not say "+phrase, func(t *testing.T) {
					assert.NotRegexp(t, re, text)
				})
			}
		})
	}
}
