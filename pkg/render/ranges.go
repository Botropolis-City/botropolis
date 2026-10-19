package render

import (
	"fmt"
	"image/color"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

// The plant's long ranges, read from the statistics store off the frame
// and drawn beside the short windows. Item 116.

// rangeQueries is what the long ranges ask of the store.
type rangeQueries interface {
	Series(r stats.Range, f stats.Filter, unit stats.Unit, loc *time.Location) ([]stats.Bucket, error)
	Totals(r stats.Range, f stats.Filter, loc *time.Location) (stats.Counters, stats.Cost, error)
	Coverage(r stats.Range, f stats.Filter, loc *time.Location) (stats.Coverage, error)
	ModelMix(r stats.Range, f stats.Filter, loc *time.Location) ([]stats.Share, error)
	Districts(root func(dir string) string) ([]string, error)
}

// rangeOpen gives the store when it can be read, or why it cannot.
type rangeOpen func() (rangeQueries, bool, string)

func ledgerOpen(l *stats.Ledger, now func() time.Time) rangeOpen {
	return func() (rangeQueries, bool, string) {
		s, ok := l.Store(now())
		if !ok {
			return nil, false, l.Reason()
		}
		return s, true, ""
	}
}

// rangeRefresh is how long a long range's figures are drawn before they
// are read again.
const rangeRefresh = 30 * time.Second

// loadRanges reads everything a long range shows, in loc's days.
func loadRanges(q rangeQueries, sel ui.RangeSelection, today stats.Date, loc *time.Location) (ui.RangeData, error) {
	days := sel.Days(today)
	d := ui.RangeData{Today: today, Days: days, Unit: sel.Unit(today)}
	f := stats.Filter{Root: city.ProjectRoot, Only: sel.Only}
	var err error
	if d.Buckets, err = q.Series(days, f, d.Unit, loc); err != nil {
		return d, err
	}
	if d.Counters, d.Cost, err = q.Totals(days, f, loc); err != nil {
		return d, err
	}
	if d.Coverage, err = q.Coverage(days, f, loc); err != nil {
		return d, err
	}
	if d.Mix, err = q.ModelMix(days, f, loc); err != nil {
		return d, err
	}
	if d.All, err = q.Districts(city.ProjectRoot); err != nil {
		return d, err
	}
	counted := sel.Only
	if counted == nil {
		counted = d.All
	}
	named := city.DistrictNames(append(append([]string(nil), d.All...), counted...))
	for _, root := range counted {
		fig := ui.DistrictFigure{Root: root, Name: named[root]}
		if fig.Counters, fig.Cost, err = q.Totals(days, stats.Filter{Root: city.ProjectRoot, Only: []string{root}}, loc); err != nil {
			return d, err
		}
		d.Districts = append(d.Districts, fig)
	}
	cmp := stats.MonthComparison(today, sel.SameDay)
	for _, side := range []struct {
		days stats.Range
		into *ui.MonthSide
	}{{cmp.This, &d.This}, {cmp.Last, &d.Last}} {
		side.into.Days = side.days
		if side.into.Counters, side.into.Cost, err = q.Totals(side.days, f, loc); err != nil {
			return d, err
		}
		if side.into.Coverage, err = q.Coverage(side.days, f, loc); err != nil {
			return d, err
		}
	}
	return d, nil
}

// rangeSource reads long ranges on its own goroutine, so a slow store
// never holds a frame: a frame draws the last figures, or "loading".
type rangeSource struct {
	open rangeOpen
	now  func() time.Time
	loc  *time.Location

	mu       sync.Mutex
	key      string
	data     ui.RangeData
	has      bool
	all      []string
	at       time.Time
	loading  bool
	failed   string
	failedAt time.Time
}

func newRangeSource(open rangeOpen, now func() time.Time, loc *time.Location) *rangeSource {
	return &rangeSource{open: open, now: now, loc: loc}
}

func rangeKey(sel ui.RangeSelection, today stats.Date) string {
	return fmt.Sprintf("%v|%v|%v|%v|%v", today, sel.Days(today), sel.Unit(today), sel.Only, sel.SameDay)
}

// state is whether the store can be read, and why not.
func (s *rangeSource) state() (ui.StoreState, string) {
	s.mu.Lock()
	failed, at := s.failed, s.failedAt
	s.mu.Unlock()
	if failed != "" && s.now().Sub(at) < rangeRefresh {
		return ui.StorePaused, failed
	}
	_, ok, reason := s.open()
	switch {
	case ok:
		return ui.StoreReadable, ""
	case reason != "":
		return ui.StorePaused, reason
	}
	return ui.StoreOpening, ""
}

// read gives the selection's figures as last read, starting a read when
// they are missing or stale; it never waits for one.
func (s *rangeSource) read(sel ui.RangeSelection, today stats.Date) ui.RangeData {
	key := rangeKey(sel, today)
	s.mu.Lock()
	defer s.mu.Unlock()
	if (key != s.key || s.now().Sub(s.at) >= rangeRefresh) && !s.loading {
		if q, ok, _ := s.open(); ok {
			s.loading = true
			go s.load(q, sel, today, key)
		}
	}
	if s.has && s.key == key {
		return s.data
	}
	return ui.RangeData{Loading: true, Today: today, Days: sel.Days(today), Unit: sel.Unit(today), All: s.all}
}

func (s *rangeSource) load(q rangeQueries, sel ui.RangeSelection, today stats.Date, key string) {
	d, err := loadRanges(q, sel, today, s.loc)
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loading, s.at = false, now
	if err != nil {
		s.failed, s.failedAt = stats.Reason(err), now
		if s.key != key {
			s.has = false
		}
		s.key = key
		return
	}
	s.key, s.data, s.has, s.all, s.failed = key, d, true, d.All, ""
}

func (g *Game) rangeNow() time.Time {
	if g.rangeClock != nil {
		return g.rangeClock()
	}
	return g.now()
}

func (g *Game) rangeToday() stats.Date { return stats.DateOf(g.rangeNow().In(time.Local)) }

// rangeSource is the long ranges' reader, made on first use from the
// statistics ledger.
func (g *Game) rangeSource() *rangeSource {
	if g.ranges == nil {
		g.ranges = newRangeSource(func() (rangeQueries, bool, string) { return ledgerOpen(g.ledger, g.rangeNow)() }, g.rangeNow, time.Local)
		g.rangeSel.SameDay = true
	}
	return g.ranges
}

func (g *Game) rangeState() (ui.StoreState, string) { return g.rangeSource().state() }

func (g *Game) rangeReadable() bool {
	s, _ := g.rangeState()
	return s == ui.StoreReadable
}

func (g *Game) rangeData() ui.RangeData { return g.rangeSource().read(g.rangeSel, g.rangeToday()) }

func (g *Game) windowChoice() ui.WindowChoice {
	if g.rangeOn {
		return ui.WindowChoice{Long: true, Kind: g.rangeSel.Kind}
	}
	return ui.WindowChoice{Window: g.window}
}

// choose moves the panel to a short window or a long range; the short
// window is kept apart from the long ranges' selection.
func (g *Game) choose(c ui.WindowChoice) {
	if !c.Long {
		g.rangeOn, g.window = false, c.Window
		return
	}
	g.rangeOn, g.rangeSel = true, g.rangeSel.Choose(c.Kind, g.rangeToday())
}

func (g *Game) rowState() ui.RowState {
	store, reason := g.rangeState()
	return ui.RowState{Current: g.windowChoice(), Store: store, Reason: reason, Off: g.statsOff(), Custom: g.rangeSel.CustomLabel()}
}

// statsOff reports whether the person turned statistics off, which the
// row says as a choice rather than a pause.
func (g *Game) statsOff() bool { return !g.stats.on }

// openStatsSetting closes the plant's panel and opens settings on the
// statistics row.
func (g *Game) openStatsSetting() {
	g.breakdown, g.settingsOpen = false, true
	for i, s := range g.settings.Items {
		if s.Key == config.KeyStats {
			g.settings.Cursor = i
		}
	}
}

func (g *Game) drawWindowRow(screen *ebiten.Image, row ui.WindowRow) {
	th := g.theme
	button := func(b ui.Button, current bool) {
		colour := th.Palette.Dim
		if current {
			colour = th.Palette.Accent
		}
		g.roundRect(screen, b.Rect, th.Radius()/2, th.Palette.Hairline)
		g.run(screen, b.Label, colour)
	}
	for i, b := range row.Short {
		button(b, !row.Current.Long && row.Current.Window == city.Window(i))
	}
	d := row.Divider
	vector.FillRect(screen, float32(d.Min.X), float32(d.Min.Y), float32(max(1, d.Width())), float32(d.Height()), th.Palette.Dim, false)
	for i, b := range row.Long {
		button(b, row.Current.Long && row.Current.Kind == ui.RangeKinds[i])
	}
	g.run(screen, row.Paused, th.Palette.Dim)
}

func fade(c color.NRGBA, alpha float64) color.NRGBA {
	c.A = uint8(float64(c.A) * alpha)
	return c
}

func fillRect(screen *ebiten.Image, r city.Rect, c color.NRGBA) {
	if r.Width() <= 0 || r.Height() <= 0 {
		return
	}
	vector.FillRect(screen, float32(r.Min.X), float32(r.Min.Y), float32(r.Width()), float32(r.Height()), c, false)
}

func (g *Game) hatch(screen *ebiten.Image, slot city.Rect, c color.NRGBA) {
	grid := g.theme.Grid()
	x := slot.Min.X + slot.Width()/2 - 0.5
	for y := slot.Min.Y; y < slot.Max.Y; y += grid {
		fillRect(screen, city.RectAt(x, y, 1, min(grid/2, slot.Max.Y-y)), c)
	}
}

func (g *Game) swatch(screen *ebiten.Image, kind ui.BarKind, r city.Rect) {
	th := g.theme
	switch kind {
	case ui.BarCost:
		fillRect(screen, r, th.Palette.Plant)
	case ui.BarNoData:
		g.hatch(screen, r, th.Palette.Dim)
	case ui.BarIncomplete:
		vector.StrokeRect(screen, float32(r.Min.X), float32(r.Min.Y), float32(r.Width()), float32(r.Height()), 1, th.Palette.Dim, false)
	case ui.BarUnknown:
		fillRect(screen, city.RectAt(r.Min.X, r.Max.Y-r.Height()/3, r.Width(), r.Height()/3), th.Palette.Dim)
	}
}

// drawRanges is the plant's panel on a long range.
func (g *Game) drawRanges(screen *ebiten.Image, width, height float64) {
	th := g.theme
	pal := th.Palette
	vector.FillRect(screen, 0, 0, float32(width), float32(height), colorScrim, false)
	p := ui.LayoutRanges(th, width, height, g.rangeData(), g.rangeSel, g.rangeChooser, g.faces.Measure)
	g.roundPanel(screen, p.Rect)
	g.run(screen, p.Title, pal.Text)
	g.run(screen, p.Counters, pal.Dim)
	g.drawWindowRow(screen, p.Row)
	g.run(screen, p.Filter, pal.Text)
	g.run(screen, p.Since, pal.Dim)

	c := p.Chart
	fillRect(screen, city.RectAt(c.Min.X, c.Max.Y, c.Width(), 1), pal.Hairline)
	for _, b := range p.Bars {
		switch b.Kind {
		case ui.BarNoData:
			g.hatch(screen, b.Slot, pal.Hairline)
		case ui.BarUnknown:
			fillRect(screen, b.Solid, pal.Dim)
		case ui.BarIncomplete:
			s := b.Slot
			vector.StrokeRect(screen, float32(s.Min.X+1), float32(s.Min.Y), float32(max(1, s.Width()-2)), float32(s.Height()), 1, pal.Dim, false)
			fillRect(screen, b.Solid, pal.Dim)
			fillRect(screen, b.Cap, fade(pal.Dim, 0.45))
		default:
			fillRect(screen, b.Solid, pal.Plant)
			fillRect(screen, b.Cap, fade(pal.Plant, 0.45))
		}
	}
	for _, t := range p.Ticks {
		g.run(screen, t, pal.Dim)
	}
	g.run(screen, p.NoData, pal.Dim)
	g.run(screen, p.Loading, pal.Dim)
	for _, e := range p.Legend {
		g.swatch(screen, e.Kind, e.Swatch)
		g.run(screen, e.Label, pal.Dim)
	}

	g.run(screen, p.MixHead, pal.Dim)
	for i, s := range p.Mix {
		r := s.Rect
		fillRect(screen, city.RectAt(r.Min.X, r.Min.Y, max(1, r.Width()-2), r.Height()), fade(pal.Plant, max(0.3, 1-float64(i)*0.18)))
	}
	g.run(screen, p.MixLine, pal.Text)
	g.run(screen, p.DistrictHead, pal.Dim)
	for _, row := range p.Districts {
		g.run(screen, row.Label, pal.Text)
		g.run(screen, row.Value, pal.Dim)
	}
	g.run(screen, p.CompareHead, pal.Dim)
	for i, t := range p.Compare {
		colour := pal.Text
		if i >= 2 {
			colour = pal.Dim
		}
		g.run(screen, t, colour)
	}
	g.run(screen, p.Footer, pal.Dim)

	if len(p.Chooser) > 0 {
		g.roundPanel(screen, p.ChooserBox)
		for _, row := range p.Chooser {
			if row.Focused {
				g.roundRect(screen, row.Rect, th.Radius()/2, pal.Hairline)
			}
			g.run(screen, row.Label, pal.Text)
		}
	}
	if h, ok := p.HitMix(g.pointer()); ok {
		at := g.pointer().Add(city.Point{X: th.Grid() * 2, Y: th.Grid() * 2})
		w, lh := g.faces.Measure(h, ui.Small)
		g.roundPanel(screen, city.RectAt(at.X-th.Grid(), at.Y-th.Grid()/2, w+2*th.Grid(), lh+th.Grid()))
		g.text(screen, at, h, ui.Small, pal.Text)
	}
	g.mu.Lock()
	g.rangesLayout = p
	g.mu.Unlock()
}

// clickRanges answers a click on the long-range panel.
func (g *Game) clickRanges(at city.Point) bool {
	g.mu.Lock()
	p := g.rangesLayout
	g.mu.Unlock()
	if g.rangeChooser.Open {
		if root, ok := p.HitChooser(at); ok {
			g.rangeSel = g.rangeSel.Toggle(root, g.rangeData().All)
			return true
		}
		if !p.ChooserBox.Contains(at) {
			g.rangeChooser.Open = false
		}
	}
	if c, ok := p.Row.Hit(at); ok {
		g.choose(c)
		return true
	}
	if p.HitFilter(at) {
		g.rangeChooser = ui.Chooser{Open: !g.rangeChooser.Open}
		return true
	}
	if !p.Rect.Contains(at) {
		g.breakdown = false
	}
	return true
}

// chooserKeys answers the keyboard while the district chooser is open.
func (g *Game) chooserKeys() {
	just := g.just
	all := g.rangeData().All
	ch := &g.rangeChooser
	switch {
	case just(ebiten.KeyEscape), just(ebiten.KeyD):
		ch.Open = false
	case just(ebiten.KeyArrowUp):
		ch.Focus = max(0, ch.Focus-1)
	case just(ebiten.KeyArrowDown):
		ch.Focus = max(0, min(len(all)-1, ch.Focus+1))
	case just(ebiten.KeySpace), just(ebiten.KeyEnter):
		if ch.Focus < len(all) {
			g.rangeSel = g.rangeSel.Toggle(all[ch.Focus], all)
		}
	case just(ebiten.KeyA):
		g.rangeSel.Only = nil
	}
}

// rangeKeys answers the keyboard on a long range; it reports whether
// the panel stays open.
func (g *Game) rangeKeys() bool {
	if g.rangeChooser.Open {
		g.chooserKeys()
		return true
	}
	just := g.just
	custom := g.rangeSel.Kind == ui.CustomRange
	month := 0
	if g.shifted() {
		month = 1
	}
	switch {
	case just(ebiten.KeyEscape), just(ebiten.KeyX), just(ebiten.KeyQ):
		return false
	case just(ebiten.KeyDigit1):
		g.choose(ui.WindowChoice{Window: city.LastHour})
	case just(ebiten.KeyDigit2):
		g.choose(ui.WindowChoice{Window: city.LastDay})
	case just(ebiten.KeyDigit3):
		g.choose(ui.WindowChoice{Window: city.LastWeek})
	case just(ebiten.KeyArrowRight):
		g.choose(g.windowChoice().Next(true))
	case just(ebiten.KeyArrowLeft):
		g.choose(g.windowChoice().Prev(true))
	case just(ebiten.KeyD):
		g.rangeChooser = ui.Chooser{Open: true}
	case just(ebiten.KeyS):
		g.rangeSel.SameDay = !g.rangeSel.SameDay
	case just(ebiten.KeyP):
		g.snap = screenshotPath(timeNow())
	case custom && just(ebiten.KeyTab):
		g.rangeSel = g.rangeSel.SwitchEnd()
	case custom && just(ebiten.KeyArrowUp):
		g.rangeSel = g.rangeSel.Nudge(1-month, month, g.rangeToday())
	case custom && just(ebiten.KeyArrowDown):
		g.rangeSel = g.rangeSel.Nudge(month-1, -month, g.rangeToday())
	}
	return true
}
