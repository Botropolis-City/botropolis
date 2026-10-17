package render

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Badges: at fit, and whenever a building would draw too small to read,
// each district draws as its sign alone, with its counts, at a size that
// reads whatever the zoom. Item 89.

// drawsBuildings reports whether the map draws buildings rather than
// badges.
func (g *Game) drawsBuildings() bool { return !g.scene.Badged() }

// badgeFoot is where a district's badge stands on screen: its middle.
func badgeFoot(cam *city.Camera, b city.Badge) city.Point {
	return cam.WorldToScreen(b.At)
}

// badgeCounts is a badge's second line: its sessions, and how many need
// you when any do.
func badgeCounts(b city.Badge) string {
	s := fmt.Sprintf("%d session", b.Sessions)
	if b.Sessions != 1 {
		s += "s"
	}
	if b.NeedsYou > 0 {
		s += fmt.Sprintf(" · %d need you", b.NeedsYou)
	}
	return s
}

// drawBadges draws every district's badge, centred on its district: its
// name, whose move it is in the move's tone, and its counts.
func (g *Game) drawBadges(screen *ebiten.Image, cam *city.Camera) {
	th := g.theme
	for _, b := range g.scene.Badges() {
		foot := badgeFoot(cam, b)
		lines := []struct {
			text string
			size ui.Size
			tone ui.Tone
			move bool
		}{
			{b.Sign.Name, ui.Body, 0, false},
			{b.Sign.Move, ui.Small, ui.MoveTone(b.Sign.Tone), true},
			{badgeCounts(b), ui.Small, 0, false},
		}
		width, height := 0.0, 0.0
		for _, l := range lines {
			if l.text == "" {
				continue
			}
			w, h := g.faces.Measure(l.text, l.size)
			width, height = max(width, w), height+h
		}
		pad := th.Grid() / 2
		rect := city.RectAt(foot.X-width/2-pad, foot.Y-height/2-pad, width+2*pad, height+2*pad)
		g.roundRect(screen, rect, th.Radius()/2, th.Palette.Panel)
		y := rect.Min.Y + pad
		for _, l := range lines {
			if l.text == "" {
				continue
			}
			w, h := g.faces.Measure(l.text, l.size)
			c := th.Palette.Text
			if l.move {
				c = th.Color(l.tone)
			} else if l.size == ui.Small {
				c = th.Palette.Dim
			}
			g.text(screen, city.Point{X: foot.X - w/2, Y: y}, l.text, l.size, c)
			y += h
		}
		g.noteHit(rect, city.Hit{Badge: b.District, District: b.District})
	}
}
