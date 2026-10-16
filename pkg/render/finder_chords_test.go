package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

func TestFinderChords(t *testing.T) {
	stopped := find.Hit{SessionID: "stopped", Dir: "/src/kiln", State: state.Parked}
	altS := config.Chords{
		Open: config.Chord{Ctrl: true, Key: "k"},
		Copy: config.Chord{Ctrl: true, Key: "y"},
		Show: config.Chord{Alt: true, Key: "s"},
	}

	t.Run("when show is set to another chord", func(t *testing.T) {
		t.Run("and that chord is pressed", func(t *testing.T) {
			g, _ := finderOver(t, 0, stopped)
			g.chords = &altS
			g.scripted, g.scriptedAlt = ebiten.KeyS, true
			require.NoError(t, g.handleKeys())

			t.Run("it should show the session on the map", func(t *testing.T) {
				require.NotNil(t, g.scene.Selected())
				assert.Equal(t, [2]any{false, "stopped"}, [2]any{g.finding(), g.scene.Selected().Session.ID})
			})
		})

		t.Run("and Ctrl+M is pressed", func(t *testing.T) {
			g, _ := finderOver(t, 0, stopped)
			g.chords = &altS
			pressKey(t, g, ebiten.KeyM, true)

			t.Run("it should not", func(t *testing.T) {
				assert.Equal(t, [2]any{true, true}, [2]any{g.finding(), g.scene.Selected() == nil})
			})
		})
	})

	t.Run("when the help is drawn", func(t *testing.T) {
		g := &Game{chords: &altS}

		t.Run("it should list the chords as configured", func(t *testing.T) {
			assert.Contains(t, g.helpKeys(), ui.Key{Key: "alt+s", Action: "in the finder: show the session on the map"})
		})
	})
}
