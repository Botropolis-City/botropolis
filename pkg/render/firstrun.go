package render

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// The first run proposes the groups found in the last month on one
// screen, where they can be kept, merged or split. It has two ways out:
// confirm keeps what is on the screen, and ask me later, or esc, brings
// it back at the next launch. Nothing is accepted quietly (item 87).
type firstRun struct {
	review group.Review
	// marked is the row m picked to merge, or -1.
	marked   int
	selected int
	rect     city.Rect
}

type firstRunLine struct {
	Name     string
	Sessions string
	Dirs     string
	Why      string
}

func (g *Game) firstRun() *firstRun {
	f, _ := g.overlays.top().(*firstRun)
	return f
}

// maybeFirstRun opens the first run when the groups have never been
// reviewed and there are some, once a launch, and only for someone
// looking: a screenshot, a recording or a driven run is not asked.
func (g *Game) maybeFirstRun() {
	if g.firstRunAsked || g.screenshot != "" || g.record != "" || g.driven {
		return
	}
	if g.openFirstRun() {
		g.firstRunAsked = true
	}
}

// openFirstRun puts the first run on the stack when there is something
// to review, and reports whether it did.
func (g *Game) openFirstRun() bool {
	snap := g.scene.Snapshot()
	if snap.GroupsReviewed || len(snap.Groups) == 0 {
		return false
	}
	g.overlays.push(&firstRun{review: reviewOf(snap.Groups), marked: -1})
	return true
}

func reviewOf(groups []state.Group) group.Review {
	var r group.Review
	for _, gr := range groups {
		row := group.ReviewRow{ID: gr.ID, Name: gr.Name, Reason: gr.Reason, Keep: true}
		for i, id := range gr.Members {
			dir := ""
			if i < len(gr.MemberDirs) {
				dir = gr.MemberDirs[i]
			}
			row.Members = append(row.Members, group.ReviewMember{Session: id, Dir: dir})
		}
		r.Rows = append(r.Rows, row)
	}
	return r
}

func (f *firstRun) title() string {
	return fmt.Sprintf("%d groups in the last 30 days", len(f.review.Rows))
}

func (f *firstRun) lines() []firstRunLine {
	lines := make([]firstRunLine, 0, len(f.review.Rows))
	for _, row := range f.review.Rows {
		dirs := row.Dirs()
		names := make([]string, 0, len(dirs))
		for _, d := range dirs {
			names = append(names, filepath.Base(d))
		}
		lines = append(lines, firstRunLine{
			Name:     row.Name,
			Sessions: fmt.Sprintf("%d sessions", len(row.Members)),
			Dirs:     strings.Join(names, ", "),
			Why:      row.Reason,
		})
	}
	return lines
}

func (f *firstRun) keys(g *Game) bool {
	switch {
	case g.just(ebiten.KeyEscape), g.just(ebiten.KeyA):
		return false
	case g.just(ebiten.KeyEnter), g.just(ebiten.KeyKPEnter):
		review := f.review
		g.act(city.Action{Kind: city.ActionReviewGroups, Review: &review})
		g.SetStatus("groups confirmed")
		return false
	case g.just(ebiten.KeyArrowDown):
		f.selected = min(f.selected+1, len(f.review.Rows)-1)
	case g.just(ebiten.KeyArrowUp):
		f.selected = max(0, f.selected-1)
	case g.just(ebiten.KeySpace):
		f.review.Toggle(f.selected)
	case g.just(ebiten.KeyS):
		f.review.Split(f.selected)
		f.selected = max(0, min(f.selected, len(f.review.Rows)-1))
	case g.just(ebiten.KeyM):
		if f.marked < 0 || f.marked == f.selected {
			f.marked = f.selected
			break
		}
		into := f.selected
		if f.marked < into {
			into--
		}
		f.review.Merge(f.marked, f.selected)
		f.selected, f.marked = max(0, into), -1
	}
	return true
}

func (f *firstRun) click(g *Game, at city.Point) bool {
	return true
}

func (f *firstRun) draw(g *Game, screen *ebiten.Image, width, height float64) {
	th := g.theme
	scrim := th.Palette.Panel
	scrim.A = 0xa0
	g.roundRect(screen, city.RectAt(0, 0, width, height), 0, scrim)
	grid := th.Grid()
	_, title := g.faces.Measure("", ui.Title)
	_, body := g.faces.Measure("", ui.Body)
	_, small := g.faces.Measure("", ui.Small)
	rowHeight := body + small + grid
	lines := f.lines()
	panelWidth := min(width-grid*8, grid*110)
	shown := min(len(lines), max(1, int((height-grid*20-title-small*3)/rowHeight)))
	panel := city.RectAt((width-panelWidth)/2, grid*6, panelWidth, grid*5+title+small*3+grid*2+float64(shown)*rowHeight)
	f.rect = panel
	g.roundRect(screen, panel, th.Radius(), th.Palette.Panel)
	inner := panelWidth - grid*4
	x, y := panel.Min.X+grid*2, panel.Min.Y+grid*2
	g.text(screen, city.Point{X: x, Y: y}, f.title(), ui.Title, th.Palette.Text)
	y += title
	g.text(screen, city.Point{X: x, Y: y}, "found from what your sessions said and how they were started; nothing is kept until you confirm", ui.Small, th.Palette.Dim)
	y += small + grid
	first := max(0, min(f.selected-shown+1, len(lines)-shown))
	for i := first; i < first+shown && i < len(lines); i++ {
		l := lines[i]
		if i == f.selected {
			g.roundRect(screen, city.RectAt(panel.Min.X+grid, y-grid/4, panelWidth-grid*2, rowHeight-grid/2), 0, th.Palette.Hairline)
		}
		ink := th.Palette.Text
		state := "keep"
		if !f.review.Rows[i].Keep {
			ink, state = th.Palette.Dim, "drop"
		}
		if i == f.marked {
			state = "merge into…"
		}
		g.text(screen, city.Point{X: x, Y: y}, g.clip(state+"   "+l.Name, inner*0.6, ui.Body), ui.Body, ink)
		meta := g.clip(l.Sessions+" · "+l.Dirs, inner*0.38, ui.Small)
		mw, _ := g.faces.Measure(meta, ui.Small)
		g.text(screen, city.Point{X: x + inner - mw, Y: y}, meta, ui.Small, th.Palette.Dim)
		g.text(screen, city.Point{X: x + grid*5, Y: y + body}, g.clip(l.Why, inner-grid*5, ui.Small), ui.Small, th.Palette.Dim)
		y += rowHeight
	}
	y += grid
	g.text(screen, city.Point{X: x, Y: y}, "↑ ↓ choose   space keep or drop   m then m merge   s split by directory", ui.Small, th.Palette.Dim)
	y += small + grid/2
	g.text(screen, city.Point{X: x, Y: y}, "enter  Confirm        a or esc  Ask me later", ui.Small, th.Palette.Accent)
}
