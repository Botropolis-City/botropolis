package shapes

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// sheet is one texture holding the white pixel and every disc stamp,
// which the painter tells its Cutter of instead of the images it draws
// with. Those are packed into Ebitengine's shared atlases wherever each
// fits, which differs by run, and the night's cuts alternated between two
// of them, a draw command each time; cut from one sheet, they batch the
// same way every run (item 199).
type sheet struct {
	img   *ebiten.Image
	white *ebiten.Image
	discs []*ebiten.Image
}

// sheetGap is the clear pixels between the sheet's stamps.
const sheetGap = 1

// cutSheetOptions makes the sheet its own texture, not packed into a
// shared one.
func cutSheetOptions() *ebiten.NewImageOptions {
	return &ebiten.NewImageOptions{Unmanaged: true}
}

// stamps is every stamp cut from the sheet.
func (s *sheet) stamps() []*ebiten.Image {
	return append([]*ebiten.Image{s.white}, s.discs...)
}

// cutSheet is the painter's sheet, made the first time it is asked for:
// a white 3×3 block at its origin, so the white pixel is where the mesh
// samples it, and each disc stamp after it in a row, drawn as disc draws
// its own.
func (p *Painter) cutSheet() *sheet {
	if p.cuts != nil {
		return p.cuts
	}
	width, height := 3, 3
	for level := range discRadii {
		width += sheetGap + discSize(level)
		height = max(height, discSize(level))
	}
	img := ebiten.NewImageWithOptions(image.Rect(0, 0, width, height), cutSheetOptions())
	img.SubImage(image.Rect(0, 0, 3, 3)).(*ebiten.Image).Fill(color.White)
	s := &sheet{img: img, white: img.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)}
	x := 3 + sheetGap
	for level, r := range discRadii {
		size := discSize(level)
		stamp := img.SubImage(image.Rect(x, 0, x+size, size)).(*ebiten.Image)
		half := float32(size) / 2
		vector.FillCircle(stamp, float32(x)+half, half, float32(r), color.White, true)
		s.discs = append(s.discs, stamp)
		x += size + sheetGap
	}
	p.cuts = s
	return s
}

// discSize is the side of the stamp for one of discRadii: the disc, and
// two clear pixels round it so linear filtering never reads past its
// edge.
func discSize(level int) int {
	return int(math.Ceil(2*discRadii[level])) + 4
}
