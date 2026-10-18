package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 102: z turns every label off and on, through the same row and
// the same save as the settings panel, so the two cannot disagree.
func TestLabelsKey(t *testing.T) {
	game := func() (*Game, *[]string) {
		var saved []string
		g := &Game{scene: city.NewScene(city.NewLayout()), scripted: -1, clicked: -1}
		g.settings = ui.NewSettings([]ui.Setting{{Key: config.KeyLabels, Label: "labels", Options: []string{"shown", "hidden"}, Value: "shown"}})
		g.apply = func(s ui.Setting) error {
			saved = append(saved, s.Value)
			return nil
		}
		return g, &saved
	}
	press := func(t *testing.T, g *Game) {
		t.Helper()
		g.scripted = ebiten.KeyZ
		require.NoError(t, g.handleKeys())
		g.scripted = -1
	}

	t.Run("when the labels row reads shown, and z is pressed", func(t *testing.T) {
		g, saved := game()
		press(t, g)

		t.Run("it should hide the labels on the map", func(t *testing.T) {
			assert.False(t, g.scene.Labels())
		})

		t.Run("it should save the row as hidden", func(t *testing.T) {
			assert.Equal(t, []string{"hidden"}, *saved)
		})

		t.Run("and z is pressed again", func(t *testing.T) {
			press(t, g)

			t.Run("it should show them again", func(t *testing.T) {
				assert.True(t, g.scene.Labels())
			})
		})
	})

	for _, c := range []struct {
		name   string
		chords *config.Chords
		key    string
	}{
		{"nothing is set", nil, "z"},
		{"labels_key is shift+l", &config.Chords{Labels: config.Chord{Shift: true, Key: "l"}}, "shift+l"},
	} {
		t.Run("when "+c.name+", and the help lists the keys", func(t *testing.T) {
			g, _ := game()
			g.chords = c.chords

			t.Run("it should name "+c.key+" as labels off and on", func(t *testing.T) {
				assert.Contains(t, g.helpKeys(), ui.Key{Key: c.key, Action: "labels off and on"})
			})
		})
	}
}
