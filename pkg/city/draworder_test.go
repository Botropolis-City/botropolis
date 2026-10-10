package city_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// piece is one thing the map draws and the footprint it stands on.
type piece struct {
	name string
	rect city.Rect
}

// turned is a rect's box in the camera's own frame, where depth is
// x + y and the larger it is, the nearer the viewer.
func turned(cam *city.Camera, r city.Rect) city.Rect {
	corners := []city.Point{r.Min, {X: r.Max.X, Y: r.Min.Y}, r.Max, {X: r.Min.X, Y: r.Max.Y}}
	var out city.Rect
	for i, p := range corners {
		t := cam.TurnPoint(p)
		if i == 0 {
			out = city.Rect{Min: t, Max: t}
			continue
		}
		out.Min.X, out.Min.Y = min(out.Min.X, t.X), min(out.Min.Y, t.Y)
		out.Max.X, out.Max.Y = max(out.Max.X, t.X), max(out.Max.Y, t.Y)
	}
	return out
}

// inFront reports whether a stands unambiguously in front of b from
// this heading: past it in both axes at once, so there is nothing to
// argue about and the eye is certain which should be painted last.
func inFront(cam *city.Camera, a, b city.Rect) bool {
	ta, tb := turned(cam, a), turned(cam, b)
	return ta.Min.X >= tb.Max.X && ta.Min.Y >= tb.Max.Y
}

// sharesScreen reports whether two footprints land on the same part of
// the window, which is the only case where their order can be seen.
// Footprints only: a piece's height carries its sprite further up the
// screen than this, so overlap here is the conservative case.
func sharesScreen(cam *city.Camera, a, b city.Rect) bool {
	return cam.Bounds(a).Overlaps(cam.Bounds(b))
}

// plazaPieces is everything standing on or around the plaza, with the
// footprint the sort is supposed to see.
func plazaPieces(c *city.City) []piece {
	var out []piece
	if c.Plant.Rect.Area() > 0 {
		out = append(out, piece{"plant", c.Plant.Rect})
	}
	if c.Hall.Rect.Area() > 0 {
		out = append(out, piece{"hall", c.Hall.Rect})
	}
	if c.Library.Rect.Area() > 0 {
		out = append(out, piece{"library", c.Library.Rect})
	}
	if c.Fountain.Area() > 0 {
		out = append(out, piece{"fountain", c.Fountain})
	}
	for i, t := range c.Towers {
		out = append(out, piece{fmt.Sprintf("tower %d", i), t.Rect})
	}
	for _, d := range c.Districts {
		for _, b := range d.Buildings {
			out = append(out, piece{"building " + b.Session.ID, b.Rect})
		}
	}
	return out
}

func TestPlazaDrawOrder(t *testing.T) {
	var sessions []state.Session
	for _, root := range []string{cinders, botropolis, mullet} {
		for i := 0; i < 5; i++ {
			sessions = append(sessions, session(fmt.Sprintf("%s-%02d", root, i), root, state.Working))
		}
	}
	c := build(t, city.NewLayout(), sessions...)
	pieces := plazaPieces(c)
	require.NotEmpty(t, pieces)

	for _, heading := range headings {
		t.Run(fmt.Sprintf("when the plaza is drawn at heading %d", heading), func(t *testing.T) {
			cam := city.NewCamera()
			cam.Heading = heading
			order := append([]piece(nil), pieces...)
			sort.SliceStable(order, func(i, j int) bool {
				return cam.DepthOf(order[i].rect) < cam.DepthOf(order[j].rect)
			})

			t.Run("it should paint what stands in front of something else after it", func(t *testing.T) {
				var wrong []string
				for i, first := range order {
					for _, later := range order[i+1:] {
						if inFront(cam, first.rect, later.rect) {
							wrong = append(wrong, fmt.Sprintf("%s stands in front of %s but is painted before it", first.name, later.name))
						}
					}
				}
				require.Empty(t, wrong)
			})

			t.Run("it should paint the fountain and the plant in the order they stand", func(t *testing.T) {
				// The exit criterion for bug 20, as a number rather than
				// a look: whichever of the two is unambiguously nearer
				// must be painted last. At a heading where neither is,
				// there is nothing to assert.
				at := map[string]int{}
				for i, p := range order {
					at[p.name] = i
				}
				plant, fountain := c.Plant.Rect, c.Fountain
				switch {
				case inFront(cam, fountain, plant):
					assert.Greater(t, at["fountain"], at["plant"])
				case inFront(cam, plant, fountain):
					assert.Greater(t, at["plant"], at["fountain"])
				default:
					t.Skip("neither stands clear of the other from this heading")
				}
			})

			t.Run("it should give no two overlapping pieces the same depth, which leaves their order to chance", func(t *testing.T) {
				var tied []string
				for i, a := range order {
					for _, b := range order[i+1:] {
						if cam.DepthOf(a.rect) == cam.DepthOf(b.rect) && sharesScreen(cam, a.rect, b.rect) {
							tied = append(tied, fmt.Sprintf("%s ties with %s", a.name, b.name))
						}
					}
				}
				require.Empty(t, tied)
			})
		})
	}
}
