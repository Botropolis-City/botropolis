package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
)

type ActionKind string

const (
	ActionNone     ActionKind = ""
	ActionAttach   ActionKind = "attach"
	ActionStop     ActionKind = "stop"
	ActionResume   ActionKind = "resume"
	ActionDemolish ActionKind = "demolish"
	ActionQuit     ActionKind = "quit"
)

const DemolishArmFor = 3 * time.Second

type Action struct {
	Kind      ActionKind
	SessionID string
}

type Model struct {
	snapshot state.Snapshot
	rows     []state.Session
	cursor   int
	all      bool
	status   string
	armed    string
	armedAt  time.Time
}

func New() *Model {
	return &Model{}
}

func (m *Model) SetSnapshot(snapshot state.Snapshot) {
	selected, hadSelection := m.Selected()
	m.snapshot = snapshot
	m.rebuild()
	if hadSelection {
		for i, r := range m.rows {
			if r.ID == selected.ID {
				m.cursor = i
				return
			}
		}
	}
	m.clamp()
}

func (m *Model) rebuild() {
	m.rows = nil
	for _, s := range m.snapshot.Sessions {
		if !m.all && s.State == state.Parked {
			continue
		}
		m.rows = append(m.rows, s)
	}
	sort.SliceStable(m.rows, func(i, j int) bool {
		if state.Rank(m.rows[i].State) != state.Rank(m.rows[j].State) {
			return state.Rank(m.rows[i].State) < state.Rank(m.rows[j].State)
		}
		return m.rows[i].LastActivity.After(m.rows[j].LastActivity)
	})
}

func (m *Model) clamp() {
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) Rows() []state.Session {
	return m.rows
}

func (m *Model) Cursor() int {
	return m.cursor
}

func (m *Model) Selected() (state.Session, bool) {
	if len(m.rows) == 0 {
		return state.Session{}, false
	}
	return m.rows[m.cursor], true
}

func (m *Model) ShowingAll() bool {
	return m.all
}

func (m *Model) Status() string {
	return m.status
}

func (m *Model) SetStatus(status string) {
	m.status = status
}

func (m *Model) Key(key string, now time.Time) Action {
	switch key {
	case "j", "down":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = len(m.rows) - 1
		m.clamp()
	case "a":
		m.all = !m.all
		m.rebuild()
		m.clamp()
	case "q", "ctrl+c", "esc":
		return Action{Kind: ActionQuit}
	case "enter":
		if s, ok := m.Selected(); ok {
			if s.State == state.Parked {
				return Action{Kind: ActionResume, SessionID: s.ID}
			}
			return Action{Kind: ActionAttach, SessionID: s.ID}
		}
	case "s":
		if s, ok := m.Selected(); ok && s.State != state.Parked {
			return Action{Kind: ActionStop, SessionID: s.ID}
		}
	case "x", "delete":
		return m.demolish(now)
	}
	return Action{}
}

func (m *Model) demolish(now time.Time) Action {
	s, ok := m.Selected()
	if !ok {
		m.status = "nothing selected"
		return Action{}
	}
	if m.armed == s.ID && now.Sub(m.armedAt) <= DemolishArmFor {
		m.armed = ""
		m.status = "demolished " + s.Title
		return Action{Kind: ActionDemolish, SessionID: s.ID}
	}
	m.armed, m.armedAt = s.ID, now
	m.status = "press x again within 3s to demolish " + s.Title
	return Action{}
}

func (m *Model) Lines(width int) []string {
	now := m.snapshot.At
	lines := []string{fmt.Sprintf("  %-10s %-24s %-14s %-40s %-5s %-8s %-8s %s", "STATE", "DOING", "PROJECT", "TITLE", "CTX", "FRESH/H", "CACHED/H", "IDLE")}
	for i, s := range m.rows {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		title := s.Title
		if len(title) > 40 {
			title = title[:39] + "…"
		}
		line := fmt.Sprintf("%s%-10s %-24s %-14s %-40s %-5s %-8s %-8s %s", marker, s.State, clip(format.Dash(s.Doing()), 24), clip(filepath.Base(s.CWD), 14), title,
			format.Percent(s.ContextPercent), format.Tokens(s.FreshTokensPerHour), format.Tokens(s.CacheReadPerHour), format.Age(now.Sub(s.LastActivity)))
		lines = append(lines, line)
	}
	if len(m.rows) == 0 {
		lines = append(lines, "  no sessions")
	}
	if width > 0 {
		for i, line := range lines {
			if len(line) > width {
				lines[i] = line[:width]
			}
		}
	}
	return lines
}

func (m *Model) Footer() string {
	if m.status != "" {
		return m.status
	}
	scope := "live"
	if m.all {
		scope = "all"
	}
	return fmt.Sprintf("%d sessions (%s) | j/k move | enter attach/resume | s stop | x x demolish | a toggle parked | q quit",
		len(m.rows), scope)
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n-1]) + "…"
}
