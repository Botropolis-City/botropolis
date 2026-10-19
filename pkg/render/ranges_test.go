package render

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

var (
	rangesNow   = time.Date(2026, 10, 19, 15, 0, 0, 0, time.Local)
	rangesToday = stats.DateOf(rangesNow)
	rangesLoc   = time.FixedZone("test", -4*3600)
)

type query struct {
	call string
	r    stats.Range
	only []string
	unit stats.Unit
	loc  *time.Location
	root func(string) string
}

type fakeQueries struct {
	mu    sync.Mutex
	calls []query
	gate  chan struct{}
	err   error
	roots []string
}

func (f *fakeQueries) record(q query) error {
	if f.gate != nil {
		<-f.gate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, q)
	return f.err
}

func (f *fakeQueries) Series(r stats.Range, flt stats.Filter, unit stats.Unit, loc *time.Location) ([]stats.Bucket, error) {
	err := f.record(query{call: "series", r: r, only: flt.Only, unit: unit, loc: loc, root: flt.Root})
	return []stats.Bucket{{Start: r.First, Last: r.First, Counters: stats.Counters{Messages: 1}}}, err
}

func (f *fakeQueries) Totals(r stats.Range, flt stats.Filter, loc *time.Location) (stats.Counters, stats.Cost, error) {
	err := f.record(query{call: "totals", r: r, only: flt.Only, loc: loc, root: flt.Root})
	usd := 1.0
	if len(flt.Only) == 1 && flt.Only[0] == "/w/kiln" {
		usd = 7
	}
	return stats.Counters{Messages: 2}, stats.Cost{Low: usd, High: usd}, err
}

func (f *fakeQueries) Coverage(r stats.Range, flt stats.Filter, loc *time.Location) (stats.Coverage, error) {
	err := f.record(query{call: "coverage", r: r, only: flt.Only, loc: loc, root: flt.Root})
	return stats.Coverage{HasData: true, Earliest: r.First}, err
}

func (f *fakeQueries) ModelMix(r stats.Range, flt stats.Filter, loc *time.Location) ([]stats.Share, error) {
	err := f.record(query{call: "mix", r: r, only: flt.Only, loc: loc, root: flt.Root})
	return []stats.Share{{Model: "claude-opus-5"}}, err
}

func (f *fakeQueries) Districts(root func(string) string) ([]string, error) {
	err := f.record(query{call: "districts", root: root})
	return f.roots, err
}

func (f *fakeQueries) find(call string, r stats.Range) []query {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []query
	for _, q := range f.calls {
		if q.call == call && q.r == r {
			out = append(out, q)
		}
	}
	return out
}

func newFake() *fakeQueries {
	return &fakeQueries{roots: []string{"/w/kiln", "/w/lanternfish", "/w/tidepool"}}
}

func TestLoadRanges(t *testing.T) {
	sel := ui.RangeSelection{Kind: ui.Last3Months, SameDay: true, Only: []string{"/w/kiln", "/w/tidepool"}}
	days := stats.Last3Months(rangesToday)
	cmp := stats.MonthComparison(rangesToday, true)

	t.Run("when a selection is loaded", func(t *testing.T) {
		f := newFake()
		d, err := loadRanges(f, sel, rangesToday, rangesLoc)
		require.NoError(t, err)

		t.Run("it should read the series for the selection's days", func(t *testing.T) {
			assert.Len(t, f.find("series", days), 1)
		})

		t.Run("it should read them in the zone it is given", func(t *testing.T) {
			assert.Same(t, rangesLoc, f.find("series", days)[0].loc)
		})

		t.Run("it should bucket by the selection's unit", func(t *testing.T) {
			assert.Equal(t, stats.Days, f.find("series", days)[0].unit)
		})

		t.Run("it should count only the chosen districts", func(t *testing.T) {
			assert.Equal(t, sel.Only, f.find("series", days)[0].only)
		})

		t.Run("it should map a directory to its district as the city does", func(t *testing.T) {
			assert.Equal(t, "/w/kiln", f.find("series", days)[0].root("/w/kiln/.claude/worktrees/item-1"))
		})

		t.Run("it should carry the buckets", func(t *testing.T) {
			assert.Len(t, d.Buckets, 1)
		})

		t.Run("it should carry the range's totals", func(t *testing.T) {
			assert.Equal(t, 2, d.Counters.Messages)
		})

		t.Run("it should carry the coverage", func(t *testing.T) {
			assert.Equal(t, days.First, d.Coverage.Earliest)
		})

		t.Run("it should carry the model mix", func(t *testing.T) {
			assert.Equal(t, "claude-opus-5", d.Mix[0].Model)
		})

		t.Run("it should carry every district in the store", func(t *testing.T) {
			assert.Equal(t, f.roots, d.All)
		})

		t.Run("it should give each counted district its own totals", func(t *testing.T) {
			assert.Equal(t, []ui.DistrictFigure{
				{Root: "/w/kiln", Name: "kiln", Counters: stats.Counters{Messages: 2}, Cost: stats.Cost{Low: 7, High: 7}},
				{Root: "/w/tidepool", Name: "tidepool", Counters: stats.Counters{Messages: 2}, Cost: stats.Cost{Low: 1, High: 1}},
			}, d.Districts)
		})

		t.Run("it should read this month for the comparison", func(t *testing.T) {
			assert.Equal(t, ui.MonthSide{Days: cmp.This, Counters: stats.Counters{Messages: 2}, Cost: stats.Cost{Low: 1, High: 1}, Coverage: stats.Coverage{HasData: true, Earliest: cmp.This.First}}, d.This)
		})

		t.Run("it should read last month to the same day for the comparison", func(t *testing.T) {
			assert.Equal(t, cmp.Last, d.Last.Days)
		})

		t.Run("it should not be loading", func(t *testing.T) {
			assert.False(t, d.Loading)
		})
	})

	t.Run("when every district is counted", func(t *testing.T) {
		f := newFake()
		d, err := loadRanges(f, ui.RangeSelection{Kind: ui.Last3Months}, rangesToday, rangesLoc)
		require.NoError(t, err)

		t.Run("it should give every district in the store its totals", func(t *testing.T) {
			assert.Len(t, d.Districts, 3)
		})
	})

	t.Run("when the same-day toggle is off", func(t *testing.T) {
		f := newFake()
		d, err := loadRanges(f, ui.RangeSelection{Kind: ui.Last3Months}, rangesToday, rangesLoc)
		require.NoError(t, err)

		t.Run("it should compare with last month in full", func(t *testing.T) {
			assert.Equal(t, stats.LastMonth(rangesToday), d.Last.Days)
		})
	})

	t.Run("when a query fails", func(t *testing.T) {
		f := newFake()
		f.err = errors.New("disk I/O error")
		_, err := loadRanges(f, sel, rangesToday, rangesLoc)

		t.Run("it should say so", func(t *testing.T) {
			assert.ErrorIs(t, err, f.err)
		})
	})
}

type fakeLedger struct {
	mu     sync.Mutex
	q      rangeQueries
	reason string
}

func (l *fakeLedger) open() (rangeQueries, bool, string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.q == nil {
		return nil, false, l.reason
	}
	return l.q, true, ""
}

func TestRangeSource(t *testing.T) {
	sel := ui.RangeSelection{Kind: ui.Last3Months, SameDay: true}
	clock := func(at *time.Time) func() time.Time { return func() time.Time { return *at } }
	settled := func(t *testing.T, s *rangeSource) ui.RangeData {
		t.Helper()
		var d ui.RangeData
		require.Eventually(t, func() bool { d = s.read(sel, rangesToday); return !d.Loading }, 2*time.Second, time.Millisecond)
		return d
	}

	t.Run("when the store is still opening", func(t *testing.T) {
		s := newRangeSource((&fakeLedger{}).open, time.Now, rangesLoc)

		t.Run("it should say so", func(t *testing.T) {
			state, _ := s.state()
			assert.Equal(t, ui.StoreOpening, state)
		})
	})

	t.Run("when the store is paused", func(t *testing.T) {
		s := newRangeSource((&fakeLedger{reason: "there are no statistics yet"}).open, time.Now, rangesLoc)
		state, reason := s.state()

		t.Run("it should say it is paused", func(t *testing.T) {
			assert.Equal(t, ui.StorePaused, state)
		})

		t.Run("it should say why", func(t *testing.T) {
			assert.Equal(t, "there are no statistics yet", reason)
		})
	})

	t.Run("when the store is readable", func(t *testing.T) {
		t.Run("and nothing has been read yet", func(t *testing.T) {
			f := newFake()
			f.gate = make(chan struct{})
			defer close(f.gate)
			s := newRangeSource((&fakeLedger{q: f}).open, time.Now, rangesLoc)
			d := s.read(sel, rangesToday)

			t.Run("it should say it is loading", func(t *testing.T) {
				assert.True(t, d.Loading)
			})

			t.Run("it should carry the selection's days", func(t *testing.T) {
				assert.Equal(t, stats.Last3Months(rangesToday), d.Days)
			})
		})

		t.Run("and the read has finished", func(t *testing.T) {
			s := newRangeSource((&fakeLedger{q: newFake()}).open, time.Now, rangesLoc)

			t.Run("it should give the data", func(t *testing.T) {
				assert.Len(t, settled(t, s).All, 3)
			})
		})

		t.Run("and the store is slow", func(t *testing.T) {
			at := rangesNow
			f := newFake()
			s := newRangeSource((&fakeLedger{q: f}).open, clock(&at), rangesLoc)
			settled(t, s)
			f.gate = make(chan struct{})
			defer close(f.gate)
			at = at.Add(31 * time.Second)

			read := make(chan ui.RangeData, 1)
			go func() {
				d := s.read(sel, rangesToday)
				s.read(sel, rangesToday)
				read <- d
			}()
			var d ui.RangeData
			var waited bool
			select {
			case d = <-read:
			case <-time.After(200 * time.Millisecond):
				waited = true
			}

			t.Run("it should not wait for the refresh", func(t *testing.T) {
				assert.False(t, waited)
			})

			t.Run("it should keep drawing the last data", func(t *testing.T) {
				assert.False(t, d.Loading)
			})
		})

		t.Run("and the data is fresh", func(t *testing.T) {
			at := rangesNow
			f := newFake()
			s := newRangeSource((&fakeLedger{q: f}).open, clock(&at), rangesLoc)
			settled(t, s)
			at = at.Add(29 * time.Second)
			s.read(sel, rangesToday)

			t.Run("it should not read again", func(t *testing.T) {
				assert.Len(t, f.find("series", stats.Last3Months(rangesToday)), 1)
			})
		})

		t.Run("and the selection changes", func(t *testing.T) {
			f := newFake()
			s := newRangeSource((&fakeLedger{q: f}).open, time.Now, rangesLoc)
			settled(t, s)
			f.gate = make(chan struct{})
			defer close(f.gate)
			d := s.read(ui.RangeSelection{Kind: ui.LastMonth}, rangesToday)

			t.Run("it should not show the old selection's data", func(t *testing.T) {
				assert.True(t, d.Loading)
			})
		})

		t.Run("and a query fails", func(t *testing.T) {
			f := newFake()
			f.err = errors.New("disk I/O error")
			s := newRangeSource((&fakeLedger{q: f}).open, time.Now, rangesLoc)
			s.read(sel, rangesToday)
			var state ui.StoreState
			var reason string
			require.Eventually(t, func() bool { state, reason = s.state(); return state == ui.StorePaused }, 2*time.Second, time.Millisecond)

			t.Run("it should say why in plain words", func(t *testing.T) {
				assert.Equal(t, stats.Reason(f.err), reason)
			})
		})
	})
}

func rangesGame(t *testing.T, q rangeQueries, reason string) *Game {
	t.Helper()
	g := &Game{scene: city.NewScene(city.NewLayout()), scripted: -1, clicked: -1, breakdown: true, window: city.LastWeek}
	g.rangeClock = func() time.Time { return rangesNow }
	g.ranges = newRangeSource((&fakeLedger{q: q, reason: reason}).open, g.rangeClock, rangesLoc)
	if q != nil {
		require.Eventually(t, func() bool { return !g.ranges.read(g.rangeSel, rangesToday).Loading }, 2*time.Second, time.Millisecond)
	}
	return g
}

func pressRange(t *testing.T, g *Game, key ebiten.Key, shift bool) {
	t.Helper()
	g.scripted, g.scriptedShift = key, shift
	require.NoError(t, g.handleKeys())
	g.scripted, g.scriptedShift = -1, false
}

func onRange(t *testing.T, kind ui.RangeKind) *Game {
	t.Helper()
	g := rangesGame(t, newFake(), "")
	g.rangeOn, g.rangeSel = true, g.rangeSel.Choose(kind, rangesToday)
	return g
}

func TestRangeKeys(t *testing.T) {
	t.Run("when the panel is on 7 d", func(t *testing.T) {
		t.Run("and → is pressed with the store readable", func(t *testing.T) {
			g := rangesGame(t, newFake(), "")
			pressRange(t, g, ebiten.KeyArrowRight, false)

			t.Run("it should go to month to date", func(t *testing.T) {
				assert.Equal(t, [2]any{true, ui.MonthToDate}, [2]any{g.rangeOn, g.rangeSel.Kind})
			})

			t.Run("it should leave the short window as it was", func(t *testing.T) {
				assert.Equal(t, city.LastWeek, g.window)
			})
		})

		t.Run("and → is pressed with the store paused", func(t *testing.T) {
			g := rangesGame(t, nil, "there are no statistics yet")
			pressRange(t, g, ebiten.KeyArrowRight, false)

			t.Run("it should wrap to 1 h", func(t *testing.T) {
				assert.Equal(t, [2]any{false, city.LastHour}, [2]any{g.rangeOn, g.window})
			})
		})
	})

	t.Run("when the panel is on a long range", func(t *testing.T) {
		t.Run("and → is pressed", func(t *testing.T) {
			g := onRange(t, ui.MonthToDate)
			pressRange(t, g, ebiten.KeyArrowRight, false)

			t.Run("it should go to the next range", func(t *testing.T) {
				assert.Equal(t, ui.LastMonth, g.rangeSel.Kind)
			})
		})

		t.Run("and ← is pressed on month to date", func(t *testing.T) {
			g := onRange(t, ui.MonthToDate)
			pressRange(t, g, ebiten.KeyArrowLeft, false)

			t.Run("it should go back to 7 d", func(t *testing.T) {
				assert.Equal(t, [2]any{false, city.LastWeek}, [2]any{g.rangeOn, g.window})
			})
		})

		t.Run("and 1 is pressed", func(t *testing.T) {
			g := onRange(t, ui.Last3Months)
			pressRange(t, g, ebiten.KeyDigit1, false)

			t.Run("it should go to 1 h", func(t *testing.T) {
				assert.Equal(t, [2]any{false, city.LastHour}, [2]any{g.rangeOn, g.window})
			})
		})

		t.Run("and s is pressed", func(t *testing.T) {
			g := onRange(t, ui.Last3Months)
			before := g.rangeSel.SameDay
			pressRange(t, g, ebiten.KeyS, false)

			t.Run("it should toggle the same day", func(t *testing.T) {
				assert.Equal(t, !before, g.rangeSel.SameDay)
			})
		})

		t.Run("and d is pressed", func(t *testing.T) {
			g := onRange(t, ui.Last3Months)
			pressRange(t, g, ebiten.KeyD, false)

			t.Run("it should open the chooser", func(t *testing.T) {
				assert.True(t, g.rangeChooser.Open)
			})
		})

		t.Run("and esc is pressed", func(t *testing.T) {
			g := onRange(t, ui.Last3Months)
			pressRange(t, g, ebiten.KeyEscape, false)

			t.Run("it should close the panel", func(t *testing.T) {
				assert.False(t, g.breakdown)
			})
		})

		t.Run("and the store pauses", func(t *testing.T) {
			g := rangesGame(t, nil, "the store is being upgraded")
			g.rangeOn, g.rangeSel.Kind = true, ui.Last3Months
			pressRange(t, g, ebiten.KeyArrowRight, false)

			t.Run("it should answer as the short windows do", func(t *testing.T) {
				assert.Equal(t, [2]any{false, city.LastHour}, [2]any{g.rangeOn, g.window})
			})
		})
	})

	t.Run("when the chooser is open", func(t *testing.T) {
		chooser := func(t *testing.T) *Game {
			g := onRange(t, ui.Last3Months)
			pressRange(t, g, ebiten.KeyD, false)
			return g
		}

		t.Run("and esc is pressed", func(t *testing.T) {
			g := chooser(t)
			pressRange(t, g, ebiten.KeyEscape, false)

			t.Run("it should close the chooser", func(t *testing.T) {
				assert.False(t, g.rangeChooser.Open)
			})

			t.Run("it should leave the panel open", func(t *testing.T) {
				assert.True(t, g.breakdown)
			})
		})

		t.Run("and ↓ then space is pressed", func(t *testing.T) {
			g := chooser(t)
			pressRange(t, g, ebiten.KeyArrowDown, false)
			pressRange(t, g, ebiten.KeySpace, false)

			t.Run("it should stop counting the focused district", func(t *testing.T) {
				assert.Equal(t, []string{"/w/kiln", "/w/tidepool"}, g.rangeSel.Only)
			})
		})

		t.Run("and ↑ is pressed on the first row", func(t *testing.T) {
			g := chooser(t)
			pressRange(t, g, ebiten.KeyArrowUp, false)

			t.Run("it should stay on the first row", func(t *testing.T) {
				assert.Equal(t, 0, g.rangeChooser.Focus)
			})
		})

		t.Run("and a is pressed", func(t *testing.T) {
			g := chooser(t)
			g.rangeSel.Only = []string{"/w/kiln"}
			pressRange(t, g, ebiten.KeyA, false)

			t.Run("it should count every district", func(t *testing.T) {
				assert.Nil(t, g.rangeSel.Only)
			})
		})

		t.Run("and → is pressed", func(t *testing.T) {
			g := chooser(t)
			pressRange(t, g, ebiten.KeyArrowRight, false)

			t.Run("it should leave the range alone", func(t *testing.T) {
				assert.Equal(t, ui.Last3Months, g.rangeSel.Kind)
			})
		})
	})

	t.Run("when the range is custom", func(t *testing.T) {
		custom := func(t *testing.T) *Game {
			g := onRange(t, ui.CustomRange)
			return g
		}
		start := stats.Last30Days(rangesToday)

		t.Run("and ↓ is pressed", func(t *testing.T) {
			g := custom(t)
			pressRange(t, g, ebiten.KeyArrowDown, false)

			t.Run("it should move the first day a day earlier", func(t *testing.T) {
				assert.Equal(t, start.First.AddDays(-1), g.rangeSel.Custom.First)
			})
		})

		t.Run("and shift+↓ is pressed", func(t *testing.T) {
			g := custom(t)
			pressRange(t, g, ebiten.KeyArrowDown, true)

			t.Run("it should move the first day a month earlier", func(t *testing.T) {
				assert.Equal(t, stats.Date{Year: 2026, Month: time.August, Day: 20}, g.rangeSel.Custom.First)
			})
		})

		t.Run("and tab then ↓ is pressed", func(t *testing.T) {
			g := custom(t)
			pressRange(t, g, ebiten.KeyTab, false)
			pressRange(t, g, ebiten.KeyArrowDown, false)

			t.Run("it should move the last day", func(t *testing.T) {
				assert.Equal(t, rangesToday.AddDays(-1), g.rangeSel.Custom.Last)
			})
		})
	})
}

func TestRangeClicks(t *testing.T) {
	th := ui.NewTheme(1)
	data := func(g *Game) ui.RangeData { return g.ranges.read(g.rangeSel, rangesToday) }
	laid := func(t *testing.T, g *Game, ch ui.Chooser) ui.RangesPanel {
		t.Helper()
		p := ui.LayoutRanges(th, 1600, 1000, data(g), g.rangeSel, ch, func(s string, _ ui.Size) (float64, float64) { return float64(len([]rune(s))) * 7, 16 })
		g.rangesLayout = p
		return p
	}

	t.Run("when a long range is clicked on the short panel's row", func(t *testing.T) {
		g := rangesGame(t, newFake(), "")
		g.rowLayout = ui.LayoutWindowRow(th, city.Point{X: 10, Y: 10}, 900, ui.RowState{Store: ui.StoreReadable}, func(s string, _ ui.Size) (float64, float64) { return float64(len(s)) * 7, 16 })
		g.clickBreakdown(g.rowLayout.Long[3].Rect.Center())

		t.Run("it should choose it", func(t *testing.T) {
			assert.Equal(t, [2]any{true, ui.Last3Months}, [2]any{g.rangeOn, g.rangeSel.Kind})
		})
	})

	t.Run("when a short window is clicked on the long panel", func(t *testing.T) {
		g := onRange(t, ui.Last3Months)
		p := laid(t, g, ui.Chooser{})
		g.clickBreakdown(p.Row.Short[0].Rect.Center())

		t.Run("it should return to the short windows on it", func(t *testing.T) {
			assert.Equal(t, [2]any{false, city.LastHour}, [2]any{g.rangeOn, g.window})
		})
	})

	t.Run("when the filter line is clicked", func(t *testing.T) {
		g := onRange(t, ui.Last3Months)
		p := laid(t, g, ui.Chooser{})
		g.clickBreakdown(p.FilterRect.Center())

		t.Run("it should open the chooser", func(t *testing.T) {
			assert.True(t, g.rangeChooser.Open)
		})
	})

	t.Run("when a chooser row is clicked", func(t *testing.T) {
		g := onRange(t, ui.Last3Months)
		g.rangeChooser.Open = true
		p := laid(t, g, g.rangeChooser)
		g.clickBreakdown(p.Chooser[2].Rect.Center())

		t.Run("it should toggle its district", func(t *testing.T) {
			assert.Equal(t, []string{"/w/kiln", "/w/lanternfish"}, g.rangeSel.Only)
		})
	})

	t.Run("when the pointer clicks outside the panel", func(t *testing.T) {
		g := onRange(t, ui.Last3Months)
		laid(t, g, ui.Chooser{})
		g.clickBreakdown(city.Point{X: 1, Y: 1})

		t.Run("it should close it", func(t *testing.T) {
			assert.False(t, g.breakdown)
		})
	})
}
