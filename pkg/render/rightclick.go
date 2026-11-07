package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// rightButton opens the action menu on a right-click (item 350): a press
// let go where it started, measured as the left button measures a click.
// A right drag still pans, which buttonPan does; this only watches for
// the press that did not move.
func (g *Game) rightButton(cursor city.Point, in mouseInput) {
	right := ebiten.MouseButtonRight
	if in.JustPressed(right) {
		g.rightPressAt, g.rightPressed = cursor, true
	}
	if !in.JustReleased(right) || !g.rightPressed {
		return
	}
	g.rightPressed = false
	if !dragged(g.rightPressAt, cursor) {
		g.rightClick(cursor)
	}
}

// scriptedMouse is the mouse with a driven or scripted right-click added.
type scriptedMouse struct{ g *Game }

func (m scriptedMouse) JustPressed(b ebiten.MouseButton) bool {
	return ebitenMouse{}.JustPressed(b) || (b == ebiten.MouseButtonRight && m.g.play.rightPress)
}

func (m scriptedMouse) Pressed(b ebiten.MouseButton) bool {
	return ebitenMouse{}.Pressed(b) || (b == ebiten.MouseButtonRight && m.g.play.rightPress)
}

func (m scriptedMouse) JustReleased(b ebiten.MouseButton) bool {
	return ebitenMouse{}.JustReleased(b) || (b == ebiten.MouseButtonRight && m.g.play.rightRelease)
}

// rightClick opens the menu for what is under the pointer: a row of the
// groups board, a building or a crate, or a district's ground or sign.
// Anything else, any chrome, or a covered city opens nothing.
func (g *Game) rightClick(at city.Point) {
	if b := g.board(); b != nil {
		b.rightClick(g, at)
		return
	}
	if g.cityCovered() || g.onChrome(at) {
		return
	}
	hit := g.scene.Hover()
	switch {
	case hit.Building != nil:
		g.act(g.scene.Click(at))
		if g.scene.Selected() == nil {
			return
		}
		g.openMenu(g.scene.Verbs().All(), at, menuFirstRows)
	case hit.District != nil && !hit.District.Depot:
		g.openMenuWith(g.scene.DistrictVerbs(hit.District), at, menuFirstRows, g.districtVerb(hit.District))
	}
}

// openSpaceMenu opens the menu from the keyboard, on its first column
// with nothing chosen, so a column's letter and then a row's runs a
// verb: the selection's, under its card, or with nothing selected the
// district's under the pointer, there (item 350).
func (g *Game) openSpaceMenu() {
	if g.scene.Selected() != nil {
		at := g.scene.Size().Scale(0.5)
		if g.pinned.Rect.Area() > 0 {
			at = city.Point{X: g.pinned.Rect.Min.X, Y: g.pinned.Rect.Max.Y + g.theme.Grid()}
		}
		g.openMenu(g.scene.Verbs().All(), at, -1)
		return
	}
	if d := g.scene.Hover().District; d != nil && !d.Depot && g.scene.Hover().Building == nil {
		g.openMenuWith(g.scene.DistrictVerbs(d), g.pointer(), -1, g.districtVerb(d))
	}
}

// districtVerb is what a district's menu row does: act on the district
// and say so.
func (g *Game) districtVerb(d *city.District) func(city.ActionKind) {
	return func(kind city.ActionKind) {
		action, note := g.scene.ActOnDistrict(d, kind)
		if note != "" && !runsClaude[action.Kind] {
			g.Notify(note, ui.ToneNone, "")
		}
		g.act(action)
	}
}

// onChrome is whether a point is on something drawn over the map: the
// card, the strip and the footer, the sidebar, the stream, the minimap
// or the lens panel.
func (g *Game) onChrome(at city.Point) bool {
	if g.pinned.Rect.Contains(at) {
		return true
	}
	in := g.scene.Insets()
	size := g.scene.Size()
	if at.Y < in.Top || at.Y > size.Y-in.Bottom || at.X < in.Left {
		return true
	}
	g.mu.Lock()
	stream, minimap := g.streamLaid.Rect, g.minimapBox
	g.mu.Unlock()
	if stream.Contains(at) || minimap.Contains(at) {
		return true
	}
	for _, o := range g.overlays {
		if a, ok := o.(ambient); ok && a.hit(at) {
			return true
		}
	}
	return false
}

// rightClick on the board chooses the row under the pointer and opens
// the menu's Group column for that group (item 350).
func (b *board) rightClick(g *Game, at city.Point) {
	groups := g.scene.Groups()
	g.mu.Lock()
	rows := b.rowRects
	g.mu.Unlock()
	for i, r := range rows {
		if i >= len(groups) || !r.Contains(at) {
			continue
		}
		b.selected = i
		gr := groups[i]
		kinds := []city.ActionKind{city.ActionFocusLine, city.ActionCopyStandup, city.ActionRenameGroup, city.ActionCloseGroup}
		g.openMenuWith(kinds, at, menuFirstRows, func(kind city.ActionKind) { b.groupVerb(g, gr.ID, gr.Name, kind) })
		return
	}
}

// groupVerb runs one of a group's verbs from the board's menu, the way
// the board's own keys do.
func (b *board) groupVerb(g *Game, id, name string, kind city.ActionKind) {
	switch kind {
	case city.ActionFocusLine:
		g.scene.FocusGroup(id)
		g.overlays.remove(b)
	case city.ActionCopyStandup:
		line := g.scene.Standup(id)
		g.Notify("copied stand-up: "+line, ui.ToneNone, "")
		g.act(city.Action{Kind: city.ActionCopy, Text: line})
	case city.ActionRenameGroup:
		g.openRenamer(id, name)
	case city.ActionCloseGroup:
		// Closing asks through the one dialog for destroying things, the
		// one Delete opens on the board (item 352).
		g.overlays.push(closeGroupDialog(name, id))
	}
}
