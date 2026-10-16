package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 86: the ladder's thresholds change the map as soon as the panel
// changes them, and never into an order where overgrowth comes first.
func TestLadderSetting(t *testing.T) {
	day := 24 * time.Hour
	game := func() (*Game, *[]ui.Setting) {
		var applied []ui.Setting
		g := &Game{scene: city.NewScene(city.NewLayout())}
		g.apply = func(s ui.Setting) error {
			applied = append(applied, s)
			return nil
		}
		return g, &applied
	}

	t.Run("when the panel changes the shuttered days", func(t *testing.T) {
		g, applied := game()
		g.applySetting(ui.Setting{Key: config.KeyShutteredDays, Label: "shuttered days", Value: "3"})

		t.Run("it should change the scene's ladder", func(t *testing.T) {
			assert.Equal(t, 3*day, g.scene.Ladder().Shuttered)
		})

		t.Run("it should hand the row on to be saved", func(t *testing.T) {
			assert.Len(t, *applied, 1)
		})
	})

	t.Run("when the panel changes the overgrown days", func(t *testing.T) {
		g, _ := game()
		g.applySetting(ui.Setting{Key: config.KeyOvergrownDays, Label: "overgrown days", Value: "60"})

		t.Run("it should change the scene's ladder", func(t *testing.T) {
			assert.Equal(t, 60*day, g.scene.Ladder().Overgrown)
		})
	})

	t.Run("when the change would overgrow before shuttering", func(t *testing.T) {
		g, applied := game()
		g.applySetting(ui.Setting{Key: config.KeyShutteredDays, Label: "shuttered days", Value: "30"})

		t.Run("it should keep the ladder as it was", func(t *testing.T) {
			assert.Equal(t, state.DefaultThresholds, g.scene.Ladder())
		})

		t.Run("it should not save it", func(t *testing.T) {
			assert.Empty(t, *applied)
		})

		t.Run("it should say why", func(t *testing.T) {
			assert.Contains(t, g.notice.Text(timeNow()), "overgrown days must be more than shuttered days")
		})
	})
}
