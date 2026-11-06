package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 347: the lens panel's switch flips the tower lines through the
// same row and the same save as the settings panel, so --set, the row
// and the switch cannot disagree.
func TestLensPanelSwitch(t *testing.T) {
	th := ui.NewTheme(1)
	game := func(t *testing.T) (*Game, *lensPanel, *[]string) {
		t.Helper()
		gc := newGroundCity(t)
		gc.scene.SetView(city.ViewServers)
		var saved []string
		g := &Game{scene: gc.scene, scripted: -1, clicked: -1}
		g.settings = ui.NewSettings([]ui.Setting{{Key: config.KeyTowerLines, Label: "tower lines", Options: []string{config.TowerLinesBuildings, config.TowerLinesAll}, Value: config.TowerLinesBuildings}})
		g.apply = func(s ui.Setting) error {
			saved = append(saved, s.Key+"="+s.Value)
			return nil
		}
		p := &lensPanel{laid: ui.LayoutLensPanel(th, g.scene.LensPanel(), city.RectAt(0, 32, 1200, 760), measure7)}
		require.Greater(t, p.laid.SwitchRect.Area(), 0.0, "precondition: the panel lays a switch")
		return g, p, &saved
	}

	t.Run("when the lens panel's switch is clicked", func(t *testing.T) {
		g, p, saved := game(t)
		p.click(g, p.laid.SwitchRect.Center())

		t.Run("it should flip the scene's tower lines", func(t *testing.T) {
			assert.True(t, g.scene.DepotBeams())
		})

		t.Run("it should hand the setting to apply for saving", func(t *testing.T) {
			assert.Equal(t, []string{"tower_lines=all"}, *saved)
		})

		t.Run("it should update the settings panel's row", func(t *testing.T) {
			assert.Equal(t, config.TowerLinesAll, g.settings.Items[0].Value)
		})

		t.Run("and it is clicked again", func(t *testing.T) {
			p.laid = ui.LayoutLensPanel(th, g.scene.LensPanel(), city.RectAt(0, 32, 1200, 760), measure7)
			p.click(g, p.laid.SwitchRect.Center())

			t.Run("it should flip them back", func(t *testing.T) {
				assert.False(t, g.scene.DepotBeams())
			})
		})
	})

	t.Run("when the panel is clicked beside the switch", func(t *testing.T) {
		g, p, saved := game(t)
		p.click(g, p.laid.Name.At)

		t.Run("it should leave the tower lines alone", func(t *testing.T) {
			assert.False(t, g.scene.DepotBeams())
			assert.Empty(t, *saved)
		})
	})
}
