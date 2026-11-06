package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
)

// Item 350, slice 5b: a key recorded in the settings is checked against
// the table as a config file would be, and takes effect at once.
func TestRebind(t *testing.T) {
	defaults := func(t *testing.T) config.Chords {
		t.Helper()
		chords, err := config.FindChords(config.Config{}, "linux", anyKey)
		require.NoError(t, err)
		return chords
	}

	t.Run("when a free chord is offered for a binding", func(t *testing.T) {
		err := config.Rebind(defaults(t), "timeline", config.Chord{Key: "y"}, "linux")

		t.Run("it should be taken", func(t *testing.T) {
			assert.NoError(t, err)
		})
	})

	t.Run("when the chord is held by another binding where both answer", func(t *testing.T) {
		err := config.Rebind(defaults(t), "timeline", config.Chord{Key: "x"}, "linux")

		t.Run("it should refuse, naming that binding", func(t *testing.T) {
			require.Error(t, err)
			assert.Contains(t, err.Error(), "breakdown")
		})
	})

	t.Run("when the chord is the binding's own", func(t *testing.T) {
		err := config.Rebind(defaults(t), "timeline", config.Chord{Key: "t"}, "linux")

		t.Run("it should be taken", func(t *testing.T) {
			assert.NoError(t, err)
		})
	})

	t.Run("when the chord is held only in another scope", func(t *testing.T) {
		err := config.Rebind(defaults(t), "menu_stop", config.Chord{Key: "x"}, "linux")

		t.Run("it should be taken", func(t *testing.T) {
			assert.NoError(t, err)
		})
	})

	t.Run("when the chord is fixed where the binding answers", func(t *testing.T) {
		err := config.Rebind(defaults(t), "timeline", config.Chord{Key: "tab"}, "linux")

		t.Run("it should refuse, naming what the key does", func(t *testing.T) {
			require.Error(t, err)
			assert.Contains(t, err.Error(), "next needs-you")
		})
	})

	t.Run("when the chord is one macOS keeps, on darwin", func(t *testing.T) {
		err := config.Rebind(defaults(t), "timeline", config.Chord{Cmd: true, Key: "q"}, "darwin")

		t.Run("it should refuse", func(t *testing.T) {
			assert.Error(t, err)
		})
	})

	t.Run("when the binding is no binding", func(t *testing.T) {
		err := config.Rebind(defaults(t), "teleport", config.Chord{Key: "y"}, "linux")

		t.Run("it should refuse", func(t *testing.T) {
			assert.Error(t, err)
		})
	})

	t.Run("when the journal is rebound through With", func(t *testing.T) {
		chords := defaults(t).With("journal", config.Chord{Key: "y"})

		t.Run("it should rebind it in the table", func(t *testing.T) {
			assert.Equal(t, config.Chord{Key: "y"}, chords.Bound["journal"])
		})

		t.Run("it should rebind the journal's own chord with it", func(t *testing.T) {
			assert.Equal(t, config.Chord{Key: "y"}, chords.Journal)
		})

		t.Run("it should leave the chords it was made from as they were", func(t *testing.T) {
			assert.Equal(t, config.Chord{Key: "j"}, defaults(t).Bound["journal"])
		})
	})
}
