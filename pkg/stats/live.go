package stats

import (
	"database/sql"
	"errors"
	"maps"
	"sort"
	"strconv"
	"time"
)

// querier is the store's database or a transaction on it.
type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// class is everything that decides a message's price but its counters,
// as pricedSelect groups by it.
type class struct {
	model, speed, geo, tier string
	long                    bool
	era                     int
}

// held is a message as a ledger keeps it to add a window up.
type held struct {
	at       int64
	class    class
	counters Counters
	dir      string
	sessions []string
}

// liveWindow is a window's messages as a ledger last read them, brought
// up to date from the change log, and the answer they last gave. Item
// 215.
type liveWindow struct {
	since   int64
	seq     int64
	logged  bool
	held    map[string]*held
	at      time.Time
	version int64
	tied    map[string]string
	window  Window
}

// liveCosts is every session's cost as a ledger last read it, and the
// change log's seq then.
type liveCosts struct {
	seq     int64
	logged  bool
	costs   map[string]Cost
	at      time.Time
	version int64
}

// changeSeq is the change log's latest seq, and whether the store has
// one: a store only an older build wrote has none.
func changeSeq(q querier) (int64, bool, error) {
	var seq int64
	err := q.QueryRow("SELECT value FROM meta WHERE key = 'change_seq'").Scan(&seq)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	var tables int
	if err := q.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name IN ('changed_messages', 'changed_sessions')").Scan(&tables); err != nil {
		return 0, false, err
	}
	return seq, tables == 2, nil
}

const heldColumns = `m.id, m.at, m.model, coalesce(m.speed, ''), coalesce(m.geo, ''), coalesce(m.service_tier, ''),
	m.input, m.output, m.cache_create, m.cache_read, m.cache_create_5m, m.cache_create_1h,
	coalesce(m.web_search_requests, 0), coalesce(m.web_fetch_requests, 0), d.path`

// readHeld reads messages, as the query selects heldColumns, and then
// the sessions holding each, as the second selects message and session.
func readHeld(q querier, eras []time.Time, messages, links string, args ...any) (map[string]*held, error) {
	rows, err := q.Query(messages, args...)
	if err != nil {
		return nil, err
	}
	out := map[string]*held{}
	for rows.Next() {
		var id string
		var h held
		var cacheCreate int64
		var fiveMinutes, oneHour sql.NullInt64
		c := &h.counters
		if err := rows.Scan(&id, &h.at, &h.class.model, &h.class.speed, &h.class.geo, &h.class.tier,
			&c.Input, &c.Output, &cacheCreate, &c.Read, &fiveMinutes, &oneHour, &c.WebSearches, &c.WebFetches, &h.dir); err != nil {
			_ = rows.Close()
			return nil, err
		}
		c.Write5m, c.Write1h, c.Messages = fiveMinutes.Int64, oneHour.Int64, 1
		if !fiveMinutes.Valid || !oneHour.Valid {
			c.WriteUnsplit = cacheCreate
		}
		h.class.long = c.Input+cacheCreate+c.Read > LongPrompt
		h.class.era = eraOf(eras, h.at)
		out[id] = &h
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	rows, err = q.Query(links, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var message, session string
		if err := rows.Scan(&message, &session); err != nil {
			return nil, err
		}
		if h, ok := out[message]; ok {
			h.sessions = append(h.sessions, session)
		}
	}
	return out, rows.Err()
}

// eraOf is the price era a moment falls in, as era numbers them.
func eraOf(bounds []time.Time, at int64) int {
	for i, b := range bounds {
		if at < b.UnixMilli() {
			return i
		}
	}
	return len(bounds)
}

func eraBounds() ([]time.Time, []time.Time) {
	bounds := Prices.Boundaries()
	sort.Slice(bounds, func(i, j int) bool { return bounds[i].Before(bounds[j]) })
	_, starts := era(Prices)
	return bounds, starts
}

// refresh brings w up to date at until, reading the window whole when
// there is nothing to bring up to date from, and counts each read of
// messages in reads.
func (s *Store) refresh(w *liveWindow, until time.Time, hours int, reads *int) (*liveWindow, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	seq, logged, err := changeSeq(tx)
	if err != nil {
		return nil, err
	}
	since := until.Add(-time.Duration(hours) * time.Hour).UnixMilli()
	bounds, _ := eraBounds()
	switch {
	case w == nil || !logged || !w.logged || seq < w.seq || since < w.since:
		all, err := readHeld(tx, bounds,
			"SELECT "+heldColumns+" FROM messages m JOIN dirs d ON d.id = m.dir_id WHERE m.at >= ?",
			"SELECT ms.message_id, ms.session_id FROM messages m JOIN message_sessions ms ON ms.message_id = m.id WHERE m.at >= ?",
			since)
		if err != nil {
			return nil, err
		}
		*reads++
		w = &liveWindow{held: all}
	case seq > w.seq:
		changed, err := readHeld(tx, bounds,
			"SELECT "+heldColumns+" FROM changed_messages c JOIN messages m ON m.id = c.message_id JOIN dirs d ON d.id = m.dir_id WHERE c.seq > ?",
			"SELECT ms.message_id, ms.session_id FROM changed_messages c JOIN message_sessions ms ON ms.message_id = c.message_id WHERE c.seq > ?",
			w.seq)
		if err != nil {
			return nil, err
		}
		*reads++
		rows, err := tx.Query("SELECT message_id FROM changed_messages WHERE seq > ?", w.seq)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return nil, err
			}
			delete(w.held, id)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		for id, h := range changed {
			if h.at >= since {
				w.held[id] = h
			}
		}
	}
	for id, h := range w.held {
		if h.at < since {
			delete(w.held, id)
		}
	}
	w.since, w.seq, w.logged = since, seq, logged
	return w, nil
}

// groupKey is a group as pricedBy makes it: a key, then a price class.
type groupKey struct {
	key   string
	class class
}

func lessGroup(a, b groupKey) bool {
	switch {
	case a.key != b.key:
		return a.key < b.key
	case a.class.model != b.class.model:
		return a.class.model < b.class.model
	case a.class.speed != b.class.speed:
		return a.class.speed < b.class.speed
	case a.class.geo != b.class.geo:
		return a.class.geo < b.class.geo
	case a.class.tier != b.class.tier:
		return a.class.tier < b.class.tier
	case a.class.long != b.class.long:
		return !a.class.long
	}
	return a.class.era < b.class.era
}

// grouper adds messages into groups as pricedBy would give them.
type grouper map[groupKey]*Counters

func (g grouper) add(key string, h *held) {
	k := groupKey{key: key, class: h.class}
	c, ok := g[k]
	if !ok {
		c = &Counters{}
		g[k] = c
	}
	*c = c.Add(h.counters)
}

func (g grouper) keyed(starts []time.Time) []keyed {
	keys := make([]groupKey, 0, len(g))
	for k := range g {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return lessGroup(keys[i], keys[j]) })
	out := make([]keyed, len(keys))
	for i, k := range keys {
		out[i] = keyed{key: k.key, priced: Priced{Model: k.class.model, Speed: k.class.speed, Geo: k.class.geo, Tier: k.class.tier,
			Long: k.class.long, At: starts[k.class.era], Counters: *g[k]}}
	}
	return out
}

// answer adds w's messages up into the window from since to until, as
// Store.Window does.
func (w *liveWindow) answer(until time.Time, hours int, root func(string) string, tied map[string]string) (Window, error) {
	_, starts := eraBounds()
	if root == nil {
		root = same
	}
	since, end := w.since, until.UnixMilli()
	all, byModel, byDir, bySession, hourly := grouper{}, grouper{}, grouper{}, grouper{}, grouper{}
	hour := time.Hour.Milliseconds()
	for _, h := range w.held {
		if h.at < since || h.at >= end {
			continue
		}
		all.add("", h)
		byModel.add(h.class.model, h)
		byDir.add(placeOf(h, tied), h)
		for _, s := range h.sessions {
			bySession.add(s, h)
		}
		if hours > 0 {
			hourly.add(strconv.FormatInt((h.at-(end-int64(hours)*hour))/hour, 10), h)
		}
	}
	var out Window
	for _, sl := range slices(all.keyed(starts), same) {
		out.Counters, out.Cost = out.Counters.Add(sl.Counters), out.Cost.Add(sl.Cost)
	}
	out.ByModel = slices(byModel.keyed(starts), same)
	out.ByProject = slices(byDir.keyed(starts), root)
	out.BySession = slices(bySession.keyed(starts), same)
	if hours > 0 {
		out.Hourly = make([]Cost, hours)
		groups := hourly.keyed(starts)
		sort.SliceStable(groups, func(i, j int) bool {
			a, _ := strconv.Atoi(groups[i].key)
			b, _ := strconv.Atoi(groups[j].key)
			return a < b
		})
		for _, g := range groups {
			i, err := strconv.Atoi(g.key)
			if err != nil {
				return out, err
			}
			out.Hourly[i] = out.Hourly[i].Add(Prices.Price(g.priced))
		}
	}
	return out, nil
}

// refreshCosts brings c up to date, pricing again only the sessions the
// change log says changed, or every session when there is nothing to
// bring up to date from; it counts the sessions it priced in priced.
func (s *Store) refreshCosts(c *liveCosts, priced *int) (*liveCosts, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	seq, logged, err := changeSeq(tx)
	if err != nil {
		return nil, err
	}
	if c == nil || !logged || !c.logged || seq < c.seq {
		groups, err := pricedOn(tx, Prices, "", "ms.session_id", "messages m JOIN message_sessions ms ON ms.message_id = m.id", "")
		if err != nil {
			return nil, err
		}
		costs := map[string]Cost{}
		for _, g := range groups {
			costs[g.key] = costs[g.key].Add(Prices.Price(g.priced))
		}
		*priced += len(costs)
		return &liveCosts{seq: seq, logged: logged, costs: costs}, nil
	}
	if seq == c.seq {
		return c, nil
	}
	rows, err := tx.Query("SELECT session_id FROM changed_sessions WHERE seq > ?", c.seq)
	if err != nil {
		return nil, err
	}
	var changed []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		changed = append(changed, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	costs := maps.Clone(c.costs)
	for _, id := range changed {
		groups, err := pricedOn(tx, Prices, "", "''", "messages m JOIN message_sessions ms ON ms.message_id = m.id", " AND ms.session_id = ?", id)
		if err != nil {
			return nil, err
		}
		*priced++
		if len(groups) == 0 {
			delete(costs, id)
			continue
		}
		var cost Cost
		for _, g := range groups {
			cost = cost.Add(Prices.Price(g.priced))
		}
		costs[id] = cost
	}
	return &liveCosts{seq: seq, logged: logged, costs: costs}, nil
}
