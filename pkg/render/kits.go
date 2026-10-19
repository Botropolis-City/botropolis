package render

import (
	"image"
	"image/color"
	"math"
	"runtime/debug"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// kitAtlas is one zoom level of the pre-rendered kit pieces on the GPU.
type kitAtlas struct {
	assets.KitAtlas
	pages []*ebiten.Image
}

type kits struct {
	atlases []kitAtlas
}

func loadKits() (*kits, error) {
	loaded, err := assets.LoadKits()
	if err != nil {
		return nil, err
	}
	k := &kits{}
	for _, a := range loaded {
		ka := kitAtlas{KitAtlas: a}
		// The kit's amber accent on the stack and the tanks becomes the
		// plant's tone before the pages go up: amber is needs-you.
		a.RetintAmber([]string{kitStack, kitTower}, ui.DefaultPalette.Plant)
		for _, page := range a.Pages {
			ka.pages = append(ka.pages, ebiten.NewImageFromImage(page))
		}
		// The pixels live on the GPU now; the decoded pages are only weight.
		ka.Pages = nil
		k.atlases = append(k.atlases, ka)
	}
	// Nine 2048-pixel pages decode to about 150 MB on the way to the
	// card, and that is the peak a small machine would feel. Nothing
	// else the city does allocates on that scale, so without a nudge the
	// collector has no reason to run and the peak is what it keeps.
	debug.FreeOSMemory()
	return k, nil
}

// pick is the atlas for a camera zoom: the largest cut at or below it,
// so a sprite is only ever scaled down or up by less than a level.
func (k *kits) pick(zoom float64) *kitAtlas {
	if len(k.atlases) == 0 {
		return nil
	}
	best := &k.atlases[0]
	for i := range k.atlases {
		if k.atlases[i].Zoom <= zoom*1.01 {
			best = &k.atlases[i]
		}
	}
	return best
}

// Colours of the kit render's ground, so the flat ground under the
// sprites matches the light they were cut under.
var (
	colorKitGrass    = ui.DefaultPalette.Ground
	colorKitFloor    = color.NRGBA{0xc2, 0xc0, 0xb8, 0xff}
	colorKitFloorHi  = color.NRGBA{0xd4, 0xd2, 0xca, 0xff}
	colorKitConcrete = color.NRGBA{0xb3, 0xb3, 0xb6, 0xff}
	colorKitKerb     = color.NRGBA{0x5c, 0x5e, 0x66, 0xff}
	colorKitWater    = color.NRGBA{0x6f, 0xa8, 0xd0, 0xff}
)

// kit draws a piece with its ground origin on a world point, turned in
// the world by turn degrees, as the camera's heading sees it. It returns
// the sprite's screen rect for anything that hangs off it.
func (g *Game) kit(screen *ebiten.Image, cam *city.Camera, name string, turn int, at city.Point, tint *ebiten.ColorScale) city.Rect {
	return g.kitLifted(screen, cam, name, turn, at, 0, tint)
}

// kitLifted draws a piece standing lift screen pixels above the ground
// point it is placed at, for a roof feature — see plantStackPerch. A
// lift of zero is a piece on the ground, which is what kit is.
func (g *Game) kitLifted(screen *ebiten.Image, cam *city.Camera, name string, turn int, at city.Point, lift float64, tint *ebiten.ColorScale) city.Rect {
	img, origin, scale, rect, ok := g.kitPlace(cam, name, turn, at, lift)
	if !ok {
		return city.Rect{}
	}
	g.drawSprite(screen, img, origin, scale, tint)
	return rect
}

// kitPlace works out where kitLifted would draw a piece, without
// drawing it: the one expression both the drawing and anything placed
// on the drawn sprite read, such as a landmark's label (item 99).
func (g *Game) kitPlace(cam *city.Camera, name string, turn int, at city.Point, lift float64) (img *ebiten.Image, origin city.Point, scale float64, rect city.Rect, ok bool) {
	if g.kits == nil {
		return nil, city.Point{}, 0, city.Rect{}, false
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return nil, city.Point{}, 0, city.Rect{}, false
	}
	sprite, found := atlas.Sprite(name, cam.Heading-turn)
	if !found || sprite.Page >= len(atlas.pages) {
		return nil, city.Point{}, 0, city.Rect{}, false
	}
	scale = cam.Zoom / atlas.Zoom
	foot := cam.WorldToScreen(at)
	foot.Y -= lift
	origin = city.Point{X: foot.X - float64(sprite.Anchor.X)*scale, Y: foot.Y - float64(sprite.Anchor.Y)*scale}
	img = atlas.pages[sprite.Page].SubImage(sprite.Rect).(*ebiten.Image)
	return img, origin, scale, city.RectAt(origin.X, origin.Y, float64(sprite.Rect.Dx())*scale, float64(sprite.Rect.Dy())*scale), true
}

// kitThroughPlace works out where a piece standing on a world point
// would draw if it were cut off at a plane cut screen pixels above that
// point, so only the part above the plane shows: a chimney rising from
// inside a building through its roof. The piece stands on the
// building's floor, which is why the cut and the lift are the same
// number. It places without drawing. The curb at the join has to go down partly before the stack
// and partly after it, so it needs the stack's rect in advance — and
// this is the one expression both of them read, rather than a second
// copy of the placement to drift out of step.
func (g *Game) kitThroughPlace(cam *city.Camera, name string, at city.Point, cut float64) (page *ebiten.Image, src image.Rectangle, origin city.Point, scale float64, rect city.Rect, ok bool) {
	if g.kits == nil || cut <= 0 {
		return nil, image.Rectangle{}, city.Point{}, 0, city.Rect{}, false
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return nil, image.Rectangle{}, city.Point{}, 0, city.Rect{}, false
	}
	sprite, found := atlas.Sprite(name, cam.Heading)
	if !found || sprite.Page >= len(atlas.pages) {
		return nil, image.Rectangle{}, city.Point{}, 0, city.Rect{}, false
	}
	scale = cam.Zoom / atlas.Zoom
	hidden := sunkRows(cut, scale, float64(sprite.Rect.Dy()), float64(sprite.Anchor.Y))
	src = sprite.Rect
	src.Max.Y -= int(hidden + 0.5)
	if src.Dy() <= 0 {
		return nil, image.Rectangle{}, city.Point{}, 0, city.Rect{}, false
	}
	foot := cam.WorldToScreen(at)
	origin = city.Point{X: foot.X - float64(sprite.Anchor.X)*scale, Y: foot.Y - float64(sprite.Anchor.Y)*scale}
	rect = city.RectAt(origin.X, origin.Y, float64(src.Dx())*scale, float64(src.Dy())*scale)
	return atlas.pages[sprite.Page], src, origin, scale, rect, true
}

// kitFootprint is a piece's footprint in world units, for standing
// something on it.
func (g *Game) kitFootprint(cam *city.Camera, name string) float64 {
	if g.kits == nil {
		return 0
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return 0
	}
	sprite, ok := atlas.Sprite(name, cam.Heading)
	if !ok {
		return 0
	}
	return footprintSide(float64(sprite.Rect.Dx()), atlas.Zoom)
}

// kitScaled draws a piece at a fraction of its natural size, for a
// sprite that is bigger than the thing it has to sit in — the liner is
// nearly two cells wide and the river is one.
//
// turn is the piece's own rotation in the world, in degrees, as
// everywhere else in this family: the atlas is indexed at heading-turn,
// so a turned piece keeps facing the same way as the camera goes round.
func (g *Game) kitScaled(screen *ebiten.Image, cam *city.Camera, name string, turn int, at city.Point, shrink float64, tint *ebiten.ColorScale) city.Rect {
	if g.kits == nil || shrink <= 0 {
		return city.Rect{}
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return city.Rect{}
	}
	sprite, ok := atlas.Sprite(name, cam.Heading-turn)
	if !ok || sprite.Page >= len(atlas.pages) {
		return city.Rect{}
	}
	scale := cam.Zoom / atlas.Zoom * shrink
	foot := cam.WorldToScreen(at)
	origin := city.Point{X: foot.X - float64(sprite.Anchor.X)*scale, Y: foot.Y - float64(sprite.Anchor.Y)*scale}
	img := atlas.pages[sprite.Page].SubImage(sprite.Rect).(*ebiten.Image)
	g.drawSprite(screen, img, origin, scale, tint)
	return city.RectAt(origin.X, origin.Y, float64(sprite.Rect.Dx())*scale, float64(sprite.Rect.Dy())*scale)
}

// kitSized draws a piece scaled so its longest side comes out at target
// pixels at zoom 1, whatever size the sprite happens to be and whatever
// cut the zoom ladder picked.
//
// Sizing by a per-piece shrink factor does not survive different
// sprites: the liner and the cargo ship are 15.2 and 10.6 long in the
// model, which looked like a clear step, but their cut sprites are 505
// and 429 wide and shrink factors chosen from the model drew them the
// same length. Model space is not screen space once the projection and
// the per-kit SCALE have had their say — the same gap that made the
// chimney's pipe measurement wrong. So the target is stated and the
// scale is worked back from the art.
// kitShrink is how far to shrink a piece so its longest side draws at
// target pixels at zoom 1 — a size in world terms, not in screen ones.
//
// Bug 53, and the atlasZoom is the whole of the fix. kitScaled then
// draws at cam.Zoom/atlas.Zoom times this, so the drawn size comes out
// at target*cam.Zoom: the sprite's own pixels cancel, and so does the
// atlas the ladder happened to pick.
//
// Without the atlasZoom the drawn size was target*cam.Zoom/atlas.Zoom.
// That looks fine at any one zoom and is wrong at exactly one place:
// atlas.Zoom is a step function, so where the z2 cut takes over the
// ratio halves and the piece halves with it. Aria saw it as "the boats
// get smaller at a certain zoom"; nobody had written a threshold.
//
// Every other piece survives that step because kit and kitLifted apply
// the ratio to the sprite's own pixels, and a z2 cut being twice its z1
// cut cancels it. That cancellation is what the ladder is for. Stating
// a size in screen pixels opts out of it — the same family as the unit
// confusions in rules/a-number-can-be-right-and-mean-nothing.md, but
// between coordinate systems rather than
// between tile sizes, and it hides at a boundary nobody screenshots.
func kitShrink(target, longest, atlasZoom float64) float64 {
	return target * atlasZoom / longest
}

func (g *Game) kitSized(screen *ebiten.Image, cam *city.Camera, name string, turn int, at city.Point, target float64, tint *ebiten.ColorScale) city.Rect {
	if g.kits == nil || target <= 0 {
		return city.Rect{}
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return city.Rect{}
	}
	sprite, ok := atlas.Sprite(name, cam.Heading-turn)
	if !ok {
		return city.Rect{}
	}
	longest := math.Max(float64(sprite.Rect.Dx()), float64(sprite.Rect.Dy()))
	if longest <= 0 {
		return city.Rect{}
	}
	return g.kitScaled(screen, cam, name, turn, at, kitShrink(target, longest, atlas.Zoom), tint)
}

// kitGround draws a ground tile a hair larger than its cell, so two tiles
// side by side overlap by their anti-aliased rims instead of letting the
// grass show through as a seam.
func (g *Game) kitGround(screen *ebiten.Image, cam *city.Camera, name string, turn int, at city.Point, tint *ebiten.ColorScale) {
	if g.kits == nil {
		return
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return
	}
	sprite, ok := atlas.Sprite(name, cam.Heading-turn)
	if !ok || sprite.Page >= len(atlas.pages) {
		return
	}
	scale := cam.Zoom / atlas.Zoom * groundBleed
	foot := cam.WorldToScreen(at)
	origin := city.Point{X: foot.X - float64(sprite.Anchor.X)*scale, Y: foot.Y - float64(sprite.Anchor.Y)*scale}
	img := atlas.pages[sprite.Page].SubImage(sprite.Rect).(*ebiten.Image)
	g.drawSprite(screen, img, origin, scale, tint)
}

// groundBleed is how much larger a ground tile is drawn than its cell:
// just enough for neighbours' anti-aliased rims to overlap.
const groundBleed = 1.01

// kitRising draws a piece standing on a world point at a fraction of its
// height, growing from its foot: a building rising as its session
// arrives. At 1 it is kit.
func (g *Game) kitRising(screen *ebiten.Image, cam *city.Camera, name string, foot city.Point, tint *ebiten.ColorScale, rise float64) city.Rect {
	if rise >= 1 || g.kits == nil {
		return g.kitAt(screen, cam, name, 0, foot, tint)
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return city.Rect{}
	}
	sprite, ok := atlas.Sprite(name, cam.Heading)
	if !ok || sprite.Page >= len(atlas.pages) {
		return city.Rect{}
	}
	scale := cam.Zoom / atlas.Zoom
	img := atlas.pages[sprite.Page].SubImage(sprite.Rect).(*ebiten.Image)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(sprite.Anchor.X), -float64(sprite.Anchor.Y))
	op.GeoM.Scale(scale, scale*rise)
	op.GeoM.Translate(foot.X, foot.Y)
	if tint != nil {
		op.ColorScale = *tint
	}
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(img, op)
	h := float64(sprite.Rect.Dy()) * scale * rise
	return city.RectAt(foot.X-float64(sprite.Anchor.X)*scale, foot.Y-float64(sprite.Anchor.Y)*scale*rise, float64(sprite.Rect.Dx())*scale, h)
}

// kitAt draws a piece with its ground origin on a screen point, for
// things that hover or bob rather than stand on a cell.
// kitAt is kit with the foot already projected, and returns the screen
// rect it drew into so the caller can make it answer when pointed at.
func (g *Game) kitAt(screen *ebiten.Image, cam *city.Camera, name string, turn int, foot city.Point, tint *ebiten.ColorScale) city.Rect {
	if g.kits == nil {
		return city.Rect{}
	}
	atlas := g.kits.pick(cam.Zoom)
	sprite, ok := atlas.Sprite(name, cam.Heading-turn)
	if !ok || sprite.Page >= len(atlas.pages) {
		return city.Rect{}
	}
	scale := cam.Zoom / atlas.Zoom
	origin := city.Point{X: foot.X - float64(sprite.Anchor.X)*scale, Y: foot.Y - float64(sprite.Anchor.Y)*scale}
	img := atlas.pages[sprite.Page].SubImage(sprite.Rect).(*ebiten.Image)
	g.drawSprite(screen, img, origin, scale, tint)
	return city.RectAt(origin.X, origin.Y, float64(sprite.Rect.Dx())*scale, float64(sprite.Rect.Dy())*scale)
}

// kitRoofLift is how high above its ground point a piece's roof plane
// lands on screen, for standing something on top of it.
func (g *Game) kitRoofLift(cam *city.Camera, name string) float64 {
	if g.kits == nil {
		return 0
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return 0
	}
	sprite, ok := atlas.Sprite(name, cam.Heading)
	if !ok {
		return 0
	}
	return roofLift(float64(sprite.Rect.Dx()), float64(sprite.Anchor.Y)) * (cam.Zoom / atlas.Zoom)
}

// kitSize is a piece's screen size at the camera's zoom without drawing it.
func (g *Game) kitSize(cam *city.Camera, name string) image.Point {
	if g.kits == nil {
		return image.Point{}
	}
	atlas := g.kits.pick(cam.Zoom)
	sprite, ok := atlas.Sprite(name, cam.Heading)
	if !ok {
		return image.Point{}
	}
	scale := cam.Zoom / atlas.Zoom
	return image.Pt(int(float64(sprite.Rect.Dx())*scale), int(float64(sprite.Rect.Dy())*scale))
}
