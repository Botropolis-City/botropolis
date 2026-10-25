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
	"botropolis.city/botropolis/pkg/plan"
)

// overGround is a texel drawn over an opaque ground colour, as a
// premultiplied draw lays it: its colour, and the ground through what
// its alpha leaves.
func overGround(rgba func() (r, g, b, a uint32), ground color.NRGBA) [3]float64 {
	r, g, b, a := rgba()
	left := 1 - float64(a)/0xffff
	return [3]float64{
		float64(r)/0xffff*255 + float64(ground.R)*left,
		float64(g)/0xffff*255 + float64(ground.G)*left,
		float64(b)/0xffff*255 + float64(ground.B)*left,
	}
}

// translucentTree is how a ground tree was drawn before item 221: its
// colour lowered and its alpha scaled, so the ground showed through.
func translucentTree(texel color.NRGBA) func() (r, g, b, a uint32) {
	return func() (r, g, b, a uint32) {
		v := groundTreeAlpha * groundTreeValue
		pr, pg, pb, pa := texel.RGBA()
		return uint32(float64(pr) * v), uint32(float64(pg) * v), uint32(float64(pb) * v), uint32(float64(pa) * groundTreeAlpha)
	}
}

// Item 85, slice 4: a tree that means nothing is drawn lower in contrast
// and saturation than anything that can be hovered. Item 221: it was
// done by letting the ground show through the tree, and the railway
// showed through with it; the mix is now baked into the tree's colour.
func TestGroundTreeTint(t *testing.T) {
	g := &Game{}

	t.Run("when a ground tree is drawn", func(t *testing.T) {
		texels := []color.NRGBA{
			{0x3a, 0x6b, 0x4a, 0xff},
			{0xc0, 0xe0, 0x90, 0xff},
			{0x3a, 0x6b, 0x4a, 0x80},
			{0x8a, 0x5a, 0x3a, 0x20},
		}
		grounds := map[string]color.NRGBA{
			"day grass":   dayShore.land,
			"night grass": nightShore.land,
			"sand":        dayShore.paint(city.GroundSand, 0.5),
		}

		for name, ground := range grounds {
			m := groundTreeMatrix(ground)

			t.Run("and it stands on "+name, func(t *testing.T) {
				t.Run("it should hide what lies under it", func(t *testing.T) {
					_, _, _, a := m.Apply(texels[0]).RGBA()
					assert.Equal(t, uint32(0xffff), a)
				})

				for _, texel := range texels {
					t.Run(fmt.Sprintf("and a texel %v is drawn", texel), func(t *testing.T) {
						now := overGround(m.Apply(texel).RGBA, ground)
						was := overGround(translucentTree(texel), ground)

						t.Run("it should look as it did over that ground", func(t *testing.T) {
							assert.InDeltaSlice(t, was[:], now[:], 1)
						})
					})
				}
			})
		}

		t.Run("it should lower its colour, so it is less saturated than the pieces round it", func(t *testing.T) {
			m := groundTreeMatrix(color.NRGBA{})
			r, gr, b, _ := m.Apply(color.NRGBA{0x3a, 0x6b, 0x4a, 0xff}).RGBA()
			assert.Less(t, overGround(func() (uint32, uint32, uint32, uint32) { return r, gr, b, 0xffff }, color.NRGBA{})[1], float64(0x6b))
		})
	})

	t.Run("when the ground a tree stands on is coloured", func(t *testing.T) {
		for _, c := range []struct {
			name           string
			night, viewing bool
			want           color.NRGBA
		}{
			{"by day", false, false, dayShore.land},
			{"by night", true, false, nightShore.land},
			{"under a view", false, true, receded(dayShore.land)},
			{"under a view by night", true, true, receded(nightShore.land)},
		} {
			t.Run("and it is grass "+c.name, func(t *testing.T) {
				t.Run("it should be the colour the ground is drawn in", func(t *testing.T) {
					assert.Equal(t, c.want, groundColour(city.GroundLand, 0, c.night, c.viewing))
				})
			})
		}
	})

	t.Run("when a hoverable piece is drawn", func(t *testing.T) {
		t.Run("it should keep its full colour", func(t *testing.T) {
			assert.Nil(t, g.scenery())
		})
	})

	t.Run("when a street tree is drawn", func(t *testing.T) {
		t.Run("it should keep its full colour, belonging to the road", func(t *testing.T) {
			_, mixed := treeMatrix(city.Tree{Kind: plan.StreetTree}, dayShore.land)
			assert.False(t, mixed)
		})
	})
}

// Item 221: Aria saw the railway through the trees on the metropolis's
// coast at zoom 2.0. Every park tree whose sprite meets the rail is
// found in the frame's record and must hide what lies under it.
func TestTheRailwayIsUnderTheTrees(t *testing.T) {
	t.Run("when the metropolis coast is recorded at zoom 2", func(t *testing.T) {
		day := time.Now()
		d, err := NewDrive(DriveOptions{Home: stageMetropolis(t, day), Now: atHour(day, 14), Width: 1920, Height: 1200})
		require.NoError(t, err)
		require.NoError(t, d.Run(30))
		g := d.game
		c := g.scene.City()
		require.GreaterOrEqual(t, len(c.Rails), 2)
		near, ok := treeBesideRail(c)
		require.True(t, ok, "no park tree stands beside the rail")
		g.scene.CenterOn(near)
		g.scene.Camera().SetZoomAt(city.Point{X: 960, Y: 600}, 2)
		require.NoError(t, d.Run(60))

		checked, translucent := g.treesOverRail(c)

		t.Run("it should find park trees over the rail to check", func(t *testing.T) {
			assert.NotZero(t, checked)
		})

		t.Run("it should draw every one of them hiding the rail", func(t *testing.T) {
			assert.Empty(t, translucent)
		})
	})
}

// treeBesideRail is a park tree standing within a cell of the rail.
func treeBesideRail(c *city.City) (city.Point, bool) {
	for _, tr := range c.Trees {
		if tr.Kind != plan.ParkTree {
			continue
		}
		for i := 1; i < len(c.Rails); i++ {
			if segmentDistance(tr.At, c.Rails[i-1], c.Rails[i]) < city.CellSize {
				return tr.At, true
			}
		}
	}
	return city.Point{}, false
}

// treesOverRail finds, in the last frame's record, the draw of every
// park tree whose sprite meets a rail on screen, and names each drawn
// letting what is under it show through.
func (g *Game) treesOverRail(c *city.City) (checked int, translucent []string) {
	cam := g.scene.Camera()
	var rails []city.Rect
	for i := 1; i < len(c.Rails); i++ {
		a, b := cam.WorldToScreen(c.Rails[i-1]), cam.WorldToScreen(c.Rails[i])
		rails = append(rails, city.Rect{Min: city.Point{X: math.Min(a.X, b.X), Y: math.Min(a.Y, b.Y)}, Max: city.Point{X: math.Max(a.X, b.X), Y: math.Max(a.Y, b.Y)}})
	}
	for _, tr := range c.Trees {
		if tr.Kind != plan.ParkTree {
			continue
		}
		img, origin, _, rect, ok := g.kitPlace(cam, treePiece(tr), 0, tr.At, 0)
		if !ok || rect.Max.X < 0 || rect.Max.Y < 0 || rect.Min.X > 1920 || rect.Min.Y > 1200 {
			continue
		}
		meets := false
		for _, r := range rails {
			if r.Overlaps(rect) && crossesRect(cam, c.Rails, rect) {
				meets = true
				break
			}
		}
		if !meets {
			continue
		}
		for _, cmd := range g.rec.cmds {
			if cmd.page != img.page || cmd.src == nil || cmd.src.Bounds() != img.img.Bounds() {
				continue
			}
			var x, y float64
			var hides bool
			switch cmd.kind {
			case cmdImage:
				o := &g.rec.imgOps[cmd.op]
				x, y = o.GeoM.Element(0, 2), o.GeoM.Element(1, 2)
				hides = o.ColorScale.A() >= 1
			case cmdColorM:
				o := &g.rec.cmOps[cmd.op]
				x, y = o.opts.GeoM.Element(0, 2), o.opts.GeoM.Element(1, 2)
				_, _, _, a := o.cm.Apply(color.NRGBA{0xff, 0xff, 0xff, 0xff}).RGBA()
				hides = a == 0xffff
			default:
				continue
			}
			if math.Abs(x-origin.X) > 0.5 || math.Abs(y-origin.Y) > 0.5 {
				continue
			}
			checked++
			if !hides {
				translucent = append(translucent, fmt.Sprintf("%s at %v", treePiece(tr), tr.At))
			}
		}
	}
	return checked, translucent
}

// crossesRect reports whether any rail segment passes through a screen
// rectangle, sampled every few pixels along it.
func crossesRect(cam *city.Camera, rails []city.Point, r city.Rect) bool {
	for i := 1; i < len(rails); i++ {
		a, b := cam.WorldToScreen(rails[i-1]), cam.WorldToScreen(rails[i])
		n := int(math.Hypot(b.X-a.X, b.Y-a.Y)/4) + 1
		for k := 0; k <= n; k++ {
			p := city.Point{X: a.X + (b.X-a.X)*float64(k)/float64(n), Y: a.Y + (b.Y-a.Y)*float64(k)/float64(n)}
			if p.X >= r.Min.X && p.X <= r.Max.X && p.Y >= r.Min.Y && p.Y <= r.Max.Y {
				return true
			}
		}
	}
	return false
}
