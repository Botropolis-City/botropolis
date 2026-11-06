package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 348, slice 4: group_line_form changed in the settings panel
// re-shapes the groups' lines at once, from the city already built.
func TestLineFormSetting(t *testing.T) {
	forms := []struct {
		value string
		form  city.LineForm
	}{
		{config.GroupLineDistricts, city.FormDistricts},
		{config.GroupLineNeighbourhoods, city.FormNeighbourhoods},
		{config.GroupLineSessions, city.FormSessions},
	}
	for _, f := range forms {
		t.Run("when group line form is set to "+f.value, func(t *testing.T) {
			rc := newRecordCity(t)
			if f.form == city.FormDistricts {
				rc.scene.SetLineForm(city.FormSessions)
			}
			rc.g.applySetting(ui.Setting{Key: config.KeyGroupLineForm, Value: f.value})

			t.Run("it should draw the lines in that form", func(t *testing.T) {
				assert.Equal(t, f.form, rc.scene.LineForm())
			})
		})
	}

	t.Run("when a form's name and the setting's value are compared", func(t *testing.T) {
		for _, f := range forms {
			t.Run("it should name "+f.value+" as the setting does", func(t *testing.T) {
				assert.Equal(t, f.value, f.form.String())
			})
		}
	})
}
