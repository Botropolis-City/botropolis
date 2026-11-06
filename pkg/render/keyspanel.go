package render

import (
	"runtime"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// The settings' Keys panel (item 350): every binding of config.Bindings
// under where it answers, what it does and its key, and a way to change
// one. Enter records the next key, which is refused on the row's own
// line when another binding or the city's grammar holds it, Backspace
// puts the row back to its default, and esc goes back to the settings.

// KeysRow is the settings' row that opens the panel, which the app
// lists last. It holds no value:
// choosing it opens the panel, and nothing of it is saved.
var KeysRow = ui.Setting{Key: config.KeySettings, Label: "keys", Value: "every key; enter changes them"}

// keyBinding is one row of the panel: the binding and what it does.
type keyBinding struct {
	name, section, label string
}

const (
	sectionMap    = "At the map"
	sectionMenu   = "In the action menu"
	sectionFinder = "In the finder"
)

// keysPanel is the panel as an overlay.
type keysPanel struct {
	rows      []keyBinding
	focus     int
	recording bool
	// refused is why the key last pressed for a row was not taken, by
	// its index.
	refused map[int]string
}

// openKeysPanel puts the panel over the settings, on its first row.
func (g *Game) openKeysPanel() {
	g.overlays.push(&keysPanel{rows: keyBindings(), refused: map[int]string{}})
}

// onKeysRow is whether the settings cursor is on the keys row.
func (g *Game) onKeysRow() bool {
	items := g.settings.Items
	return g.settings.Cursor < len(items) && items[g.settings.Cursor].Key == config.KeySettings
}

// keyBindings is every binding as the panel lists it: the map's, then
// the menu's, then the finder's, each in the table's order.
func keyBindings() []keyBinding {
	var rows []keyBinding
	for _, section := range []string{sectionMap, sectionMenu, sectionFinder} {
		for _, b := range config.Bindings {
			if sectionOf(b.Scope) == section {
				rows = append(rows, keyBinding{name: b.Name, section: section, label: bindingLabel(b.Name)})
			}
		}
	}
	return rows
}

func sectionOf(s config.Scope) string {
	switch s {
	case config.ScopeFinder:
		return sectionFinder
	case config.ScopeMenu, config.ScopeMenuSession, config.ScopeMenuProject, config.ScopeMenuGroup:
		return sectionMenu
	}
	return sectionMap
}

// ownLabels are what the bindings say that share a reference row with
// another, or have none: each needs words of its own.
var ownLabels = map[string]string{
	"lens_1": "the first lens", "lens_2": "the second lens", "lens_3": "the third lens",
	"lens_4": "the fourth lens", "lens_5": "the fifth lens", "lens_6": "the sixth lens",
	"fit": "Skyline, the whole city", "fit_zero": "Skyline, the whole city, again",
	"clock_back": "the clock an hour back", "clock_on": "the clock an hour on",
	"finder_slash": "find a session", "find": "find a session, from anywhere",
	"find_copy": "copy how to resume the session", "find_show": "show the session on the map",
	"menu_session": "the Session column", "menu_project": "the Project column", "menu_group": "the Group column",
}

// bindingLabel is what a binding does, in the reference's words.
func bindingLabel(name string) string {
	if label, ok := ownLabels[name]; ok {
		return label
	}
	for key, row := range boundRows {
		if !slices.Contains(row.names, name) {
			continue
		}
		for _, k := range bindings {
			if k.Key == key {
				return k.Action
			}
		}
	}
	var kinds []string
	for kind, bound := range menuKeys {
		if bound == name {
			kinds = append(kinds, string(kind))
		}
	}
	slices.Sort(kinds)
	if len(kinds) > 0 {
		return strings.Join(kinds, " or ")
	}
	return strings.ReplaceAll(name, "_", " ")
}

// keyRows is the panel's rows with each key as it is bound now.
func (p *keysPanel) keyRows(g *Game) []ui.KeyRow {
	chords := g.findChords()
	var rows []ui.KeyRow
	for i, b := range p.rows {
		key := chords.Bound[b.name].Label()
		if b.name == "menu_demolish" {
			key += " " + key
		}
		rows = append(rows, ui.KeyRow{Section: b.section, Label: b.label, Key: key, Recording: p.recording && i == p.focus, Refused: p.refused[i]})
	}
	return rows
}

func (p *keysPanel) layout(g *Game, width, height float64) ui.KeysPanel {
	_, _, measure := g.metrics()
	return ui.LayoutKeysPanel(g.theme, width, height, p.keyRows(g), p.focus, measure)
}

func (p *keysPanel) keys(g *Game) bool {
	if p.recording {
		p.record(g)
		return true
	}
	var starts []int
	for i, r := range p.rows {
		if i == 0 || r.section != p.rows[i-1].section {
			starts = append(starts, i)
		}
	}
	switch g.listKeys(&p.focus, len(p.rows), listShape{starts: starts}) {
	case listBack:
		return false
	case listChoose:
		p.recording = true
		delete(p.refused, p.focus)
		return true
	case listMoved:
		return true
	}
	if g.just(ebiten.KeyBackspace) || g.just(ebiten.KeyDelete) {
		p.reset(g)
	}
	return true
}

// record takes the next key pressed, with what is held with it, for the
// focused row: esc cancels, and a key the table refuses is said why on
// the row and not saved.
func (p *keysPanel) record(g *Game) {
	if g.just(ebiten.KeyEscape) {
		p.recording = false
		return
	}
	key, ok := g.justPressedKey()
	if !ok {
		return
	}
	p.recording = false
	chord := config.Chord{
		Ctrl:  g.scriptedCtrl || ebiten.IsKeyPressed(ebiten.KeyControl),
		Alt:   g.scriptedAlt || ebiten.IsKeyPressed(ebiten.KeyAlt),
		Shift: g.shifted(),
		Cmd:   g.scriptedCmd || ebiten.IsKeyPressed(ebiten.KeyMeta),
		Key:   strings.ToLower(key.String()),
	}
	name := p.rows[p.focus].name
	if err := config.Rebind(g.findChords(), name, chord, runtime.GOOS); err != nil {
		p.refused[p.focus] = err.Error()
		return
	}
	g.rebind(name, chord, chord.String())
}

// reset puts the focused row back to its default.
func (p *keysPanel) reset(g *Game) {
	name := p.rows[p.focus].name
	for _, b := range config.Bindings {
		if b.Name != name {
			continue
		}
		chord, err := config.ParseChord(name, b.Default, runtime.GOOS, KnownKey)
		if err != nil {
			p.refused[p.focus] = err.Error()
			return
		}
		delete(p.refused, p.focus)
		g.rebind(name, chord, "")
	}
}

// rebind binds a key at once and saves it; an empty value saves the
// binding back to its default, which is no line in the config file.
func (g *Game) rebind(name string, chord config.Chord, value string) {
	chords := g.findChords().With(name, chord)
	g.chords = &chords
	if g.apply == nil {
		return
	}
	if err := g.apply(ui.Setting{Key: config.KeySettings + "." + name, Value: value}); err != nil {
		g.failed(err)
	}
}

// click focuses the row under the pointer; anywhere else on the panel
// does nothing, and off it the panel closes.
func (p *keysPanel) click(g *Game, at city.Point) bool {
	size := g.scene.Size()
	laid := p.layout(g, size.X, size.Y)
	for _, row := range laid.Rows {
		if row.Row.Contains(at) {
			p.focus = row.Index
			return true
		}
	}
	return laid.Rect.Contains(at)
}

func (p *keysPanel) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	laid := p.layout(g, width, height)
	g.roundPanel(screen, laid.Rect)
	g.run(screen, laid.Title, th.Palette.Text)
	for _, h := range laid.Headings {
		g.run(screen, h, th.Palette.Dim)
	}
	for _, row := range laid.Rows {
		if row.Focused {
			g.roundRect(screen, row.Row.Inset(th.Grid()/2), th.Radius()/2, th.Palette.Hairline)
		}
		g.run(screen, row.Label, th.Palette.Text)
		g.keyCap(screen, row.Cap)
		if row.Note.Text != "" {
			g.run(screen, row.Note, th.Palette.Error)
		}
	}
}
