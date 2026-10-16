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
	Agents(ctx context.Context, all bool) ([]control.Agent, error)
	Reveal(dir string) error
	Copy(ctx context.Context, text string) error
	ResumeIn(dir, sessionID string) error
	StartIn(dir string) error
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
		case known && session.State != state.Parked:
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
		}
		pruned = append(pruned, Pruned{ID: agent.ID, Title: title, LastSeen: lastSeen})
	}
	return pruned, nil
}

type Sessions struct {
	home string
	ctl  Controller
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

// ResumeIn resumes a stopped session in a terminal in its own directory.
func (s *Sessions) ResumeIn(dir, sessionID string) error {
	return s.ctl.ResumeIn(dir, sessionID)
}

// StartIn starts a new session in a terminal in a directory.
func (s *Sessions) StartIn(dir string) error {
	return s.ctl.StartIn(dir)
}

func (s *Sessions) Remove(ctx context.Context, id string) error {
	return s.ctl.Remove(ctx, id)
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
