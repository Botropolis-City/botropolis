package assets_test

import (
	"fmt"
	"image"
	"math"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
)

var kindPiece = regexp.MustCompile(`^botropolis/(residential|shopping|office|industrial)-[1-5]$`)

// inQuad is whether a pixel's centre lies inside a pane, its edges
// included.
func inQuad(q assets.Pane, x, y int) bool {
	px, py := float64(x)+0.5, float64(y)+0.5
	sign := 0.0
	for i := range q {
		a, b := q[i], q[(i+1)%4]
		cross := float64(b.X-a.X)*(py-float64(a.Y)) - float64(b.Y-a.Y)*(px-float64(a.X))
		if cross == 0 {
			continue
		}
		if sign == 0 {
			sign = cross
		} else if (cross > 0) != (sign > 0) {
			return false
		}
	}
	return true
}

func inAnyPane(panes []assets.Pane, x, y int) bool {
	for _, q := range panes {
		if inQuad(q, x, y) {
			return true
		}
	}
	return false
}

// Item 166: a building's lit windows are drawn from its panes, so the
// panes have to cover its windows exactly: every pixel of glass, and
// nothing off the building. The glass is told by the rule the pipeline
// measured it with, which it writes into the manifest, so this test and
// the pipeline cannot disagree about what glass is.
func TestPanesCoverTheWindows(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)

	for _, atlas := range atlases {
		rules := atlas.Glass
		t.Run(fmt.Sprintf("when the atlas at zoom %g is read", atlas.Zoom), func(t *testing.T) {
			t.Run("it should carry the glass rule the pipeline measured with", func(t *testing.T) {
				assert.True(t, rules.Lit.Max[0] > 0 && rules.Core.Max[0] > 0)
			})
		})
		for _, name := range atlas.Names() {
			if !kindPiece.MatchString(name) {
				continue
			}
			for _, heading := range assets.Headings {
				sprite, _ := atlas.Sprite(name, heading)
				page := atlas.Pages[sprite.Page].(*image.NRGBA)
				var glass, core, rim, offBuilding int
				w, h := sprite.Rect.Dx(), sprite.Rect.Dy()
				at := func(x, y int) (r, g, b, a uint8) {
					c := page.NRGBAAt(sprite.Rect.Min.X+x, sprite.Rect.Min.Y+y)
					return c.R, c.G, c.B, c.A
				}
				isCore := func(x, y int) bool {
					return x >= 0 && y >= 0 && x < w && y < h && rules.Core.Matches(at(x, y))
				}
				for y := range h {
					for x := range w {
						if isCore(x, y) {
							glass++
						}
						covered := inAnyPane(sprite.Panes, x, y)
						if _, _, _, a := at(x, y); covered && a == 0 {
							offBuilding++
						}
						if covered {
							continue
						}
						if isCore(x, y) {
							core++
							continue
						}
						if rules.Lit.Matches(at(x, y)) {
							for dy := -1; dy <= 1; dy++ {
								for dx := -1; dx <= 1; dx++ {
									if isCore(x+dx, y+dy) {
										rim++
										dx, dy = 2, 2
									}
								}
							}
						}
					}
				}
				var skewed []assets.Pane
				for _, q := range sprite.Panes {
					run := q[1].X - q[0].X
					if run <= 2 {
						continue
					}
					slope := math.Abs(float64(q[1].Y-q[0].Y) / float64(run))
					if math.Abs(slope-0.5) > 1/float64(run)+1e-9 {
						skewed = append(skewed, q)
					}
				}

				t.Run(fmt.Sprintf("when %s is cut from %d at zoom %g", name, heading, atlas.Zoom), func(t *testing.T) {
					t.Run("it should run every pane along its face, at the projection's slope", func(t *testing.T) {
						assert.Empty(t, skewed)
					})

					t.Run("it should have a pane wherever it shows glass", func(t *testing.T) {
						assert.True(t, glass == 0 || len(sprite.Panes) > 0)
					})

					t.Run("it should cover every pixel of window glass with a pane", func(t *testing.T) {
						assert.Zero(t, core)
					})

					t.Run("it should cover the glass's soft edge too", func(t *testing.T) {
						assert.Zero(t, rim)
					})

					t.Run("it should keep every pane on the building's own pixels", func(t *testing.T) {
						assert.Zero(t, offBuilding)
					})
				})
			}
		}
	}
}
