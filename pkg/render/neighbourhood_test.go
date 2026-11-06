package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 348, slice 2: the ground layer is composed again when the look
// changes, since neighbourhoods are painted into it.
func TestNeighbourhoodLookKey(t *testing.T) {
	t.Run("when the neighbourhoods' look changes", func(t *testing.T) {
		rc := newRecordCity(t)
		cam, hover := rc.scene.Camera(), rc.scene.Hover()
		before := rc.g.keyFor(cam, hover, recordWidth, recordHeight)
		rc.scene.SetNeighbourhoodLook(city.NeighbourhoodLook{Ground: true})
		after := rc.g.keyFor(cam, hover, recordWidth, recordHeight)

		t.Run("it should change the ground layer's key", func(t *testing.T) {
			assert.NotEqual(t, before, after)
		})
	})
}

// Item 348, slice 2: what a neighbourhood paints, for each treatment
// tried on frames.
func TestNeighbourhoodMarks(t *testing.T) {
	ground := []city.Rect{city.RectAt(0, 0, 10, 10), {Min: city.Point{X: 10, Y: 0}, Max: city.Point{X: 14, Y: 10}}, city.RectAt(14, 0, 10, 10)}
	n := city.Neighbourhood{Group: &state.Group{ID: "g1", Short: "API"}, Ground: ground, Gate: city.Point{X: 24, Y: 10}}
	looks := []struct {
		name                      string
		look                      city.NeighbourhoodLook
		paves, kerbs, gate, named bool
	}{
		{"nothing", city.NeighbourhoodLook{}, false, false, false, false},
		{"the shared ground", city.NeighbourhoodLook{Ground: true}, true, false, false, false},
		{"the edge", city.NeighbourhoodLook{Kerb: true}, false, true, false, false},
		{"the gate", city.NeighbourhoodLook{Gate: true}, false, false, true, false},
		{"the gate with its name", city.NeighbourhoodLook{Gate: true, Name: true}, false, false, true, true},
		{"the name without a gate", city.NeighbourhoodLook{Name: true}, false, false, false, false},
	}
	for _, l := range looks {
		t.Run("when the look is "+l.name, func(t *testing.T) {
			m := neighbourhoodMarks(n, l.look)

			t.Run("it should pave the block's ground, or none", func(t *testing.T) {
				assert.Equal(t, l.paves, len(m.pave) == len(ground))
			})

			t.Run("it should run a kerb round the outline, or none", func(t *testing.T) {
				assert.Equal(t, l.kerbs, len(m.kerb) == len(n.Edges()) && len(m.kerb) > 0)
			})

			t.Run("it should stand a gate at the block's gate, or none", func(t *testing.T) {
				assert.Equal(t, l.gate, m.gate && m.at == n.Gate)
			})

			t.Run("it should letter the gate with the group's short name, or nothing", func(t *testing.T) {
				want := ""
				if l.named {
					want = "API"
				}
				assert.Equal(t, want, m.label)
			})
		})
	}
}
