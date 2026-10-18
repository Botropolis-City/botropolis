package render

import (
	"image/color"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Each compaction slides a course under its session's building, a slab
// of the building's own material, so a building raised on courses has
// been compacted that often. Three are drawn, stepping in as they rise
// like steps up to its door; past that the bottom one carries the count.
// Item 109.

const maxCourses = 3

// courseGlow is how long the newest course stays lit after the
// compaction that laid it.
const courseGlow = 45 * time.Second

var (
	colorPlaque     = color.NRGBA{0x1d, 0x20, 0x26, 0xff}
	colorPlaqueFace = color.NRGBA{0xf2, 0xea, 0xd8, 0xff}
)

// courseCount is how many courses a session's compactions draw.
func courseCount(compactions int) int { return min(max(compactions, 0), maxCourses) }

// coursePlaque is the count the bottom course shows, once there are more
// compactions than courses drawn.
func coursePlaque(compactions int) string {
	if compactions <= maxCourses {
		return ""
	}
	return strconv.Itoa(compactions)
}

// courseHeight is how tall a course stands on screen: a sixth of a
// storey, as tools/render-sprites/buildings.py cuts it.
func courseHeight(zoom float64) float64 { return storeyHeight(zoom) / 6 }

// coursePiece is the course cut for a building's piece, step k from the
// top: 1 just under the building, each further one a step wider.
func coursePiece(name string, k int) string {
	steps, ok := kindCourses[name]
	if !ok || k < 1 || k > maxCourses {
		return ""
	}
	return steps[k-1]
}

// courseLit is whether a session's newest course is still lit.
func courseLit(s state.Session, now time.Time) bool {
	return s.Compactions > 0 && !s.LastCompactionAt.IsZero() && now.Sub(s.LastCompactionAt) < courseGlow
}

// courseLift is how far a building's courses raise it on screen; while a
// course is being poured the building rises with it.
func (g *Game) courseLift(cam *city.Camera, b *city.Building) float64 {
	if b.Packed || b.Vacant {
		return 0
	}
	n := float64(courseCount(b.Session.Compactions))
	if pour, ok := g.pouring(b); ok {
		n += pour - 1
	}
	return n * courseHeight(cam.Zoom)
}

// pouring is how far a building's newest course has been poured, while
// a compaction job is pouring one that adds to those drawn.
func (g *Game) pouring(b *city.Building) (float64, bool) {
	if g.scene == nil || b.Session.Compactions > maxCourses || b.Session.Compactions < 1 {
		return 0, false
	}
	job, ok := g.scene.CompactionJob(b.Session.ID)
	if !ok || job.Pour >= 1 {
		return 0, false
	}
	return job.Pour, true
}

// footOf is where a building stands on screen: its lot's centre, raised
// by its courses. Everything drawn on the building is drawn from here.
func (g *Game) footOf(cam *city.Camera, b *city.Building) city.Point {
	foot := cam.WorldToScreen(b.Rect.Center())
	foot.Y -= g.courseLift(cam, b)
	return foot
}

// drawCourses draws a building's courses from the ground up, the newest
// on top and lit while it is new, and the count on the bottom one when
// there are more compactions than courses.
func (g *Game) drawCourses(screen *ebiten.Image, cam *city.Camera, b *city.Building, tint *ebiten.ColorScale) {
	n := courseCount(b.Session.Compactions)
	if n == 0 || b.Packed || b.Vacant {
		return
	}
	name := buildingPiece(b)
	ground := cam.WorldToScreen(b.Rect.Center())
	h := courseHeight(cam.Zoom)
	pour, pouring := g.pouring(b)
	var bottom city.Rect
	for i := range n {
		at := city.Point{X: ground.X, Y: ground.Y - float64(i)*h}
		t := tint
		if i == n-1 && courseLit(b.Session, g.scene.Now()) {
			t = litCourse(tint)
		}
		if i == n-1 && pouring {
			if pour <= 0 {
				continue
			}
			faded := &ebiten.ColorScale{}
			if t != nil {
				*faded = *t
			}
			faded.ScaleAlpha(float32(pour))
			t = faded
		}
		r := g.kitAt(screen, cam, coursePiece(name, n-i), 0, at, t)
		if i == 0 {
			bottom = r
		}
	}
	if label := coursePlaque(b.Session.Compactions); label != "" && bottom.Area() > 0 {
		w, th := g.measure(label)
		pad := max(2, th*0.2)
		centre := city.Point{X: (bottom.Min.X + ground.X) / 2, Y: ground.Y - h*0.5}
		plate := city.RectAt(centre.X-w/2-pad, centre.Y-th/2-pad*0.5, w+2*pad, th+pad)
		vector.FillRect(screen, float32(plate.Min.X), float32(plate.Min.Y), float32(plate.Width()), float32(plate.Height()), colorPlaqueFace, false)
		vector.StrokeRect(screen, float32(plate.Min.X), float32(plate.Min.Y), float32(plate.Width()), float32(plate.Height()), 1, colorPlaque, false)
		g.text(screen, city.Point{X: plate.Min.X + pad, Y: plate.Min.Y + pad*0.5}, label, ui.Small, colorPlaque)
	}
}

// litCourse warms a course the way the board's mock-ups light a new one.
func litCourse(tint *ebiten.ColorScale) *ebiten.ColorScale {
	out := &ebiten.ColorScale{}
	if tint != nil {
		*out = *tint
	}
	out.Scale(1.25, 1.12, 0.7, 1)
	return out
}
