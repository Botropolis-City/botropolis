package stats

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Slice is one key's part of a window: a model, a project or a session.
type Slice struct {
	Key      string
	Counters Counters
	Cost     Cost
}

// Window is what was used between two moments: in all, by model, by
// project and by session, and the cost of each hour, oldest first. A
// message a fork copied counts once in the total and in every session
// that holds it. Item 116.
type Window struct {
	Counters                      Counters
	Cost                          Cost
	ByModel, ByProject, BySession []Slice
	Hourly                        []Cost
}

func slices(groups []keyed, key func(string) string) []Slice {
	byKey := map[string]*Slice{}
	var out []*Slice
	for _, g := range groups {
		k := key(g.key)
		sl, ok := byKey[k]
		if !ok {
			sl = &Slice{Key: k}
			byKey[k] = sl
			out = append(out, sl)
		}
		sl.Counters = sl.Counters.Add(g.priced.Counters)
		sl.Cost = sl.Cost.Add(Prices.Price(g.priced))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cost.High != out[j].Cost.High {
			return out[i].Cost.High > out[j].Cost.High
		}
		return out[i].Key < out[j].Key
	})
	res := make([]Slice, len(out))
	for i, sl := range out {
		res[i] = *sl
	}
	return res
}

func same(k string) string { return k }

// Window is what was used from since until until, with hours buckets for
// the hourly cost; root maps a directory to its project.
func (s *Store) Window(since, until time.Time, hours int, root func(string) string) (Window, error) {
	var w Window
	in := " AND m.at >= ? AND m.at < ?"
	lo, hi := since.UnixMilli(), until.UnixMilli()
	all, err := s.pricedBy(Prices, "", "", "messages m", in, lo, hi)
	if err != nil {
		return w, err
	}
	for _, sl := range slices(all, same) {
		w.Counters, w.Cost = w.Counters.Add(sl.Counters), w.Cost.Add(sl.Cost)
	}
	if w.ByModel, err = s.windowSlices("m.model", "messages m", in, same, lo, hi); err != nil {
		return w, err
	}
	if root == nil {
		root = same
	}
	if w.ByProject, err = s.windowSlices("d.path", "messages m JOIN dirs d ON d.id = m.dir_id", in, root, lo, hi); err != nil {
		return w, err
	}
	if w.BySession, err = s.windowSlices("ms.session_id", "messages m JOIN message_sessions ms ON ms.message_id = m.id", in, same, lo, hi); err != nil {
		return w, err
	}
	w.Hourly, err = s.hourly(until, hours)
	return w, err
}

func (s *Store) windowSlices(key, from, where string, mapKey func(string) string, args ...any) ([]Slice, error) {
	groups, err := s.pricedBy(Prices, "", key, from, where, args...)
	if err != nil {
		return nil, err
	}
	return slices(groups, mapKey), nil
}

func (s *Store) hourly(until time.Time, hours int) ([]Cost, error) {
	if hours <= 0 {
		return nil, nil
	}
	end := until.UnixMilli()
	values := make([]string, hours)
	for i := range hours {
		lo := end - int64(hours-i)*time.Hour.Milliseconds()
		values[i] = fmt.Sprintf("(%d, %d, %d)", i, lo, lo+time.Hour.Milliseconds())
	}
	groups, err := s.pricedBy(Prices, "WITH edges (i, lo, hi) AS (VALUES "+strings.Join(values, ", ")+") ", "e.i",
		"edges e JOIN messages m ON m.at >= e.lo AND m.at < e.hi", "")
	if err != nil {
		return nil, err
	}
	out := make([]Cost, hours)
	for _, g := range groups {
		i, err := strconv.Atoi(g.key)
		if err != nil {
			return nil, err
		}
		out[i] = out[i].Add(Prices.Price(g.priced))
	}
	return out, nil
}
