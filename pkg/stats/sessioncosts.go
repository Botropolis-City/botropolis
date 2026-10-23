package stats

import (
	"errors"
	"time"
)

// LedgerSessions is how long a ledger reuses every session's cost: it
// reads all of history, so it is read less often than a window.
const LedgerSessions = 30 * time.Second

// SessionCosts is every stored session's cost at list prices, over every
// message it holds, a fork's copies included. Item 116.
func (s *Store) SessionCosts() (map[string]Cost, error) {
	groups, err := s.pricedBy(Prices, "", "ms.session_id", "messages m JOIN message_sessions ms ON ms.message_id = m.id", "")
	if err != nil {
		return nil, err
	}
	out := map[string]Cost{}
	for _, g := range groups {
		out[g.key] = out[g.key].Add(Prices.Price(g.priced))
	}
	return out, nil
}

// SessionCosts is every session's cost, reused for LedgerSessions, and
// for as long as nothing is written, since costs do not age (item 215).
func (l *Ledger) SessionCosts(at time.Time) (map[string]Cost, error) {
	if l == nil {
		return nil, errors.New(ReasonOff)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	s, ok := l.storeLocked(at)
	if !ok {
		if l.err == nil {
			return nil, ErrNoStore
		}
		return nil, l.err
	}
	if l.sessions != nil && fresh(s, l.sessionsAt, at, l.sessionsVersion, LedgerSessions, 0) {
		return l.sessions, nil
	}
	version, err := s.DataVersion()
	if err != nil {
		return nil, err
	}
	costs, err := s.SessionCosts()
	if err != nil {
		return nil, err
	}
	l.sessions, l.sessionsAt, l.sessionsVersion = costs, at, version
	return costs, nil
}
