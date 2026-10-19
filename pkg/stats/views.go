package stats

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/claude"
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
// numbers match what is drawn; Only nil means every district in the
// store. Item 116.
type Filter struct {
	Root func(dir string) string
	Only []string
}

// Bucket is one day or month of a series. NoData is a stretch wholly
// before the earliest message the filter counts, which is not the same
// as a stretch of zero; Incomplete is one that touches a gap recorded
// while statistics were off.
type Bucket struct {
	Start, Last Date
	Usage       claude.Usage
	NoData      bool
	Incomplete  bool
}

// Share is one model at one speed's part of a range.
type Share struct {
	Model, Speed string
	Usage        claude.Usage
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
	if len(ids) == 0 {
		return " AND 0", nil
	}
	return " AND m.dir_id IN (" + strings.Join(ids, ",") + ")", nil
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

const sums = "coalesce(sum(m.input), 0), coalesce(sum(m.output), 0), coalesce(sum(m.cache_create), 0), coalesce(sum(m.cache_read), 0), count(m.id)"

// Series is the range's usage, day by day or month by month, in loc's
// local days: each message counted once, by its id, from the raw
// counters.
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
	for i, sp := range sps {
		values[i] = fmt.Sprintf("(%d, %d, %d)", i, sp.lo, sp.hi)
	}
	rows, err := s.db.Query(`WITH edges (i, lo, hi) AS (VALUES ` + strings.Join(values, ", ") + `)
		SELECT e.i, ` + sums + ` FROM edges e
		LEFT JOIN messages m ON m.at >= e.lo AND m.at < e.hi` + where + `
		GROUP BY e.i ORDER BY e.i`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	buckets := make([]Bucket, len(sps))
	for rows.Next() {
		var i int
		var u claude.Usage
		if err := rows.Scan(&i, &u.Input, &u.Output, &u.CacheCreate, &u.CacheRead, &u.Messages); err != nil {
			return nil, err
		}
		sp := sps[i]
		buckets[i] = Bucket{Start: sp.first, Last: sp.last, Usage: u,
			NoData: !has || sp.last.Before(earliest), Incomplete: touches(gaps, sp)}
	}
	return buckets, rows.Err()
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

// ModelMix is the range's usage split by model and speed, in that
// order.
func (s *Store) ModelMix(r Range, f Filter, loc *time.Location) ([]Share, error) {
	where, err := s.dirs(f)
	if err != nil {
		return nil, err
	}
	sp := spanOf(r.First, r.Last, loc)
	rows, err := s.db.Query(`SELECT m.model, coalesce(m.speed, ''), `+sums+` FROM messages m
		WHERE m.at >= ? AND m.at < ?`+where+`
		GROUP BY 1, 2 ORDER BY 1, 2`, sp.lo, sp.hi)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var shares []Share
	for rows.Next() {
		var sh Share
		u := &sh.Usage
		if err := rows.Scan(&sh.Model, &sh.Speed, &u.Input, &u.Output, &u.CacheCreate, &u.CacheRead, &u.Messages); err != nil {
			return nil, err
		}
		shares = append(shares, sh)
	}
	return shares, rows.Err()
}

// Districts is every district the store holds messages for, by root,
// in order: hidden ones and those whose transcripts are gone included.
func (s *Store) Districts(root func(dir string) string) ([]string, error) {
	rows, err := s.db.Query("SELECT path FROM dirs WHERE id IN (SELECT DISTINCT dir_id FROM messages)")
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
