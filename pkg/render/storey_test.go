package render

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func kitGame(t *testing.T) *Game {
	t.Helper()
	k, err := loadKits()
	require.NoError(t, err)
	return &Game{kits: k}
}

func kindBuilding(kind city.DistrictType, storeys int) *city.Building {
	return &city.Building{
		Type:    kind,
		Rect:    city.RectAt(0, 0, city.BuildingSize, city.BuildingSize),
		Session: state.Session{ID: "work", State: state.Working, Storeys: storeys},
	}
}

func poleHeights(lines []segment) []float64 {
	var out []float64
	for _, s := range lines {
		if s.pole {
			out = append(out, math.Round((s.a.Y-s.b.Y)*1e6)/1e6)
		}
	}
	return out
}

func inside(p city.Point, lot [4]city.Point, lift float64) bool {
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, c := range lot {
		minX, maxX = math.Min(minX, c.X), math.Max(maxX, c.X)
		minY, maxY = math.Min(minY, c.Y-lift), math.Max(maxY, c.Y-lift)
	}
	return p.X >= minX-1e-6 && p.X <= maxX+1e-6 && p.Y >= minY-1e-6 && p.Y <= maxY+1e-6
}

// Item 109, slice 6: mid-turn, the storey being built is scaffolded on
// the roof, one storey high and on the building's own footprint, and the
// hoist runs up to it.
func TestStoreyScaffold(t *testing.T) {
	g := kitGame(t)
	cam := city.NewCamera()

	t.Run("when a working session's building is drawn", func(t *testing.T) {
		frame, ok := g.workFrame(cam, kindBuilding(city.ResidentialDistrict, 2))
		require.True(t, ok)
		lines := roofScaffold(frame.lot, frame.wallTop, frame.storey, 1)

		t.Run("it should stand the scaffold one storey high", func(t *testing.T) {
			for _, h := range poleHeights(lines) {
				assert.InDelta(t, storeyHeight(cam.Zoom), h, 1e-6)
			}
		})

		t.Run("it should keep the scaffold inside the building's footprint", func(t *testing.T) {
			var out int
			for _, s := range lines {
				if s.pole && !inside(s.a, frame.lot, frame.wallTop) {
					out++
				}
			}
			assert.Zero(t, out)
		})

		t.Run("it should be the building's footprint, not the lot", func(t *testing.T) {
			lot := lotCorners(cam, kindBuilding(city.ResidentialDistrict, 2).Rect)
			assert.Less(t, frame.lot[1].X-frame.lot[3].X, lot[1].X-lot[3].X)
		})

		t.Run("and the building is taller", func(t *testing.T) {
			tall, ok := g.workFrame(cam, kindBuilding(city.ResidentialDistrict, 5))
			require.True(t, ok)

			t.Run("it should keep the scaffold one storey high", func(t *testing.T) {
				for _, h := range poleHeights(roofScaffold(tall.lot, tall.wallTop, tall.storey, 1)) {
					assert.InDelta(t, storeyHeight(cam.Zoom), h, 1e-6)
				}
			})

			t.Run("it should stand three storeys higher", func(t *testing.T) {
				assert.InDelta(t, 3*storeyHeight(cam.Zoom), tall.wallTop-frame.wallTop, 1e-6)
			})
		})
	})

	t.Run("when a load goes up the hoist", func(t *testing.T) {
		frame, _ := g.workFrame(cam, kindBuilding(city.OfficeDistrict, 3))
		front, _ := frontOf(frame.lot)

		t.Run("it should end at the storey being built", func(t *testing.T) {
			assert.InDelta(t, front.Y-frame.wallTop, hoistAt(frame.lot, frame.wallTop, 1).Y, 1e-6)
		})
	})
}

// The footprints the scaffold stands on come from the pipeline, which
// writes each kind's body into the atlas; the kinds with nothing beside
// them are held to the width they were cut at.
func TestKindFootprints(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	atlas := atlases[len(atlases)-1]

	for _, kind := range city.DistrictTypes {
		for n := 1; n <= state.MaxStoreys; n++ {
			name := fmt.Sprintf("botropolis/%s-%d", kind, n)
			sprite, ok := atlas.Sprite(name, 0)
			require.True(t, ok)

			t.Run("when "+name+" is cut", func(t *testing.T) {
				t.Run("it should carry its body", func(t *testing.T) {
					assert.Positive(t, sprite.Body.W*sprite.Body.D)
				})
			})
		}
	}

	for _, name := range []string{"botropolis/office-1", "botropolis/office-3", "botropolis/office-5", "botropolis/residential-4"} {
		sprite, _ := atlas.Sprite(name, 0)
		t.Run("when "+name+" stands alone on its lot", func(t *testing.T) {
			plinth := 0.03
			want := (sprite.Body.W + sprite.Body.D + 2*plinth) * city.IsoTileWidth / 2 * atlas.Zoom

			// Within the antialiased edge, a pixel or two each side; a body
			// off by a roof's overhang is ten.
			t.Run("it should match the width the atlas cut it at", func(t *testing.T) {
				assert.InDelta(t, want, float64(sprite.Rect.Dx()), 4)
			})
		})
	}
}
