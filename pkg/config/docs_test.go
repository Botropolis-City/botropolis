package config_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/config"
)

func TestDocs(t *testing.T) {
	docs := map[string]config.KeyDoc{}
	for _, d := range config.Docs() {
		docs[d.Key] = d
	}

	for _, key := range config.Keys(config.NewViper()) {
		t.Run("when "+key+" is documented", func(t *testing.T) {
			doc := docs[key]

			t.Run("it should say what it does", func(t *testing.T) {
				assert.NotEmpty(t, doc.Help)
			})

			t.Run("it should name its environment variable", func(t *testing.T) {
				assert.Equal(t, "BOTROPOLIS_"+strings.ToUpper(key), doc.Env)
			})
		})
	}

	t.Run("when a key has a default", func(t *testing.T) {
		t.Run("it should give it", func(t *testing.T) {
			assert.Equal(t, "steady", docs[config.KeyLiveliness].Default)
		})
	})

	t.Run("when a key's default is empty", func(t *testing.T) {
		t.Run("it should give none", func(t *testing.T) {
			assert.Empty(t, docs[config.KeyTerminal].Default)
		})
	})

	t.Run("when the machine building the pages sets keys in its environment", func(t *testing.T) {
		t.Setenv("BOTROPOLIS_TERMINAL", "kitty")
		t.Setenv("BOTROPOLIS_LIVELINESS", "lively")
		built := map[string]config.KeyDoc{}
		for _, d := range config.Docs() {
			built[d.Key] = d
		}

		t.Run("it should not take an unflagged key's default from it", func(t *testing.T) {
			assert.Empty(t, built[config.KeyTerminal].Default)
		})

		t.Run("it should not take a flagged key's default from it", func(t *testing.T) {
			assert.Equal(t, "steady", built[config.KeyLiveliness].Default)
		})
	})

	t.Run("when the docs are listed", func(t *testing.T) {
		t.Run("it should list every key and nothing else", func(t *testing.T) {
			assert.Len(t, docs, len(config.Keys(config.NewViper())))
		})
	})
}
