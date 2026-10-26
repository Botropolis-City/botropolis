package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/statedir"
)

const (
	ledgerFile = "sessions.json"
	// ledgerVersion is the shape this build writes, and
	// ledgerReadableFrom the oldest version whose builds can read it
	// (item 149).
	ledgerVersion      = 2
	ledgerReadableFrom = 2
)

// Ledger is every session the city has shown, by id: Botropolis's own
// record of where each one was and when, so a session keeps its lot
// after Claude Code deletes its transcript (item 86). It holds ids,
// directories and times, and never anything Claude or you wrote.
type Ledger map[string]LedgerEntry

type LedgerEntry struct {
	CWD          string    `json:"cwd"`
	StartedAt    time.Time `json:"startedAt"`
	LastActivity time.Time `json:"lastActivity"`
	// PackedAt is when you last packed the session up, and UnpackedAt
	// when you last unpacked it (item 86, slice 3).
	PackedAt   time.Time `json:"packedAt,omitempty"`
	UnpackedAt time.Time `json:"unpackedAt,omitempty"`
	// DemolishedAt is when you demolished the session: claude rm keeps
	// its transcript, so the ledger keeps it off the map (item 145).
	DemolishedAt time.Time `json:"demolishedAt,omitempty"`
	// Harness is the harness that showed the session, empty for Claude's,
	// so ledgers written before it read the same (item 97).
	Harness string `json:"harness,omitempty"`
}

func ledgerHarness(name string) string {
	if name == "claude" {
		return ""
	}
	return name
}

// Demolished is whether you demolished the session.
func (e LedgerEntry) Demolished() bool {
	return !e.DemolishedAt.IsZero()
}

// Packed is whether the session is packed up: packed, and not unpacked
// since.
func (e LedgerEntry) Packed() bool {
	return !e.PackedAt.IsZero() && e.PackedAt.After(e.UnpackedAt)
}

// Pack packs a session up at a time.
func (l Ledger) Pack(id string, at time.Time) {
	e := l[id]
	e.PackedAt = at
	l[id] = e
}

// Unpack brings a session back at a time.
func (l Ledger) Unpack(id string, at time.Time) {
	e := l[id]
	e.UnpackedAt = at
	l[id] = e
}

// LedgerPath is where the daemon keeps the ledger.
func LedgerPath() string {
	return filepath.Join(statedir.Dir(), ledgerFile)
}

// Record notes a session and reports whether the ledger changed. An
// empty session is never recorded: prune clears it on purpose.
func (l Ledger) Record(s Session) bool {
	changed, _ := l.record(&s)
	return changed
}

// record notes a session and reports whether the ledger changed, and
// whether by more than the session's last activity (item 205).
func (l Ledger) record(s *Session) (changed, more bool) {
	if s.State == Empty || s.ID == "" {
		return false, false
	}
	was, ok := l[s.ID]
	entry := LedgerEntry{CWD: s.CWD, StartedAt: s.StartedAt, LastActivity: s.LastActivity, PackedAt: was.PackedAt, UnpackedAt: was.UnpackedAt, DemolishedAt: was.DemolishedAt, Harness: ledgerHarness(s.Harness)}
	more = !ok || was.CWD != entry.CWD || !was.StartedAt.Equal(entry.StartedAt) || was.Harness != entry.Harness
	if !more && was.LastActivity.Equal(entry.LastActivity) {
		return false, false
	}
	l[s.ID] = entry
	return true, more
}

func LoadLedger(path string) (Ledger, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Ledger{}, nil
	}
	if err != nil {
		return Ledger{}, err
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return Ledger{}, err
	}
	ledger := Ledger{}
	if _, versioned := probe["version"]; !versioned {
		if err := json.Unmarshal(data, &ledger); err != nil {
			return Ledger{}, err
		}
		return ledger, nil
	}
	var file ledgerFileShape
	if err := json.Unmarshal(data, &file); err != nil {
		return Ledger{}, err
	}
	if file.Version > ledgerVersion && (file.ReadableFrom == 0 || file.ReadableFrom > ledgerVersion) {
		return Ledger{}, statedir.NewerError{Path: path, Kind: "ledger", Version: file.Version, Ours: ledgerVersion}
	}
	for id, e := range file.Sessions {
		ledger[id] = e
	}
	return ledger, nil
}

// ledgerFileShape is the ledger on disk. Before version 2 the file was
// the bare map, which LoadLedger still reads (item 149).
type ledgerFileShape struct {
	Version      int    `json:"version"`
	ReadableFrom int    `json:"readableFrom"`
	Sessions     Ledger `json:"sessions"`
}

// onDisk is the version a save writes: this build's, or a newer one
// already there that this build can read, so a save never marks the
// file older than what a newer build put in it.
func onDisk(path string) (version, readableFrom int) {
	var file ledgerFileShape
	data, err := os.ReadFile(path)
	if err == nil && json.Unmarshal(data, &file) == nil && file.Version > ledgerVersion && file.ReadableFrom > 0 && file.ReadableFrom <= ledgerVersion {
		return file.Version, file.ReadableFrom
	}
	return ledgerVersion, ledgerReadableFrom
}

func (l Ledger) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	version, readableFrom := onDisk(path)
	return l.write(path, version, readableFrom)
}

func (l Ledger) write(path string, version, readableFrom int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ledgerFileShape{Version: version, ReadableFrom: readableFrom, Sessions: l}, "", "  ")
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

// activitySaveEvery is how often a change that only moves sessions' last
// activity is saved: a session working moves it on every rescan, and it
// only places a session whose transcript is gone (item 205).
const activitySaveEvery = time.Minute

// ledgerWrite is what the loader last wrote, so a save over the file it
// wrote need not read it again for its version.
type ledgerWrite struct {
	key                   fileKey
	version, readableFrom int
	ok                    bool
}

// saveLedger writes the ledger, keeping the version a newer build left
// unless the file is still the one this loader wrote.
func (l *Loader) saveLedger() error {
	version, readableFrom := ledgerVersion, ledgerReadableFrom
	if key, err := stat(l.ledgerPath); err == nil {
		if l.ledgerWrote.ok && key == l.ledgerWrote.key {
			version, readableFrom = l.ledgerWrote.version, l.ledgerWrote.readableFrom
		} else {
			l.ledgerReads++
			version, readableFrom = onDisk(l.ledgerPath)
		}
	}
	if err := l.ledger.write(l.ledgerPath, version, readableFrom); err != nil {
		l.ledgerWrote = ledgerWrite{}
		return err
	}
	key, err := stat(l.ledgerPath)
	l.ledgerWrote = ledgerWrite{key: key, version: version, readableFrom: readableFrom, ok: err == nil}
	l.ledgerPending = false
	return nil
}

// settleLedger saves a change to more than last activity at once, and
// one to last activity alone at most once a minute.
func (l *Loader) settleLedger(changed, more bool, now time.Time) error {
	l.ledgerPending = l.ledgerPending || changed
	if !more && (!l.ledgerPending || now.Sub(l.ledgerSavedAt) < activitySaveEvery) {
		return nil
	}
	l.ledgerSavedAt = now
	return l.saveLedger()
}

// LedgerReads is how many times the loader has read its ledger file.
func (l *Loader) LedgerReads() int {
	return l.ledgerReads
}

// Flush saves what the ledger holds and has not saved yet; the daemon
// calls it on shutdown.
func (l *Loader) Flush() error {
	if l.ledgerPath == "" {
		return nil
	}
	defer l.gate.hold()()
	if !l.ledgerPending {
		return nil
	}
	return l.saveLedger()
}
