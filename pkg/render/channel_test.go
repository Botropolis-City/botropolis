package render

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 252: a piece's drawn top is the least height its highest pixel
// can stand at, column by column over the far edge of its base diamond.
func TestDrawnTop(t *testing.T) {
	const w, h = 80, 30
	// box draws a w-wide base diamond raised h pixels as a solid prism,
	// with a post standing post pixels over the roof's left corner, and
	// crops the sprite to what it drew, as the atlas does, at an offset in
	// the page so the measure has to use the rect.
	box := func(post int) (*image.NRGBA, assets.KitSprite) {
		page := image.NewNRGBA(image.Rect(0, 0, 160, 160))
		foot := image.Pt(20+w/2, 140)
		opaque := image.Rectangle{}
		paint := func(x, y int) {
			page.SetNRGBA(x, y, color.NRGBA{A: 0xff})
			opaque = opaque.Union(image.Rect(x, y, x+1, y+1))
		}
		for y := 0; y < foot.Y+w/4; y++ {
			for x := foot.X - w/2; x < foot.X+w/2; x++ {
				fx, fy := float64(x-foot.X)+0.5, float64(y-foot.Y)+0.5
				for lift := 0.0; lift <= h; lift++ {
					if math.Abs(fx)/2+math.Abs(fy+lift) <= w/4 {
						paint(x, y)
						break
					}
				}
			}
		}
		for y := foot.Y - h - post; y < foot.Y-h; y++ {
			paint(foot.X-w/2, y)
		}
		return page, assets.KitSprite{Rect: opaque, Anchor: foot.Sub(opaque.Min)}
	}

	t.Run("when measured on a box", func(t *testing.T) {
		page, s := box(0)
		t.Run("it should find the box's height, as roofLift does", func(t *testing.T) {
			assert.InDelta(t, roofLift(float64(s.Rect.Dx()), float64(s.Anchor.Y)), drawnTop(page, s), 1)
		})
	})
	t.Run("when measured on a box with a post standing over its roof's side corner, higher than the far corner", func(t *testing.T) {
		const post = 30
		page, s := box(post)
		require.Less(t, roofLift(float64(s.Rect.Dx()), float64(s.Anchor.Y)), float64(h+post-5), "precondition: roofLift misses the post")
		t.Run("it should find the post's top", func(t *testing.T) {
			assert.InDelta(t, float64(h+post), drawnTop(page, s), 1)
		})
	})
}

// riverBoat is a boat that sails under the second bridge, and how long
// it is drawn, or 0 for its natural size.
type riverBoat struct {
	piece  string
	extent float64
}

// The boats on the river, listed here rather than read from the
// renderer, so a boat dropped from its list is still held.
var riverBoats = []riverBoat{
	{kitGaugeLiner, gaugeHulls[city.GaugeBig].extent},
	{kitGaugeCargo, gaugeHulls[city.GaugeMedium].extent},
	{kitGaugeSail, gaugeHulls[city.GaugeSmall].extent},
	{kitCourier, courierExtent},
	{kitTugIn, 0},
	{kitTugOut, 0},
}

// preparedKits is every atlas with every page measured, as the window
// measures each page it puts up.
func preparedKits(t *testing.T) *kits {
	t.Helper()
	k := testKits(t)
	for i := range k.atlases {
		for p, page := range k.atlases[i].Pages {
			nrgba, ok := page.(*image.NRGBA)
			require.True(t, ok, "precondition: page %d decodes to NRGBA", p)
			k.atlases[i].prepareKinds(p, nrgba)
		}
	}
	return k
}

// boatTop is how high a boat's drawn top stands on screen at a camera,
// measured off its sprite at the size it is drawn.
func boatTop(t *testing.T, cam *city.Camera, k *kits, b riverBoat, turn int) float64 {
	t.Helper()
	atlas := k.pick(cam.Zoom)
	s, ok := atlas.Sprite(b.piece, cam.Heading-turn)
	require.True(t, ok, b.piece)
	page, ok := atlas.Pages[s.Page].(*image.NRGBA)
	require.True(t, ok)
	scale := cam.Zoom / atlas.Zoom
	if b.extent > 0 {
		scale *= kitShrink(b.extent, math.Max(float64(s.Rect.Dx()), float64(s.Rect.Dy())), atlas.Zoom)
	}
	return drawnTop(page, s) * scale
}

// largeCity is a city with two bridges, the second over the river.
func largeCity(t *testing.T) *city.City {
	t.Helper()
	start := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	var sessions []state.Session
	for i := 0; i <= city.SecondBridgeAt; i++ {
		sessions = append(sessions, state.Session{ID: fmt.Sprintf("s%02d", i), CWD: fmt.Sprintf("/p/d%02d", i), State: state.Working, Alive: true, StartedAt: start})
	}
	c := city.Build(state.Snapshot{At: start, Sessions: sessions}, city.NewLayout())
	require.Len(t, c.Bridges(), 2, "precondition: the city has two bridges")
	require.NotEqual(t, city.Span{}, c.Bridges()[1].Channel, "precondition: the second bridge crosses the river")
	return c
}

// Item 252, Aria 2026-10-25: "Raise the deck". Over the river the second
// bridge clears the tallest boat drawn, at every atlas and heading, and
// climbs to it no more steeply than the first bridge's ramp.
func TestSecondBridgeClearsTheBoats(t *testing.T) {
	k := preparedKits(t)
	g := &Game{kits: k}
	c := largeCity(t)
	first, b := c.Bridges()[0], c.Bridges()[1]
	rail := map[city.Cell]bool{}
	for _, p := range c.Rails {
		rail[city.CellOf(p)] = true
	}
	var over []city.Cell
	for _, cell := range b.Deck {
		if rail[cell] {
			over = append(over, cell)
		}
	}
	require.NotEmpty(t, over, "precondition: the second bridge crosses the rail")
	trains := append(append([]string{}, kitLocos...), kitWagons...)

	for i := range k.atlases {
		for _, heading := range []int{0, 90, 180, 270} {
			cam := city.NewCamera()
			cam.Projection, cam.Zoom, cam.Heading = city.Isometric, zoomPicking(t, k, i), heading
			height := g.heights(cam)
			thick := deckThickness * cam.Zoom

			t.Run(fmt.Sprintf("at zoom %g and heading %d", cam.Zoom, heading), func(t *testing.T) {
				tallest, by := 0.0, ""
				for _, boat := range riverBoats {
					for _, turn := range []int{0, 90, 180, 270} {
						if top := boatTop(t, cam, k, boat, turn); top > tallest {
							tallest, by = top, boat.piece
						}
					}
				}
				require.Equal(t, kitGaugeLiner, by, "precondition: the liner is the tallest boat")

				t.Run("it should leave the deck's underside over the channel above the tallest boat's drawn top", func(t *testing.T) {
					assert.Greater(t, height(city.AtChannel)-thick, tallest)
				})

				t.Run("it should leave the underside over every rail cell above every train piece's roof", func(t *testing.T) {
					roof := 0.0
					for _, piece := range trains {
						for _, turn := range []int{0, 90, 180, 270} {
							roof = max(roof, g.kitRoofLiftTurned(cam, piece, turn))
						}
					}
					require.Positive(t, roof, "precondition: the atlas has train pieces")
					low := math.Inf(1)
					for _, cell := range over {
						r := cell.Rect()
						for _, x := range []float64{r.Min.X, r.Center().X, r.Max.X} {
							low = min(low, b.Lift(city.Point{X: x, Y: b.Land.Y}, height))
						}
					}
					assert.Greater(t, low-thick, roof)
				})

				gentlest := liftToWorld(cam, height(city.AtDeck)) / first.RampRun()
				knots := b.Profile()
				for j := 1; j < len(knots); j++ {
					a, z := knots[j-1], knots[j]
					if a.Height == z.Height {
						continue
					}
					t.Run(fmt.Sprintf("it should go from %v to %v no more steeply than the first bridge's ramp", a.Height, z.Height), func(t *testing.T) {
						slope := liftToWorld(cam, math.Abs(height(z.Height)-height(a.Height))) / (z.Along - a.Along)
						assert.LessOrEqual(t, slope, gentlest+1e-9)
					})
				}
			})
		}
	}
}
