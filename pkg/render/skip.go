package render

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// How full a live session's context is stands at the kerb beside its
// building as a skip, and a compaction sends a skip truck for it and a
// cement mixer to pour the new course. Item 109.

// skipInset is how far in from the lot's edge the skip stands, as a
// share of the lot.
const skipInset = 0.03

// skipPiece is the skip for a fill level, or none.
func skipPiece(level int) string {
	if level < 1 || level > len(kitSkips) {
		return ""
	}
	return kitSkips[level-1]
}

// shownSkip is the skip at a building's kerb now: while a job collects
// it, the one that stood there before the compaction until the truck has
// hooked it, then none until the job is done; otherwise the session's
// own fill.
func shownSkip(b *city.Building, job city.Job, collecting bool) int {
	if b.Session.ConversationGone {
		return 0
	}
	if collecting {
		if job.SkipAtKerb {
			return job.SkipLevel
		}
		return 0
	}
	return city.SkipLevel(b.Session)
}

// lotCorner is the corner of a building's lot nearest the viewer, inset,
// in world units, and the lot corners either side of it.
func lotCorner(cam *city.Camera, b *city.Building) (city.Point, [2]city.Point) {
	r := b.Rect
	in := r.Width() * skipInset
	corners := []city.Point{
		{X: r.Min.X + in, Y: r.Min.Y + in}, {X: r.Max.X - in, Y: r.Min.Y + in},
		{X: r.Max.X - in, Y: r.Max.Y - in}, {X: r.Min.X + in, Y: r.Max.Y - in},
	}
	best := 0
	for i, c := range corners {
		if cam.WorldToScreen(c).Y > cam.WorldToScreen(corners[best]).Y {
			best = i
		}
	}
	return corners[best], [2]city.Point{corners[(best+3)%4], corners[(best+1)%4]}
}

// skipAt is where a building's skip stands: at the kerb, at its lot's
// front corner.
func (g *Game) skipAt(cam *city.Camera, b *city.Building) city.Point {
	at, _ := lotCorner(cam, b)
	return at
}

// kerbPath is the way the job's trucks come and go: in along the lot's
// front edge, stopping at the skip and, for the mixer, beside the
// building, and on out the far end.
func (g *Game) kerbPath(cam *city.Camera, b *city.Building) (from, skip, pour, to city.Point) {
	corner, sides := lotCorner(cam, b)
	far := sides[0]
	if cam.WorldToScreen(sides[1]).X < cam.WorldToScreen(far).X {
		far = sides[1]
	}
	dir := far.Sub(corner)
	span := b.Rect.Width()
	unit := dir.Scale(1 / max(1e-9, math.Hypot(dir.X, dir.Y)))
	return corner.Sub(unit.Scale(span)), corner, corner.Add(dir.Scale(0.5)), far.Add(unit.Scale(span))
}

// alongPath is a truck Along of the way from in through its stop to out.
func alongPath(from, stop, to city.Point, along float64) (city.Point, city.Point) {
	if along <= 1 {
		return lerp(from, stop, along), stop.Sub(from)
	}
	return lerp(stop, to, along-1), to.Sub(stop)
}

// drawSkip draws the skip at a building's kerb and the trucks of any
// compaction job there.
func (g *Game) drawSkip(screen *ebiten.Image, cam *city.Camera, b *city.Building, tint *ebiten.ColorScale) {
	if b.Packed || b.Vacant {
		return
	}
	job, collecting := g.scene.CompactionJob(b.Session.ID)
	if piece := skipPiece(shownSkip(b, job, collecting)); piece != "" {
		g.kit(screen, cam, piece, 0, g.skipAt(cam, b), tint)
	}
	if !collecting {
		return
	}
	from, skip, pour, to := g.kerbPath(cam, b)
	if t := job.SkipTruck; t.Present {
		at, dir := alongPath(from, skip, to, t.Along)
		piece := kitSkipTruck
		if t.Loaded {
			piece = kitSkipTruckLoaded
		}
		g.kit(screen, cam, piece, carTurn(dir), at, tint)
	}
	if t := job.Mixer; t.Present {
		at, dir := alongPath(from, pour, to, t.Along)
		g.kit(screen, cam, kitMixer, carTurn(dir), at, tint)
	}
}
