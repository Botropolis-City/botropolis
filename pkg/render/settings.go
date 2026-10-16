package render

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// settingsKeys answers the keyboard while the settings panel is open;
// it reports whether the panel is still open.
func (g *Game) settingsKeys() bool {
	just := g.just
	if just(ebiten.KeyEscape) || just(ebiten.KeyS) || just(ebiten.KeyQ) {
		return false
	}
	if just(ebiten.KeyP) {
		g.snap = screenshotPath(time.Now())
	}
	if g.onShortcutRow() && g.shortcutRowKeys() {
		return true
	}
	if just(ebiten.KeyArrowDown) || just(ebiten.KeyJ) {
		g.settings.Move(1)
	}
	if just(ebiten.KeyArrowUp) || just(ebiten.KeyK) {
		g.settings.Move(-1)
	}
	if just(ebiten.KeyArrowRight) || just(ebiten.KeyL) || just(ebiten.KeyEnter) || just(ebiten.KeySpace) {
		g.applySetting(g.settings.Adjust(1))
	}
	if just(ebiten.KeyArrowLeft) || just(ebiten.KeyH) {
		g.applySetting(g.settings.Adjust(-1))
	}
	return true
}

// applySetting takes a changed row live where it can, then hands it to
// the app to persist.
func (g *Game) applySetting(s ui.Setting) {
	note := fmt.Sprintf("%s: %s", s.Label, s.Value)
	switch s.Key {
	case config.KeyReducedMotion:
		g.reduced = s.Value == "on"
	case config.KeyRenderScale:
		scale, _ := strconv.ParseFloat(s.Value, 64)
		if err := g.setScale(scale); err != nil {
			g.SetStatus(err.Error())
			return
		}
	case config.KeyProjection:
		if p, ok := city.ParseProjection(s.Value); ok {
			g.scene.SetProjection(p)
		}
	case config.KeyDetail:
		if d, ok := city.ParseDetail(s.Value); ok {
			g.scene.SetDetail(d)
		}
	case config.KeyGroupLines:
		g.scene.SetGroupLines(s.Value == "on")
	case config.KeyShutteredDays, config.KeyOvergrownDays:
		days, _ := strconv.Atoi(s.Value)
		ladder := g.scene.Ladder()
		if s.Key == config.KeyShutteredDays {
			ladder.Shuttered = time.Duration(days) * 24 * time.Hour
		} else {
			ladder.Overgrown = time.Duration(days) * 24 * time.Hour
		}
		if ladder.Overgrown <= ladder.Shuttered {
			g.SetStatus("overgrown days must be more than shuttered days")
			return
		}
		g.scene.SetLadder(ladder)
	case config.KeyUsageRefresh:
		every, err := config.ParseUsageRefresh(s.Value)
		if err != nil {
			g.SetStatus(err.Error())
			return
		}
		g.usageEvery = every
		g.scene.SetGaugesManual(every <= 0)
	}
	if g.apply != nil {
		if err := g.apply(s); err != nil {
			g.SetStatus(err.Error())
			return
		}
	}
	g.SetStatus(note)
}

// setScale rebuilds the theme and faces for a new scale; 0 follows the
// display again.
func (g *Game) setScale(scale float64) error {
	if scale <= 0 {
		scale = ebiten.Monitor().DeviceScaleFactor()
	}
	theme := ui.NewTheme(scale)
	faces, err := newFaces(theme)
	if err != nil {
		return err
	}
	g.theme, g.faces = theme, faces
	return nil
}

func (g *Game) drawSettings(screen *ebiten.Image, width, height float64) {
	th := g.theme
	vector.FillRect(screen, 0, 0, float32(width), float32(height), colorScrim, false)
	p := ui.LayoutSettings(th, width, height, g.settings, g.faces.Measure)
	// Kept so a click lands on the row that was drawn. Bug 43.
	g.mu.Lock()
	g.settingsPanel = p
	g.mu.Unlock()
	g.roundPanel(screen, p.Rect)
	g.run(screen, p.Title, th.Palette.Text)
	g.run(screen, p.Subtitle, th.Palette.Dim)
	for _, row := range p.Rows {
		label, value := th.Palette.Dim, th.Palette.Dim
		if row.Selected {
			label, value = th.Palette.Text, th.Palette.Accent
		}
		g.run(screen, row.Label, label)
		g.run(screen, row.Value, value)
	}
	for _, line := range p.Note {
		g.run(screen, line, th.Palette.Text)
	}
}

// clickSettings moves the cursor to the row under the pointer and
// presses Enter on it, which is exactly what the keyboard does — the
// click borrows the key rather than repeating the action behind it.
func (g *Game) clickSettings(cursor city.Point) bool {
	if !g.settingsOpen {
		return false
	}
	g.mu.Lock()
	panel := g.settingsPanel
	g.mu.Unlock()
	if !panel.Rect.Contains(cursor) {
		// A click off the panel closes it, the way Escape does, and
		// stops recording the global shortcut.
		g.settingsOpen = false
		g.recordingShortcut, g.pendingShortcut = false, nil
		return true
	}
	if i, ok := panel.Hit(cursor); ok {
		g.settings.Cursor = i
		g.clicked = ebiten.KeyEnter
	}
	return true
}
