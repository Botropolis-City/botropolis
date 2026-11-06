package render

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// board is City Hall's board of groups, a panel on the overlay stack:
// one row per group, and enter focuses the map on the one chosen
// (item 87).
type board struct {
	selected int
	rect     city.Rect
	// newButton is New session's button on the board, and quietButton
	// Pack up everything quiet's (item 333).
	newButton   city.Rect
	quietButton city.Rect
	// tidyButton opens the tidy-up walk (item 333).
	tidyButton city.Rect
}

type boardRow struct {
	Name string
	// Members names the members on the map, and Count how many they are
	// (item 333).
	Members  string
	Count    string
	Move     string
	Waited   string
	LastTime string
	Next     string
}

func boardRows(groups []state.Group, now time.Time) []boardRow {
	rows := make([]boardRow, 0, len(groups))
	for _, g := range groups {
		rows = append(rows, boardRow{
			Name:     g.Name,
			Members:  city.MemberList(g.MemberTitles, 4),
			Count:    plural(len(g.Members), "session"),
			Move:     string(g.Move) + ": " + g.Why,
			Waited:   format.Age(now.Sub(g.Journal.Active)),
			LastTime: g.Journal.LastTime,
			Next:     g.Journal.Next,
		})
	}
	return rows
}

func boardStatus(rows []boardRow) string {
	if len(rows) == 0 {
		return "no groups yet: sessions join one when they share a ticket, a link or their wording"
	}
	return fmt.Sprintf("%d groups   ↑ ↓ choose, enter focuses it on the map, c copies its stand-up, F2 renames it, Delete closes it, esc closes", len(rows))
}

func (g *Game) board() *board {
	b, _ := g.overlays.top().(*board)
	return b
}

func (g *Game) openBoard() {
	if g.board() != nil {
		return
	}
	g.overlays.push(&board{})
}

// clickLandmark is what a click on a landmark opens beside selecting it:
// the plant its breakdown, City Hall its board, the clinic its menu.
func (g *Game) clickLandmark(l city.Landmark) {
	switch l {
	case city.LandmarkPlant:
		g.breakdown, g.window = true, city.LastDay
	case city.LandmarkHall:
		g.openBoard()
	case city.LandmarkClinic:
		g.openClinic()
	}
}

func (b *board) keys(g *Game) bool {
	groups := g.scene.Groups()
	switch {
	case g.just(ebiten.KeyEscape):
		return false
	case g.just(ebiten.KeyEnter), g.just(ebiten.KeyKPEnter):
		if b.selected < len(groups) {
			g.scene.FocusGroup(groups[b.selected].ID)
		}
		return false
	case g.just(ebiten.KeyC):
		if b.selected < len(groups) {
			line := g.scene.Standup(groups[b.selected].ID)
			g.Notify("copied stand-up: "+line, ui.ToneNone, "")
			g.act(city.Action{Kind: city.ActionCopy, Text: line})
		}
	case g.just(ebiten.KeyP):
		g.sweepQuiet()
	case g.just(ebiten.KeyT):
		b.tidyUp(g)
	case g.just(ebiten.KeyDelete):
		if b.selected < len(groups) {
			b.closeKey(g, groups[b.selected])
		}
	case g.just(ebiten.KeyF2):
		if b.selected < len(groups) {
			g.openRenamer(groups[b.selected].ID, groups[b.selected].Name)
		}
	case g.just(ebiten.KeyArrowDown):
		b.selected = max(0, min(b.selected+1, len(groups)-1))
	case g.just(ebiten.KeyArrowUp):
		b.selected = max(0, b.selected-1)
	}
	return true
}

// tidyUp opens the walk in the board's place, or says there is nothing
// to tidy (item 333).
func (b *board) tidyUp(g *Game) {
	g.overlays.remove(b)
	if !g.openWalk() {
		g.overlays.push(b)
		g.Notify("nothing to tidy: every group was settled and nothing in no group is quiet", ui.ToneNone, "")
	}
}

// closeKey asks before the chosen group is closed, so one key never
// packs a group up; Delete again answers (items 333, 352).
func (b *board) closeKey(g *Game, gr state.Group) {
	g.overlays.push(closeGroupDialog(gr.Name, gr.ID))
}

// closeOffer is what a group's row says when closing it is the next
// thing to do: every pull request in it merged (item 333).
func (g *Game) closeOffer(id string) string {
	if g.scene.AllMerged(id) {
		return "every pull request merged: Delete closes it"
	}
	return ""
}

// click closes the board when the press lands outside it.
func (b *board) click(g *Game, at city.Point) bool {
	g.mu.Lock()
	panel, button, quiet, tidy := b.rect, b.newButton, b.quietButton, b.tidyButton
	g.mu.Unlock()
	if tidy.Contains(at) {
		b.tidyUp(g)
		return true
	}
	if button.Contains(at) {
		g.openNewSession()
		return true
	}
	if quiet.Contains(at) {
		g.sweepQuiet()
		return true
	}
	return panel.Contains(at)
}

func (b *board) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	scrim := th.Palette.Panel
	scrim.A = 0x60
	g.roundRect(screen, city.RectAt(0, 0, width, height), 0, scrim)
	rows := boardRows(g.scene.Groups(), g.now())
	grid := th.Grid()
	_, title := g.faces.Measure("", ui.Title)
	_, body := g.faces.Measure("", ui.Body)
	_, small := g.faces.Measure("", ui.Small)
	rowHeight := body + small*3 + grid
	panelWidth := min(width-grid*8, grid*120)
	shown := min(len(rows), max(1, int((height-grid*16-title-small)/rowHeight)))
	panel := city.RectAt((width-panelWidth)/2, grid*6, panelWidth, grid*4+title+small+grid+float64(shown)*rowHeight)
	g.mu.Lock()
	b.rect = panel
	g.mu.Unlock()
	g.roundRect(screen, panel, th.Radius(), th.Palette.Panel)
	inner := panelWidth - grid*4
	x, y := panel.Min.X+grid*2, panel.Min.Y+grid*2
	g.text(screen, city.Point{X: x, Y: y}, "City Hall · groups", ui.Title, th.Palette.Text)
	label := "New session  " + g.findChords().New.String()
	lw, lh := g.faces.Measure(label, ui.Small)
	button := city.RectAt(panel.Max.X-grid*2-lw-grid*2, y, lw+grid*2, lh+grid/2)
	g.mu.Lock()
	b.newButton = button
	g.mu.Unlock()
	g.roundRect(screen, button, th.Radius()/2, th.Palette.Hairline)
	g.text(screen, city.Point{X: button.Min.X + grid, Y: button.Min.Y + grid/4}, label, ui.Small, th.Palette.Accent)
	quietText := g.quietButtonLabel(len(g.quietIDs())) + "  p"
	qw, qh := g.faces.Measure(quietText, ui.Small)
	quiet := city.RectAt(button.Min.X-grid-qw-grid*2, y, qw+grid*2, qh+grid/2)
	g.mu.Lock()
	b.quietButton = quiet
	g.mu.Unlock()
	tidyText := "Tidy up  t"
	tw, th2 := g.faces.Measure(tidyText, ui.Small)
	tidy := city.RectAt(quiet.Min.X-grid-tw-grid*2, y, tw+grid*2, th2+grid/2)
	g.mu.Lock()
	b.tidyButton = tidy
	g.mu.Unlock()
	g.roundRect(screen, tidy, th.Radius()/2, th.Palette.Hairline)
	g.text(screen, city.Point{X: tidy.Min.X + grid, Y: tidy.Min.Y + grid/4}, tidyText, ui.Small, th.Palette.Accent)
	g.roundRect(screen, quiet, th.Radius()/2, th.Palette.Hairline)
	g.text(screen, city.Point{X: quiet.Min.X + grid, Y: quiet.Min.Y + grid/4}, quietText, ui.Small, th.Palette.Accent)
	y += title
	status := boardStatus(rows)
	g.text(screen, city.Point{X: x, Y: y}, status, ui.Small, th.Palette.Dim)
	y += small + grid
	first := max(0, min(b.selected-shown+1, len(rows)-shown))
	for i := first; i < first+shown && i < len(rows); i++ {
		row := rows[i]
		if i == b.selected {
			g.roundRect(screen, city.RectAt(panel.Min.X+grid, y-grid/4, panelWidth-grid*2, rowHeight-grid/2), 0, th.Palette.Hairline)
		}
		head := g.clip(row.Name+"  · "+row.Count, inner*0.45, ui.Body)
		g.text(screen, city.Point{X: x, Y: y}, head, ui.Body, th.Palette.Text)
		move := g.clip(row.Move+"   waited "+row.Waited, inner*0.53, ui.Small)
		mw, _ := g.faces.Measure(move, ui.Small)
		g.text(screen, city.Point{X: x + inner - mw, Y: y}, move, ui.Small, th.Color(ui.MoveTone(g.scene.Groups()[i].Move)))
		members := "members    " + row.Members
		if offer := g.closeOffer(g.scene.Groups()[i].ID); offer != "" {
			members += "   " + offer
		}
		g.text(screen, city.Point{X: x, Y: y + body}, g.clip(members, inner, ui.Small), ui.Small, th.Palette.Dim)
		g.text(screen, city.Point{X: x, Y: y + body + small}, g.clip("last time  "+row.LastTime, inner, ui.Small), ui.Small, th.Palette.Dim)
		g.text(screen, city.Point{X: x, Y: y + body + small*2}, g.clip("next       "+row.Next, inner, ui.Small), ui.Small, th.Palette.Dim)
		y += rowHeight
	}
}

// clip cuts text to a width, ending it with an ellipsis when it is cut.
// The clips are remembered for the faces they were measured in.
func (g *Game) clip(s string, width float64, size ui.Size) string {
	if g.clips == nil || g.clipsIn != g.faces {
		g.clips, g.clipsIn = newClipper(g.faces.Measure), g.faces
	}
	return g.clips.clip(s, width, size)
}

// clipTo is s whole if it fits in width, or else its longest prefix that
// fits with an ellipsis after it, or "" when not even the ellipsis does.
//
// It doubles a prefix until it no longer fits and halves back, so it
// measures no string much longer than what fits. Cutting a rune at a time
// from the end put hundreds of near-whole copies of a long row through
// the shaper every frame, and Ebitengine's shaping cache kept 51 MB of
// them (item 162). It takes the whole string to be too wide once a prefix
// of half of it or less is.
func clipTo(s string, width float64, size ui.Size, measure ui.Measure) string {
	r := []rune(s)
	fits := func(k int) bool {
		w, _ := measure(string(r[:k])+"…", size)
		return w <= width
	}
	whole := func() bool {
		w, _ := measure(s, size)
		return w <= width
	}
	lo, hi := -1, len(r)
	for k := clipFirstProbe; ; k *= 2 {
		if k >= len(r) {
			if whole() {
				return s
			}
			break
		}
		if !fits(k) {
			if len(r) < 2*k && whole() {
				return s
			}
			hi = k
			break
		}
		lo = k
	}
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		if fits(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	if lo < 0 {
		return ""
	}
	return string(r[:lo]) + "…"
}

// clipFirstProbe is the first prefix clipTo tries, in runes.
const clipFirstProbe = 8

// clipMemoLimit is how many clips a clipper keeps before starting over:
// more than any frame's rows, so a panel drawn again measures nothing.
const clipMemoLimit = 1024

type clipKey struct {
	s     string
	width float64
	size  ui.Size
}

// clipper remembers each clip it has made, so a row drawn again unchanged
// is not clipped again.
type clipper struct {
	measure ui.Measure
	memo    map[clipKey]string
}

func newClipper(measure ui.Measure) *clipper {
	return &clipper{measure: measure, memo: map[clipKey]string{}}
}

func (c *clipper) clip(s string, width float64, size ui.Size) string {
	key := clipKey{s, width, size}
	if out, ok := c.memo[key]; ok {
		return out
	}
	if len(c.memo) >= clipMemoLimit {
		clear(c.memo)
	}
	out := clipTo(s, width, size, c.measure)
	c.memo[key] = out
	return out
}
