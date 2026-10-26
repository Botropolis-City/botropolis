package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 274: the deleted transcripts row changes the map as soon as the
// panel changes it.
func TestGoneSessionsRow(t *testing.T) {
	for _, c := range []struct {
		from  city.Shown
		value string
		want  bool
	}{
		{city.Shown{}, config.LabelsShown, true},
		{city.Shown{Gone: true}, config.LabelsHidden, false},
	} {
		t.Run("when the panel sets deleted transcripts to "+c.value, func(t *testing.T) {
			g := &Game{scene: city.NewScene(city.NewLayout())}
			g.scene.SetShown(c.from)
			g.applySetting(ui.Setting{Key: config.KeyGoneSessions, Label: "deleted transcripts", Value: c.value})

			t.Run("it should change what the scene draws", func(t *testing.T) {
				assert.Equal(t, c.want, g.scene.Shown().Gone)
			})
		})
	}
}
