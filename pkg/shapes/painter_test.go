package shapes

import (
	"image/color"
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

type cutLog struct {
	meshes, discs, rings int
	indices              int
	ringPoints           int
	discAlpha, ringAlpha float32
	stamps               []*ebiten.Image
	geoms                []ebiten.GeoM
}

func (c *cutLog) CutMesh(vertices []ebiten.Vertex, indices []uint32) {
	c.meshes++
	c.indices += len(indices)
}

func (c *cutLog) CutImage(stamp *ebiten.Image, op *ebiten.DrawImageOptions) {
	c.discs++
	c.stamps = append(c.stamps, stamp)
	c.geoms = append(c.geoms, op.GeoM)
	c.discAlpha = op.ColorScale.A()
}

func (c *cutLog) CutRing(pts []city.Point, alpha float32) {
	c.rings++
	c.ringPoints = len(pts)
	c.ringAlpha = alpha
}

func TestPainterCuts(t *testing.T) {
	half := color.NRGBA{R: 200, G: 100, B: 50, A: 128}
	square := []city.Point{{X: 10, Y: 10}, {X: 30, Y: 10}, {X: 30, Y: 30}, {X: 10, Y: 30}}
	bowtie := []city.Point{{X: 10, Y: 10}, {X: 30, Y: 30}, {X: 30, Y: 10}, {X: 10, Y: 30}}

	t.Run("when a painter has a cut hook", func(t *testing.T) {
		draw := func(paint func(p *Painter, dst *ebiten.Image)) *cutLog {
			dst := ebiten.NewImage(40, 40)
			defer dst.Deallocate()
			log := &cutLog{}
			p := &Painter{Cut: log}
			paint(p, dst)
			return log
		}

		t.Run("and it fills a ring", func(t *testing.T) {
			log := draw(func(p *Painter, dst *ebiten.Image) { p.Fill(dst, square, half) })

			t.Run("it should report the ring's mesh", func(t *testing.T) {
				assert.Equal(t, 1, log.meshes)
			})
		})

		t.Run("and it strokes a line", func(t *testing.T) {
			log := draw(func(p *Painter, dst *ebiten.Image) { p.Line(dst, 5, 5, 35, 35, 2, half) })

			t.Run("it should report it as one hard-edged bar at its alpha, and not its mesh", func(t *testing.T) {
				assert.Equal(t, []any{0, 1, float32(128) / 255}, []any{log.meshes, log.discs, log.discAlpha})
			})
		})

		t.Run("and it adds strokes and flushes them", func(t *testing.T) {
			log := draw(func(p *Painter, dst *ebiten.Image) {
				p.AddLine(city.Point{X: 5, Y: 5}, city.Point{X: 35, Y: 5}, 2, half)
				p.AddLine(city.Point{X: 5, Y: 9}, city.Point{X: 35, Y: 9}, 2, half)
				p.Flush(dst)
			})

			t.Run("it should report them as one mesh", func(t *testing.T) {
				assert.Equal(t, 1, log.meshes)
			})
		})

		// Item 199: what the cutter is told of is laid as the night's
		// lights are, and stamps from one texture lay in one draw. Packed
		// into shared atlases wherever each fitted, they alternated
		// between two, by run.
		t.Run("and it fills a disc of every stamp size and strokes a line", func(t *testing.T) {
			p := &Painter{}
			log := draw(func(q *Painter, dst *ebiten.Image) {
				p = q
				for _, r := range discRadii {
					q.Disc(dst, city.Point{X: 20, Y: 20}, r, half)
				}
				q.Line(dst, 5, 5, 35, 35, 2, half)
			})
			sheet := p.cutSheet()

			t.Run("it should tell the cutter only of stamps that lie on its cut sheet", func(t *testing.T) {
				for _, s := range log.stamps {
					require.True(t, slices.Contains(sheet.stamps(), s), "a stamp not cut from the sheet: %v", s.Bounds())
					require.True(t, s.Bounds().In(sheet.img.Bounds()), "a stamp off the sheet: %v", s.Bounds())
				}
				assert.Len(t, log.stamps, len(discRadii)+1)
			})

			t.Run("it should tell it of no two stamps that overlap there", func(t *testing.T) {
				overlaps := 0
				for i, a := range log.stamps {
					for _, b := range log.stamps[i+1:] {
						if a != b && a.Bounds().Overlaps(b.Bounds()) {
							overlaps++
						}
					}
				}
				assert.Zero(t, overlaps)
			})
		})

		t.Run("and it fills a disc", func(t *testing.T) {
			log := draw(func(p *Painter, dst *ebiten.Image) { p.Disc(dst, city.Point{X: 20, Y: 20}, 6, half) })

			t.Run("it should report the disc's stamp at the disc's alpha", func(t *testing.T) {
				assert.Equal(t, []any{1, float32(128) / 255}, []any{log.discs, log.discAlpha})
			})
		})

		for _, c := range []struct {
			what  string
			want  int
			paint func(p *Painter, dst *ebiten.Image)
		}{
			{"fills a hard-edged box", 1, func(p *Painter, dst *ebiten.Image) { p.Box(dst, 5, 5, 20, 10, half) }},
			{"strokes a hard-edged box", 4, func(p *Painter, dst *ebiten.Image) { p.BoxStroke(dst, 5, 5, 20, 10, 1, half) }},
			{"strokes a hard-edged box thicker than it is", 1, func(p *Painter, dst *ebiten.Image) { p.BoxStroke(dst, 5, 5, 4, 4, 6, half) }},
			{"strokes a hard-edged bar", 1, func(p *Painter, dst *ebiten.Image) { p.Bar(dst, 5, 5, 30, 20, 2, half) }},
		} {
			t.Run("and it "+c.what, func(t *testing.T) {
				log := draw(c.paint)

				t.Run("it should report each piece at the shape's alpha", func(t *testing.T) {
					assert.Equal(t, []any{c.want, float32(128) / 255}, []any{log.discs, log.discAlpha})
				})
			})
		}

		t.Run("and it fills a ring that crosses itself", func(t *testing.T) {
			log := draw(func(p *Painter, dst *ebiten.Image) { p.Fill(dst, bowtie, half) })

			t.Run("it should report the fallback's ring at the fill's alpha", func(t *testing.T) {
				assert.Equal(t, []any{1, 4, float32(128) / 255}, []any{log.rings, log.ringPoints, log.ringAlpha})
			})
		})

		t.Run("and it fills a disc with the lighter blend", func(t *testing.T) {
			log := draw(func(p *Painter, dst *ebiten.Image) {
				p.DiscBlend(dst, city.Point{X: 20, Y: 20}, 6, half, ebiten.BlendLighter)
			})

			t.Run("it should report nothing, a glow being light and not cover", func(t *testing.T) {
				assert.Equal(t, cutLog{}, *log)
			})
		})

		t.Run("and it fills a ring with the lighter blend", func(t *testing.T) {
			log := draw(func(p *Painter, dst *ebiten.Image) { p.FillBlend(dst, square, half, ebiten.BlendLighter) })

			t.Run("it should report nothing", func(t *testing.T) {
				assert.Equal(t, cutLog{}, *log)
			})
		})
	})

	t.Run("when a painter makes its cut sheet", func(t *testing.T) {
		t.Run("it should be its own texture, not packed into a shared one", func(t *testing.T) {
			assert.True(t, cutSheetOptions().Unmanaged)
		})
	})
}
