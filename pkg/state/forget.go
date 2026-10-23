package state

import (
	"errors"
	"os"
	"strings"
	"time"
)

// Forget marks a session you demolished, recorded or not, and reports
// whether the ledger changed. It stays as a tombstone, since claude rm
// keeps the transcript that would otherwise bring the session back. A
// session Claude Code cleaned up on its own is never marked: it keeps its
// lot (items 86, 144 and 145).
func (l Ledger) Forget(id string, at time.Time) bool {
	e := l[id]
	if e.Demolished() {
		return false
	}
	e.DemolishedAt = at
	l[id] = e
	return true
}

// ForgottenPath is where demolishes are noted beside the ledger at
// ledgerPath while no daemon runs to forget them, since only the daemon
// writes the ledger.
func ForgottenPath(ledgerPath string) string {
	return ledgerPath + ".forgotten"
}

// NoteForgotten notes a demolished session for the daemon to forget when
// it next loads the ledger at ledgerPath. With no ledger there is
// nothing to forget, and nothing is written.
func NoteForgotten(ledgerPath, id string) error {
	if _, err := os.Stat(ledgerPath); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	f, err := os.OpenFile(ForgottenPath(ledgerPath), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(id + "\t" + time.Now().UTC().Format(time.RFC3339Nano) + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// Forget marks a demolished session in the ledger and saves it. A
// loader that keeps no ledger has nothing to forget.
func (l *Loader) Forget(id string) error {
	if l.ledgerPath == "" {
		return nil
	}
	if err := l.loadLedger(); err != nil {
		return err
	}
	if !l.ledger.Forget(id, time.Now().UTC()) {
		return nil
	}
	return l.saveLedger()
}

// loadLedger reads the ledger once, forgetting what was demolished while
// no daemon ran, and clears those notes once the ledger is saved.
func (l *Loader) loadLedger() error {
	if l.ledger != nil {
		return nil
	}
	l.ledgerReads++
	ledger, err := LoadLedger(l.ledgerPath)
	if err != nil {
		return err
	}
	notes := ForgottenPath(l.ledgerPath)
	data, err := os.ReadFile(notes)
	if errors.Is(err, os.ErrNotExist) {
		l.ledger = ledger
		return nil
	}
	if err != nil {
		return err
	}
	changed := false
	for _, line := range strings.Split(string(data), "\n") {
		id, at := noted(line)
		if id != "" && ledger.Forget(id, at) {
			changed = true
		}
	}
	l.ledger = ledger
	if changed {
		if err := l.saveLedger(); err != nil {
			l.ledger = nil
			return err
		}
	}
	return os.Remove(notes)
}

// noted reads one noted demolish: the session and when it was noted,
// or now for a note that has no time.
func noted(line string) (string, time.Time) {
	id, stamp, _ := strings.Cut(strings.TrimSpace(line), "\t")
	at, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		at = time.Now().UTC()
	}
	return strings.TrimSpace(id), at
}
