package stats

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// era is a SQL expression numbering the price eras a message's time
// falls in, and the moment each era starts.
func era(rates Rates) (string, []time.Time) {
	bounds := rates.Boundaries()
	sort.Slice(bounds, func(i, j int) bool { return bounds[i].Before(bounds[j]) })
	starts := []time.Time{{}}
	if len(bounds) == 0 {
		return "0", starts
	}
	var b strings.Builder
	b.WriteString("CASE")
	for i, at := range bounds {
		fmt.Fprintf(&b, " WHEN m.at < %d THEN %d", at.UnixMilli(), i)
		starts = append(starts, at)
	}
	fmt.Fprintf(&b, " ELSE %d END", len(bounds))
	return b.String(), starts
}

// pricedSelect is the columns that group messages by everything that
// decides their price, and sum their counters.
func pricedSelect(rates Rates) (string, []time.Time) {
	e, starts := era(rates)
	return fmt.Sprintf(`m.model, coalesce(m.speed, ''), coalesce(m.geo, ''), coalesce(m.service_tier, ''),
		(m.input + m.cache_create + m.cache_read) > %d, %s,
		sum(m.input), sum(m.output), sum(coalesce(m.cache_create_5m, 0)), sum(coalesce(m.cache_create_1h, 0)),
		sum(CASE WHEN m.cache_create_5m IS NULL OR m.cache_create_1h IS NULL THEN m.cache_create ELSE 0 END),
		sum(m.cache_read), sum(coalesce(m.web_search_requests, 0)), sum(coalesce(m.web_fetch_requests, 0)), count(*)`,
		LongPrompt, e), starts
}

// keyed is priced groups, each under the key the query put first.
type keyed struct {
	key    string
	priced Priced
}

// pricedBy runs a query whose columns are a key, then pricedSelect, and
// gives each group with its key.
func (s *Store) pricedBy(rates Rates, with, key, from, where string, args ...any) ([]keyed, error) {
	cols, starts := pricedSelect(rates)
	group := " GROUP BY 1, 2, 3, 4, 5, 6, 7"
	if key == "" {
		key = "''"
	}
	rows, err := s.db.Query(with+"SELECT "+key+", "+cols+" FROM "+from+" WHERE 1"+where+group, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []keyed
	for rows.Next() {
		var k keyed
		var e int
		p, c := &k.priced, &k.priced.Counters
		if err := rows.Scan(&k.key, &p.Model, &p.Speed, &p.Geo, &p.Tier, &p.Long, &e,
			&c.Input, &c.Output, &c.Write5m, &c.Write1h, &c.WriteUnsplit, &c.Read, &c.WebSearches, &c.WebFetches, &c.Messages); err != nil {
			return nil, err
		}
		p.At = starts[e]
		out = append(out, k)
	}
	return out, rows.Err()
}

// SessionCost is what a session's messages cost at the given rates, in
// all and by model: every message it holds, its subagents' and a fork's
// copies included. Item 116.
func (s *Store) SessionCost(session string, rates Rates) (Cost, map[string]Cost, error) {
	groups, err := s.pricedBy(rates, "", "m.model", "messages m JOIN message_sessions ms ON ms.message_id = m.id", " AND ms.session_id = ?", session)
	if err != nil {
		return Cost{}, nil, err
	}
	var total Cost
	byModel := map[string]Cost{}
	for _, g := range groups {
		c := rates.Price(g.priced)
		total = total.Add(c)
		byModel[g.key] = byModel[g.key].Add(c)
	}
	return total, byModel, nil
}
