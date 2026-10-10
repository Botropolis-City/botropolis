package render

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
)

type atlas struct {
	sheet *ebiten.Image
}

func loadAtlas(pack assets.Pack) (*atlas, error) {
	img, err := assets.Tilemap(pack)
	if err != nil {
		return nil, err
	}
	return &atlas{sheet: ebiten.NewImageFromImage(img)}, nil
}

func (a *atlas) tile(col, row int) *ebiten.Image {
	r := image.Rect(col*assets.TileSize, row*assets.TileSize, (col+1)*assets.TileSize, (row+1)*assets.TileSize)
	return a.sheet.SubImage(r).(*ebiten.Image)
}

type sprites struct {
	town, factory, modern *atlas
}

func loadSprites() (*sprites, error) {
	town, err := loadAtlas(assets.TinyTown)
	if err != nil {
		return nil, err
	}
	factory, err := loadAtlas(assets.TinyFactory)
	if err != nil {
		return nil, err
	}
	modern, err := loadAtlas(assets.ModernCity)
	if err != nil {
		return nil, err
	}
	return &sprites{town: town, factory: factory, modern: modern}, nil
}

// Tile coordinates (column, row) in the packed 16 px sheets.
var (
	townRoofGrey   = [3][2]int{{0, 4}, {1, 4}, {2, 4}}
	townRoofOrange = [3][2]int{{4, 4}, {5, 4}, {6, 4}}
	townWallWood   = [3][2]int{{0, 6}, {2, 6}, {3, 6}}
	townWallStone  = [3][2]int{{4, 6}, {6, 6}, {7, 6}}
	townWindowLit  = [2]int{0, 7}
	townWindowDark = [2]int{4, 7}
	townSign       = [2]int{11, 6}

	factoryWorker  = [2]int{0, 10}
	factoryGear    = [2]int{6, 9}
	factoryChain   = [2]int{7, 9}
	factoryHook    = [2]int{7, 10}
	factoryMachine = [3][2]int{{3, 6}, {4, 6}, {5, 6}}
	factoryMachLow = [3][2]int{{3, 7}, {4, 7}, {5, 7}}
	factoryMast    = [2]int{8, 9}

	modernPave = [2][2]int{{0, 20}, {1, 20}}
)

// draw places a 16 px tile so that it covers the world rect r (which need not be square).
// drawTile draws one 16 px tile in world space.
//
// While a view is up it recedes, whatever scale it was handed. That is
// the same rule as drawSprite in the isometric path and for the same
// reason — the view is subtractive — but it has to be stated here too,
// because these callers hand over a scale of their own (the plant's
// glow, a tower's warmth) and a scale can only darken. An object that
// always carries its own tint is one a view can never drain, which is
// exactly how the water towers went on shining over a city that had
// stepped back. The building path wants the view's colour rather than
// the recede, and asks for it with drawTileTinted.
func (g *Game) drawTile(screen *ebiten.Image, cam *city.Camera, t *ebiten.Image, r city.Rect, scale *ebiten.ColorScale) {
	if g.viewing() {
		g.drawTileMatrix(screen, cam, t, r, recedeMatrix())
		return
	}
	g.drawTileScaled(screen, cam, t, r, scale)
}

// drawTileTinted is drawTile for the things a view has something to say
// about: it takes the view's colour instead of receding.
func (g *Game) drawTileTinted(screen *ebiten.Image, cam *city.Camera, t *ebiten.Image, r city.Rect, scale *ebiten.ColorScale, tint *ebiten.ColorScale) {
	if g.viewing() {
		if tint != nil {
			g.drawTileScaled(screen, cam, t, r, tint)
			return
		}
		g.drawTileMatrix(screen, cam, t, r, recedeMatrix())
		return
	}
	g.drawTileScaled(screen, cam, t, r, scale)
}

func (g *Game) drawTileScaled(screen *ebiten.Image, cam *city.Camera, t *ebiten.Image, r city.Rect, scale *ebiten.ColorScale) {
	min := cam.WorldToScreen(r.Min)
	max := cam.WorldToScreen(r.Max)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale((max.X-min.X)/assets.TileSize, (max.Y-min.Y)/assets.TileSize)
	op.GeoM.Translate(min.X, min.Y)
	if scale != nil {
		op.ColorScale = *scale
	}
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(t, op)
}

func (g *Game) drawTileMatrix(screen *ebiten.Image, cam *city.Camera, t *ebiten.Image, r city.Rect, m colorm.ColorM) {
	min := cam.WorldToScreen(r.Min)
	max := cam.WorldToScreen(r.Max)
	op := &colorm.DrawImageOptions{}
	op.GeoM.Scale((max.X-min.X)/assets.TileSize, (max.Y-min.Y)/assets.TileSize)
	op.GeoM.Translate(min.X, min.Y)
	op.Filter = ebiten.FilterNearest
	colorm.DrawImage(screen, t, m, op)
}

func cell(r city.Rect, cols, rows, col, row int) city.Rect {
	w := r.Width() / float64(cols)
	h := r.Height() / float64(rows)
	return city.RectAt(r.Min.X+float64(col)*w, r.Min.Y+float64(row)*h, w, h)
}
