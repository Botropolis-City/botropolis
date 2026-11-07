package render

import (
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// The hold (item 351): while the explain key is down, every visible
// control that a key also does wears that key as caps, read from the
// live bindings, and letting go takes them away. The menu's rows and a
// dialog's buttons show their caps all the time, so they get none here.

// capsHolder is an overlay the hold reaches through: one that does not
// take letters while it is up, or not yet.
type capsHolder interface {
	holdsCaps() bool
}

func (p *menuPanel) holdsCaps() bool { return true }
func (d *dialog) holdsCaps() bool    { return true }
func (c *clinic) holdsCaps() bool    { return !c.filter.on }
func (b *board) holdsCaps() bool     { return !b.filter.on }
func (j *journal) holdsCaps() bool   { return !j.filter.on }
func (p *closeUp) holdsCaps() bool   { return !p.filter.on }
func (p *picker) holdsCaps() bool    { return !p.filter.on && !p.naming }
func (p *keysPanel) holdsCaps() bool { return !p.recording }

// heldRow is a list panel's row as it was drawn, for the hold to cap
// with its digit.
type heldRow struct {
	owner overlay
	index int
	rect  city.Rect
}

// rowShown records a list panel's row as drawn this frame.
func (g *Game) rowShown(owner overlay, index int, rect city.Rect) {
	g.heldRows = append(g.heldRows, heldRow{owner: owner, index: index, rect: rect})
}

// keyHeld is whether a key is down: by hand, by hold+ in --keys, or by
// a recording script's hold.
func (g *Game) keyHeld(key ebiten.Key) bool {
	if g.scriptedHeld[key] || ebiten.IsKeyPressed(key) {
		return true
	}
	held, ok := keyByName(g.play.hold)
	return ok && g.play.hold != "" && held == key
}

// capsHeld is whether the hold is down: the explain key held over the
// map, the menu, a dialog or a list panel whose filter is not typing.
func (g *Game) capsHeld() bool {
	key, ok := keyByName(g.findChords().Explain.Key)
	if !ok || !g.keyHeld(key) {
		return false
	}
	top := g.overlays.top()
	if top == nil {
		return true
	}
	h, ok := top.(capsHolder)
	return ok && h.holdsCaps()
}

// heldControls is every visible control with a key: over the map or
// the menu, the card's buttons and numbers row, the needs-you count,
// the lens panel, the stream and the sidebar; over a list panel, its
// first nine rows.
func (g *Game) heldControls() []ui.Control {
	top := g.overlays.top()
	if top != nil {
		if _, menu := top.(*menuPanel); !menu {
			return g.heldRowControls(top)
		}
	}
	chords := g.findChords()
	var controls []ui.Control
	face := g.faceKeys()
	for i, b := range g.pinned.Buttons {
		if i < len(face)-1 {
			controls = append(controls, ui.Control{Name: "button:" + b.Label.Text, Rect: b.Rect, Keys: face[i]})
		}
	}
	if g.pinned.NumbersRect.Area() > 0 {
		controls = append(controls, ui.Control{Name: "numbers", Rect: g.pinned.NumbersRect, Keys: face[len(face)-1]})
	}
	if chip, b := g.waitingChip(); b != nil {
		controls = append(controls, ui.Control{Name: "needs-you", Rect: chip, Keys: []string{keyName(ebiten.KeyTab)}})
	}
	for _, o := range g.overlays {
		if p, ok := o.(*lensPanel); ok && p.laid.Rect.Area() > 0 {
			controls = append(controls, ui.Control{Name: "lenses", Rect: p.laid.Rect, Keys: []string{chords.Bound["lenses"].Label()}})
		}
	}
	if g.heldStream.Area() > 0 {
		controls = append(controls, ui.Control{Name: "stream", Rect: g.heldStream, Keys: []string{chords.Bound["stream"].Label()}})
	}
	if g.heldSidebar.Area() > 0 {
		controls = append(controls, ui.Control{Name: "sidebar", Rect: g.heldSidebar, Keys: []string{chords.Bound["sidebar"].Label()}})
	}
	return controls
}

// heldRowControls is a list panel's first nine rows as drawn, each with
// the digit that chooses it.
func (g *Game) heldRowControls(owner overlay) []ui.Control {
	var controls []ui.Control
	for _, r := range g.heldRows {
		if r.owner == owner && r.index < 9 {
			n := strconv.Itoa(r.index + 1)
			controls = append(controls, ui.Control{Name: "row:" + n, Rect: r.rect, Keys: []string{n}})
		}
	}
	return controls
}

// drawHeldCaps draws the hold's caps over everything while it is down.
func (g *Game) drawHeldCaps(screen *ebiten.Image, width, height float64) {
	var held []ui.HeldCaps
	if g.capsHeld() {
		held = ui.LayoutCaps(g.theme, g.heldControls(), g.pinned.TextRects(g.faces.Measure), city.RectAt(0, 0, width, height), g.faces.Measure)
	}
	g.mu.Lock()
	g.holdCaps = held
	g.mu.Unlock()
	for _, h := range held {
		for _, c := range h.Caps {
			g.roundRect(screen, c.Rect.Inset(-g.theme.Hairline()), g.theme.Radius()/2, g.theme.Palette.Panel)
			g.keyCap(screen, c)
		}
	}
}

// keysSightingText is the keys' first-time caption, naming the hold key
// as it is bound.
func (g *Game) keysSightingText() string {
	return "hold " + g.findChords().Explain.Label() + " to see every key"
}

// drawKeysSighting puts the keys' first-time caption over the pinned
// card, clear of the menu, which opens under it, the first time one is
// pinned on this machine (item 351). It is drawn under the panels.
func (g *Game) drawKeysSighting(screen *ebiten.Image, height float64) {
	text := ""
	if g.pinned.Rect.Area() > 0 {
		g.scene.SightKeys()
		if g.scene.KeysSighting() {
			text = g.keysSightingText()
		}
	}
	var p ui.Plate
	if text != "" {
		grid := g.theme.Grid()
		p = ui.LayoutPlate(g.theme, text, city.Point{}, ui.Small, g.faces.Measure)
		above := city.Point{X: g.pinned.Rect.Min.X, Y: g.pinned.Rect.Min.Y - grid - p.Rect.Height()}
		if above.Y < g.scene.TopChrome() {
			above.Y = min(g.pinned.Rect.Max.Y+grid, height-p.Rect.Height())
		}
		p = p.Moved(above)
	}
	g.mu.Lock()
	g.keysSighting, g.keysSightingRect = text, p.Rect
	g.mu.Unlock()
	if text != "" {
		g.drawCaption(screen, text, p)
	}
}
