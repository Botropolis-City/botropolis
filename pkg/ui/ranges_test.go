package ui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

var rangeToday = stats.Date{Year: 2026, Month: time.October, Day: 19}

func day(m time.Month, d int) stats.Date { return stats.Date{Year: 2026, Month: m, Day: d} }

func texts(ts []ui.Text) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Text
	}
	return out
}

func labels(bs []ui.Button) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Label.Text
	}
	return out
}

func priced(usd float64, messages int) (stats.Cost, stats.Counters) {
	return stats.Cost{Low: usd, High: usd}, stats.Counters{Input: 100, Output: 10, Read: 1000, Messages: messages}
}

func someRangeData() ui.RangeData {
	r := stats.Last3Months(rangeToday)
	var buckets []stats.Bucket
	for d := r.First; !r.Last.Before(d); d = d.AddDays(1) {
		b := stats.Bucket{Start: d, Last: d}
		switch {
		case d.Before(day(time.July, 31)):
			b.NoData = true
		case d == day(time.August, 10):
			b.Cost, b.Counters = stats.Cost{Low: 1, High: 4}, stats.Counters{Messages: 3}
		case d == day(time.August, 11):
			b.Cost, b.Counters = stats.Cost{Unpriced: 2}, stats.Counters{Messages: 2}
		case d == day(time.August, 12):
			b.Cost, b.Counters = priced(8, 5)
		default:
			b.Cost, b.Counters = priced(0.5, 1)
		}
		if !d.Before(day(time.September, 14)) && !day(time.September, 16).Before(d) {
			b.Incomplete = true
		}
		buckets = append(buckets, b)
	}
	cost, counters := priced(10, 120)
	share := func(model, speed string, usd float64) stats.Share {
		c, n := priced(usd, 10)
		return stats.Share{Model: model, Speed: speed, Cost: c, Counters: n}
	}
	thisCost, thisN := priced(6, 60)
	lastCost, lastN := priced(5, 50)
	kilnCost, kilnN := priced(7, 70)
	tideCost, tideN := priced(3, 50)
	return ui.RangeData{
		Today: rangeToday, Days: r, Unit: stats.Days, Buckets: buckets,
		Coverage: stats.Coverage{Earliest: day(time.July, 31), HasData: true, StartsBeforeData: true, TouchesGap: true},
		Cost:     cost, Counters: counters,
		Mix: []stats.Share{
			share("claude-opus-5", "standard", 5), share("claude-sonnet-5", "fast", 3),
			share("claude-haiku-5", stats.Unrecorded, 1.9), share("claude-tiny-1", "standard", 0.1),
		},
		Districts: []ui.DistrictFigure{
			{Root: "/w/tidepool", Name: "tidepool", Cost: tideCost, Counters: tideN},
			{Root: "/w/kiln", Name: "kiln", Cost: kilnCost, Counters: kilnN},
		},
		All:  []string{"/w/kiln", "/w/lanternfish", "/w/tidepool"},
		This: ui.MonthSide{Days: stats.MonthToDate(rangeToday), Cost: thisCost, Counters: thisN},
		Last: ui.MonthSide{Days: stats.MonthComparison(rangeToday, true).Last, Cost: lastCost, Counters: lastN},
	}
}

func TestRangeKind(t *testing.T) {
	for _, c := range []struct {
		kind  ui.RangeKind
		label string
		days  stats.Range
	}{
		{ui.MonthToDate, "month to date", stats.MonthToDate(rangeToday)},
		{ui.LastMonth, "last month", stats.LastMonth(rangeToday)},
		{ui.Last30Days, "30 d", stats.Last30Days(rangeToday)},
		{ui.Last3Months, "3 mo", stats.Last3Months(rangeToday)},
		{ui.Last12Months, "12 mo", stats.Last12Months(rangeToday)},
		{ui.YearToDate, "year to date", stats.YearToDate(rangeToday)},
	} {
		t.Run("when "+c.label+" is asked for", func(t *testing.T) {
			t.Run("it should read "+c.label, func(t *testing.T) {
				assert.Equal(t, c.label, c.kind.String())
			})

			t.Run("it should be the matching days", func(t *testing.T) {
				assert.Equal(t, c.days, ui.RangeSelection{Kind: c.kind}.Days(rangeToday))
			})
		})
	}

	t.Run("when custom is asked for", func(t *testing.T) {
		own := stats.Custom(day(time.September, 1), day(time.September, 9))

		t.Run("it should read custom…", func(t *testing.T) {
			assert.Equal(t, "custom…", ui.CustomRange.String())
		})

		t.Run("it should be the selection's own days", func(t *testing.T) {
			assert.Equal(t, own, ui.RangeSelection{Kind: ui.CustomRange, Custom: own}.Days(rangeToday))
		})
	})

	t.Run("when there are the long ranges", func(t *testing.T) {
		t.Run("it should be seven, custom last", func(t *testing.T) {
			assert.Equal(t, []ui.RangeKind{ui.MonthToDate, ui.LastMonth, ui.Last30Days, ui.Last3Months, ui.Last12Months, ui.YearToDate, ui.CustomRange}, ui.RangeKinds)
		})
	})
}

func TestRangeSelectionUnit(t *testing.T) {
	for _, c := range []struct {
		name string
		sel  ui.RangeSelection
		unit stats.Unit
	}{
		{"12 mo", ui.RangeSelection{Kind: ui.Last12Months}, stats.Days},
		{"year to date", ui.RangeSelection{Kind: ui.YearToDate}, stats.Days},
		{"a custom range of 400 days", ui.RangeSelection{Kind: ui.CustomRange, Custom: stats.Range{First: rangeToday.AddDays(-399), Last: rangeToday}}, stats.Days},
		{"a custom range of 401 days", ui.RangeSelection{Kind: ui.CustomRange, Custom: stats.Range{First: rangeToday.AddDays(-400), Last: rangeToday}}, stats.Months},
	} {
		t.Run("when the range is "+c.name, func(t *testing.T) {
			t.Run("it should bucket by "+map[stats.Unit]string{stats.Days: "day", stats.Months: "month"}[c.unit], func(t *testing.T) {
				assert.Equal(t, c.unit, c.sel.Unit(rangeToday))
			})
		})
	}
}

func TestRangeSelectionToggle(t *testing.T) {
	all := []string{"/w/kiln", "/w/lanternfish", "/w/tidepool"}

	for _, c := range []struct {
		name   string
		only   []string
		toggle string
		want   []string
	}{
		{"when every district is counted, and one is toggled, it should count the others", nil, "/w/kiln", []string{"/w/lanternfish", "/w/tidepool"}},
		{"when some are counted, and an uncounted one is toggled, it should count it too", []string{"/w/kiln"}, "/w/tidepool", []string{"/w/kiln", "/w/tidepool"}},
		{"when some are counted, and the last missing one is toggled, it should count all", []string{"/w/kiln", "/w/tidepool"}, "/w/lanternfish", nil},
		{"when one is counted, and it is toggled off, it should return to all", []string{"/w/kiln"}, "/w/kiln", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, ui.RangeSelection{Only: c.only}.Toggle(c.toggle, all).Only)
		})
	}
}

func TestRangeSelectionNudge(t *testing.T) {
	start := ui.RangeSelection{Kind: ui.CustomRange, Custom: stats.Range{First: day(time.September, 1), Last: day(time.September, 30)}}

	for _, c := range []struct {
		name         string
		end          int
		days, months int
		first, last  stats.Date
	}{
		{"when the first day is moved a day later, it should move only the first", 0, 1, 0, day(time.September, 2), day(time.September, 30)},
		{"when the last day is moved a month later, it should move only the last", 1, 0, 1, day(time.September, 1), day(time.October, 19)},
		{"when the last day is moved past today, it should stop on today", 1, 30, 0, day(time.September, 1), rangeToday},
		{"when the first day is moved past the last, it should stop on the last", 0, 40, 0, day(time.September, 30), day(time.September, 30)},
		{"when the last day is moved before the first, it should stop on the first", 1, -40, 0, day(time.September, 1), day(time.September, 1)},
		{"when the first day is moved a month earlier, it should clamp to the shorter month", 0, 0, -7, stats.Date{Year: 2026, Month: time.February, Day: 1}, day(time.September, 30)},
	} {
		t.Run(c.name, func(t *testing.T) {
			sel := start
			sel.End = c.end
			assert.Equal(t, stats.Range{First: c.first, Last: c.last}, sel.Nudge(c.days, c.months, rangeToday).Custom)
		})
	}

	t.Run("when a month-end first day is moved a month earlier, it should clamp to the shorter month", func(t *testing.T) {
		sel := ui.RangeSelection{Kind: ui.CustomRange, Custom: stats.Range{First: day(time.March, 31), Last: day(time.April, 30)}}
		assert.Equal(t, stats.Date{Year: 2026, Month: time.February, Day: 28}, sel.Nudge(0, -1, rangeToday).Custom.First)
	})

	t.Run("when the end is switched, it should focus the other", func(t *testing.T) {
		assert.Equal(t, 1, start.SwitchEnd().End)
	})

	t.Run("when custom is chosen fresh, it should start at the last 30 days", func(t *testing.T) {
		assert.Equal(t, stats.Last30Days(rangeToday), ui.RangeSelection{}.Choose(ui.CustomRange, rangeToday).Custom)
	})

	t.Run("when custom is chosen again, it should keep its days", func(t *testing.T) {
		assert.Equal(t, start.Custom, start.Choose(ui.CustomRange, rangeToday).Custom)
	})
}

func TestWindowChoiceSteps(t *testing.T) {
	short := func(w city.Window) ui.WindowChoice { return ui.WindowChoice{Window: w} }
	long := func(k ui.RangeKind) ui.WindowChoice { return ui.WindowChoice{Long: true, Kind: k} }

	for _, c := range []struct {
		name     string
		from     ui.WindowChoice
		readable bool
		next     ui.WindowChoice
		prev     ui.WindowChoice
	}{
		{"on 24 h", short(city.LastDay), true, short(city.LastWeek), short(city.LastHour)},
		{"on 7 d, with the store readable", short(city.LastWeek), true, long(ui.MonthToDate), short(city.LastDay)},
		{"on 7 d, with the store paused", short(city.LastWeek), false, short(city.LastHour), short(city.LastDay)},
		{"on 1 h, with the store readable", short(city.LastHour), true, short(city.LastDay), long(ui.CustomRange)},
		{"on 1 h, with the store paused", short(city.LastHour), false, short(city.LastDay), short(city.LastWeek)},
		{"on month to date", long(ui.MonthToDate), true, long(ui.LastMonth), short(city.LastWeek)},
		{"on custom", long(ui.CustomRange), true, short(city.LastHour), long(ui.YearToDate)},
	} {
		t.Run("when "+c.name, func(t *testing.T) {
			t.Run("it should step right to the next", func(t *testing.T) {
				assert.Equal(t, c.next, c.from.Next(c.readable))
			})

			t.Run("it should step left to the one before", func(t *testing.T) {
				assert.Equal(t, c.prev, c.from.Prev(c.readable))
			})
		})
	}
}

func TestLayoutWindowRow(t *testing.T) {
	th := ui.NewTheme(1)
	at := city.Point{X: 40, Y: 60}
	lay := func(s ui.RowState) ui.WindowRow { return ui.LayoutWindowRow(th, at, 900, s, measure7) }

	t.Run("when the store is readable", func(t *testing.T) {
		row := lay(ui.RowState{Current: ui.WindowChoice{Window: city.LastDay}, Store: ui.StoreReadable, Custom: "custom…"})

		t.Run("it should lay 1 h, 24 h and 7 d first", func(t *testing.T) {
			assert.Equal(t, []string{"1 h", "24 h", "7 d"}, labels(row.Short))
		})

		t.Run("it should put a hairline after 7 d", func(t *testing.T) {
			assert.Greater(t, row.Divider.Min.X, row.Short[2].Rect.Max.X)
		})

		t.Run("it should lay the seven long ranges after the hairline", func(t *testing.T) {
			assert.Equal(t, []string{"month to date", "last month", "30 d", "3 mo", "12 mo", "year to date", "custom…"}, labels(row.Long))
		})

		t.Run("it should start the long ranges past the hairline", func(t *testing.T) {
			assert.Greater(t, row.Long[0].Rect.Min.X, row.Divider.Max.X)
		})

		t.Run("it should say nothing is paused", func(t *testing.T) {
			assert.Empty(t, row.Paused.Text)
		})

		t.Run("and a long range is chosen", func(t *testing.T) {
			row := lay(ui.RowState{Current: ui.WindowChoice{Long: true, Kind: ui.Last3Months}, Store: ui.StoreReadable})

			t.Run("it should mark that one", func(t *testing.T) {
				assert.Equal(t, ui.WindowChoice{Long: true, Kind: ui.Last3Months}, row.Current)
			})
		})

		t.Run("and custom carries its days", func(t *testing.T) {
			row := lay(ui.RowState{Store: ui.StoreReadable, Custom: "[1 Sep] – 30 Sep"})

			t.Run("it should label the custom button with them", func(t *testing.T) {
				assert.Equal(t, "[1 Sep] – 30 Sep", row.Long[6].Label.Text)
			})
		})
	})

	t.Run("when the store is paused", func(t *testing.T) {
		row := lay(ui.RowState{Store: ui.StorePaused, Reason: "there are no statistics yet"})

		t.Run("it should lay only the three short windows", func(t *testing.T) {
			assert.Empty(t, row.Long)
		})

		t.Run("it should say why after the hairline", func(t *testing.T) {
			assert.Equal(t, ui.PausedLine("there are no statistics yet"), row.Paused.Text)
		})

		t.Run("it should put the reason past the hairline", func(t *testing.T) {
			assert.Greater(t, row.Paused.At.X, row.Divider.Max.X)
		})

		t.Run("and the reason is wider than the row", func(t *testing.T) {
			row := ui.LayoutWindowRow(th, at, 300, ui.RowState{Store: ui.StorePaused, Reason: strings.Repeat("long ", 40)}, measure7)

			t.Run("it should clip it to the row", func(t *testing.T) {
				w, _ := measure7(row.Paused.Text, ui.Small)
				assert.LessOrEqual(t, row.Paused.At.X+w, at.X+300)
			})
		})
	})

	t.Run("when the store is still opening", func(t *testing.T) {
		row := lay(ui.RowState{Store: ui.StoreOpening})

		t.Run("it should lay only the short windows", func(t *testing.T) {
			assert.Empty(t, row.Long)
		})

		t.Run("it should say nothing is paused", func(t *testing.T) {
			assert.Empty(t, row.Paused.Text)
		})
	})

	t.Run("when a point is hit", func(t *testing.T) {
		row := lay(ui.RowState{Store: ui.StoreReadable})

		t.Run("and it is on a short window, it should name it", func(t *testing.T) {
			c, _ := row.Hit(row.Short[2].Rect.Center())
			assert.Equal(t, ui.WindowChoice{Window: city.LastWeek}, c)
		})

		t.Run("and it is on a long range, it should name it", func(t *testing.T) {
			c, _ := row.Hit(row.Long[4].Rect.Center())
			assert.Equal(t, ui.WindowChoice{Long: true, Kind: ui.Last12Months}, c)
		})

		t.Run("and it is beside them, it should name nothing", func(t *testing.T) {
			_, ok := row.Hit(city.Point{X: 1, Y: 1})
			assert.False(t, ok)
		})
	})
}

func TestLayoutRanges(t *testing.T) {
	th := ui.NewTheme(1)
	grid := th.Grid()
	sel := ui.RangeSelection{Kind: ui.Last3Months, SameDay: true}
	lay := func(d ui.RangeData, s ui.RangeSelection, c ui.Chooser) ui.RangesPanel {
		return ui.LayoutRanges(th, 1600, 1000, d, s, c, measure7)
	}
	p := lay(someRangeData(), sel, ui.Chooser{})

	t.Run("when the panel is laid out", func(t *testing.T) {
		t.Run("it should centre it", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 800, Y: 500}, p.Rect.Center())
		})

		t.Run("it should headline the range with its cost and messages", func(t *testing.T) {
			c, n := priced(10, 120)
			assert.Equal(t, "Power, last 3 months: "+ui.Headline(c, n.Messages), p.Title.Text)
		})

		t.Run("it should put the counters under the title", func(t *testing.T) {
			_, n := priced(10, 120)
			assert.Equal(t, ui.CountersLine(n), p.Counters.Text)
		})

		t.Run("it should lay the window row with the long range marked", func(t *testing.T) {
			assert.Equal(t, ui.WindowChoice{Long: true, Kind: ui.Last3Months}, p.Row.Current)
		})
	})

	t.Run("when the filter line is laid", func(t *testing.T) {
		t.Run("and every district is counted, it should say all", func(t *testing.T) {
			assert.Equal(t, "districts: all 3  (d to choose)", p.Filter.Text)
		})

		t.Run("and some are chosen, it should name them", func(t *testing.T) {
			s := sel
			s.Only = []string{"/w/kiln", "/w/tidepool"}
			assert.Equal(t, "districts: kiln, tidepool  (d to choose)", lay(someRangeData(), s, ui.Chooser{}).Filter.Text)
		})

		t.Run("and the data begins inside the range, it should say so", func(t *testing.T) {
			assert.Equal(t, "data since 31 Jul: this range starts 19 Jul, before it", p.Since.Text)
		})

		t.Run("and the range is inside the data, it should say only since when", func(t *testing.T) {
			d := someRangeData()
			d.Coverage.StartsBeforeData = false
			assert.Equal(t, "data since 31 Jul", lay(d, sel, ui.Chooser{}).Since.Text)
		})

		t.Run("and there is no data at all, it should say so", func(t *testing.T) {
			d := someRangeData()
			d.Coverage = stats.Coverage{StartsBeforeData: true}
			assert.Equal(t, "no data yet", lay(d, sel, ui.Chooser{}).Since.Text)
		})

		t.Run("and it is hit, it should say so", func(t *testing.T) {
			assert.True(t, p.HitFilter(p.Filter.At.Add(city.Point{X: 2, Y: 2})))
		})
	})

	t.Run("when the bars are laid", func(t *testing.T) {
		d := someRangeData()
		bar := func(at stats.Date) ui.Bar {
			for i, b := range d.Buckets {
				if b.Start == at {
					return p.Bars[i]
				}
			}
			t.Fatalf("no bucket on %v", at)
			return ui.Bar{}
		}

		t.Run("it should lay one bar per bucket", func(t *testing.T) {
			assert.Len(t, p.Bars, len(d.Buckets))
		})

		t.Run("it should span the chart's width", func(t *testing.T) {
			assert.InDelta(t, p.Chart.Max.X, p.Bars[len(p.Bars)-1].Slot.Max.X, 0.001)
		})

		t.Run("and a bucket is before the data, it should be hatched", func(t *testing.T) {
			assert.Equal(t, ui.BarNoData, bar(day(time.July, 20)).Kind)
		})

		t.Run("and a bucket touches a gap, it should be marked possibly incomplete", func(t *testing.T) {
			assert.Equal(t, ui.BarIncomplete, bar(day(time.September, 15)).Kind)
		})

		t.Run("and a bucket's cost is wholly unknown, it should be a stub", func(t *testing.T) {
			assert.Equal(t, ui.BarUnknown, bar(day(time.August, 11)).Kind)
		})

		t.Run("it should scale the costliest bucket to the chart's height", func(t *testing.T) {
			assert.InDelta(t, p.Chart.Height(), bar(day(time.August, 12)).Solid.Height(), 0.001)
		})

		t.Run("and a bucket's cost is a range, it should be solid to the low end", func(t *testing.T) {
			assert.InDelta(t, p.Chart.Height()/8, bar(day(time.August, 10)).Solid.Height(), 0.001)
		})

		t.Run("and a bucket's cost is a range, it should cap it to the high end", func(t *testing.T) {
			assert.InDelta(t, p.Chart.Height()*3/8, bar(day(time.August, 10)).Cap.Height(), 0.001)
		})

		t.Run("and a bucket's cost is a range, it should stand the cap on the solid", func(t *testing.T) {
			b := bar(day(time.August, 10))
			assert.InDelta(t, b.Solid.Min.Y, b.Cap.Max.Y, 0.001)
		})

		t.Run("it should tick the first of each month with its name", func(t *testing.T) {
			assert.Equal(t, []string{"Aug", "Sep", "Oct"}, texts(p.Ticks))
		})
	})

	t.Run("when the legend is laid", func(t *testing.T) {
		t.Run("and there is a gap and an unknown day, it should name every mark", func(t *testing.T) {
			assert.Equal(t, []string{
				"cost a day at API list prices",
				"no data: before the store's first message",
				"possibly incomplete: statistics were off 14 Sep – 16 Sep",
				"cost unknown",
			}, legend(p))
		})

		t.Run("and there is neither, it should name only the bars and the hatch", func(t *testing.T) {
			d := someRangeData()
			for i := range d.Buckets {
				d.Buckets[i].Incomplete = false
				if d.Buckets[i].Cost.Unpriced > 0 {
					d.Buckets[i].Cost, d.Buckets[i].Counters = priced(1, 1)
				}
			}
			assert.Equal(t, []string{"cost a day at API list prices", "no data: before the store's first message"}, legend(lay(d, sel, ui.Chooser{})))
		})
	})

	t.Run("when the model mix is laid", func(t *testing.T) {
		t.Run("it should name each share with its part of the cost, dearest first", func(t *testing.T) {
			assert.Equal(t, "opus-5 50%  ·  sonnet-5 fast 30%  ·  haiku-5 (speed unrecorded) 19%  ·  other 1%", p.MixLine.Text)
		})

		t.Run("it should split the bar by cost", func(t *testing.T) {
			assert.InDelta(t, p.Chart.Width()/2, p.Mix[0].Rect.Width(), 1)
		})

		t.Run("and a share is hovered, it should give its headline", func(t *testing.T) {
			c, n := priced(3, 10)
			h, _ := p.HitMix(p.Mix[1].Rect.Center())
			assert.Equal(t, ui.Headline(c, n.Messages), h)
		})

		t.Run("and the folded share is hovered, it should give their headline together", func(t *testing.T) {
			c, n := priced(0.1, 10)
			h, _ := p.HitMix(p.Mix[3].Rect.Center())
			assert.Equal(t, ui.Headline(c, n.Messages), h)
		})

		t.Run("and nothing is hovered, it should give nothing", func(t *testing.T) {
			_, ok := p.HitMix(city.Point{X: 1, Y: 1})
			assert.False(t, ok)
		})
	})

	t.Run("when the districts are laid", func(t *testing.T) {
		t.Run("it should list each counted district, dearest first", func(t *testing.T) {
			assert.Equal(t, []string{"kiln", "tidepool"}, rowLabels(p.Districts))
		})

		t.Run("it should give each its headline", func(t *testing.T) {
			c, n := priced(7, 70)
			assert.Equal(t, ui.Headline(c, n.Messages), p.Districts[0].Value.Text)
		})
	})

	t.Run("when the comparison is laid", func(t *testing.T) {
		thisC, thisN := priced(6, 60)
		lastC, lastN := priced(5, 50)

		t.Run("it should headline this month and last, to the same day", func(t *testing.T) {
			assert.Equal(t, []string{
				"1–19 Oct   " + ui.Headline(thisC, thisN.Messages),
				"1–19 Sep   " + ui.Headline(lastC, lastN.Messages),
				"cost +20%  ·  messages +20%      [✓] only to the same day (s)",
			}, texts(p.Compare))
		})

		t.Run("and the toggle is off, it should read last month in full", func(t *testing.T) {
			d := someRangeData()
			d.Last.Days = stats.LastMonth(rangeToday)
			s := sel
			s.SameDay = false
			assert.Equal(t, []string{
				"1–19 Oct   " + ui.Headline(thisC, thisN.Messages),
				"1–30 Sep   " + ui.Headline(lastC, lastN.Messages),
				"cost +20%  ·  messages +20%      [ ] only to the same day (s)",
			}, texts(lay(d, s, ui.Chooser{}).Compare))
		})

		t.Run("and last month starts before the data, it should say so", func(t *testing.T) {
			d := someRangeData()
			d.Last.Coverage.StartsBeforeData = true
			assert.Contains(t, texts(lay(d, sel, ui.Chooser{}).Compare), "last month starts before the data")
		})

		t.Run("and either side touches a gap, it should say possibly incomplete", func(t *testing.T) {
			d := someRangeData()
			d.This.Coverage.TouchesGap = true
			assert.Contains(t, texts(lay(d, sel, ui.Chooser{}).Compare), "touches a gap: possibly incomplete")
		})

		t.Run("and last month cost nothing, it should not divide by it", func(t *testing.T) {
			d := someRangeData()
			d.Last.Cost, d.Last.Counters = stats.Cost{}, stats.Counters{}
			assert.Contains(t, texts(lay(d, sel, ui.Chooser{}).Compare)[2], "cost n/a  ·  messages n/a")
		})
	})

	t.Run("when the data is still loading", func(t *testing.T) {
		lp := lay(ui.RangeData{Loading: true, Today: rangeToday, Days: stats.Last3Months(rangeToday)}, sel, ui.Chooser{})

		t.Run("it should say so in place of the bars", func(t *testing.T) {
			assert.Equal(t, "reading the statistics store…", lp.Loading.Text)
		})

		t.Run("it should lay no bars", func(t *testing.T) {
			assert.Empty(t, lp.Bars)
		})
	})

	t.Run("when the district chooser is open", func(t *testing.T) {
		s := sel
		s.Only = []string{"/w/kiln"}
		cp := lay(someRangeData(), s, ui.Chooser{Open: true, Focus: 2})

		t.Run("it should list every district in the store with a check each", func(t *testing.T) {
			assert.Equal(t, []string{"[✓] kiln", "[ ] lanternfish", "[ ] tidepool"}, chooserLabels(cp))
		})

		t.Run("it should mark the focused row", func(t *testing.T) {
			assert.Equal(t, []bool{false, false, true}, []bool{cp.Chooser[0].Focused, cp.Chooser[1].Focused, cp.Chooser[2].Focused})
		})

		t.Run("and a row is hit, it should name its district", func(t *testing.T) {
			root, _ := cp.HitChooser(cp.Chooser[1].Rect.Center())
			assert.Equal(t, "/w/lanternfish", root)
		})

		t.Run("and every district is counted, it should check them all", func(t *testing.T) {
			all := lay(someRangeData(), sel, ui.Chooser{Open: true})
			assert.Equal(t, []string{"[✓] kiln", "[✓] lanternfish", "[✓] tidepool"}, chooserLabels(all))
		})

		t.Run("it should stay inside the panel", func(t *testing.T) {
			last := cp.Chooser[len(cp.Chooser)-1].Rect
			assert.LessOrEqual(t, last.Max.Y, cp.Rect.Max.Y-grid)
		})
	})

	t.Run("when the range is custom", func(t *testing.T) {
		s := ui.RangeSelection{Kind: ui.CustomRange, Custom: stats.Range{First: day(time.September, 1), Last: day(time.September, 30)}, End: 1}
		d := someRangeData()
		d.Days = s.Custom
		cp := lay(d, s, ui.Chooser{})

		t.Run("it should title it with its days", func(t *testing.T) {
			assert.True(t, strings.HasPrefix(cp.Title.Text, "Power, 1 Sep – 30 Sep: "))
		})

		t.Run("it should bracket the focused end on its button", func(t *testing.T) {
			assert.Equal(t, "1 Sep – [30 Sep]", cp.Row.Long[6].Label.Text)
		})

		t.Run("it should say how to move the days", func(t *testing.T) {
			assert.Contains(t, cp.Footer.Text, "tab switches ends   ↑ ↓ a day   shift a month")
		})
	})
}

func legend(p ui.RangesPanel) []string {
	out := make([]string, len(p.Legend))
	for i, e := range p.Legend {
		out[i] = e.Label.Text
	}
	return out
}

func rowLabels(rows []ui.BreakdownRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Label.Text
	}
	return out
}

func chooserLabels(p ui.RangesPanel) []string {
	out := make([]string, len(p.Chooser))
	for i, r := range p.Chooser {
		out[i] = r.Label.Text
	}
	return out
}

func TestCustomLabel(t *testing.T) {
	s := ui.RangeSelection{Kind: ui.CustomRange, Custom: stats.Range{First: day(time.September, 1), Last: day(time.September, 30)}}

	t.Run("when custom is not chosen, it should read custom…", func(t *testing.T) {
		assert.Equal(t, "custom…", ui.RangeSelection{Kind: ui.Last3Months}.CustomLabel())
	})

	t.Run("when custom is chosen with the first end focused, it should bracket the first", func(t *testing.T) {
		assert.Equal(t, "[1 Sep] – 30 Sep", s.CustomLabel())
	})

	t.Run("when the days are in another year, it should name the year", func(t *testing.T) {
		s := s
		s.Custom.First.Year = 2025
		require.NotEqual(t, s.Custom.First.Year, s.Custom.Last.Year)
		assert.Equal(t, "[1 Sep 2025] – 30 Sep", s.CustomLabel())
	})
}
