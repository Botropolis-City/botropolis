package render

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

var colorScrim = color.NRGBA{0x00, 0x00, 0x00, 0x80}

// chrome is everything fixed to the window: strip, minimap, card, footer.
func (g *Game) chrome(screen *ebiten.Image, width, height float64) {
	th := g.theme
	g.mu.Lock()
	status := g.notice.Text(timeNow())
	g.mu.Unlock()
	footer := ui.LayoutFooter(th, width, height, status, footerKeys, g.faces.Measure)
	waitingID := ""
	if step, ok := g.scene.TriageStep(); ok && g.triage() != nil {
		footer = ui.LayoutTriageBar(th, width, height, step.Index, step.Count, step.Building.Card(g.scene.Now()).Title, g.faces.Measure)
	} else if walk := g.scene.TriageWalk(); len(walk) > 0 {
		// The needs-you line names what has waited longest, in the order
		// triage walks, and a click on it attaches that session. Item 91.
		line := "needs-you: " + walk[0].Card(g.scene.Now()).Title
		if len(walk) > 1 {
			line += fmt.Sprintf(" (+%d more)", len(walk)-1)
		}
		footer = footer.WithWaiting(th, width, line, g.faces.Measure)
		waitingID = walk[0].Session.ID
	}
	// Kept so a click hit-tests the bar that was actually drawn, the
	// same way the view key does. Bug 43.
	g.mu.Lock()
	g.footerBar, g.waitingID = footer, waitingID
	g.mu.Unlock()
	legend := ui.LayoutLegend(th, width, footer.Rect.Min.Y, g.scene.Legend(), g.faces.Measure)
	top := g.strip(screen, width)
	g.scene.SetTopChrome(top)
	bottom := footer.KeyRow.Height() + legend.Rect.Height()
	g.scene.SetBottomChrome(bottom)
	streamFoot := max(bottom, height-footer.Rect.Min.Y)
	minimapBox := city.Rect{}
	if box, ok := ui.LayoutMinimap(th, width, height, bottom); ok && g.minimapAllowed() {
		if g.streamShown() {
			box = ui.MinimapBesideStream(th, box, ui.StreamArea(th, ui.StreamCorner, width, height, top, streamFoot), ui.StreamCorner)
		}
		g.minimap(screen, box)
		minimapBox = box
	}
	g.mu.Lock()
	g.minimapBox = minimapBox
	g.mu.Unlock()
	pointer := g.pointer()
	bounds := city.RectAt(0, top, width, height-top-bottom)
	bounds.Max.X = g.drawStream(screen, width, height, top, streamFoot)
	hint := g.buttonHint(pointer)
	g.pinned = ui.Card{}
	g.pinnedKinds = nil
	g.cardOver = ui.Card{}
	if g.sidebar {
		g.drawSidebar(screen, top, min(height-bottom, footer.Rect.Min.Y))
		g.scene.SetLeftChrome(ui.SidebarWidth(th))
		bounds.Min.X = ui.SidebarWidth(th)
	} else {
		g.scene.SetLeftChrome(0)
	}
	if dock := g.drawDive(screen, width, height); dock.Area() > 0 {
		if dock.Center().X < width/2 {
			bounds.Min.X = dock.Max.X
		} else {
			bounds.Max.X = dock.Min.X
		}
	}
	if card, b, ok := g.scene.SelectedCard(); ok && !g.sidebar {
		card.Hint = hint
		beside := g.buildingOnScreen(b)
		pinned := ui.LayoutCard(th, card, bounds, g.faces.Measure).PinTo(beside, bounds, th.Grid())
		g.card(screen, pinned)
		g.pinned = pinned
		g.pinnedKinds = g.scene.Actions()
	}
	if st, ok := g.stripHover(pointer); ok {
		g.card(screen, ui.LayoutCard(th, g.scene.City().StateCard(st), bounds, g.faces.Measure))
	} else if hover := g.scene.Hover(); hover.Building == nil || hover.Building != g.scene.Selected() {
		if card, ok := g.scene.Card(); ok {
			g.card(screen, g.raisedCard(card, bounds))
		}
	}
	g.drawSearch(screen, top, width)
	g.footer(screen, footer)
	g.legend(screen, legend)
}

// buildingOnScreen is the screen rect a building's footprint covers, in
// either projection, for pinning its card beside it.
func (g *Game) buildingOnScreen(b *city.Building) city.Rect {
	cam := g.scene.Camera()
	corners := []city.Point{b.Rect.Min, {X: b.Rect.Max.X, Y: b.Rect.Min.Y}, b.Rect.Max, {X: b.Rect.Min.X, Y: b.Rect.Max.Y}}
	r := city.Rect{Min: cam.WorldToScreen(corners[0]), Max: cam.WorldToScreen(corners[0])}
	for _, c := range corners[1:] {
		r = r.Union(city.Rect{Min: cam.WorldToScreen(c), Max: cam.WorldToScreen(c)})
	}
	return r
}

// clickCard is a click on the pinned card: a button runs its action and
// the click stops there; anywhere else on the card is swallowed too.
func (g *Game) clickCard(at city.Point) bool {
	if g.pinned.Rect.Area() == 0 || !g.pinned.Rect.Contains(at) {
		return false
	}
	if btn, ok := ui.HitButton(g.pinned.Buttons, at); ok {
		for i, kind := range g.pinnedKinds {
			if i < len(g.pinned.Buttons) && g.pinned.Buttons[i].Label.Text == btn.Label.Text {
				action, note := g.scene.Act(kind)
				if note != "" {
					g.SetStatus(note)
				}
				g.act(action)
				break
			}
		}
	}
	return true
}

func (g *Game) card(screen *ebiten.Image, c ui.Card) {
	th := g.theme
	g.roundPanel(screen, c.Rect)
	g.run(screen, c.Kind, th.Palette.Dim)
	g.run(screen, c.Title, th.Palette.Text)
	g.run(screen, c.Move, g.moveTone(c.Move.Text))
	g.run(screen, c.Group, th.Palette.Dim)
	for _, line := range c.Lines {
		g.run(screen, line, th.Palette.Dim)
	}
	for _, line := range c.Sentence {
		g.run(screen, line, th.Palette.Text)
	}
	for _, line := range c.Numbers {
		g.run(screen, line, th.Palette.Text)
	}
	for _, line := range c.Para {
		g.run(screen, line, th.Palette.Text)
	}
	g.sparkline(screen, c.Spark)
	at := g.pointer()
	for _, b := range c.Buttons {
		g.button(screen, b.Rect, b.Label, th.Palette.Accent, b.Rect.Contains(at))
	}
	g.run(screen, c.Hint, th.Palette.Dim)
	for _, line := range c.Details {
		g.run(screen, line, th.Palette.Dim)
	}
}

// button draws a plate with its label, brighter under the pointer, the
// one look for anything a click acts on: the card's buttons and the
// footer's needs-you line. Item 91.
func (g *Game) button(screen *ebiten.Image, plate city.Rect, label ui.Text, tone color.NRGBA, hovered bool) {
	fill := g.theme.Palette.Hairline
	if hovered {
		fill = buttonHover(fill)
	}
	g.roundRect(screen, plate, g.theme.Radius()/2, fill)
	g.run(screen, label, tone)
}

// buttonHover is a plate's tone under the pointer: the same hairline,
// nearly three times as strong.
func buttonHover(c color.NRGBA) color.NRGBA {
	c.A = uint8(min(255, int(c.A)*8/3))
	return c
}

// buttonHint is what the pinned card's button under the pointer does,
// from the card as last drawn (item 88).
func (g *Game) buttonHint(at city.Point) string {
	for i, b := range g.pinned.Buttons {
		if b.Rect.Contains(at) && i < len(g.pinnedKinds) {
			return g.pinnedKinds[i].Describe()
		}
	}
	return ""
}

// moveTone is the colour a card says whose move it is in: the state
// tone it stands for, or dim when the move is nobody's here.
func (g *Game) moveTone(move string) color.NRGBA {
	switch move {
	case "your move":
		return g.theme.Palette.NeedsYou
	case "the agents' move":
		return g.theme.Palette.Working
	}
	return g.theme.Palette.Dim
}

// sparkline strokes a series in the accent with its last point marked.
func (g *Game) sparkline(screen *ebiten.Image, sp ui.Sparkline) {
	if len(sp.Points) == 0 {
		return
	}
	th := g.theme
	vector.FillRect(screen, float32(sp.Box.Min.X), float32(sp.Box.Max.Y-th.Hairline()), float32(sp.Box.Width()), float32(th.Hairline()), th.Palette.Hairline, false)
	if len(sp.Points) > 1 {
		var path vector.Path
		path.MoveTo(float32(sp.Points[0].X), float32(sp.Points[0].Y))
		for _, p := range sp.Points[1:] {
			path.LineTo(float32(p.X), float32(p.Y))
		}
		op := &vector.DrawPathOptions{AntiAlias: true}
		op.ColorScale.ScaleWithColor(th.Palette.Accent)
		vector.StrokePath(screen, &path, &vector.StrokeOptions{Width: float32(th.Px(1.5))}, op)
	}
	vector.FillCircle(screen, float32(sp.Last.X), float32(sp.Last.Y), float32(th.Px(2.5)), th.Palette.Accent, true)
}

func (g *Game) footer(screen *ebiten.Image, f ui.Footer) {
	g.bar(screen, f.Rect, f.Rect.Min.Y)
	for _, line := range f.Lines {
		g.run(screen, line, g.theme.Palette.Dim)
	}
	if b, ok := g.quitButton(); ok {
		g.button(screen, b.Rect, b.Label, g.theme.Palette.Accent, b.Rect.Contains(g.pointer()))
	}
	g.waitingHovered = false
	if f.Waiting.Text != "" {
		g.waitingHovered = f.WaitingRect.Contains(g.pointer())
		g.button(screen, f.WaitingRect, f.Waiting, g.theme.Palette.NeedsYou, g.waitingHovered)
	}
	g.keyRow(screen, f.Keys, f.Size)
}

// keyRow draws key chips with their actions beside them.
func (g *Game) keyRow(screen *ebiten.Image, keys []ui.PlacedKey, size ui.Size) {
	th := g.theme
	for _, k := range keys {
		g.roundRect(screen, k.Chip, th.Radius()/2, th.Palette.Hairline)
		g.text(screen, k.KeyAt, k.Key.Key, size, th.Palette.Text)
		g.text(screen, k.LabelAt, k.Action, size, th.Palette.Dim)
	}
}

// run draws one placed text run.
func (g *Game) run(screen *ebiten.Image, t ui.Text, c color.NRGBA) {
	g.text(screen, t.At, t.Text, t.Size, c)
}

// plate draws an in-world label on its plate.
func (g *Game) plate(screen *ebiten.Image, p ui.Plate, s string, c color.NRGBA) {
	g.roundRect(screen, p.Rect, g.theme.Radius()/2, g.theme.Palette.Panel)
	g.text(screen, p.TextAt, s, p.Size, c)
}

// bar fills a full-width chrome rectangle with the hairline along the
// edge at y, which is its top for a footer and its bottom for a strip.
func (g *Game) bar(screen *ebiten.Image, r city.Rect, edge float64) {
	th := g.theme
	vector.FillRect(screen, float32(r.Min.X), float32(r.Min.Y), float32(r.Width()), float32(r.Height()), th.Palette.Panel, false)
	y := edge
	if edge >= r.Max.Y {
		y = edge - th.Hairline()
	}
	vector.FillRect(screen, float32(r.Min.X), float32(y), float32(r.Width()), float32(th.Hairline()), th.Palette.Hairline, false)
}

// roundPanel is a floating panel: rounded, translucent, hairline all round.
func (g *Game) roundPanel(screen *ebiten.Image, r city.Rect) {
	th := g.theme
	g.roundRect(screen, r, th.Radius(), th.Palette.Panel)
	g.roundStroke(screen, r, th.Radius(), th.Hairline(), th.Palette.Hairline)
}

func roundPath(r city.Rect, radius float64) *vector.Path {
	x0, y0, x1, y1 := float32(r.Min.X), float32(r.Min.Y), float32(r.Max.X), float32(r.Max.Y)
	rad := float32(radius)
	p := &vector.Path{}
	p.MoveTo(x0+rad, y0)
	p.ArcTo(x1, y0, x1, y1, rad)
	p.ArcTo(x1, y1, x0, y1, rad)
	p.ArcTo(x0, y1, x0, y0, rad)
	p.ArcTo(x0, y0, x1, y0, rad)
	p.Close()
	return p
}

func (g *Game) roundRect(screen *ebiten.Image, r city.Rect, radius float64, c color.NRGBA) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.FillPath(screen, roundPath(r, radius), nil, op)
}

func (g *Game) roundStroke(screen *ebiten.Image, r city.Rect, radius, width float64, c color.NRGBA) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.StrokePath(screen, roundPath(r.Inset(width/2), radius), &vector.StrokeOptions{Width: float32(width)}, op)
}
