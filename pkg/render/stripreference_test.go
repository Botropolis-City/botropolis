package render

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
)

// Item 353: with the footer gone, a newcomer finds the keys through the
// reference, and the strip's hover card names its key as it is bound.
func TestStripCardNamesTheReference(t *testing.T) {
	at := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	hovered := func(t *testing.T, keys map[string]string) city.Card {
		t.Helper()
		scene := city.NewScene(city.NewLayout())
		scene.SetClock(func() time.Time { return at }, time.UTC)
		scene.Resize(1280, 800)
		scene.SetSnapshot(state.Snapshot{At: at, Sessions: []state.Session{
			{ID: "busy", Title: "Busy", CWD: "/src/c", State: state.Working, LastActivity: at},
		}})
		chords, err := config.FindChords(config.Config{Keys: keys}, runtime.GOOS, KnownKey)
		require.NoError(t, err)
		g := &Game{scene: scene, chords: &chords, scripted: -1, clicked: -1}
		return g.stripHoverCard(state.Working)
	}

	t.Run("when the strip is hovered", func(t *testing.T) {
		card := hovered(t, nil)

		t.Run("it should end the card with the reference's key", func(t *testing.T) {
			assert.Equal(t, []string{"?"}, card.HintKey)
		})

		t.Run("it should say what the key opens", func(t *testing.T) {
			assert.Equal(t, referenceHint, card.Hint)
		})
	})

	t.Run("when the strip is hovered, and the reference is rebound", func(t *testing.T) {
		card := hovered(t, map[string]string{"reference": "f2"})

		t.Run("it should name the key it is bound to", func(t *testing.T) {
			assert.Equal(t, []string{"f2"}, card.HintKey)
		})
	})
}
