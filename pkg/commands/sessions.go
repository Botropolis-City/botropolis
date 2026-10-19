package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/control"
	"botropolis.city/botropolis/pkg/state"
)

type Controller interface {
	New(ctx context.Context, dir, prompt string) (string, error)
	Attach(id string) error
	Stop(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
	Resume(ctx context.Context, dir, sessionID string) (string, error)
	Fork(ctx context.Context, dir, sessionID string) (string, error)
	Agents(ctx context.Context, all bool) ([]control.Agent, error)
	Reveal(dir string) error
	Copy(ctx context.Context, text string) error
}

type Pruned struct {
	ID       string
	Title    string
	LastSeen time.Time
}

// EmptyAfter is how long an empty session — one nothing was typed into —
// may sit before prune clears it, whatever the parked cutoff is.
const EmptyAfter = time.Hour

// Prune removes parked background jobs last seen before the cutoff, and
// empty ones that have sat for EmptyAfter.
func (s *Sessions) Prune(ctx context.Context, snapshot state.Snapshot, olderThan time.Duration, now time.Time, dryRun bool) ([]Pruned, error) {
	agents, err := s.ctl.Agents(ctx, true)
	if err != nil {
		return nil, err
	}
	byID := map[string]state.Session{}
	for _, session := range snapshot.Sessions {
		byID[session.ID] = session
	}
	cutoff := now.Add(-olderThan)
	var pruned []Pruned
	for _, agent := range agents {
		if agent.Kind != "background" {
			continue
		}
		session, known := byID[agent.SessionID]
		limit := cutoff
		switch {
		case known && session.State == state.Empty:
			limit = now.Add(-EmptyAfter)
		case known && session.State != state.Resting:
			continue
		}
		lastSeen := time.UnixMilli(agent.StartedAt).UTC()
		if known && !session.LastActivity.IsZero() {
			lastSeen = session.LastActivity
		}
		if !lastSeen.Before(limit) {
			continue
		}
		title := session.Title
		if title == "" {
			title = agent.SessionID
		}
		if !dryRun {
			if err := s.ctl.Remove(ctx, agent.ID); err != nil {
				return pruned, err
			}
			if s.forget != nil {
				if err := s.forget(agent.SessionID); err != nil {
					return pruned, err
				}
			}
			if s.demolished != nil {
				if err := s.demolished(agent.SessionID); err != nil {
					return pruned, err
				}
			}
		}
		pruned = append(pruned, Pruned{ID: agent.ID, Title: title, LastSeen: lastSeen})
	}
	return pruned, nil
}

type Sessions struct {
	home       string
	ctl        Controller
	forget     func(session string) error
	demolished func(session string) error
}

// WithDemolished is told the session id of every session a remove or a
// prune demolishes, once the remove has succeeded, so the ledger forgets
// it. Item 144.
func (s *Sessions) WithDemolished(demolished func(session string) error) *Sessions {
	s.demolished = demolished
	return s
}

// WithForget has prune forget each session it removes from the
// statistics store, which only a run with statistics on asks for.
// Item 110.
func (s *Sessions) WithForget(forget func(session string) error) *Sessions {
	s.forget = forget
	return s
}

func NewSessions(home string, ctl Controller) *Sessions {
	return &Sessions{home: home, ctl: ctl}
}

func (s *Sessions) New(ctx context.Context, dir, prompt string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return s.ctl.New(ctx, abs, prompt)
}

func (s *Sessions) Attach(id string) error {
	return s.ctl.Attach(id)
}

func (s *Sessions) Stop(ctx context.Context, id string) error {
	return s.ctl.Stop(ctx, id)
}

func (s *Sessions) Reveal(dir string) error {
	return s.ctl.Reveal(dir)
}

func (s *Sessions) Copy(ctx context.Context, text string) error {
	return s.ctl.Copy(ctx, text)
}

func (s *Sessions) Remove(ctx context.Context, id string) error {
	if s.demolished == nil {
		return s.ctl.Remove(ctx, id)
	}
	session, err := s.sessionOf(ctx, id)
	if err != nil {
		return err
	}
	if err := s.ctl.Remove(ctx, id); err != nil {
		return err
	}
	if session == "" {
		return nil
	}
	return s.demolished(session)
}

// sessionOf is the session id a remove names: itself when it is one,
// else the job's, looked up before the job is gone, since a resumed job's
// id need not start its session's. "" when no job has it.
func (s *Sessions) sessionOf(ctx context.Context, id string) (string, error) {
	if len(id) > 8 && id[8] == '-' {
		return id, nil
	}
	agents, err := s.ctl.Agents(ctx, true)
	if err != nil {
		return "", err
	}
	for _, agent := range agents {
		if agent.ID == id {
			return agent.SessionID, nil
		}
	}
	return "", nil
}

func (s *Sessions) Resume(ctx context.Context, dir, sessionID string) (string, error) {
	if dir == "" {
		var err error
		if dir, err = s.sessionDir(sessionID); err != nil {
			return "", err
		}
	}
	id, err := s.ctl.Resume(ctx, dir, sessionID)
	if err != nil {
		return "", err
	}
	return id, s.ctl.Attach(id)
}

// ResumeBackground resumes a session in the background, with claude --bg
// --resume, and attaches nothing: Restore the desk reopens several at
// once (item 86).
func (s *Sessions) ResumeBackground(ctx context.Context, sessionID string) (string, error) {
	dir, err := s.sessionDir(sessionID)
	if err != nil {
		return "", err
	}
	return s.ctl.Resume(ctx, dir, sessionID)
}

// Fork starts a fresh session from a copy of another's conversation and
// attaches to it.
func (s *Sessions) Fork(ctx context.Context, dir, sessionID string) (string, error) {
	if dir == "" {
		var err error
		if dir, err = s.sessionDir(sessionID); err != nil {
			return "", err
		}
	}
	id, err := s.ctl.Fork(ctx, dir, sessionID)
	if err != nil {
		return "", err
	}
	return id, s.ctl.Attach(id)
}

func (s *Sessions) sessionDir(sessionID string) (string, error) {
	path, ok := claude.FindTranscript(filepath.Join(s.home, ".claude", "projects"), sessionID)
	if !ok {
		return "", fmt.Errorf("no transcript for session %s; pass --dir", sessionID)
	}
	transcript, err := claude.ReadTranscript(path)
	if err != nil {
		return "", err
	}
	if transcript.CWD == "" {
		return "", fmt.Errorf("transcript %s has no cwd; pass --dir", path)
	}
	return transcript.CWD, nil
}
