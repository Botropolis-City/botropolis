package render

import (
	"encoding/binary"
	"hash/maphash"
	"image"
	"image/color"
	"math"
	"reflect"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/shapes"
)

// The dry pass (item 193). Each frame every drawable runs as it always
// has, onto a stand-in image that is never drawn: each draw it makes is
// recorded instead, with the box it would ink and a hash of what it
// would draw. The still city is composed from the record once, while
// its hash holds, and only where something moves is the record played
// again, clipped. Playing the record rather than running the drawable
// again means nothing a drawable notes as it draws — a hit, a light, a
// word seen, a look baked — can happen twice.

// inkPad is how far past what a draw covers its box reaches: linear
// filtering spreads an edge by a pixel, and a box rounds out to whole
// pixels.
const inkPad = 2

var inkSeed = maphash.MakeSeed()

type cmdKind uint8

const (
	cmdImage cmdKind = iota + 1
	cmdColorM
	cmdMesh
	cmdRing
	cmdWords
	cmdFill
)

// drawCmd is one recorded draw.
type drawCmd struct {
	kind  cmdKind
	src   *ebiten.Image
	img   ebiten.DrawImageOptions
	cm    colorm.ColorM
	cmImg colorm.DrawImageOptions
	tri   ebiten.DrawTrianglesOptions
	path  vector.DrawPathOptions
	text  text.DrawOptions
	words string
	face  text.Face
	fill  color.Color
	// v, i and p are ranges in the list's vertices, indices and points.
	v, i, p [2]int
	// fade is the hover fade group the draw belongs to, 0 for none: the
	// draws of one group are laid on the fade layer together and the
	// layer put down when the group ends (item 108).
	fade int
	box  image.Rectangle
	// hash is what the command draws, positions relative to the origin.
	hash uint64
}

// inked is one drawable's commands in the list, the box they ink and
// their hash.
type inked struct {
	from, to int
	box      image.Rectangle
	hash     uint64
}

// drawList records the draws made onto its dry image.
type drawList struct {
	dry      *ebiten.Image
	cmds     []drawCmd
	vertices []ebiten.Vertex
	indices  []uint32
	points   []city.Point
	painter  shapes.Painter

	// fade is the fade group draws are recorded into now, and fades
	// the last group begun.
	fade, fades int
	// moving is whether the drawable recorded now said it moves.
	moving bool
	// batchKeys numbers the sources draws read, for grouping them.
	batchKeys map[batchSource]int
	// origin is where the camera puts the world's origin this frame:
	// positions are hashed relative to it, so a pan by whole pixels
	// changes no hash (item 193).
	origin city.Point

	open  bool
	start int
	box   image.Rectangle
	h, ch maphash.Hash
}

func newDrawList() *drawList {
	l := &drawList{dry: ebiten.NewImage(1, 1)}
	l.h.SetSeed(inkSeed)
	l.ch.SetSeed(inkSeed)
	return l
}

// reset empties the list for a new frame, keeping its buffers.
func (l *drawList) reset() {
	l.cmds, l.vertices, l.indices, l.points = l.cmds[:0], l.vertices[:0], l.indices[:0], l.points[:0]
	l.fade, l.fades, l.open = 0, 0, false
}

// seal closes the command recorded last: its hash is kept with it, and
// folded into its drawable's.
func (l *drawList) seal() {
	c := &l.cmds[len(l.cmds)-1]
	c.hash = l.ch.Sum64()
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], c.hash)
	l.h.Write(b[:])
}

// begin starts a drawable's record.
func (l *drawList) begin() {
	l.open, l.start, l.box = true, len(l.cmds), image.Rectangle{}
	l.h.Reset()
}

// end closes the inked begun last.
func (l *drawList) end() inked {
	l.open = false
	return inked{from: l.start, to: len(l.cmds), box: l.box, hash: l.h.Sum64()}
}

func (l *drawList) records(dst *ebiten.Image) bool { return dst == l.dry }

// newFade starts a fade group and records into it.
func (l *drawList) newFade() {
	l.fades++
	l.fade = l.fades
}

// moves says the drawable recorded now moves this frame, while the
// dry pass is recording: a live building, a spraying fountain.
func (l *drawList) moves() {
	if l != nil && l.open {
		l.moving = true
	}
}

func (l *drawList) push(c drawCmd, box image.Rectangle) {
	c.fade = l.fade
	c.box = box
	l.cmds = append(l.cmds, c)
	l.ch.Reset()
	l.box = l.box.Union(box)
	l.hashInt(uint64(c.kind))
	l.hashInt(uint64(c.fade))
}

// Image draws img onto dst, or records it.
func (l *drawList) Image(dst, img *ebiten.Image, op *ebiten.DrawImageOptions) {
	if !l.records(dst) {
		dst.DrawImage(img, op)
		return
	}
	l.push(drawCmd{kind: cmdImage, src: img, img: *op}, imageBox(img, op.GeoM))
	l.hashImage(img)
	l.hashGeoM(op.GeoM)
	l.hashScale(op.ColorScale)
	l.hashInt(uint64(op.Filter), uint64(blendBits(op.Blend)))
	l.seal()
}

// colorm draws img through a colour matrix, or records it.
func (l *drawList) colorm(dst, img *ebiten.Image, m colorm.ColorM, op *colorm.DrawImageOptions) {
	if !l.records(dst) {
		colorm.DrawImage(dst, img, m, op)
		return
	}
	l.push(drawCmd{kind: cmdColorM, src: img, cm: m, cmImg: *op}, imageBox(img, op.GeoM))
	l.hashImage(img)
	l.hashGeoM(op.GeoM)
	for i := range 4 {
		for j := range 5 {
			l.hashInt(math.Float64bits(m.Element(i, j)))
		}
	}
	l.hashInt(uint64(op.Filter), uint64(blendBits(op.Blend)))
	l.seal()
}

// Mesh draws triangles, or records them.
func (l *drawList) Mesh(dst *ebiten.Image, vertices []ebiten.Vertex, indices []uint32, src *ebiten.Image, op *ebiten.DrawTrianglesOptions, key shapes.MeshKey) {
	if !l.records(dst) {
		dst.DrawTriangles32(vertices, indices, src, op)
		return
	}
	c := drawCmd{kind: cmdMesh, src: src, tri: *op}
	c.v = [2]int{len(l.vertices), len(l.vertices) + len(vertices)}
	c.i = [2]int{len(l.indices), len(l.indices) + len(indices)}
	l.vertices = append(l.vertices, vertices...)
	l.indices = append(l.indices, indices...)
	l.push(c, vertexBox(vertices))
	l.hashImage(src)
	if key.Values != nil {
		// What the painter was asked for, not its float32 vertices.
		for _, p := range key.Points {
			l.hashInt(relQ(p.X, l.origin.X), relQ(p.Y, l.origin.Y))
		}
		for _, v := range key.Values {
			l.hashInt(math.Float64bits(v))
		}
		l.hashInt(uint64(len(indices)))
		l.hashInt(uint64(op.Filter), uint64(blendBits(op.Blend)))
		l.seal()
		return
	}
	l.hashVertices(vertices)
	if len(indices) > 0 {
		l.ch.Write(unsafe.Slice((*byte)(unsafe.Pointer(&indices[0])), len(indices)*4))
	}
	l.hashInt(uint64(op.Filter), uint64(blendBits(op.Blend)))
	l.seal()
}

// triangles16 is Mesh for 16-bit indices.
func (l *drawList) triangles16(dst *ebiten.Image, vertices []ebiten.Vertex, indices []uint16, src *ebiten.Image, op *ebiten.DrawTrianglesOptions) {
	if !l.records(dst) {
		dst.DrawTriangles(vertices, indices, src, op)
		return
	}
	wide := make([]uint32, len(indices))
	for i, x := range indices {
		wide[i] = uint32(x)
	}
	l.Mesh(dst, vertices, wide, src, op, shapes.MeshKey{})
}

// Ring fills a ring by the non-zero rule, or records it.
func (l *drawList) Ring(dst *ebiten.Image, pts []city.Point, op *vector.DrawPathOptions) {
	if !l.records(dst) {
		l.painter.FillPath(dst, pts, op)
		return
	}
	c := drawCmd{kind: cmdRing, path: *op}
	c.p = [2]int{len(l.points), len(l.points) + len(pts)}
	l.points = append(l.points, pts...)
	box := image.Rectangle{}
	for i, p := range pts {
		b := pixelBox(p.X, p.Y, p.X, p.Y)
		if i == 0 {
			box = b
		} else {
			box = box.Union(b)
		}
	}
	l.push(c, box)
	for _, p := range pts {
		l.hashInt(relQ(p.X, l.origin.X), relQ(p.Y, l.origin.Y))
	}
	l.hashScale(op.ColorScale)
	l.hashInt(uint64(blendBits(op.Blend)))
	l.seal()
}

// words draws text, or records it.
func (l *drawList) words(dst *ebiten.Image, s string, face text.Face, op *text.DrawOptions) {
	if !l.records(dst) {
		text.Draw(dst, s, face, op)
		return
	}
	l.push(drawCmd{kind: cmdWords, words: s, face: face, text: *op}, wordsBox(s, face, op))
	l.ch.WriteString(s)
	l.hashInt(uint64(uintptr(faceID(face))))
	l.hashGeoM(op.GeoM)
	l.hashScale(op.ColorScale)
	l.hashInt(uint64(op.PrimaryAlign), uint64(op.SecondaryAlign), uint64(blendBits(op.Blend)))
	l.hashFloat(float32(op.LineSpacing))
	l.seal()
}

// everywhere is the box of a draw that covers whatever it is drawn on.
var everywhere = image.Rect(-1<<24, -1<<24, 1<<24, 1<<24)

// fill fills dst with a colour, or records it.
func (l *drawList) fill(dst *ebiten.Image, c color.Color) {
	if !l.records(dst) {
		dst.Fill(c)
		return
	}
	l.push(drawCmd{kind: cmdFill, fill: c}, everywhere)
	r, g, b, a := c.RGBA()
	l.hashInt(uint64(r)<<48 | uint64(g)<<32 | uint64(b)<<16 | uint64(a))
	l.seal()
}

// play draws the commands of a drawable onto dst, sending each fade group's
// to layer, put down at alpha when the group ends.
func (l *drawList) play(dst *ebiten.Image, s inked, f *fadePlay) {
	for _, c := range l.cmds[s.from:s.to] {
		target := dst
		if f != nil {
			target = f.target(dst, c.fade)
		}
		l.playCmd(target, &c)
	}
}

// playIn plays the commands of a drawable whose boxes meet r onto dst,
// which is clipped to r: the rest would draw nothing there.
func (l *drawList) playIn(dst *ebiten.Image, s inked, f *fadePlay, r image.Rectangle) {
	for _, c := range l.cmds[s.from:s.to] {
		if !c.box.Overlaps(r) {
			continue
		}
		target := dst
		if f != nil {
			target = f.target(dst, c.fade)
		}
		l.playCmd(target, &c)
	}
}

// cmdBox is the box the ith recorded command inks.
func (l *drawList) cmdBox(i int) image.Rectangle { return l.cmds[i].box }

func (l *drawList) playCmd(dst *ebiten.Image, c *drawCmd) {
	switch c.kind {
	case cmdImage:
		dst.DrawImage(c.src, &c.img)
	case cmdColorM:
		colorm.DrawImage(dst, c.src, c.cm, &c.cmImg)
	case cmdMesh:
		dst.DrawTriangles32(l.vertices[c.v[0]:c.v[1]], l.indices[c.i[0]:c.i[1]], c.src, &c.tri)
	case cmdRing:
		l.painter.FillPath(dst, l.points[c.p[0]:c.p[1]], &c.path)
	case cmdWords:
		text.Draw(dst, c.words, c.face, &c.text)
	case cmdFill:
		dst.Fill(c.fill)
	}
}

// imageBox is the pixels an image drawn through geo covers.
func imageBox(img *ebiten.Image, geo ebiten.GeoM) image.Rectangle {
	b := img.Bounds()
	return geoBox(geo, 0, 0, float64(b.Dx()), float64(b.Dy()))
}

func geoBox(geo ebiten.GeoM, x0, y0, x1, y1 float64) image.Rectangle {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, c := range [4][2]float64{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
		x, y := geo.Apply(c[0], c[1])
		minX, minY, maxX, maxY = math.Min(minX, x), math.Min(minY, y), math.Max(maxX, x), math.Max(maxY, y)
	}
	return pixelBox(minX, minY, maxX, maxY)
}

func vertexBox(vertices []ebiten.Vertex) image.Rectangle {
	if len(vertices) == 0 {
		return image.Rectangle{}
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, v := range vertices {
		x, y := float64(v.DstX), float64(v.DstY)
		minX, minY, maxX, maxY = math.Min(minX, x), math.Min(minY, y), math.Max(maxX, x), math.Max(maxY, y)
	}
	return pixelBox(minX, minY, maxX, maxY)
}

// wordsBox is the box text drawn with op covers, its alignment taken in.
func wordsBox(s string, face text.Face, op *text.DrawOptions) image.Rectangle {
	w, h := text.Measure(s, face, op.LineSpacing)
	x0, y0 := 0.0, 0.0
	switch op.PrimaryAlign {
	case text.AlignCenter:
		x0 = -w / 2
	case text.AlignEnd:
		x0 = -w
	}
	switch op.SecondaryAlign {
	case text.AlignCenter:
		y0 = -h / 2
	case text.AlignEnd:
		y0 = -h
	}
	return geoBox(op.GeoM, x0, y0, x0+w, y0+h)
}

// pixelBox rounds a box out to whole pixels and pads it.
func pixelBox(minX, minY, maxX, maxY float64) image.Rectangle {
	if math.IsNaN(minX+minY+maxX+maxY) || math.IsInf(minX+minY+maxX+maxY, 0) {
		return image.Rectangle{}
	}
	// An edge within a hair of a whole pixel is on it, so rounding in
	// the last bits of a transform does not move the box as the camera
	// does (item 193); the pad covers the hair.
	const hair = 1e-6
	return image.Rect(int(math.Floor(minX+hair))-inkPad, int(math.Floor(minY+hair))-inkPad, int(math.Ceil(maxX-hair))+inkPad, int(math.Ceil(maxY-hair))+inkPad)
}

func (l *drawList) hashInt(vs ...uint64) {
	var b [8]byte
	for _, v := range vs {
		binary.LittleEndian.PutUint64(b[:], v)
		l.ch.Write(b[:])
	}
}

func (l *drawList) hashFloat(f float32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], math.Float32bits(f))
	l.ch.Write(b[:])
}

// hashGeoM hashes a transform, its translation relative to the origin and
// its scale to 2^-24: worked out from projected points, a scale comes out
// a few bits apart as the camera moves, which draws nothing differently.
func (l *drawList) hashGeoM(g ebiten.GeoM) {
	for i := range 2 {
		for j := range 2 {
			l.hashInt(uint64(int64(math.Round(g.Element(i, j) * (1 << 24)))))
		}
	}
	l.hashInt(relQ(g.Element(0, 2), l.origin.X), relQ(g.Element(1, 2), l.origin.Y))
}

// hashVertices hashes vertices, their positions relative to the origin.
func (l *drawList) hashVertices(vertices []ebiten.Vertex) {
	for _, v := range vertices {
		l.hashInt(relQ(float64(v.DstX), l.origin.X), relQ(float64(v.DstY), l.origin.Y))
		l.hashFloat(v.SrcX)
		l.hashFloat(v.SrcY)
		l.hashFloat(v.ColorR)
		l.hashFloat(v.ColorG)
		l.hashFloat(v.ColorB)
		l.hashFloat(v.ColorA)
	}
}

func (l *drawList) hashScale(s ebiten.ColorScale) {
	l.hashFloat(s.R())
	l.hashFloat(s.G())
	l.hashFloat(s.B())
	l.hashFloat(s.A())
}

// hashImage hashes which image a draw reads. Ebitengine keeps each
// sub-image it cuts while it is in use, so the same sprite is the same
// value from frame to frame.
func (l *drawList) hashImage(img *ebiten.Image) {
	b := img.Bounds()
	l.hashInt(uint64(uintptr(unsafe.Pointer(img))), uint64(b.Min.X), uint64(b.Min.Y), uint64(b.Max.X), uint64(b.Max.Y))
}

func faceID(face text.Face) unsafe.Pointer {
	if face == nil {
		return nil
	}
	return reflect.ValueOf(face).UnsafePointer()
}

func blendBits(b ebiten.Blend) uint64 {
	return uint64(b.BlendFactorSourceRGB) | uint64(b.BlendFactorSourceAlpha)<<8 | uint64(b.BlendFactorDestinationRGB)<<16 |
		uint64(b.BlendFactorDestinationAlpha)<<24 | uint64(b.BlendOperationRGB)<<32 | uint64(b.BlendOperationAlpha)<<40
}

// recorder is the game's draw list, made the first time it is asked for
// and given to the painter, so every draw of the world goes through it.
func (g *Game) recorder() *drawList {
	if g.rec == nil {
		g.rec = newDrawList()
		g.paint.Record = g.rec
	}
	return g.rec
}
