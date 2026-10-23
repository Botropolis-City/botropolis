package stats

import "time"

// WindowNow is Window read now, whatever its cadence says.
func (l *Ledger) WindowNow(at time.Time, hours int, root func(string) string) (Window, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.windowLocked(at, hours, root)
}

// SessionCostsNow is SessionCosts read now, whatever its cadence says.
func (l *Ledger) SessionCostsNow(at time.Time) (map[string]Cost, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sessionCostsLocked(at)
}

// SessionsPriced is how many sessions the ledger has priced.
func (l *Ledger) SessionsPriced() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.priced
}

// MessageReads is how many times the ledger has read messages.
func (l *Ledger) MessageReads() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.messageReads
}
