package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/stats"
)

// The plant's long ranges: month to date and the rest beside the short
// windows, every one read from the statistics store. Item 116.

// RangeKind is one of the long ranges.
type RangeKind int

const (
	MonthToDate RangeKind = iota
	LastMonth
	Last30Days
	Last3Months
	Last12Months
	YearToDate
	CustomRange
)

// RangeKinds is the long ranges in the order the row lays them.
var RangeKinds = []RangeKind{MonthToDate, LastMonth, Last30Days, Last3Months, Last12Months, YearToDate, CustomRange}

func (k RangeKind) String() string {
	return [...]string{"month to date", "last month", "30 d", "3 mo", "12 mo", "year to date", "custom…"}[k]
}

func (k RangeKind) long() string {
	return [...]string{"month to date", "last month", "last 30 days", "last 3 months", "last 12 months", "year to date", "custom"}[k]
}

// monthlyAfter is the most days a range is drawn in daily bars.
const monthlyAfter = 400

// RangeSelection is what the long ranges show: which range, a custom
// range's days and which end of it moves, the districts counted (nil
// for all) and whether last month stops on today's day.
type RangeSelection struct {
	Kind    RangeKind
	Custom  stats.Range
	End     int
	Only    []string
	SameDay bool
}

// Days is the selection's days on today.
func (s RangeSelection) Days(today stats.Date) stats.Range {
	switch s.Kind {
	case MonthToDate:
		return stats.MonthToDate(today)
	case LastMonth:
		return stats.LastMonth(today)
	case Last30Days:
		return stats.Last30Days(today)
	case Last3Months:
		return stats.Last3Months(today)
	case Last12Months:
		return stats.Last12Months(today)
	case YearToDate:
		return stats.YearToDate(today)
	}
	return s.Custom
}

func noon(d stats.Date) time.Time { return time.Date(d.Year, d.Month, d.Day, 12, 0, 0, 0, time.UTC) }

func daysIn(r stats.Range) int {
	return int(noon(r.Last).Sub(noon(r.First)).Round(24*time.Hour)/(24*time.Hour)) + 1
}

// Unit is how the selection is bucketed: by day, except a custom range
// too long for daily bars.
func (s RangeSelection) Unit(today stats.Date) stats.Unit {
	if s.Kind == CustomRange && daysIn(s.Days(today)) > monthlyAfter {
		return stats.Months
	}
	return stats.Days
}

// Choose picks a range; custom starts at the last 30 days the first
// time and keeps its days after.
func (s RangeSelection) Choose(k RangeKind, today stats.Date) RangeSelection {
	s.Kind = k
	if k == CustomRange && s.Custom == (stats.Range{}) {
		s.Custom = stats.Last30Days(today)
	}
	return s
}

// Toggle counts a district or stops counting it; counting all or none
// is counting all.
func (s RangeSelection) Toggle(root string, all []string) RangeSelection {
	on := map[string]bool{}
	if s.Only == nil {
		for _, r := range all {
			on[r] = true
		}
	}
	for _, r := range s.Only {
		on[r] = true
	}
	on[root] = !on[root]
	var only []string
	for _, r := range all {
		if on[r] {
			only = append(only, r)
		}
	}
	if len(only) == 0 || len(only) == len(all) {
		only = nil
	}
	s.Only = only
	return s
}

// SwitchEnd moves the focus to the custom range's other end.
func (s RangeSelection) SwitchEnd() RangeSelection {
	s.End = 1 - s.End
	return s
}

func monthsLater(d stats.Date, n int) stats.Date {
	first := stats.DateOf(time.Date(d.Year, d.Month+time.Month(n), 1, 12, 0, 0, 0, time.UTC))
	last := time.Date(first.Year, first.Month+1, 0, 12, 0, 0, 0, time.UTC).Day()
	first.Day = min(d.Day, last)
	return first
}

// Nudge moves the custom range's focused end by days and months,
// clamped to the end of a shorter month, to today and to the other end.
func (s RangeSelection) Nudge(days, months int, today stats.Date) RangeSelection {
	d := s.Custom.First
	if s.End == 1 {
		d = s.Custom.Last
	}
	d = monthsLater(d, months).AddDays(days)
	if today.Before(d) {
		d = today
	}
	if s.End == 0 {
		if s.Custom.Last.Before(d) {
			d = s.Custom.Last
		}
		s.Custom.First = d
	} else {
		if d.Before(s.Custom.First) {
			d = s.Custom.First
		}
		s.Custom.Last = d
	}
	return s
}

func dayLabel(d stats.Date, year int) string {
	l := fmt.Sprintf("%d %s", d.Day, d.Month.String()[:3])
	if d.Year != year {
		l += fmt.Sprintf(" %d", d.Year)
	}
	return l
}

func (s RangeSelection) customDays(brackets bool) string {
	first, last := dayLabel(s.Custom.First, s.Custom.Last.Year), dayLabel(s.Custom.Last, s.Custom.Last.Year)
	if brackets {
		if s.End == 0 {
			first = "[" + first + "]"
		} else {
			last = "[" + last + "]"
		}
	}
	return first + " – " + last
}

// CustomLabel is the custom button's label: its days, the focused end
// bracketed, once custom is chosen.
func (s RangeSelection) CustomLabel() string {
	if s.Kind != CustomRange {
		return CustomRange.String()
	}
	return s.customDays(true)
}

func (s RangeSelection) title() string {
	if s.Kind == CustomRange {
		return s.customDays(false)
	}
	return s.Kind.long()
}

// WindowChoice is one button of the plant's row: a short window, or a
// long range.
type WindowChoice struct {
	Long   bool
	Window city.Window
	Kind   RangeKind
}

func choices(readable bool) []WindowChoice {
	out := []WindowChoice{{Window: city.LastHour}, {Window: city.LastDay}, {Window: city.LastWeek}}
	if readable {
		for _, k := range RangeKinds {
			out = append(out, WindowChoice{Long: true, Kind: k})
		}
	}
	return out
}

func (c WindowChoice) step(by int, readable bool) WindowChoice {
	all := choices(readable)
	i := 0
	for j, o := range all {
		if o == c {
			i = j
		}
	}
	return all[(i+by+len(all))%len(all)]
}

// Next is the button to the right, wrapping; the long ranges are there
// only while the store is readable.
func (c WindowChoice) Next(readable bool) WindowChoice { return c.step(1, readable) }

// Prev is the button to the left, wrapping.
func (c WindowChoice) Prev(readable bool) WindowChoice { return c.step(-1, readable) }

// StoreState is whether the statistics store can be read.
type StoreState int

const (
	StoreOpening StoreState = iota
	StoreReadable
	StorePaused
)

// RowState is what the row shows: the current button, the store's
// state and why it is paused, whether statistics were turned off, and
// the custom button's label.
type RowState struct {
	Current WindowChoice
	Store   StoreState
	Reason  string
	Off     bool
	Custom  string
}

// statsOffLine is the row's words when statistics were turned off: a
// choice, with the way back, rather than a fault.
const statsOffLine = "statistics are off · s opens settings to turn them on"

// WindowRow is the plant's row: the short windows, a hairline, then
// the long ranges or why they are not offered.
type WindowRow struct {
	Short   []Button
	Divider city.Rect
	Long    []Button
	Paused  Text
	Current WindowChoice
}

// LayoutWindowRow lays the row from a point, no wider than width.
func LayoutWindowRow(th Theme, at city.Point, width float64, s RowState, measure Measure) WindowRow {
	grid := th.Grid()
	_, lineH := measure("", Body)
	row := WindowRow{Current: s.Current}
	row.Short = LayoutButtons(th, []string{city.LastHour.String(), city.LastDay.String(), city.LastWeek.String()}, at, measure)
	x := row.Short[len(row.Short)-1].Rect.Max.X + grid
	row.Divider = city.RectAt(x, at.Y, th.Hairline(), lineH+grid)
	x = row.Divider.Max.X + grid
	switch s.Store {
	case StoreReadable:
		custom := s.Custom
		if custom == "" {
			custom = CustomRange.String()
		}
		var names []string
		for _, k := range RangeKinds[:len(RangeKinds)-1] {
			names = append(names, k.String())
		}
		row.Long = LayoutButtons(th, append(names, custom), city.Point{X: x, Y: at.Y}, measure)
	case StorePaused:
		_, smallH := measure("", Small)
		line := PausedLine(s.Reason)
		if s.Off {
			line = statsOffLine
		}
		line = clipTo(line, at.X+width-x, Small, measure)
		row.Paused = Text{Text: line, At: city.Point{X: x, Y: at.Y + (lineH+grid-smallH)/2}, Size: Small}
	}
	return row
}

// Hit is the button under a point, if any.
func (r WindowRow) Hit(at city.Point) (WindowChoice, bool) {
	for i, b := range r.Short {
		if b.Rect.Contains(at) {
			return WindowChoice{Window: city.Window(i)}, true
		}
	}
	for i, b := range r.Long {
		if b.Rect.Contains(at) {
			return WindowChoice{Long: true, Kind: RangeKinds[i]}, true
		}
	}
	return WindowChoice{}, false
}

// DistrictFigure is one district's part of a range.
type DistrictFigure struct {
	Root, Name string
	Counters   stats.Counters
	Cost       stats.Cost
}

// MonthSide is one side of this month against last.
type MonthSide struct {
	Days     stats.Range
	Counters stats.Counters
	Cost     stats.Cost
	Coverage stats.Coverage
}

// RangeData is what a long range read from the store: its buckets, how
// far back its data goes, its totals, the model mix, the districts it
// counted, every district the store holds, and the month comparison.
type RangeData struct {
	Loading   bool
	Today     stats.Date
	Days      stats.Range
	Unit      stats.Unit
	Buckets   []stats.Bucket
	Coverage  stats.Coverage
	Counters  stats.Counters
	Cost      stats.Cost
	Mix       []stats.Share
	Districts []DistrictFigure
	All       []string
	This      MonthSide
	Last      MonthSide
}

// BarKind is how a bucket is drawn.
type BarKind int

const (
	BarCost BarKind = iota
	BarNoData
	BarIncomplete
	BarUnknown
)

// Bar is one bucket: its column, the cost solid to its low end and a
// cap to its high end where a write's lifetime is unknown.
type Bar struct {
	Kind       BarKind
	Slot       city.Rect
	Solid, Cap city.Rect
}

// ChartKey is one mark the chart uses, named.
type ChartKey struct {
	Kind   BarKind
	Swatch city.Rect
	Label  Text
}

// MixSegment is one model's part of the mix bar.
type MixSegment struct {
	Rect     city.Rect
	Name     string
	Headline string
}

// Chooser is the district chooser: open or not, and the focused row.
type Chooser struct {
	Open  bool
	Focus int
}

// ChooserRow is one district, checked when it is counted.
type ChooserRow struct {
	Rect    city.Rect
	Root    string
	Label   Text
	Focused bool
}

// RangesPanel is the plant's panel on a long range, laid out.
type RangesPanel struct {
	Rect         city.Rect
	Title        Text
	Counters     Text
	Row          WindowRow
	Filter       Text
	FilterRect   city.Rect
	Since        Text
	Chart        city.Rect
	Loading      Text
	Bars         []Bar
	Ticks        []Text
	NoData       Text
	Legend       []ChartKey
	MixHead      Text
	Mix          []MixSegment
	MixBar       city.Rect
	MixLine      Text
	DistrictHead Text
	Districts    []BreakdownRow
	CompareHead  Text
	Compare      []Text
	Footer       Text
	ChooserBox   city.Rect
	Chooser      []ChooserRow
}

const (
	chartGrids   = 16
	mixFold      = 0.02
	districtRows = 8
)

func weight(c stats.Cost) float64 { return (c.Low + c.High) / 2 }

func mixName(s stats.Share) string {
	name := strings.TrimPrefix(s.Model, "claude-")
	switch s.Speed {
	case "standard":
	case stats.Unrecorded:
		name += " (speed unrecorded)"
	default:
		name += " " + s.Speed
	}
	return name
}

func percent(share float64) string {
	if share < 0.005 {
		return "<1%"
	}
	return fmt.Sprintf("%.0f%%", share*100)
}

func change(now, was float64) string {
	if was == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%+.0f%%", (now/was-1)*100)
}

func sideLabel(r stats.Range) string {
	return fmt.Sprintf("%d–%d %s", r.First.Day, r.Last.Day, r.First.Month.String()[:3])
}

// LayoutRanges centres the long-range panel in a width×height window.
func LayoutRanges(th Theme, width, height float64, d RangeData, sel RangeSelection, ch Chooser, measure Measure) RangesPanel {
	grid := th.Grid()
	pad := 3 * grid
	_, titleH := measure("", Title)
	_, bodyH := measure("", Body)
	_, smallH := measure("", Small)
	w := min(width-8*grid, 132*grid)
	inner := w - 2*pad
	chartH := chartGrids * grid
	sideRows := max(min(len(d.Districts), districtRows), 5)
	h := pad + titleH + smallH + 2*grid + bodyH + grid + 2*grid + smallH + 2*grid + chartH + smallH + 2*grid +
		smallH + 2*grid + smallH + grid/2 + 1.5*grid + grid/2 + smallH + 3*grid + smallH + grid/2 + float64(sideRows)*smallH + 2*grid + smallH + pad
	p := RangesPanel{Rect: city.RectAt((width-w)/2, (height-h)/2, w, h)}
	x0, y := p.Rect.Min.X+pad, p.Rect.Min.Y+pad

	title := "Power, " + sel.title()
	if !d.Loading {
		title += ": " + Headline(d.Cost, d.Counters.Messages)
		p.Counters = Text{Text: CountersLine(d.Counters), At: city.Point{X: x0, Y: y + titleH}, Size: Small}
	}
	p.Title = Text{Text: clipTo(title, inner, Title, measure), At: city.Point{X: x0, Y: y}, Size: Title}
	y += titleH + smallH + 2*grid

	p.Row = LayoutWindowRow(th, city.Point{X: x0, Y: y}, inner, RowState{
		Current: WindowChoice{Long: true, Kind: sel.Kind}, Store: StoreReadable, Custom: sel.CustomLabel(),
	}, measure)
	y += bodyH + grid + 2*grid

	filter := fmt.Sprintf("districts: all %d  (d to choose)", len(d.All))
	if sel.Only != nil {
		names := make([]string, len(sel.Only))
		for i, r := range sel.Only {
			names[i] = filepath.Base(r)
		}
		filter = "districts: " + strings.Join(names, ", ") + "  (d to choose)"
	}
	p.Filter = Text{Text: filter, At: city.Point{X: x0, Y: y}, Size: Small}
	fw, _ := measure(filter, Small)
	p.FilterRect = city.RectAt(x0, y, fw, smallH)
	since := "no data yet"
	if d.Coverage.HasData {
		since = "data since " + dayLabel(d.Coverage.Earliest, d.Today.Year)
		if d.Coverage.StartsBeforeData {
			since += ": this range starts " + dayLabel(d.Days.First, d.Today.Year) + ", before it"
		}
	}
	p.Since = Text{Text: since, At: city.Point{X: x0 + fw + 4*grid, Y: y}, Size: Small}
	filterY := y
	y += smallH + 2*grid

	p.Chart = city.RectAt(x0, y, inner, chartH)
	if d.Loading {
		msg := "reading the statistics store…"
		mw, _ := measure(msg, Small)
		p.Loading = Text{Text: msg, At: city.Point{X: x0 + (inner-mw)/2, Y: y + (chartH-smallH)/2}, Size: Small}
	} else {
		p.layBars(th, d, measure)
	}
	y += chartH + smallH + 2*grid

	if !d.Loading {
		p.layLegend(th, d, y, measure)
	}
	y += smallH + 2*grid

	p.MixHead = Text{Text: "by model", At: city.Point{X: x0, Y: y}, Size: Small}
	y += smallH + grid/2
	p.MixBar = city.RectAt(x0, y, inner, 1.5*grid)
	p.layMix(d.Mix)
	y += 1.5*grid + grid/2
	var parts []string
	for _, s := range p.Mix {
		parts = append(parts, s.Name)
	}
	p.MixLine = Text{Text: clipTo(strings.Join(parts, "  ·  "), inner, Small, measure), At: city.Point{X: x0, Y: y}, Size: Small}
	y += smallH + 3*grid

	colW := inner / 2
	p.DistrictHead = Text{Text: "by district", At: city.Point{X: x0, Y: y}, Size: Small}
	p.CompareHead = Text{Text: "this month against last", At: city.Point{X: x0 + colW, Y: y}, Size: Small}
	y += smallH + grid/2
	districts := append([]DistrictFigure(nil), d.Districts...)
	sort.SliceStable(districts, func(i, j int) bool { return weight(districts[i].Cost) > weight(districts[j].Cost) })
	for i, f := range districts {
		if i == districtRows {
			break
		}
		ry := y + float64(i)*smallH
		value := Headline(f.Cost, f.Counters.Messages)
		vw, _ := measure(value, Small)
		p.Districts = append(p.Districts, BreakdownRow{
			Label: Text{Text: clipTo(f.Name, colW-vw-6*grid, Small, measure), At: city.Point{X: x0, Y: ry}, Size: Small},
			Value: Text{Text: value, At: city.Point{X: x0 + colW - 4*grid - vw, Y: ry}, Size: Small},
		})
	}
	if !d.Loading {
		check := "[ ]"
		if sel.SameDay {
			check = "[✓]"
		}
		lines := []string{
			sideLabel(d.This.Days) + "   " + Headline(d.This.Cost, d.This.Counters.Messages),
			sideLabel(d.Last.Days) + "   " + Headline(d.Last.Cost, d.Last.Counters.Messages),
			fmt.Sprintf("cost %s  ·  messages %s      %s only to the same day (s)",
				change(weight(d.This.Cost), weight(d.Last.Cost)), change(float64(d.This.Counters.Messages), float64(d.Last.Counters.Messages)), check),
		}
		if d.Last.Coverage.StartsBeforeData {
			lines = append(lines, "last month starts before the data")
		}
		if d.This.Coverage.TouchesGap || d.Last.Coverage.TouchesGap {
			lines = append(lines, "touches a gap: possibly incomplete")
		}
		for i, l := range lines {
			p.Compare = append(p.Compare, Text{Text: clipTo(l, colW, Small, measure), At: city.Point{X: x0 + colW, Y: y + float64(i)*smallH}, Size: Small})
		}
	}
	y += float64(sideRows)*smallH + 2*grid

	foot := "← → choose a range   d chooses districts   s toggles the same day   esc closes"
	if sel.Kind == CustomRange {
		foot = "tab switches ends   ↑ ↓ a day   shift a month   " + foot
	}
	p.Footer = Text{Text: clipTo(foot, inner, Small, measure), At: city.Point{X: x0, Y: y}, Size: Small}

	if ch.Open {
		p.layChooser(th, d.All, sel.Only, ch.Focus, filterY+smallH+grid, measure)
		p.NoData = Text{}
	}
	return p
}

func (p *RangesPanel) layBars(th Theme, d RangeData, measure Measure) {
	grid := th.Grid()
	if len(d.Buckets) == 0 {
		return
	}
	peak := 0.0
	for _, b := range d.Buckets {
		if !b.NoData {
			peak = max(peak, b.Cost.High)
		}
	}
	if peak == 0 {
		peak = 1
	}
	c := p.Chart
	slot := c.Width() / float64(len(d.Buckets))
	base := c.Max.Y
	noData := 0
	for i, b := range d.Buckets {
		x := c.Min.X + float64(i)*slot
		bar := Bar{Slot: city.RectAt(x, c.Min.Y, slot, c.Height())}
		bw := max(1, slot-2)
		switch {
		case b.NoData:
			bar.Kind = BarNoData
			noData++
		case b.Counters.Messages > 0 && b.Cost.Unpriced >= b.Counters.Messages:
			bar.Kind = BarUnknown
			bar.Solid = city.RectAt(x+1, base-grid/2, bw, grid/2)
		default:
			if b.Incomplete {
				bar.Kind = BarIncomplete
			}
			lo := c.Height() * b.Cost.Low / peak
			hi := c.Height() * b.Cost.High / peak
			bar.Solid = city.RectAt(x+1, base-lo, bw, lo)
			bar.Cap = city.RectAt(x+1, base-hi, bw, hi-lo)
		}
		if b.NoData && b.Incomplete {
			bar.Kind = BarNoData
		}
		p.Bars = append(p.Bars, bar)
		if b.Start.Day == 1 {
			p.Ticks = append(p.Ticks, Text{Text: b.Start.Month.String()[:3], At: city.Point{X: x, Y: base + grid/2}, Size: Small})
		}
	}
	if noData > 2 {
		_, smallH := measure("", Small)
		p.NoData = Text{Text: "no data", At: city.Point{X: c.Min.X + grid, Y: c.Min.Y + (c.Height()-smallH)/2}, Size: Small}
	}
}

func (p *RangesPanel) layLegend(th Theme, d RangeData, y float64, measure Measure) {
	grid := th.Grid()
	_, smallH := measure("", Small)
	var first, last stats.Date
	gap, unknown, noData := false, false, false
	for _, b := range d.Buckets {
		if b.NoData {
			noData = true
			continue
		}
		if b.Incomplete {
			if !gap {
				first = b.Start
			}
			gap, last = true, b.Last
		}
		if b.Counters.Messages > 0 && b.Cost.Unpriced >= b.Counters.Messages {
			unknown = true
		}
	}
	entries := []struct {
		kind  BarKind
		label string
	}{
		{BarCost, "cost a day at API list prices"},
	}
	if d.Unit == stats.Months {
		entries[0].label = "cost a month at API list prices"
	}
	if noData {
		entries = append(entries, struct {
			kind  BarKind
			label string
		}{BarNoData, "no data: before the store's first message"})
	}
	if gap {
		off := dayLabel(first, d.Today.Year)
		if last != first {
			off += " – " + dayLabel(last, d.Today.Year)
		}
		entries = append(entries, struct {
			kind  BarKind
			label string
		}{BarIncomplete, "possibly incomplete: statistics were off " + off})
	}
	if unknown {
		entries = append(entries, struct {
			kind  BarKind
			label string
		}{BarUnknown, "cost unknown"})
	}
	x := p.Chart.Min.X
	for _, e := range entries {
		sw := city.RectAt(x, y+(smallH-grid)/2, grid, grid)
		lw, _ := measure(e.label, Small)
		p.Legend = append(p.Legend, ChartKey{Kind: e.kind, Swatch: sw, Label: Text{Text: e.label, At: city.Point{X: x + 1.75*grid, Y: y}, Size: Small}})
		x += 1.75*grid + lw + 4*grid
	}
}

func (p *RangesPanel) layMix(mix []stats.Share) {
	type part struct {
		name     string
		cost     stats.Cost
		messages int
		weight   float64
	}
	byCost := true
	total := 0.0
	for _, s := range mix {
		total += weight(s.Cost)
	}
	if total == 0 {
		byCost = false
		for _, s := range mix {
			total += float64(s.Counters.Messages)
		}
	}
	if total == 0 {
		return
	}
	var parts []part
	other := part{name: "other"}
	folded := false
	for _, s := range mix {
		wt := weight(s.Cost)
		if !byCost {
			wt = float64(s.Counters.Messages)
		}
		if wt/total < mixFold {
			other.cost, other.messages, other.weight = other.cost.Add(s.Cost), other.messages+s.Counters.Messages, other.weight+wt
			folded = true
			continue
		}
		parts = append(parts, part{name: mixName(s), cost: s.Cost, messages: s.Counters.Messages, weight: wt})
	}
	if folded {
		parts = append(parts, other)
	}
	x := p.MixBar.Min.X
	for _, pt := range parts {
		share := pt.weight / total
		sw := p.MixBar.Width() * share
		p.Mix = append(p.Mix, MixSegment{
			Rect:     city.RectAt(x, p.MixBar.Min.Y, sw, p.MixBar.Height()),
			Name:     pt.name + " " + percent(share),
			Headline: pt.name + ": " + Headline(pt.cost, pt.messages),
		})
		x += sw
	}
}

func (p *RangesPanel) layChooser(th Theme, all, only []string, focus int, y float64, measure Measure) {
	grid := th.Grid()
	_, smallH := measure("", Small)
	counted := map[string]bool{}
	for _, r := range only {
		counted[r] = true
	}
	step := smallH + grid/2
	limit := p.Rect.Max.Y - grid
	w := 0.0
	for _, r := range all {
		lw, _ := measure("[✓] "+filepath.Base(r), Small)
		w = max(w, lw)
	}
	x := p.Chart.Min.X
	ry := y + grid
	for i, r := range all {
		if ry+step > limit {
			break
		}
		check := "[ ]"
		if only == nil || counted[r] {
			check = "[✓]"
		}
		p.Chooser = append(p.Chooser, ChooserRow{
			Rect: city.RectAt(x, ry, w+2*grid, step), Root: r, Focused: i == focus,
			Label: Text{Text: check + " " + filepath.Base(r), At: city.Point{X: x + grid, Y: ry + grid/4}, Size: Small},
		})
		ry += step
	}
	p.ChooserBox = city.RectAt(x-grid, y, w+4*grid, ry-y+grid)
}

// HitFilter reports whether a point is on the district filter's line.
func (p RangesPanel) HitFilter(at city.Point) bool { return p.FilterRect.Contains(at) }

// HitMix is the headline of the model share under a point, if any.
func (p RangesPanel) HitMix(at city.Point) (string, bool) {
	for _, s := range p.Mix {
		if s.Rect.Contains(at) {
			return s.Headline, true
		}
	}
	return "", false
}

// HitChooser is the district whose chooser row is under a point, if
// any.
func (p RangesPanel) HitChooser(at city.Point) (string, bool) {
	for _, r := range p.Chooser {
		if r.Rect.Contains(at) {
			return r.Root, true
		}
	}
	return "", false
}
