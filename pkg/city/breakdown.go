package city

import (
	"path/filepath"
	"sort"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/stats"
)

// Window is how far back a breakdown or a series looks.
type Window int

const (
	LastHour Window = iota
	LastDay
	LastWeek
)

func (w Window) String() string {
	switch w {
	case LastHour:
		return "1 h"
	case LastDay:
		return "24 h"
	}
	return "7 d"
}

// Hours is the window's length in hours.
func (w Window) Hours() int {
	switch w {
	case LastHour:
		return 1
	case LastDay:
		return 24
	}
	return 7 * 24
}

// Share is one slice of a breakdown: a model, a project or a session
// and what it used in the window.
type Share struct {
	Key      string
	Counters stats.Counters
	Cost     stats.Cost
}

// Breakdown is what the city used in a window, three ways. Priced is a
// breakdown read from the statistics store, each message under its own
// model and priced at list prices; otherwise it is counted from the
// sessions in memory, which cannot say which model sent what, so it has
// no cost and no split by model, and Reason says why. Item 116.
type Breakdown struct {
	Window    Window
	Counters  stats.Counters
	Cost      stats.Cost
	Priced    bool
	Reason    string
	ByModel   []Share
	ByProject []Share
	BySession []Share
	// Series is the cost of each hour when priced, and messages an
	// hour when not, oldest first.
	Series []float64
}

// Ledger is the statistics store as the city reads it.
type Ledger interface {
	Window(at time.Time, hours int, root func(string) string) (stats.Window, error)
	Reason() string
}

func counters(u claude.Usage) stats.Counters {
	return stats.Counters{Input: u.Input, Output: u.Output, WriteUnsplit: u.CacheCreate, Read: u.CacheRead, Messages: u.Messages}
}

func sharesOf(slices []stats.Slice, key func(string) string) []Share {
	out := make([]Share, 0, len(slices))
	byKey := map[string]int{}
	for _, sl := range slices {
		k := key(sl.Key)
		if i, ok := byKey[k]; ok {
			out[i].Counters, out[i].Cost = out[i].Counters.Add(sl.Counters), out[i].Cost.Add(sl.Cost)
			continue
		}
		byKey[k] = len(out)
		out = append(out, Share{Key: k, Counters: sl.Counters, Cost: sl.Cost})
	}
	return out
}

func same(s string) string { return s }

// Breakdown adds the city's usage up over a window: from the store when
// it can be read, and from the sessions in memory when it cannot.
func (c *City) Breakdown(w Window) Breakdown {
	if c.ledger == nil {
		return c.memoryBreakdown(w, stats.ReasonOff)
	}
	stored, err := c.ledger.Window(c.Time, w.Hours(), ProjectRoot)
	if err != nil {
		reason := c.ledger.Reason()
		if reason == "" {
			reason = stats.Reason(err)
		}
		return c.memoryBreakdown(w, reason)
	}
	b := Breakdown{Window: w, Counters: stored.Counters, Cost: stored.Cost, Priced: true,
		ByModel: sharesOf(stored.ByModel, same), ByProject: sharesOf(stored.ByProject, baseName), BySession: sharesOf(stored.BySession, same)}
	for _, h := range stored.Hourly {
		b.Series = append(b.Series, h.High)
	}
	return b
}

func (c *City) memoryBreakdown(w Window, reason string) Breakdown {
	b := Breakdown{Window: w, Reason: reason}
	byProject, bySession := map[string]*Share{}, map[string]*Share{}
	add := func(m map[string]*Share, key string, u stats.Counters) {
		if key == "" {
			key = "-"
		}
		share, ok := m[key]
		if !ok {
			share = &Share{Key: key}
			m[key] = share
		}
		share.Counters = share.Counters.Add(u)
	}
	var buckets []map[int64]claude.Usage
	for _, bld := range c.Buildings() {
		u := counters(windowUsage(bld, w, c.Time))
		buckets = append(buckets, bld.Session.Hourly)
		if u == (stats.Counters{}) {
			continue
		}
		b.Counters = b.Counters.Add(u)
		add(byProject, filepath.Base(ProjectRoot(bld.Session.CWD)), u)
		add(bySession, bld.Session.ID, u)
	}
	b.Cost = stats.Cost{Unpriced: b.Counters.Messages}
	b.ByProject, b.BySession = shares(byProject), shares(bySession)
	b.Series = series(w, c.Time, buckets...)
	return b
}

// windowUsage sums a session's buckets inside the window.
func windowUsage(s *Building, w Window, at time.Time) claude.Usage {
	since := at.Add(-time.Duration(w.Hours())*time.Hour).Unix() / 3600
	var sum claude.Usage
	for hour, u := range s.Session.Hourly {
		if hour > since {
			sum = sum.Add(u)
		}
	}
	return sum
}

func shares(m map[string]*Share) []Share {
	out := make([]Share, 0, len(m))
	for _, s := range m {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Counters.Messages != out[j].Counters.Messages {
			return out[i].Counters.Messages > out[j].Counters.Messages
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// series is messages an hour over a window, oldest first, from a set of
// bucket maps: what memory can say without adding differently priced
// counters together.
func series(w Window, at time.Time, buckets ...map[int64]claude.Usage) []float64 {
	n := w.Hours()
	last := at.Unix() / 3600
	out := make([]float64, n)
	for _, m := range buckets {
		for hour, u := range m {
			i := int(hour - (last - int64(n) + 1))
			if i >= 0 && i < n {
				out[i] += float64(u.Messages)
			}
		}
	}
	return out
}

// Series is the city's series over a window, oldest first: cost an
// hour from the store, messages an hour from memory.
func (c *City) Series(w Window) []float64 {
	return c.Breakdown(w).Series
}

// Series is the session's messages an hour over a window, oldest first.
func (b *Building) Series(w Window, at time.Time) []float64 {
	return series(w, at, b.Session.Hourly)
}

// Series is the district's messages an hour over a window, oldest first.
func (d *District) Series(w Window, at time.Time) []float64 {
	var buckets []map[int64]claude.Usage
	for _, b := range d.Buildings {
		buckets = append(buckets, b.Session.Hourly)
	}
	return series(w, at, buckets...)
}
