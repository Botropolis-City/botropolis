package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/claude"
)

const ledgerFile = "sessions.json"

// Ledger is every session the city has shown, by id: Botropolis's own
// record of where each one was and when, so a session keeps its lot
// after Claude Code deletes its transcript (item 86). It holds ids,
// directories and times, and never anything Claude or you wrote.
type Ledger map[string]LedgerEntry

type LedgerEntry struct {
	CWD          string    `json:"cwd"`
	StartedAt    time.Time `json:"startedAt"`
	LastActivity time.Time `json:"lastActivity"`
}

// LedgerPath is where the daemon keeps the ledger.
func LedgerPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".", ledgerFile)
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "botropolis", ledgerFile)
}

// Record notes a session and reports whether the ledger changed. An
// empty session is never recorded: prune clears it on purpose.
func (l Ledger) Record(s Session) bool {
	if s.State == Empty || s.ID == "" {
		return false
	}
	entry := LedgerEntry{CWD: s.CWD, StartedAt: s.StartedAt, LastActivity: s.LastActivity}
	if was, ok := l[s.ID]; ok && was.CWD == entry.CWD && was.StartedAt.Equal(entry.StartedAt) && was.LastActivity.Equal(entry.LastActivity) {
		return false
	}
	l[s.ID] = entry
	return true
}

func LoadLedger(path string) (Ledger, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Ledger{}, nil
	}
	if err != nil {
		return Ledger{}, err
	}
	ledger := Ledger{}
	if err := json.Unmarshal(data, &ledger); err != nil {
		return Ledger{}, err
	}
	return ledger, nil
}

func (l Ledger) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// goneSession is a session the ledger knows and Claude Code no longer
// has a transcript for: rebuilt from your prompts when the history still
// holds them, else from the ledger alone.
func goneSession(id string, entry LedgerEntry, prompts []claude.Prompt) Session {
	s := Session{
		ID: id, CWD: entry.CWD, State: Resting, ConversationGone: true,
		StartedAt: entry.StartedAt, LastActivity: entry.LastActivity,
	}
	if len(prompts) > 0 {
		first, last := prompts[0], prompts[len(prompts)-1]
		if last.Dir != "" {
			s.CWD = last.Dir
		}
		s.Title = promptTitle(first.Text)
		s.LastPrompt = last.Text
		s.StartedAt = first.At.UTC()
		s.LastActivity = last.At.UTC()
	}
	if s.Title == "" {
		s.Title = ShortID(id)
	}
	return s
}

const promptTitleRunes = 80

func promptTitle(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	if r := []rune(line); len(r) > promptTitleRunes {
		return string(r[:promptTitleRunes-1]) + "…"
	}
	return line
}
