package render

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// lensPanel is the lens introducing itself (item 88), a tenant of the
// overlay stack that never takes the keyboard: the map behind it keeps
// working, and only a click on its own chip reaches it.
type lensPanel struct {
	laid ui.LensPanel
}

func (*lensPanel) modal() bool              { return false }
func (*lensPanel) keys(*Game) bool          { return true }
func (p *lensPanel) hit(at city.Point) bool { return p.laid.Rect.Contains(at) }

func (p *lensPanel) click(g *Game, at city.Point) bool {
	if p.laid.Chip {
		g.scene.ShowLensPanel()
	}
	return true
}

func (p *lensPanel) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	if !g.lensPanelShown() {
		p.laid = ui.LensPanel{}
		return
	}
	g.mu.Lock()
	plate := g.searchPlate
	g.mu.Unlock()
	p.laid = ui.LayoutLensPanel(th, g.scene.LensPanel(), lensBounds(th, g.scene.TopChrome(), width, height, g.sidebar, plate), g.faces.Measure)
	l := p.laid
	g.roundPanel(screen, l.Rect)
	if l.Chip {
		g.run(screen, l.Name, th.Palette.Accent)
		return
	}
	g.run(screen, l.Name, th.Palette.Text)
	for _, t := range l.About {
		g.run(screen, t, th.Palette.Text)
	}
	if l.Bar.Width() > 0 {
		const steps = 24
		w := l.Bar.Width() / steps
		for i := range steps {
			c := ui.Sequential.At(float64(i) / (steps - 1))
			vector.FillRect(screen, float32(l.Bar.Min.X+w*float64(i)), float32(l.Bar.Min.Y), float32(w+1), float32(l.Bar.Height()), c, false)
		}
		g.run(screen, l.Low, th.Palette.Dim)
		g.run(screen, l.High, th.Palette.Dim)
	}
	for i, sw := range l.Swatches {
		vector.FillRect(screen, float32(sw.Min.X), float32(sw.Min.Y), float32(sw.Width()), float32(sw.Height()), ui.Category(i), false)
		g.run(screen, l.Labels[i], th.Palette.Dim)
	}
	g.run(screen, l.Note, th.Palette.Dim)
	g.run(screen, l.Aggregate, th.Palette.Text)
	for _, t := range l.Do {
		g.run(screen, t, th.Palette.Dim)
	}
}

// lensBounds is where the lens panel may sit: under the strip, right of
// the sidebar, and under the finder's query when one is left there.
func lensBounds(th ui.Theme, top, width, height float64, sidebar bool, plate city.Rect) city.Rect {
	bounds := city.RectAt(0, top, width, height-top)
	if sidebar {
		bounds.Min.X = ui.SidebarWidth(th)
	}
	if plate.Area() > 0 {
		bounds.Min.Y = max(bounds.Min.Y, plate.Max.Y)
	}
	return bounds
}

// lensPanelShown is whether the lens panel is drawn: not while a panel
// that takes the keyboard, such as the finder, is open over the map.
func (g *Game) lensPanelShown() bool { return g.overlays.top() == nil }

// selectLens is a lens's key, and says which lens and view are up,
// because a mode you entered is a mode you can forget you are in.
func (g *Game) selectLens(l city.Lens) {
	g.scene.SelectLens(l)
	g.SetStatus(l.Name() + " — " + g.scene.View().Name() + ": " + g.scene.View().Question())
}
