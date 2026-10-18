package render

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// The City Plan, drawn: a flat sheet with a faint grid, the groups' lines
// in ink, a badge per district and a legend along the foot. Item 106.

var (
	colorPlanPaper = color.NRGBA{0x17, 0x1b, 0x24, 0xff}
	colorPlanGrid  = color.NRGBA{0xff, 0xff, 0xff, 0x0c}
	colorPlanInk   = color.NRGBA{0xc8, 0xcf, 0xdb, 0xb0}
	colorPlanTrack = color.NRGBA{0xff, 0xff, 0xff, 0x22}
	colorPlanLot   = color.NRGBA{0xc8, 0xcf, 0xdb, 0x40}
)

const planGrid = 48.0

// planHits are the badges, for the pointer: each stands for its district.
func planHits(p city.CityPlan) []spriteHit {
	out := make([]spriteHit, 0, len(p.Badges))
	for _, b := range p.Badges {
		out = append(out, spriteHit{rect: b.Rect, hit: city.Hit{District: b.District}})
	}
	return out
}

// planCounts is a badge's last line: its sessions, how many need you,
// and the subagents in flight.
func planCounts(b city.PlanBadge) string {
	s := fmt.Sprintf("%d session", b.Sessions)
	if b.Sessions != 1 {
		s += "s"
	}
	if b.NeedsYou > 0 {
		s += fmt.Sprintf(" · %d need you", b.NeedsYou)
	}
	if b.Subagents > 0 {
		s += fmt.Sprintf(" · %d agent", b.Subagents)
		if b.Subagents != 1 {
			s += "s"
		}
	}
	return s
}

// drawCityPlan draws the plan over whatever is on screen, at the
// cross-fade's alpha.
func (g *Game) drawCityPlan(screen *ebiten.Image, width, height, alpha float64) {
	if alpha <= 0 {
		return
	}
	target := screen
	if alpha < 1 {
		w, h := int(math.Ceil(width)), int(math.Ceil(height))
		if g.planLayer == nil || g.planLayer.Bounds().Dx() != w || g.planLayer.Bounds().Dy() != h {
			if g.planLayer != nil {
				g.planLayer.Deallocate()
			}
			g.planLayer = ebiten.NewImage(w, h)
		}
		g.planLayer.Clear()
		target = g.planLayer
	}
	plan := g.scene.CityPlan()
	g.drawPlanSheet(target, width, height)
	for _, b := range plan.Badges {
		for i := range b.Lot {
			a, c := b.Lot[i], b.Lot[(i+1)%len(b.Lot)]
			vector.StrokeLine(target, float32(a.X), float32(a.Y), float32(c.X), float32(c.Y), 1, colorPlanLot, true)
		}
	}
	for _, l := range plan.Lines {
		for i := 1; i < len(l.Points); i++ {
			a, b := l.Points[i-1], l.Points[i]
			vector.StrokeLine(target, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y), 3, colorPlanInk, true)
		}
	}
	for _, b := range plan.Badges {
		g.drawPlanBadge(target, b)
	}
	g.drawPlanLegend(target, plan)
	g.mu.Lock()
	g.frameHits = append(g.frameHits[:0], planHits(plan)...)
	g.hits = append(g.hits[:0], g.frameHits...)
	g.mu.Unlock()
	if target != screen {
		op := &ebiten.DrawImageOptions{}
		op.ColorScale.ScaleAlpha(float32(alpha))
		screen.DrawImage(target, op)
	}
}

// drawPlanSheet is the paper and its grid.
func (g *Game) drawPlanSheet(screen *ebiten.Image, width, height float64) {
	vector.FillRect(screen, 0, 0, float32(width), float32(height), colorPlanPaper, false)
	for x := math.Mod(width/2, planGrid); x < width; x += planGrid {
		vector.FillRect(screen, float32(x), 0, 1, float32(height), colorPlanGrid, false)
	}
	for y := math.Mod(height/2, planGrid); y < height; y += planGrid {
		vector.FillRect(screen, 0, float32(y), float32(width), 1, colorPlanGrid, false)
	}
}

// drawPlanBadge is one district: a flat card edged in the tone of whose
// move it is, with its name, the move, and its counts.
func (g *Game) drawPlanBadge(screen *ebiten.Image, b city.PlanBadge) {
	th := g.theme
	tone := th.Color(ui.MoveTone(b.Tone))
	r := b.Rect
	g.roundRect(screen, r, th.Radius()/2, colorPlanPaper)
	g.roundRect(screen, r, th.Radius()/2, th.Palette.Panel)
	edge := city.RectAt(r.Min.X, r.Min.Y, 4, r.Height())
	vector.FillRect(screen, float32(edge.Min.X), float32(edge.Min.Y), float32(edge.Width()), float32(edge.Height()), tone, false)
	pad := th.Grid() / 2
	x := r.Min.X + 4 + pad
	room := r.Max.X - pad - x
	y := r.Min.Y + pad/2
	lines := []struct {
		text string
		size ui.Size
		c    color.NRGBA
	}{
		{b.Name, ui.Body, th.Palette.Text},
		{b.Move, ui.Small, tone},
		{planCounts(b), ui.Small, th.Palette.Dim},
	}
	for _, l := range lines {
		if l.text == "" {
			continue
		}
		text := g.fitText(l.text, l.size, room)
		_, h := g.faces.Measure(text, l.size)
		g.text(screen, city.Point{X: x, Y: y}, text, l.size, l.c)
		y += h
	}
}

// fitText shortens a string with an ellipsis until it fits a width.
func (g *Game) fitText(s string, size ui.Size, room float64) string {
	if w, _ := g.faces.Measure(s, size); w <= room {
		return s
	}
	r := []rune(s)
	for len(r) > 1 {
		r = r[:len(r)-1]
		t := string(r) + "…"
		if w, _ := g.faces.Measure(t, size); w <= room {
			return t
		}
	}
	return s
}

// drawPlanLegend is the strip along the foot: the moves the badges carry,
// and each usage reading as a title, a bar and a percent.
func (g *Game) drawPlanLegend(screen *ebiten.Image, plan city.CityPlan) {
	th := g.theme
	r := plan.Legend
	if r.Area() == 0 {
		return
	}
	pad := th.Grid() / 2
	x := r.Min.X + 2*pad
	mid := r.Min.Y + r.Height()/2
	for _, m := range plan.Tones {
		label := city.MovePhrase(m)
		_, h := g.faces.Measure(label, ui.Small)
		vector.FillCircle(screen, float32(x+5), float32(mid), 5, th.Color(ui.MoveTone(m)), true)
		g.text(screen, city.Point{X: x + 16, Y: mid - h/2}, label, ui.Small, th.Palette.Text)
		w, _ := g.faces.Measure(label, ui.Small)
		x += 16 + w + 3*pad
	}
	const barW, barH = 96.0, 6.0
	if len(plan.Readings) > 0 && len(plan.Tones) > 0 {
		vector.FillRect(screen, float32(x), float32(mid-10), 1, 20, th.Palette.Hairline, false)
		x += 3 * pad
	}
	for _, rd := range plan.Readings {
		pct := fmt.Sprintf("%.0f%%", rd.Percent)
		tw, th1 := g.faces.Measure(rd.Title, ui.Small)
		g.text(screen, city.Point{X: x, Y: mid - th1 - 1}, rd.Title, ui.Small, th.Palette.Dim)
		bar := city.RectAt(x, mid+3, barW, barH)
		vector.FillRect(screen, float32(bar.Min.X), float32(bar.Min.Y), float32(barW), float32(barH), colorPlanTrack, false)
		fill := ui.Sequential.At(math.Min(1, math.Max(0, rd.Percent/100)))
		vector.FillRect(screen, float32(bar.Min.X), float32(bar.Min.Y), float32(barW*math.Min(1, math.Max(0, rd.Percent/100))), float32(barH), fill, false)
		pw, ph := g.faces.Measure(pct, ui.Small)
		g.text(screen, city.Point{X: x + barW + pad/2, Y: bar.Min.Y + barH/2 - ph/2}, pct, ui.Small, th.Palette.Text)
		x += math.Max(tw, barW+pad/2+pw) + 4*pad
	}
}
