package city

import (
	"path/filepath"
	"sort"
	"time"

	"botropolis.city/botropolis/pkg/claude"
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
// and what it spent in the window.
type Share struct {
	Key     string
	Tokens  int64
	CostUSD float64
	// CostKnown is whether any session in the share carried a cost.
	CostKnown bool
}

// Breakdown is what the city spent in a window, three ways.
type Breakdown struct {
	Window    Window
	Tokens    int64
	CostUSD   float64
	CostKnown bool
	ByModel   []Share
	ByProject []Share
	BySession []Share
}

// tokens is what a usage bucket spent: everything the API had to read
// or write.
func tokens(u claude.Usage) int64 {
	return u.Context() + u.Output
}

// windowUsage sums a session's buckets inside the window.
func windowUsage(s *Building, w Window, at time.Time) int64 {
	since := at.Add(-time.Duration(w.Hours())*time.Hour).Unix() / 3600
	var sum int64
	for hour, u := range s.Session.Hourly {
		if hour > since {
			sum += tokens(u)
		}
	}
	return sum
}

// Breakdown adds the city's spend up over a window by model, project and
// session; cost is each session's lifetime cost pro-rated by the
// window's share of its tokens, an estimate shown as one.
func (c *City) Breakdown(w Window) Breakdown {
	b := Breakdown{Window: w}
	byModel, byProject, bySession := map[string]*Share{}, map[string]*Share{}, map[string]*Share{}
	add := func(m map[string]*Share, key string, tok int64, usd float64, known bool) {
		if key == "" {
			key = "-"
		}
		share, ok := m[key]
		if !ok {
			share = &Share{Key: key}
			m[key] = share
		}
		share.Tokens += tok
		share.CostUSD += usd
		share.CostKnown = share.CostKnown || known
	}
	for _, bld := range c.Buildings() {
		tok := windowUsage(bld, w, c.Time)
		if tok == 0 {
			continue
		}
		usd := 0.0
		if all := tokens(bld.Session.Usage); all > 0 {
			usd = bld.Session.CostUSD * float64(tok) / float64(all)
		}
		known := bld.Session.CostKnown
		b.Tokens += tok
		b.CostUSD += usd
		b.CostKnown = b.CostKnown || known
		add(byModel, bld.Session.Model, tok, usd, known)
		add(byProject, filepath.Base(ProjectRoot(bld.Session.CWD)), tok, usd, known)
		add(bySession, bld.Session.ID, tok, usd, known)
	}
	b.ByModel, b.ByProject, b.BySession = shares(byModel), shares(byProject), shares(bySession)
	return b
}

func shares(m map[string]*Share) []Share {
	out := make([]Share, 0, len(m))
	for _, s := range m {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tokens != out[j].Tokens {
			return out[i].Tokens > out[j].Tokens
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// series is tokens per hour over a window, oldest first, from a set of
// bucket maps.
func series(w Window, at time.Time, buckets ...map[int64]claude.Usage) []float64 {
	n := w.Hours()
	last := at.Unix() / 3600
	out := make([]float64, n)
	for _, m := range buckets {
		for hour, u := range m {
			i := int(hour - (last - int64(n) + 1))
			if i >= 0 && i < n {
				out[i] += float64(tokens(u))
			}
		}
	}
	return out
}

// Series is the city's tokens per hour over a window, oldest first.
func (c *City) Series(w Window) []float64 {
	var buckets []map[int64]claude.Usage
	for _, b := range c.Buildings() {
		buckets = append(buckets, b.Session.Hourly)
	}
	return series(w, c.Time, buckets...)
}

// Series is the session's tokens per hour over a window, oldest first.
func (b *Building) Series(w Window, at time.Time) []float64 {
	return series(w, at, b.Session.Hourly)
}

// Series is the district's tokens per hour over a window, oldest first.
func (d *District) Series(w Window, at time.Time) []float64 {
	var buckets []map[int64]claude.Usage
	for _, b := range d.Buildings {
		buckets = append(buckets, b.Session.Hourly)
	}
	return series(w, at, buckets...)
}
