package render

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// New session starts a session in any directory with its group already
// guessed: a directory completed with Tab, recent ones first, a prompt,
// and the group guessed from both, with its reason, recomputed as the
// prompt is typed. Enter keeps the guess, Tab walks the alternatives,
// and typing a name makes a new group (item 87).

const (
	fieldDir = iota
	fieldPrompt
	fieldGroup
)

type newSession struct {
	fields  [3]field
	focus   int
	guesser *group.Guesser
	recent  []string
	guess   group.Guess
	// alt is which of the guess's alternatives, then no group, is
	// chosen: 0 is the guess.
	alt int
	// completions are the directories Tab is cycling through, and
	// completing the one shown.
	completions []string
	completing  int
	asked       string
	rect        city.Rect
}

func (g *Game) newSession() *newSession {
	n, _ := g.overlays.top().(*newSession)
	return n
}

// openNewSession opens the panel with the selected district's directory
// in it, ready to edit.
func (g *Game) openNewSession() {
	if g.newSession() != nil {
		return
	}
	n := &newSession{focus: fieldDir}
	if g.groupSource != nil {
		n.guesser, n.recent = g.groupSource()
	}
	if b := g.scene.Selected(); b != nil {
		n.fields[fieldDir].insert(city.ProjectRoot(b.Session.CWD))
		n.focus = fieldPrompt
	}
	n.reguess(g)
	g.overlays.push(n)
}

// reguess weighs the directory and prompt as they stand.
func (n *newSession) reguess(g *Game) {
	asked := n.fields[fieldDir].query() + "\x00" + n.fields[fieldPrompt].query()
	if n.guesser == nil || asked == n.asked {
		return
	}
	n.asked, n.alt = asked, 0
	n.guess = n.guesser.Guess(group.Member{ID: "new session", Dir: n.fields[fieldDir].query(),
		Started: g.scene.Clock(), Prompts: []string{n.fields[fieldPrompt].query()}})
	if n.guess.Group == "" {
		// With no guess, the walk starts at no group and the groups follow.
		n.alt = len(n.guess.Alternatives)
	}
}

// choice is where the session will start: the alternative chosen, a new
// group by the name typed, or no group.
func (n *newSession) choice() city.Action {
	if name := strings.TrimSpace(n.fields[fieldGroup].query()); name != "" {
		return city.Action{GroupName: name}
	}
	if n.alt < len(n.guess.Alternatives) {
		return city.Action{Group: n.guess.Alternatives[n.alt]}
	}
	return city.Action{}
}

func (n *newSession) keys(g *Game) bool {
	f := &n.fields[n.focus]
	switch {
	case g.just(ebiten.KeyEscape):
		return false
	case g.just(ebiten.KeyEnter), g.just(ebiten.KeyKPEnter):
		if n.focus == fieldDir {
			n.focus = fieldPrompt
			return true
		}
		dir := strings.TrimSpace(n.fields[fieldDir].query())
		if dir == "" {
			g.SetStatus("New session needs a directory")
			return true
		}
		action := n.choice()
		action.Kind, action.Dir, action.Prompt = city.ActionNewSession, dir, n.fields[fieldPrompt].query()
		g.act(action)
		g.SetStatus("starting a session in " + dir)
		return false
	case g.just(ebiten.KeyTab):
		if n.focus == fieldDir {
			n.complete()
		} else {
			n.alt = (n.alt + 1) % (len(n.guess.Alternatives) + 1)
		}
	case g.just(ebiten.KeyArrowDown):
		n.focus = min(n.focus+1, fieldGroup)
	case g.just(ebiten.KeyArrowUp):
		n.focus = max(n.focus-1, fieldDir)
	case g.just(ebiten.KeyBackspace):
		f.backspace()
		n.completions = nil
	}
	if g.scriptedRune != 0 {
		f.insert(string(g.scriptedRune))
		n.completions = nil
	}
	n.reguess(g)
	return true
}

// complete replaces the directory with the next completion of what was
// typed, recently used directories first.
func (n *newSession) complete() {
	f := &n.fields[fieldDir]
	if n.completions == nil {
		n.completions, n.completing = completeDir(f.query(), n.recent), -1
	}
	if len(n.completions) == 0 {
		return
	}
	n.completing = (n.completing + 1) % len(n.completions)
	*f = field{}
	f.insert(n.completions[n.completing])
}

// completeDir is every directory that completes prefix: the recently
// used first, in the order given, then the filesystem's, by name.
func completeDir(prefix string, recent []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, d := range recent {
		if strings.HasPrefix(d, prefix) && !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	parent := filepath.Dir(prefix)
	if strings.HasSuffix(prefix, "/") {
		parent = strings.TrimSuffix(prefix, "/")
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		return out
	}
	var found []string
	for _, e := range entries {
		path := filepath.Join(parent, e.Name())
		if e.IsDir() && strings.HasPrefix(path, prefix) && !seen[path] {
			found = append(found, path)
		}
	}
	sort.Strings(found)
	return append(out, found...)
}

func (n *newSession) click(g *Game, at city.Point) bool {
	return n.rect.Contains(at)
}

func (n *newSession) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	scrim := th.Palette.Panel
	scrim.A = 0x60
	g.roundRect(screen, city.RectAt(0, 0, width, height), 0, scrim)
	grid := th.Grid()
	_, title := g.faces.Measure("", ui.Title)
	_, body := g.faces.Measure("", ui.Body)
	_, small := g.faces.Measure("", ui.Small)
	panelWidth := min(width-grid*8, grid*90)
	panel := city.RectAt((width-panelWidth)/2, grid*10, panelWidth, grid*6+title+body*3+small*3+grid*3)
	n.rect = panel
	g.roundRect(screen, panel, th.Radius(), th.Palette.Panel)
	inner := panelWidth - grid*4
	x, y := panel.Min.X+grid*2, panel.Min.Y+grid*2
	g.text(screen, city.Point{X: x, Y: y}, "New session", ui.Title, th.Palette.Text)
	y += title + grid
	labels := [3]string{"directory", "prompt", "group"}
	values := [3]string{n.fields[fieldDir].query(), n.fields[fieldPrompt].query(), n.groupLine(g)}
	for i := range labels {
		ink := th.Palette.Dim
		if i == n.focus {
			ink = th.Palette.Text
			g.roundRect(screen, city.RectAt(panel.Min.X+grid, y-grid/4, panelWidth-grid*2, body+grid/2), 0, th.Palette.Hairline)
		}
		g.text(screen, city.Point{X: x, Y: y}, labels[i], ui.Body, th.Palette.Dim)
		value := values[i]
		if i == n.focus && i != fieldGroup {
			value += "▏"
		}
		g.text(screen, city.Point{X: x + grid*12, Y: y}, g.clip(value, inner-grid*12, ui.Body), ui.Body, ink)
		y += body + grid/2
	}
	if why := n.why(g); why != "" {
		g.text(screen, city.Point{X: x + grid*12, Y: y}, g.clip(why, inner-grid*12, ui.Small), ui.Small, th.Palette.Dim)
	}
	y += small + grid/2
	g.text(screen, city.Point{X: x + grid*12, Y: y}, g.clip("Tab: "+strings.Join(n.alternatives(g), " · "), inner-grid*12, ui.Small), ui.Small, th.Palette.Dim)
	y += small + grid
	g.text(screen, city.Point{X: x, Y: y}, "↑ ↓ field   Tab completes the directory, or walks the groups   type a name for a new group   enter starts   esc cancels", ui.Small, th.Palette.Accent)
}

// groupLine is the group the session will start in, by name.
func (n *newSession) groupLine(g *Game) string {
	if typed := n.fields[fieldGroup].query(); typed != "" {
		return typed + " (new group)"
	}
	return n.nameOf(g, n.choice().Group)
}

func (n *newSession) nameOf(g *Game, id string) string {
	if id == "" {
		return noGroup
	}
	if n.guesser != nil {
		for _, gr := range n.guesser.Groups() {
			if gr.ID == id {
				return gr.Name
			}
		}
	}
	return id
}

// why is the guess's reason, while the guess is what is chosen.
func (n *newSession) why(g *Game) string {
	if n.fields[fieldGroup].query() != "" || n.guess.Group == "" || n.alt != 0 {
		return ""
	}
	tier := map[group.Tier]string{group.Sure: "because it ", group.Likely: "likely, because it ", group.Possible: "perhaps, because it "}[n.guess.Tier]
	return tier + n.guess.Reason.Describe(state.TitleOf(g.scene.Snapshot().Sessions))
}

func (n *newSession) alternatives(g *Game) []string {
	var names []string
	for i, id := range n.guess.Alternatives {
		if i != n.alt {
			names = append(names, n.nameOf(g, id))
		}
	}
	if n.alt != len(n.guess.Alternatives) {
		names = append(names, noGroup)
	}
	return append(names, "or type a new group")
}
