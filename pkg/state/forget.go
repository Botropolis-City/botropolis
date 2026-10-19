package state

import (
	"errors"
	"os"
	"strings"
)

// Forget drops a session you demolished and reports whether the ledger
// changed. A session Claude Code cleaned up on its own is never
// forgotten: it keeps its lot (items 86 and 144).
func (l Ledger) Forget(id string) bool {
	if _, ok := l[id]; !ok {
		return false
	}
	delete(l, id)
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
	if _, err := f.WriteString(id + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// Forget drops a demolished session from the ledger and saves it. A
// loader that keeps no ledger has nothing to forget.
func (l *Loader) Forget(id string) error {
	if l.ledgerPath == "" {
		return nil
	}
	if err := l.loadLedger(); err != nil {
		return err
	}
	if !l.ledger.Forget(id) {
		return nil
	}
	return l.ledger.Save(l.ledgerPath)
}

// loadLedger reads the ledger once, forgetting what was demolished while
// no daemon ran, and clears those notes once the ledger is saved.
func (l *Loader) loadLedger() error {
	if l.ledger != nil {
		return nil
	}
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
	for _, id := range strings.Split(string(data), "\n") {
		if id = strings.TrimSpace(id); id != "" && ledger.Forget(id) {
			changed = true
		}
	}
	if changed {
		if err := ledger.Save(l.ledgerPath); err != nil {
			return err
		}
	}
	l.ledger = ledger
	return os.Remove(notes)
}
