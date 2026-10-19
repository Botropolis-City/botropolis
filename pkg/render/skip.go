package render

import (
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

// jobPaths is a truck's way in to a stop at a building and back out:
// off the street by the lot's driveway, along the strip inside the
// lot's front to the service lane on the stop's side of the building,
// up the lane, and across to the stop; out the same way. Item 108.
func (g *Game) jobPaths(c *city.City, b *city.Building, stop city.Point) (in, out []city.Point) {
	d := c.DistrictOf(b)
	dw, ok := c.Driveway(d)
	if !ok {
		return []city.Point{stop}, []city.Point{stop}
	}
	street, lot := dw.Cell.Center(), dw.Lot
	r, gap := b.Rect, city.BuildingGap/2
	in = []city.Point{street, lot}
	if lot.X == street.X {
		lane := r.Max.X + gap
		if stop.X < r.Center().X {
			lane = r.Min.X - gap
		}
		in = append(in, city.Point{X: lane, Y: lot.Y}, city.Point{X: lane, Y: stop.Y})
	} else {
		lane := r.Max.Y + gap
		if stop.Y < r.Center().Y {
			lane = r.Min.Y - gap
		}
		in = append(in, city.Point{X: lot.X, Y: lane}, city.Point{X: stop.X, Y: lane})
	}
	in = append(in, stop)
	out = make([]city.Point, len(in))
	for i, p := range in {
		out[len(in)-1-i] = p
	}
	return in, out
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
	skip, sides := lotCorner(cam, b)
	pour := skip.Add(sides[0].Sub(skip).Scale(0.5))
	c := g.scene.City()
	if t := job.SkipTruck; t.Present {
		in, out := g.jobPaths(c, b, skip)
		at, dir := jobPlace(in, out, t.Along)
		piece := kitSkipTruck
		if t.Loaded {
			piece = kitSkipTruckLoaded
		}
		g.kit(screen, cam, piece, vehicleTurn(dir), at, tint)
	}
	if t := job.Mixer; t.Present {
		in, out := g.jobPaths(c, b, pour)
		at, dir := jobPlace(in, out, t.Along)
		g.kit(screen, cam, kitMixer, vehicleTurn(dir), at, tint)
	}
}
