package stats

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Unrecorded is the speed of a message whose line carried none.
const Unrecorded = ""

// Unit is how a series is bucketed.
type Unit int

const (
	Days Unit = iota
	Months
)

// Filter picks the districts a view counts. Root maps a message's
// directory to its district, and is the rule the city map uses, so the
// numbers match what is drawn; Tied places a tied session's messages
// at its district's directory instead (item 285); Only nil means every
// district in the store. Item 116.
type Filter struct {
	Root func(dir string) string
	Tied map[string]string
	Only []string
}

// Bucket is one day or month of a series. NoData is a stretch wholly
// before the earliest message the filter counts, which is not the same
// as a stretch of zero; Incomplete is one that touches a gap recorded
// while statistics were off.
type Bucket struct {
	Start, Last Date
	Counters    Counters
	Cost        Cost
	NoData      bool
	Incomplete  bool
}

// Share is one model at one speed's part of a range.
type Share struct {
	Model, Speed string
	Counters     Counters
	Cost         Cost
}

// Coverage is how far back a view's data goes and what that means for
// the range it shows.
type Coverage struct {
	Earliest         Date
	HasData          bool
	StartsBeforeData bool
	TouchesGap       bool
}

type span struct {
	first, last Date
	lo, hi      int64
}

func spanOf(first, last Date, loc *time.Location) span {
	return span{first: first, last: last, lo: first.Start(loc).UnixMilli(), hi: last.AddDays(1).Start(loc).UnixMilli()}
}

func spans(r Range, unit Unit, loc *time.Location) []span {
	var out []span
	for d := r.First; !r.Last.Before(d); {
		last := d
		if unit == Months {
			last = clamped(d.Year, d.Month, 31)
			if r.Last.Before(last) {
				last = r.Last
			}
		}
		out = append(out, spanOf(d, last, loc))
		d = last.AddDays(1)
	}
	return out
}

// dirs is the SQL condition on m.dir_id the filter makes, or "" for
// every district.
func (s *Store) dirs(f Filter) (string, error) {
	if f.Only == nil {
		return "", nil
	}
	root := f.Root
	if root == nil {
		root = func(dir string) string { return dir }
	}
	chosen := map[string]bool{}
	for _, r := range f.Only {
		chosen[r] = true
	}
	rows, err := s.db.Query("SELECT id, path FROM dirs")
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return "", err
		}
		if chosen[root(path)] {
			ids = append(ids, strconv.FormatInt(id, 10))
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	own := "0"
	if len(ids) > 0 {
		own = "m.dir_id IN (" + strings.Join(ids, ",") + ")"
	}
	if len(f.Tied) == 0 {
		return " AND " + own, nil
	}
	var places []string
	seen := map[string]bool{}
	for _, d := range f.Tied {
		if !seen[d] && chosen[root(d)] {
			places = append(places, quote(d))
		}
		seen[d] = true
	}
	tied := tiedPlaces(f.Tied)
	where := "(m.id NOT IN (SELECT id FROM " + tied + ") AND " + own + ")"
	if len(places) > 0 {
		sort.Strings(places)
		at := "(SELECT id FROM " + tied + " WHERE place IN (" + strings.Join(places, ", ") + "))"
		where += " OR m.id IN " + at
		more, err := s.dirsOf(at)
		if err != nil {
			return "", err
		}
		ids = append(ids, more...)
	}
	if len(ids) == 0 {
		return " AND 0", nil
	}
	return " AND m.dir_id IN (" + strings.Join(ids, ",") + ") AND (" + where + ")", nil
}

// dirsOf is the directories the messages a query selects sit in, so a
// filter that also reads ties can still be found by directory: without
// it, SQLite reads every message in the range (item 285).
func (s *Store) dirsOf(messages string) ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT dir_id FROM messages WHERE id IN " + messages)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, strconv.FormatInt(id, 10))
	}
	return out, rows.Err()
}

func (s *Store) earliest(where string, loc *time.Location) (Date, bool, error) {
	var at sql.NullInt64
	if err := s.db.QueryRow("SELECT min(m.at) FROM messages m WHERE 1" + where).Scan(&at); err != nil {
		return Date{}, false, err
	}
	if !at.Valid {
		return Date{}, false, nil
	}
	return DateOf(time.UnixMilli(at.Int64).In(loc)), true, nil
}

type gap struct{ off, on int64 }

func (s *Store) gaps() ([]gap, error) {
	rows, err := s.db.Query("SELECT off_at, on_at FROM gaps")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []gap
	for rows.Next() {
		var g gap
		if err := rows.Scan(&g.off, &g.on); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func touches(gaps []gap, sp span) bool {
	for _, g := range gaps {
		if g.off < sp.hi && g.on > sp.lo {
			return true
		}
	}
	return false
}

// Series is the range's counters and cost, day by day or month by
// month, in loc's local days: each message counted once, by its id,
// and priced by the row in force when it was sent.
func (s *Store) Series(r Range, f Filter, unit Unit, loc *time.Location) ([]Bucket, error) {
	where, err := s.dirs(f)
	if err != nil {
		return nil, err
	}
	earliest, has, err := s.earliest(where, loc)
	if err != nil {
		return nil, err
	}
	gaps, err := s.gaps()
	if err != nil {
		return nil, err
	}
	sps := spans(r, unit, loc)
	if len(sps) == 0 {
		return nil, nil
	}
	values := make([]string, len(sps))
	buckets := make([]Bucket, len(sps))
	for i, sp := range sps {
		values[i] = fmt.Sprintf("(%d, %d, %d)", i, sp.lo, sp.hi)
		buckets[i] = Bucket{Start: sp.first, Last: sp.last,
			NoData: !has || sp.last.Before(earliest), Incomplete: touches(gaps, sp)}
	}
	groups, err := s.pricedBy(Prices, "WITH edges (i, lo, hi) AS (VALUES "+strings.Join(values, ", ")+") ", "e.i",
		"edges e JOIN messages m ON m.at >= e.lo AND m.at < e.hi", where)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		i, err := strconv.Atoi(g.key)
		if err != nil {
			return nil, err
		}
		buckets[i].Counters = buckets[i].Counters.Add(g.priced.Counters)
		buckets[i].Cost = buckets[i].Cost.Add(Prices.Price(g.priced))
	}
	return buckets, nil
}

// Totals is the range's counters and cost in all.
func (s *Store) Totals(r Range, f Filter, loc *time.Location) (Counters, Cost, error) {
	where, err := s.dirs(f)
	if err != nil {
		return Counters{}, Cost{}, err
	}
	sp := spanOf(r.First, r.Last, loc)
	groups, err := s.pricedBy(Prices, "", "", "messages m", " AND m.at >= ? AND m.at < ?"+where, sp.lo, sp.hi)
	if err != nil {
		return Counters{}, Cost{}, err
	}
	var counters Counters
	var cost Cost
	for _, g := range groups {
		counters = counters.Add(g.priced.Counters)
		cost = cost.Add(Prices.Price(g.priced))
	}
	return counters, cost, nil
}

// Coverage says how far back the filter's data goes, whether the range
// reaches before it, and whether it touches a recorded gap.
func (s *Store) Coverage(r Range, f Filter, loc *time.Location) (Coverage, error) {
	where, err := s.dirs(f)
	if err != nil {
		return Coverage{}, err
	}
	earliest, has, err := s.earliest(where, loc)
	if err != nil {
		return Coverage{}, err
	}
	gaps, err := s.gaps()
	if err != nil {
		return Coverage{}, err
	}
	return Coverage{Earliest: earliest, HasData: has,
		StartsBeforeData: !has || r.First.Before(earliest),
		TouchesGap:       touches(gaps, spanOf(r.First, r.Last, loc))}, nil
}

// ModelMix is the range's counters and cost by model and speed, the
// dearest first.
func (s *Store) ModelMix(r Range, f Filter, loc *time.Location) ([]Share, error) {
	where, err := s.dirs(f)
	if err != nil {
		return nil, err
	}
	sp := spanOf(r.First, r.Last, loc)
	groups, err := s.pricedBy(Prices, "", "", "messages m", " AND m.at >= ? AND m.at < ?"+where, sp.lo, sp.hi)
	if err != nil {
		return nil, err
	}
	byKey := map[[2]string]*Share{}
	var shares []*Share
	for _, g := range groups {
		k := [2]string{g.priced.Model, g.priced.Speed}
		sh, ok := byKey[k]
		if !ok {
			sh = &Share{Model: k[0], Speed: k[1]}
			byKey[k] = sh
			shares = append(shares, sh)
		}
		sh.Counters = sh.Counters.Add(g.priced.Counters)
		sh.Cost = sh.Cost.Add(Prices.Price(g.priced))
	}
	sort.Slice(shares, func(i, j int) bool {
		a, b := shares[i], shares[j]
		if a.Cost.High != b.Cost.High {
			return a.Cost.High > b.Cost.High
		}
		if a.Model != b.Model {
			return a.Model < b.Model
		}
		return a.Speed < b.Speed
	})
	out := make([]Share, len(shares))
	for i, sh := range shares {
		out[i] = *sh
	}
	return out, nil
}

// Districts is every district the store holds messages for, by root,
// in order: hidden ones and those whose transcripts are gone included,
// and a tied session's at its district's directory.
func (s *Store) Districts(root func(dir string) string, tied map[string]string) ([]string, error) {
	query := "SELECT path FROM dirs WHERE id IN (SELECT DISTINCT dir_id FROM messages)"
	if len(tied) > 0 {
		places := tiedPlaces(tied)
		query = "SELECT path FROM dirs WHERE id IN (SELECT DISTINCT dir_id FROM messages WHERE id NOT IN (SELECT id FROM " + places +
			")) UNION SELECT DISTINCT place FROM " + places
	}
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	seen := map[string]bool{}
	var out []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		if root != nil {
			path = root(path)
		}
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out, rows.Err()
}
