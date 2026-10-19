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

// SessionCosts is every session's cost, reused for LedgerSessions.
func (l *Ledger) SessionCosts(at time.Time) (map[string]Cost, error) {
	if l == nil {
		return nil, errors.New(ReasonOff)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.sessions != nil && at.Sub(l.sessionsAt) < LedgerSessions && !at.Before(l.sessionsAt) {
		return l.sessions, nil
	}
	s, ok := l.storeLocked(at)
	if !ok {
		if l.err == nil {
			return nil, ErrNoStore
		}
		return nil, l.err
	}
	costs, err := s.SessionCosts()
	if err != nil {
		return nil, err
	}
	l.sessions, l.sessionsAt = costs, at
	return costs, nil
}
