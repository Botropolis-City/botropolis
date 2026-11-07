package render

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/shapes"
	"botropolis.city/botropolis/pkg/ui"
)

var colorScrim = color.NRGBA{0x00, 0x00, 0x00, 0x80}

// chrome is everything fixed to the window: strip, minimap, card, and
// the reading band while a first launch reads the history. There is no
// bar along the bottom; the map runs to the window's edge (item 353).
func (g *Game) chrome(screen *ebiten.Image, width, height float64) {
	th := g.theme
	g.mu.Lock()
	message, _ := g.notice.Current(timeNow())
	g.mu.Unlock()
	// Triage's place is its card's header (item 352).
	triaging := g.triage() != nil
	if _, ok := g.scene.TriageStep(); !ok {
		triaging = false
	}
	var reading ui.Reading
	if scan := g.scene.Snapshot().Scan; scan != nil && scan.Known > 0 {
		g.scanRate.Observe(timeNow(), scan.Done)
		left, settled := g.scanRate.Remaining(scan.Done, scan.Known)
		reading = ui.LayoutReading(th, width, height, ui.ReadingLine(*scan, left, settled), float64(scan.Done)/float64(scan.Known), g.faces.Measure)
		g.logReading(reading.Line.Text)
	}
	floor := height
	if reading.Rect.Area() > 0 {
		floor = reading.Rect.Min.Y
	}
	legend := ui.LayoutLegend(th, width, floor, g.scene.Legend(), g.faces.Measure)
	var top float64
	g.baked(bakeStrip, screen, func(dst *ebiten.Image) { top = g.strip(dst, width) })
	g.scene.SetTopChrome(top)
	bottom := legend.Rect.Height()
	g.scene.SetBottomChrome(bottom)
	g.mu.Lock()
	g.mapBottom = height - bottom
	g.mu.Unlock()
	streamFoot := max(bottom, height-floor)
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
	g.baked(bakeStream, screen, func(dst *ebiten.Image) { bounds.Max.X = g.drawStream(dst, width, height, top, streamFoot) })
	if g.streamShown() {
		g.heldStream = ui.StreamArea(th, ui.StreamCorner, width, height, top, streamFoot)
	}
	hint, hintKey := g.buttonHint(pointer)
	g.pinned = ui.Card{}
	g.pinnedVerbs, g.pinnedHints = city.Verbs{}, nil
	g.cardOver = ui.Card{}
	if g.sidebar {
		g.drawSidebar(screen, top, min(height-bottom, floor))
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
		card.Hint, card.HintKey = hint, hintKey
		card.HintKeys = g.faceKeys()
		card.Actions = g.progressLabels(g.scene.Verbs(), b)
		pressed := g.pressedButton()
		if pressed > 0 && pressed < len(card.Actions) {
			card.Actions[pressed] = strings.Replace(card.Actions[pressed], city.MenuMark, city.MenuMarkOpen, 1)
		}
		if step, ok := g.scene.TriageStep(); ok && triaging {
			card.Kind = step.Header()
		}
		beside := g.buildingOnScreen(b)
		pinned := ui.LayoutCard(th, card, bounds, g.faces.Measure).PinTo(beside, bounds, th.Grid())
		g.cardPressed = pressed
		g.card(screen, pinned)
		g.cardPressed = -1
		g.pinned, g.pinnedPressed = pinned, pressed
		g.pinnedVerbs, g.pinnedHints = g.scene.Verbs(), card.Hints
	}
	g.stripCard = nil
	if st, ok := g.stripHover(pointer); ok {
		card := g.stripHoverCard(st)
		g.stripCard = card.Lines
		g.card(screen, ui.LayoutCard(th, card, bounds, g.faces.Measure))
	} else if hover := g.scene.Hover(); hover.Building == nil || hover.Building != g.scene.Selected() {
		if card, ok := g.scene.Card(); ok {
			g.card(screen, g.raisedCard(card, bounds))
		}
	}
	g.drawSearch(screen, top, width)
	g.readingBand(screen, reading)
	g.legend(screen, legend)
	g.drawNotice(screen, message, width, top)
}

// drawNotice draws the notice on its plate, centred under the strip, and
// keeps the layout for the frame's probe (item 352).
func (g *Game) drawNotice(screen *ebiten.Image, m ui.Message, width, top float64) {
	th := g.theme
	plate := ui.LayoutNotice(th, m, width, top, g.faces.Measure)
	g.mu.Lock()
	g.noticePlate = plate
	g.mu.Unlock()
	if plate.Rect.Area() == 0 {
		return
	}
	backing := th.Palette.Panel
	backing.A = 0xff
	g.roundRect(screen, plate.Rect, th.Radius(), backing)
	g.roundStroke(screen, plate.Rect, th.Radius(), th.Hairline(), th.Palette.Hairline)
	if plate.Dot.Area() > 0 {
		centre := plate.Dot.Center()
		g.fillCircle(screen, float32(centre.X), float32(centre.Y), float32(plate.Dot.Width()/2), th.Color(plate.Tone))
	}
	for _, line := range plate.Lines {
		g.run(screen, line, th.Palette.Text)
	}
	g.keyCap(screen, plate.Cap)
}

// keyCap draws a key's cap: the one look for a key wherever it shows.
func (g *Game) keyCap(screen *ebiten.Image, c ui.KeyCap) {
	if c.Rect.Area() == 0 {
		return
	}
	th := g.theme
	g.roundRect(screen, c.Rect, th.Radius()/2, th.Palette.Hairline)
	g.run(screen, c.Label, th.Palette.Text)
}

// endingKey draws the dot and caps that end a line with its key.
func (g *Game) endingKey(screen *ebiten.Image, dot ui.Text, caps []ui.KeyCap) {
	if dot.Text != "" {
		g.run(screen, dot, g.theme.Palette.Dim)
	}
	for _, c := range caps {
		g.keyCap(screen, c)
	}
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

// clickCard is a click on the pinned card: the first button runs the
// direct verb, each other opens the menu on its column under the button,
// the numbers row folds the details, and the click stops there; anywhere
// else on the card is swallowed too (item 350).
func (g *Game) clickCard(at city.Point) bool {
	if g.pinned.Rect.Area() == 0 || !g.pinned.Rect.Contains(at) {
		return false
	}
	if btn, ok := ui.HitButton(g.pinned.Buttons, at); ok {
		for i, b := range g.pinned.Buttons {
			if b.Label.Text != btn.Label.Text {
				continue
			}
			if i == 0 {
				g.runVerb(g.pinnedVerbs.Direct)
			} else {
				g.openMenuUnder(g.pinnedVerbs.All(), b.Rect, i-1)
			}
			break
		}
		return true
	}
	if g.pinned.NumbersRect.Contains(at) {
		if _, note := g.scene.ToggleDetails(); note != "" {
			g.Notify(note, ui.ToneNone, "")
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
	at := g.pointer()
	// The numbers row is the click that folds the details, so it reads
	// as the buttons do: in the accent, on a plate under the pointer
	// (item 350).
	if c.NumbersRect.Area() > 0 && c.NumbersRect.Contains(at) {
		g.roundRect(screen, c.NumbersRect, th.Radius()/2, buttonHover(th.Palette.Hairline))
	}
	for _, line := range c.Numbers {
		g.run(screen, line, th.Palette.Accent)
	}
	for _, line := range c.Para {
		g.run(screen, line, th.Palette.Text)
	}
	g.sparkline(screen, c.Spark)
	for i, b := range c.Buttons {
		g.button(screen, b.Rect, b.Label, th.Palette.Accent, b.Rect.Contains(at) || i == g.cardPressed)
	}
	for _, line := range c.Hint {
		g.run(screen, line, th.Palette.Dim)
	}
	g.endingKey(screen, c.HintDot, c.HintCaps)
	for _, line := range c.Details {
		g.run(screen, line, th.Palette.Dim)
	}
}

// button draws a plate with its label, brighter under the pointer, the
// one look for anything a click acts on: the card's buttons and the
// strip's needs-you count. Items 91, 352.
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
// from the card as last drawn (item 88), or what its numbers row does.
// An open menu hangs over the hint's line, so then there is none.
func (g *Game) buttonHint(at city.Point) (string, []string) {
	if g.menu() != nil {
		return "", nil
	}
	keys := g.faceKeys()
	keyAt := func(i int) []string {
		if i < len(keys) {
			return keys[i]
		}
		return nil
	}
	for i, b := range g.pinned.Buttons {
		if b.Rect.Contains(at) && i < len(g.pinnedHints) {
			return g.pinnedHints[i], keyAt(i)
		}
	}
	if g.pinned.NumbersRect.Area() > 0 && g.pinned.NumbersRect.Contains(at) {
		if _, _, ok := g.scene.SelectedCard(); ok && len(g.pinnedHints) > len(g.pinned.Buttons) {
			return g.pinnedHints[len(g.pinnedHints)-1], keyAt(len(keys) - 1)
		}
	}
	return "", nil
}

// faceKeys is the keys that do what each of the card's hints says, in
// the hints' order: Enter for the direct verb, the menu's key and a
// column's letter for each column's button, and the details' key for
// the numbers row, read from the live bindings (item 351).
func (g *Game) faceKeys() [][]string {
	chords := g.findChords()
	open := chords.Bound["action_menu"].Label()
	keys := [][]string{{keyName(ebiten.KeyEnter)}}
	for _, name := range menuColumnKeys {
		keys = append(keys, []string{open, chords.Bound[name].Label()})
	}
	return append(keys, []string{chords.Bound["details"].Label()})
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
	g.paint.Box(screen, float32(sp.Box.Min.X), float32(sp.Box.Max.Y-th.Hairline()), float32(sp.Box.Width()), float32(th.Hairline()), th.Palette.Hairline)
	if len(sp.Points) > 1 {
		g.paint.Stroke(screen, sp.Points, th.Px(1.5), false, th.Palette.Accent)
	}
	g.fillCircle(screen, float32(sp.Last.X), float32(sp.Last.Y), float32(th.Px(2.5)), th.Palette.Accent)
}

// readingBand draws the reading band, when there is one (items 159, 353).
func (g *Game) readingBand(screen *ebiten.Image, r ui.Reading) {
	if r.Rect.Area() == 0 {
		return
	}
	g.bar(screen, r.Rect, r.Rect.Min.Y)
	g.run(screen, r.Line, g.theme.Palette.Dim)
	g.roundRect(screen, r.Bar, r.Bar.Height()/2, g.theme.Palette.Hairline)
	if r.Fill.Width() > 0 {
		g.roundRect(screen, r.Fill, r.Fill.Height()/2, g.theme.Palette.Accent)
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
// edge at y, which is its top for a band along the bottom and its bottom for a strip.
func (g *Game) bar(screen *ebiten.Image, r city.Rect, edge float64) {
	th := g.theme
	g.paint.Box(screen, float32(r.Min.X), float32(r.Min.Y), float32(r.Width()), float32(r.Height()), th.Palette.Panel)
	y := edge
	if edge >= r.Max.Y {
		y = edge - th.Hairline()
	}
	g.paint.Box(screen, float32(r.Min.X), float32(y), float32(r.Width()), float32(th.Hairline()), th.Palette.Hairline)
}

// roundPanel is a floating panel: rounded, translucent, hairline all round.
func (g *Game) roundPanel(screen *ebiten.Image, r city.Rect) {
	th := g.theme
	g.roundRect(screen, r, th.Radius(), th.Palette.Panel)
	g.roundStroke(screen, r, th.Radius(), th.Hairline(), th.Palette.Hairline)
}

func (g *Game) roundRect(screen *ebiten.Image, r city.Rect, radius float64, c color.NRGBA) {
	pts := shapes.AppendRoundRect(g.paint.Points(), r, radius)
	g.paint.Fill(screen, pts, c)
	g.paint.Keep(pts)
}

func (g *Game) roundStroke(screen *ebiten.Image, r city.Rect, radius, width float64, c color.NRGBA) {
	pts := shapes.AppendRoundRect(g.paint.Points(), r.Inset(width/2), radius)
	g.paint.Stroke(screen, pts, width, true, c)
	g.paint.Keep(pts)
}

// keyLegend draws a legend's caps and the words beside them (item 351).
func (g *Game) keyLegend(screen *ebiten.Image, l ui.KeyLegend) {
	for _, k := range l.Keys {
		g.keyCap(screen, k.Cap)
		g.text(screen, k.LabelAt, k.Action, ui.Small, g.theme.Palette.Dim)
	}
}
