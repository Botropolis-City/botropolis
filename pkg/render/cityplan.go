package render

import (
	"fmt"
	"image/color"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// The City Plan, drawn: a flat sheet with a faint grid, the groups' lines
// in ink, a badge per district and a legend along the foot. Item 106.
// The island stands in a wash of water inside an inked coast, each lot
// is tinted faintly by whose move it is, and the plaza is a civic mark.
// Item 112.

var (
	colorPlanPaper = color.NRGBA{0x17, 0x1b, 0x24, 0xff}
	colorPlanGrid  = color.NRGBA{0xff, 0xff, 0xff, 0x0c}
	colorPlanInk   = color.NRGBA{0xc8, 0xcf, 0xdb, 0xb0}
	colorPlanTrack = color.NRGBA{0xff, 0xff, 0xff, 0x22}
	colorPlanLot   = color.NRGBA{0xc8, 0xcf, 0xdb, 0x40}
	colorPlanWater = color.NRGBA{0x1d, 0x2b, 0x3c, 0xff}
	colorPlanCoast = color.NRGBA{0xc8, 0xcf, 0xdb, 0x90}
	colorPlanQuiet = color.NRGBA{0xc8, 0xcf, 0xdb, 0x48}
)

// planDashOn and planDashOff are traffic's dash and the gap after it.
const planDashOn, planDashOff = 6.0, 5.0

// planLotAlpha is how strongly a lot is tinted by whose move it is.
const planLotAlpha = 0x22

// planLotTint is a lot's fill: whose move it is, faintly.
func planLotTint(th ui.Theme, m state.Move) color.NRGBA {
	c := th.Color(ui.MoveTone(m))
	c.A = planLotAlpha
	return c
}

// ring is a closed path through points.
func ring(points []city.Point) *vector.Path {
	var path vector.Path
	for i, p := range points {
		if i == 0 {
			path.MoveTo(float32(p.X), float32(p.Y))
		} else {
			path.LineTo(float32(p.X), float32(p.Y))
		}
	}
	path.Close()
	return &path
}

// fillRing fills a closed ring of points in a colour.
func fillRing(screen *ebiten.Image, points []city.Point, c color.NRGBA) {
	if len(points) < 3 {
		return
	}
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.FillPath(screen, ring(points), nil, op)
}

// strokeRing inks a closed ring of points.
func strokeRing(screen *ebiten.Image, points []city.Point, width float32, c color.NRGBA) {
	if len(points) < 2 {
		return
	}
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.StrokePath(screen, ring(points), &vector.StrokeOptions{Width: width, LineJoin: vector.LineJoinRound}, op)
}

// planDashes cuts a path into dashes of on pixels with off pixels
// between them, from its first point.
func planDashes(path []city.Point, on, off float64) [][]city.Point {
	if len(path) < 2 || on <= 0 {
		return nil
	}
	var out [][]city.Point
	inking, left := true, on
	dash := []city.Point{path[0]}
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		seg := math.Hypot(b.X-a.X, b.Y-a.Y)
		at := 0.0
		for seg-at > left {
			at += left
			p := a.Add(b.Sub(a).Scale(at / seg))
			if inking {
				out = append(out, append(dash, p))
				left = off
			} else {
				dash = []city.Point{p}
				left = on
			}
			inking = !inking
		}
		left -= seg - at
		if inking {
			dash = append(dash, b)
		}
	}
	if inking && len(dash) > 1 {
		out = append(out, dash)
	}
	return out
}

// drawPlanTraffic is each road between districts in dashed ink: full
// while travellers work on it, faint while it is quiet.
func drawPlanTraffic(screen *ebiten.Image, traffic []city.PlanTraffic) {
	for _, tr := range traffic {
		c := colorPlanQuiet
		if tr.Active {
			c = colorPlanInk
		}
		for _, d := range planDashes(tr.Points, planDashOn, planDashOff) {
			for i := 1; i < len(d); i++ {
				vector.StrokeLine(screen, float32(d[i-1].X), float32(d[i-1].Y), float32(d[i].X), float32(d[i].Y), 1.5, c, true)
			}
		}
	}
}

// drawPlanCoast is the water beyond the island, the island's land, and
// its coast in ink.
func drawPlanCoast(screen *ebiten.Image, width, height float64, coast []city.Point) {
	if len(coast) < 3 {
		return
	}
	vector.FillRect(screen, 0, 0, float32(width), float32(height), colorPlanWater, false)
	fillRing(screen, coast, colorPlanPaper)
	strokeRing(screen, coast, 1.5, colorPlanCoast)
}

// drawPlanPlaza is the plaza as a small civic mark: a portico of three
// columns under a pediment, on a disc of paper.
func drawPlanPlaza(screen *ebiten.Image, r city.Rect) {
	if r.Area() == 0 {
		return
	}
	c := r.Center()
	u := r.Width() / 30
	vector.FillCircle(screen, float32(c.X), float32(c.Y), float32(14*u), colorPlanPaper, true)
	vector.StrokeCircle(screen, float32(c.X), float32(c.Y), float32(14*u), float32(1.2), colorPlanInk, true)
	at := func(dx, dy float64) city.Point { return city.Point{X: c.X + dx*u, Y: c.Y + dy*u} }
	fillRing(screen, []city.Point{at(-8, -3), at(0, -8), at(8, -3)}, colorPlanInk)
	for _, dx := range []float64{-5.5, 0, 5.5} {
		a, b := at(dx, -2), at(dx, 5)
		vector.StrokeLine(screen, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y), float32(1.6*u), colorPlanInk, true)
	}
	a, b := at(-8.5, 6.5), at(8.5, 6.5)
	vector.StrokeLine(screen, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y), float32(1.6*u), colorPlanInk, true)
}

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
		s += " · " + format.Count(b.NeedsYou, "needs you", "need you")
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
	g.scene.SetPlanCovers(g.planCovers(width, height))
	plan := g.scene.CityPlan()
	vector.FillRect(target, 0, 0, float32(width), float32(height), colorPlanPaper, false)
	drawPlanCoast(target, width, height, plan.Coast)
	g.drawPlanGrid(target, width, height)
	for _, b := range plan.Badges {
		fillRing(target, b.Lot[:], planLotTint(g.theme, b.Tone))
	}
	drawPlanTraffic(target, plan.Traffic)
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
	drawPlanPlaza(target, plan.Plaza)
	g.drawPlanCompass(target, plan.Compass)
	g.drawPlanCartouche(target, plan.Cartouche)
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

// drawPlanGrid is the sheet's faint grid.
func (g *Game) drawPlanGrid(screen *ebiten.Image, width, height float64) {
	for x := math.Mod(width/2, planGrid); x < width; x += planGrid {
		vector.FillRect(screen, float32(x), 0, 1, float32(height), colorPlanGrid, false)
	}
	for y := math.Mod(height/2, planGrid); y < height; y += planGrid {
		vector.FillRect(screen, 0, float32(y), float32(width), 1, colorPlanGrid, false)
	}
}

// compactCount is what a compact badge keeps of its counts: how many
// need you, when any do.
func compactCount(b city.PlanBadge) string {
	if b.NeedsYou == 0 {
		return ""
	}
	return strconv.Itoa(b.NeedsYou)
}

// drawCompactBadge is a district on a crowded plan: a status dot in the
// tone of whose move it is, its name, and how many need you.
func (g *Game) drawCompactBadge(screen *ebiten.Image, b city.PlanBadge) {
	th := g.theme
	r := b.Rect
	g.roundRect(screen, r, r.Height()/2, colorPlanPaper)
	g.roundRect(screen, r, r.Height()/2, th.Palette.Panel)
	mid := r.Min.Y + r.Height()/2
	vector.FillCircle(screen, float32(r.Min.X+r.Height()/2), float32(mid), 4, th.Color(ui.MoveTone(b.Tone)), true)
	x := r.Min.X + r.Height()
	room := r.Max.X - r.Height()/2 - x
	count := compactCount(b)
	if count != "" {
		cw, ch := g.faces.Measure(count, ui.Small)
		g.text(screen, city.Point{X: r.Max.X - r.Height()/2 - cw, Y: mid - ch/2}, count, ui.Small, th.Palette.NeedsYou)
		room -= cw + 4
	}
	name := g.fitText(b.Name, ui.Small, room)
	_, nh := g.faces.Measure(name, ui.Small)
	g.text(screen, city.Point{X: x, Y: mid - nh/2}, name, ui.Small, th.Palette.Text)
}

// drawPlanBadge is one district: a flat card edged in the tone of whose
// move it is, with its name, the move, and its counts.
func (g *Game) drawPlanBadge(screen *ebiten.Image, b city.PlanBadge) {
	if b.Compact {
		g.drawCompactBadge(screen, b)
		return
	}
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

// planCovers is the chrome lying over the plan's area, for the plan to
// lay itself out around: the lens panel, in full or as its chip.
func (g *Game) planCovers(width, height float64) []city.Rect {
	if !g.lensPanelShown() {
		return nil
	}
	laid := ui.LayoutLensPanel(g.theme, g.scene.LensPanel(), g.lensArea(width, height, g.faces.Measure), g.faces.Measure)
	if laid.Rect.Area() == 0 {
		return nil
	}
	return []city.Rect{laid.Rect}
}

// raisedCard is a hover card laid out in bounds: in its corner on the
// map, and beside its badge on the plan.
func (g *Game) raisedCard(card city.Card, bounds city.Rect) ui.Card {
	laid := ui.LayoutCard(g.theme, card, bounds, g.faces.Measure)
	hover := g.scene.Hover()
	if !g.scene.CityPlanOpen() || hover.District == nil {
		return laid
	}
	for _, b := range g.scene.CityPlan().Badges {
		if b.District == hover.District {
			return laid.PinTo(b.Rect, bounds, g.theme.Grid())
		}
	}
	return laid
}

// minimapAllowed is whether the minimap may be drawn: not on the plan,
// where the view it frames is hidden.
func (g *Game) minimapAllowed() bool { return !g.scene.CityPlanOpen() }

// drawPlanCompass is a compass rose whose needle points north as the
// plan lies, with an N at its tip.
func (g *Game) drawPlanCompass(screen *ebiten.Image, c city.PlanCompass) {
	if c.Box.Area() == 0 {
		return
	}
	th := g.theme
	mid := c.Box.Center()
	r := c.Box.Width()/2 - 2
	vector.FillCircle(screen, float32(mid.X), float32(mid.Y), float32(r), colorPlanPaper, true)
	vector.StrokeCircle(screen, float32(mid.X), float32(mid.Y), float32(r), 1.2, colorPlanInk, true)
	n := c.North
	side := city.Point{X: -n.Y, Y: n.X}
	needle := r * 0.62
	tip, tail := mid.Add(n.Scale(needle)), mid.Sub(n.Scale(needle))
	fillRing(screen, []city.Point{tip, mid.Add(side.Scale(r * 0.18)), mid.Sub(side.Scale(r * 0.18))}, colorPlanInk)
	strokeRing(screen, []city.Point{tail, mid.Add(side.Scale(r * 0.18)), mid.Sub(side.Scale(r * 0.18))}, 1.2, colorPlanInk)
	w, h := g.faces.Measure("N", ui.Small)
	at := mid.Add(n.Scale(r * 0.82))
	g.text(screen, city.Point{X: at.X - w/2, Y: at.Y - h/2}, "N", ui.Small, th.Palette.Text)
}

// drawPlanCartouche is the plan's title block: its name and the time, in
// a double-ruled frame.
func (g *Game) drawPlanCartouche(screen *ebiten.Image, c city.PlanCartouche) {
	if c.Box.Area() == 0 {
		return
	}
	th := g.theme
	r := c.Box
	g.roundRect(screen, r, th.Radius()/2, colorPlanPaper)
	g.roundRect(screen, r, th.Radius()/2, th.Palette.Panel)
	for _, in := range []float64{1, 4} {
		box := r.Inset(in)
		strokeRing(screen, []city.Point{box.Min, {X: box.Max.X, Y: box.Min.Y}, box.Max, {X: box.Min.X, Y: box.Max.Y}}, 1, colorPlanCoast)
	}
	tw, th1 := g.faces.Measure(c.Title, ui.Body)
	mw, mh := g.faces.Measure(c.Time, ui.Small)
	top := r.Min.Y + (r.Height()-th1-mh)/2
	mid := r.Center().X
	g.text(screen, city.Point{X: mid - tw/2, Y: top}, c.Title, ui.Body, th.Palette.Text)
	g.text(screen, city.Point{X: mid - mw/2, Y: top + th1}, c.Time, ui.Small, th.Palette.Dim)
}
