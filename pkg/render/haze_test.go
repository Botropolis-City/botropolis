package render

import (
	"fmt"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 219, Aria 2026-10-24: the bridge runs off into the haze. Past
// the haze's start the deck is drawn in pieces mixed toward the sea's
// own colour, so it melts into the water, and a truck on it fades with
// it.
func TestDeckIntoTheHaze(t *testing.T) {
	k := testKits(t)
	g := &Game{kits: k}
	start := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	snap := state.Snapshot{At: start, Sessions: []state.Session{{ID: "a", CWD: "/p/a", State: state.Working, StartedAt: start}}}
	c := city.Build(snap, city.NewLayout())
	b := c.Bridge()
	require.NotEmpty(t, b.Deck, "the bridge has no deck: nothing below can fail")
	out := math.Copysign(1, b.Far.X-b.Land.X)
	along := func(p city.Point) float64 { return (p.X - b.Land.X) * out }
	inner := func(r city.Rect) float64 { return min(along(r.Min), along(r.Max)) }
	outer := func(r city.Rect) float64 { return max(along(r.Min), along(r.Max)) }
	parts := bridgeParts(b)
	var hazy []bridgePart
	for _, p := range parts {
		if outer(p.Rect) > along(b.Haze)+1e-9 {
			hazy = append(hazy, p)
		}
	}
	require.NotEmpty(t, hazy, "no part of the deck reaches the haze: nothing below can fail")

	t.Run("when the deck runs into the haze", func(t *testing.T) {
		t.Run("it should draw every part before the haze whole", func(t *testing.T) {
			var mixed []bridgePart
			for _, p := range parts {
				if outer(p.Rect) <= along(b.Haze)+1e-9 && p.Mist != 0 {
					mixed = append(mixed, p)
				}
			}
			assert.Empty(t, mixed)
		})

		t.Run("it should cut the haze into pieces no longer than a quarter cell", func(t *testing.T) {
			var long []bridgePart
			for _, p := range hazy {
				if outer(p.Rect)-inner(p.Rect) > city.CellSize/4+1e-9 {
					long = append(long, p)
				}
			}
			assert.Empty(t, long)
		})

		t.Run("it should mix each piece toward the sea by how little of the deck shows at its outer end", func(t *testing.T) {
			var off []string
			for _, p := range hazy {
				end := city.Point{X: p.Rect.Max.X, Y: p.Rect.Center().Y}
				if out < 0 {
					end.X = p.Rect.Min.X
				}
				if want := 1 - b.Alpha(end); math.Abs(p.Mist-want) > 1e-9 {
					off = append(off, fmt.Sprintf("%v: %v, want %v", p.Rect, p.Mist, want))
				}
			}
			assert.Empty(t, off)
		})

		t.Run("it should mix the last piece wholly into the sea", func(t *testing.T) {
			assert.Equal(t, 1.0, hazy[len(hazy)-1].Mist)
		})

		for _, night := range []bool{false, true} {
			t.Run(fmt.Sprintf("and night is %v, it should paint the deck wholly mixed in the sea's own colour", night), func(t *testing.T) {
				var off []string
				for _, base := range []colorName{{"pier", colorPier}, {"girder", colorGirder}, {"railing", colorRailing}, {"road", colorRampRoad}, {"line", colorRampLine}} {
					if got, want := hazeColour(base.c, 1, night), shorePaletteFor(night).sea; got != want {
						off = append(off, fmt.Sprintf("%s: %v, want %v", base.name, got, want))
					}
				}
				assert.Empty(t, off)
			})
		}

		island := c.Island()
		span := hazy[0].Rect
		for _, p := range hazy {
			span = span.Union(p.Rect)
		}
		for i := range k.atlases {
			for _, heading := range []int{0, 90, 180, 270} {
				cam := city.NewCamera()
				cam.Projection, cam.Zoom, cam.Heading = city.Isometric, zoomPicking(t, k, i), heading
				lift := liftToWorld(cam, g.deckLift(cam)+railHeight*cam.Zoom)
				under := city.Rect{Min: city.Point{X: span.Min.X - lift, Y: span.Min.Y - lift}, Max: city.Point{X: span.Max.X + lift, Y: span.Max.Y + lift}}
				t.Run(fmt.Sprintf("at zoom %g and heading %d, it should draw nothing but flat sea under the haze span", cam.Zoom, heading), func(t *testing.T) {
					var land []string
					for x := under.Min.X; x <= under.Max.X; x += city.CellSize / 8 {
						for y := under.Min.Y; y <= under.Max.Y; y += city.CellSize / 8 {
							if ground, _ := island.At(city.Point{X: x, Y: y}); ground != city.GroundSea {
								land = append(land, fmt.Sprintf("%v at (%.0f, %.0f)", ground, x, y))
							}
						}
					}
					north, south := island.Mouths()
					for _, m := range []city.Point{north, south} {
						if m.X >= under.Min.X && m.X <= under.Max.X {
							land = append(land, fmt.Sprintf("a river mouth at x %.0f", m.X))
						}
					}
					assert.Empty(t, land)
				})
			}
		}
	})

	t.Run("when a truck is on its way", func(t *testing.T) {
		d := city.Delivery{Path: []city.Point{b.Land, b.Far}, Since: start}
		trip := d.For()

		t.Run("and it is on land or on the deck before the haze", func(t *testing.T) {
			t.Run("it should be drawn fully opaque", func(t *testing.T) {
				var dim []string
				for at := time.Duration(0); at < trip; at += 20 * time.Millisecond {
					now := start.Add(at)
					if along(d.At(now)) > along(b.Haze) {
						break
					}
					if a := deliveryAlpha(d, b, now); a != 1 {
						dim = append(dim, fmt.Sprintf("%v: %v", at, a))
					}
				}
				assert.Empty(t, dim)
			})
		})

		t.Run("and it is in the haze", func(t *testing.T) {
			t.Run("its alpha should be the deck's at its position", func(t *testing.T) {
				var off []string
				checked := 0
				for at := time.Duration(0); at <= trip; at += 20 * time.Millisecond {
					now := start.Add(at)
					pos := d.At(now)
					if along(pos) <= along(b.Haze) {
						continue
					}
					checked++
					if got, want := deliveryAlpha(d, b, now), b.Alpha(pos); math.Abs(got-want) > 1e-9 {
						off = append(off, fmt.Sprintf("%v: %v, want %v", at, got, want))
					}
				}
				require.Positive(t, checked, "the truck never reached the haze")
				assert.Empty(t, off)
			})
		})
	})
}

type colorName struct {
	name string
	c    color.NRGBA
}
