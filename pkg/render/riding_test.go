package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 348: a building moved by hand rides from lot to lot, drawn as
// itself beside its truck, so sixteen moving at once stay tellable
// apart; a packed crate rides on the truck as before.
func TestHaulPieces(t *testing.T) {
	at := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	b := &city.Building{Session: state.Session{ID: "s", Storeys: 2}, Type: city.OfficeDistrict}
	from, to := city.Point{X: 0, Y: 0}, city.Point{X: 400, Y: 0}
	haul := func(kind city.HaulKind) city.Haul {
		return city.Haul{SessionID: "s", Kind: kind, From: from, To: to, Since: at, For: 4 * time.Second, Hue: 1}
	}
	named := func(pieces []haulPiece, name string) (haulPiece, bool) {
		for _, p := range pieces {
			if p.name == name {
				return p, true
			}
		}
		return haulPiece{}, false
	}

	// Aria, 2026-11-06: "Use a crate, as packing does."
	t.Run("when a building is on the move", func(t *testing.T) {
		h := haul(city.HaulMove)

		t.Run("and it has just left", func(t *testing.T) {
			pieces := haulPieces(h, b, at)
			building, found := named(pieces, buildingPiece(b))
			require.True(t, found, "the building should still stand at its old lot")

			t.Run("it should draw the building at its old lot", func(t *testing.T) {
				assert.Equal(t, from, building.at)
			})

			t.Run("it should draw it whole", func(t *testing.T) {
				assert.Equal(t, 1.0, building.alpha)
			})

			t.Run("it should draw the truck and its crate after it", func(t *testing.T) {
				var names []string
				for _, p := range pieces {
					names = append(names, p.name)
				}
				assert.Equal(t, []string{buildingPiece(b), kitTruck, crates[1]}, names)
			})
		})

		t.Run("and it is half way", func(t *testing.T) {
			pieces := haulPieces(h, b, at.Add(2*time.Second))

			t.Run("it should draw the truck and its crate, and no building", func(t *testing.T) {
				var names []string
				for _, p := range pieces {
					names = append(names, p.name)
				}
				assert.Equal(t, []string{kitTruck, crates[1]}, names)
			})
		})
	})

	t.Run("when a crate is packed up", func(t *testing.T) {
		pieces := haulPieces(haul(city.HaulPack), b, at.Add(2*time.Second))

		t.Run("it should draw the truck and the crate, and no building", func(t *testing.T) {
			var names []string
			for _, p := range pieces {
				names = append(names, p.name)
			}
			assert.Equal(t, []string{kitTruck, crates[1]}, names)
		})
	})
}
