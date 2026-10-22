package render

import (
	"image"
	"image/color"
	imagedraw "image/draw"
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
	// walls is each kind's span on every row at each heading, which
	// holds its planks to the building, and storeys the storey each of
	// its panes is on, which says which light as it rises; both measured
	// as its page is put up (item 166).
	walls   map[string][4][]span
	storeys map[string][4][]int
}

// kitAtlas is the atlas for a camera, or nil without kits.
func (g *Game) kitAtlas(cam *city.Camera) *kitAtlas {
	if g.kits == nil {
		return nil
	}
	return g.kits.pick(cam.Zoom)
}

type kits struct {
	atlases []kitAtlas
	// send draws from a page just put up; see sendPage.
	send func(*ebiten.Image)
	// keepAll holds every level whatever the zoom, for a drive and the
	// sheet, which put everything up at once.
	keepAll bool
	// updates counts calls to view, and picked is the update each level
	// was last picked on.
	updates int
	picked  []int
}

// kitHoldUpdates is how long a window keeps a level its camera no longer
// picks: 5 s of updates, so zooming back and forth across the threshold
// puts nothing up again (item 162). A throttled window updates less
// often, which only makes the wait longer in wall time.
const kitHoldUpdates = 300

// loadKits puts every page up at once, for a drive and the tests, which
// have no loop to wait for, and keeps them all.
func loadKits() (*kits, error) {
	k, err := loadKitsLazily()
	if err != nil {
		return nil, err
	}
	k.keepAll = true
	for i := range k.atlases {
		for !k.complete(i) {
			if err := k.uploadPage(i); err != nil {
				return nil, err
			}
		}
	}
	return k, nil
}

// loadKitsLazily reads the atlases' manifests and puts no page up: the
// window puts them up a page an update, from inside the loop (item 70).
// Ebitengine copies anything uploaded before the loop starts and holds
// the copy until the first frame, so pages put up beforehand are all
// alive twice at once however they are decoded.
func loadKitsLazily() (*kits, error) {
	loaded, err := assets.LoadKitManifests()
	if err != nil {
		return nil, err
	}
	k := &kits{}
	for _, a := range loaded {
		// The kit's amber accent on the stack and the tanks becomes the
		// plant's tone as each page is decoded: amber is needs-you.
		a.RetintAmber([]string{kitStack, kitTower}, ui.DefaultPalette.Plant)
		k.atlases = append(k.atlases, kitAtlas{KitAtlas: a})
	}
	k.picked = make([]int, len(k.atlases))
	return k, nil
}

// ready reports whether a whole level is up to draw with.
func (k *kits) ready() bool {
	for i := range k.atlases {
		if k.complete(i) {
			return true
		}
	}
	return false
}

// progress is how many of the zoom's level's pages are up, of how many
// it has: the only level going up before the city is drawn (item 159).
func (k *kits) progress(zoom float64) (done, total int) {
	a := &k.atlases[k.index(zoom)]
	return len(a.pages), a.PageCount()
}

func (k *kits) complete(i int) bool {
	return len(k.atlases[i].pages) == k.atlases[i].PageCount()
}

// view is a window's update: it puts up a page of the level the zoom
// picks while that level is not all up, and once it is, lets go of any
// other level not picked for kitHoldUpdates (item 162). Only the level in
// use is resident: z1 is picked below zoom 1.98, and z2's six pages are
// 96 MiB that most sessions never draw.
func (k *kits) view(zoom float64) error {
	if k.keepAll {
		return nil
	}
	want := k.index(zoom)
	k.updates++
	k.picked[want] = k.updates
	if !k.complete(want) {
		return k.uploadPage(want)
	}
	for i := range k.atlases {
		if i != want && len(k.atlases[i].pages) > 0 && k.updates-k.picked[i] >= kitHoldUpdates {
			k.letGo(i)
		}
	}
	return nil
}

// uploadPage decodes level i's next page, puts it up and drops the
// decoded copy.
func (k *kits) uploadPage(i int) error {
	a := &k.atlases[i]
	page, err := a.DecodePage(len(a.pages))
	if err != nil {
		return err
	}
	a.prepareKinds(len(a.pages), page)
	up := uploadPage(page)
	if k.send == nil {
		k.send = sendPage
	}
	k.send(up)
	a.pages = append(a.pages, up)
	if k.complete(i) {
		// Nothing else the city does allocates on this scale, so
		// without a nudge the collector has no reason to run and the
		// last page's garbage is what it keeps.
		debug.FreeOSMemory()
	}
	return nil
}

// letGo gives level i's textures back.
func (k *kits) letGo(i int) {
	for _, p := range k.atlases[i].pages {
		p.Deallocate()
	}
	k.atlases[i].pages = nil
}

// pageOptions makes each page its own texture. Managed, a 2048-pixel
// page is padded to 2050 and packed into shared 4096-pixel textures that
// grow by copying, which held about twice the atlas (item 70).
func pageOptions() *ebiten.NewImageOptions {
	return &ebiten.NewImageOptions{Unmanaged: true}
}

// kitPageSide is the side of a full kit page, in pixels.
const kitPageSide = 2048

// stripRows is how many rows of a page go up at once. Ebitengine copies
// each upload into a buffer from a pool that keeps up to
// ebitenPooledBuffers released ones however large, so pages sent whole
// could leave 16 MiB in it for each buffer it kept (item 162).
const stripRows = 64

// ebitenPooledBuffers is how many released upload buffers Ebitengine's
// pool keeps (internal/graphics/bytes.go, v2.10).
const ebitenPooledBuffers = 31

// kitsReadFromEbitengine is the Ebitengine that ebitenPooledBuffers and
// sendPage's reason were read from.
const kitsReadFromEbitengine = "v2.10.2"

// uploadPage puts a decoded page up as its own texture, a strip at a time.
func uploadPage(page *image.NRGBA) *ebiten.Image {
	b := page.Bounds()
	img := ebiten.NewImageWithOptions(image.Rect(0, 0, b.Dx(), b.Dy()), pageOptions())
	pageStrips(page, func(pix []byte, region image.Rectangle) {
		img.SubImage(region).(*ebiten.Image).WritePixels(pix)
	})
	return img
}

// pageSink is what sendPage draws into. It is its own texture: managed,
// Ebitengine moves an image it has stopped drawing into onto a shared
// one, and the copy was a draw command in a settling frame.
var pageSink = ebiten.NewImageWithOptions(image.Rect(0, 0, 1, 1), &ebiten.NewImageOptions{Unmanaged: true})

// sendPage draws one pixel of a page into a 1-pixel image. Ebitengine
// keeps the pixels written to an image on the CPU until the image is
// first drawn from, and the z2 pages are not drawn below zoom 1.98, so
// six of them stayed on the heap whole: 96 MiB at r871 (item 162).
// Drawing from it sends them to the GPU at the end of the frame and
// lets them go.
func sendPage(page *ebiten.Image) {
	pageSink.DrawImage(page.SubImage(image.Rect(0, 0, 1, 1)).(*ebiten.Image), nil)
}

// pageStrips hands a page over stripRows rows at a time, premultiplied as
// Ebitengine wants them, through one buffer it reuses: put must copy what
// it keeps. Regions are relative to the page's top left.
func pageStrips(page *image.NRGBA, put func(pix []byte, region image.Rectangle)) {
	b := page.Bounds()
	w := b.Dx()
	buf := make([]byte, 4*w*min(stripRows, b.Dy()))
	for y := 0; y < b.Dy(); y += stripRows {
		region := image.Rect(0, y, w, min(y+stripRows, b.Dy()))
		dst := &image.RGBA{Pix: buf[:4*w*region.Dy()], Stride: 4 * w, Rect: image.Rect(0, 0, w, region.Dy())}
		imagedraw.Draw(dst, dst.Rect, page, b.Min.Add(region.Min), imagedraw.Src)
		put(dst.Pix, region)
	}
}

// index is the level for a camera zoom: the largest cut at or below it,
// so a sprite is only ever scaled down or up by less than a level.
func (k *kits) index(zoom float64) int {
	best := 0
	for i := range k.atlases {
		if k.atlases[i].Zoom <= zoom*1.01 {
			best = i
		}
	}
	return best
}

// pick is the atlas to draw with at a camera zoom: its level when that is
// all up, or else the nearest level that is, while it goes up.
func (k *kits) pick(zoom float64) *kitAtlas {
	if len(k.atlases) == 0 {
		return nil
	}
	want := k.index(zoom)
	if k.complete(want) {
		return &k.atlases[want]
	}
	for d := 1; d < len(k.atlases); d++ {
		for _, i := range []int{want - d, want + d} {
			if i >= 0 && i < len(k.atlases) && k.complete(i) {
				return &k.atlases[i]
			}
		}
	}
	return &k.atlases[want]
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
func (g *Game) kitPlace(cam *city.Camera, name string, turn int, at city.Point, lift float64) (img pagedSprite, origin city.Point, scale float64, rect city.Rect, ok bool) {
	if g.kits == nil {
		return pagedSprite{}, city.Point{}, 0, city.Rect{}, false
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return pagedSprite{}, city.Point{}, 0, city.Rect{}, false
	}
	sprite, found := atlas.Sprite(name, cam.Heading-turn)
	if !found || sprite.Page >= len(atlas.pages) {
		return pagedSprite{}, city.Point{}, 0, city.Rect{}, false
	}
	scale = cam.Zoom / atlas.Zoom
	foot := cam.WorldToScreen(at)
	foot.Y -= lift
	origin = city.Point{X: foot.X - float64(sprite.Anchor.X)*scale, Y: foot.Y - float64(sprite.Anchor.Y)*scale}
	img = spriteOn(atlas.pages[sprite.Page], sprite.Rect)
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

// kitFootprintTurned is a piece's footprint in world units, for standing
// something on it, as a piece turned in the world by turn degrees shows it.
func (g *Game) kitFootprintTurned(cam *city.Camera, name string, turn int) float64 {
	if g.kits == nil {
		return 0
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return 0
	}
	sprite, ok := atlas.Sprite(name, cam.Heading-turn)
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
	return g.kitScaledLifted(screen, cam, name, turn, at, shrink, 0, tint)
}

// kitScaledLifted is kitScaled lift screen pixels above the ground: the
// crate on a truck crossing the bridge (item 132).
func (g *Game) kitScaledLifted(screen *ebiten.Image, cam *city.Camera, name string, turn int, at city.Point, shrink, lift float64, tint *ebiten.ColorScale) city.Rect {
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
	foot.Y -= lift
	origin := city.Point{X: foot.X - float64(sprite.Anchor.X)*scale, Y: foot.Y - float64(sprite.Anchor.Y)*scale}
	img := spriteOn(atlas.pages[sprite.Page], sprite.Rect)
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
	g.kitGroundLifted(screen, cam, name, turn, at, 0, tint)
}

// kitGroundLifted is kitGround lift screen pixels above the ground: the
// road on the bridge's deck (item 132).
func (g *Game) kitGroundLifted(screen *ebiten.Image, cam *city.Camera, name string, turn int, at city.Point, lift float64, tint *ebiten.ColorScale) {
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
	foot.Y -= lift
	origin := city.Point{X: foot.X - float64(sprite.Anchor.X)*scale, Y: foot.Y - float64(sprite.Anchor.Y)*scale}
	img := spriteOn(atlas.pages[sprite.Page], sprite.Rect)
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
	g.drawing.world()
	g.recorder().Image(screen, img, op)
	g.punch(img, atlas.pages[sprite.Page], op.GeoM)
	h := float64(sprite.Rect.Dy()) * scale * rise
	return city.RectAt(foot.X-float64(sprite.Anchor.X)*scale, foot.Y-float64(sprite.Anchor.Y)*scale*rise, float64(sprite.Rect.Dx())*scale, h)
}

// kitAt draws a piece with its ground origin on a screen point, for
// things that hover or bob rather than stand on a cell.
// kitAt is kit with the foot already projected, and returns the screen
// rect it drew into so the caller can make it answer when pointed at.
func (g *Game) kitAt(screen *ebiten.Image, cam *city.Camera, name string, turn int, foot city.Point, tint *ebiten.ColorScale) city.Rect {
	img, origin, scale, rect, ok := g.kitAtPlace(cam, name, turn, foot)
	if !ok {
		return city.Rect{}
	}
	g.drawSprite(screen, img, origin, scale, tint)
	return rect
}

// kitAtPlace works out where kitAt would draw a piece, without drawing
// it.
func (g *Game) kitAtPlace(cam *city.Camera, name string, turn int, foot city.Point) (img pagedSprite, origin city.Point, scale float64, rect city.Rect, ok bool) {
	if g.kits == nil {
		return pagedSprite{}, city.Point{}, 0, city.Rect{}, false
	}
	atlas := g.kits.pick(cam.Zoom)
	sprite, found := atlas.Sprite(name, cam.Heading-turn)
	if !found || sprite.Page >= len(atlas.pages) {
		return pagedSprite{}, city.Point{}, 0, city.Rect{}, false
	}
	scale = cam.Zoom / atlas.Zoom
	origin = city.Point{X: foot.X - float64(sprite.Anchor.X)*scale, Y: foot.Y - float64(sprite.Anchor.Y)*scale}
	img = spriteOn(atlas.pages[sprite.Page], sprite.Rect)
	return img, origin, scale, city.RectAt(origin.X, origin.Y, float64(sprite.Rect.Dx())*scale, float64(sprite.Rect.Dy())*scale), true
}

// kitRoofLift is how high above its ground point a piece's roof plane
// lands on screen, for standing something on top of it.
func (g *Game) kitRoofLift(cam *city.Camera, name string) float64 {
	return g.kitRoofLiftTurned(cam, name, 0)
}

// kitRoofLiftTurned is kitRoofLift for a piece turned in the world.
func (g *Game) kitRoofLiftTurned(cam *city.Camera, name string, turn int) float64 {
	if g.kits == nil {
		return 0
	}
	atlas := g.kits.pick(cam.Zoom)
	if atlas == nil {
		return 0
	}
	sprite, ok := atlas.Sprite(name, cam.Heading-turn)
	if !ok {
		return 0
	}
	return roofLift(float64(sprite.Rect.Dx()), float64(sprite.Anchor.Y)) * (cam.Zoom / atlas.Zoom)
}

// kitSize is a piece's screen size at the camera's zoom without drawing it.
func (g *Game) kitSize(cam *city.Camera, name string) image.Point {
	return g.kitSizeTurned(cam, name, 0)
}

// kitSizeTurned is kitSize for a piece turned in the world.
func (g *Game) kitSizeTurned(cam *city.Camera, name string, turn int) image.Point {
	if g.kits == nil {
		return image.Point{}
	}
	atlas := g.kits.pick(cam.Zoom)
	sprite, ok := atlas.Sprite(name, cam.Heading-turn)
	if !ok {
		return image.Point{}
	}
	scale := cam.Zoom / atlas.Zoom
	return image.Pt(int(float64(sprite.Rect.Dx())*scale), int(float64(sprite.Rect.Dy())*scale))
}
