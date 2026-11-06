package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 348, slice 3: a line in one of the three forms draws its shape
// and no stations; what stations carry is slice 4's question.
func TestLineFormStations(t *testing.T) {
	stations := func(rc *recordCity) int {
		rc.g.frameHits = rc.g.frameHits[:0]
		rc.frame()
		n := 0
		for _, h := range rc.g.frameHits {
			if h.hit.Station != nil {
				n++
			}
		}
		return n
	}

	t.Run("when the form is stations", func(t *testing.T) {
		rc := newRecordCity(t)

		t.Run("it should draw the stations", func(t *testing.T) {
			assert.Positive(t, stations(rc))
		})
	})

	for _, form := range []city.LineForm{city.FormSessions, city.FormDistricts, city.FormNeighbourhoods} {
		t.Run("when the form is "+form.String(), func(t *testing.T) {
			rc := newRecordCity(t)
			rc.scene.SetLineForm(form)
			require.NotEmpty(t, rc.scene.Shapes(), "precondition: the group has a shape")

			t.Run("it should draw no station", func(t *testing.T) {
				assert.Zero(t, stations(rc))
			})
		})
	}
}
