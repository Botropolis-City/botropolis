package group

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/statedir"
)

const (
	storeFile = "groups.json"
)

// Memory is what Botropolis keeps of groups: the groups themselves, by
// id and member, and the corrections. Never a prompt.
type Memory struct {
	Groups     []Stored `json:"groups,omitempty"`
	NotRelated []Pair   `json:"notRelated,omitempty"`
	Muted      []string `json:"muted,omitempty"`
	// Moves are sessions put in a group by hand, or in none.
	Moves []Move `json:"moves,omitempty"`
	// Confirmed are likely members prompted in after they joined, so
	// their ties are solid; LikelySince is when each likely member was
	// first seen as one.
	Confirmed   []string             `json:"confirmed,omitempty"`
	LikelySince map[string]time.Time `json:"likelySince,omitempty"`
	// Reviewed is when the first run's proposal was confirmed; until then
	// it is shown at every launch.
	Reviewed *time.Time `json:"reviewed,omitempty"`
	// StartedFrom is the group each session New session started was
	// started in, by its background job.
	StartedFrom map[string]string `json:"startedFrom,omitempty"`
}

// Move puts a session in a group by hand; an empty Group is no group.
type Move struct {
	Session string `json:"session"`
	Group   string `json:"group"`
}

// WithMove is memory with the session moved, replacing any earlier move
// of it.
func (m Memory) WithMove(session, group string) Memory {
	moves := make([]Move, 0, len(m.Moves)+1)
	for _, mv := range m.Moves {
		if mv.Session != session {
			moves = append(moves, mv)
		}
	}
	m.Moves = append(moves, Move{Session: session, Group: group})
	return m
}

// WithMuted is memory with a word muted, so it no longer joins sessions
// by their wording; a word already muted is muted once (item 90).
func (m Memory) WithMuted(word string) Memory {
	word = strings.ToLower(strings.TrimSpace(word))
	for _, w := range m.Muted {
		if strings.EqualFold(w, word) {
			return m
		}
	}
	m.Muted = append(append([]string(nil), m.Muted...), word)
	return m
}

type Stored struct {
	ID      string   `json:"id"`
	Name    string   `json:"name,omitempty"`
	Renamed bool     `json:"renamed,omitempty"`
	Anchor  string   `json:"anchor,omitempty"`
	Members []string `json:"members"`
}

// Pair is two sessions marked not related.
type Pair [2]string

// Remember is memory with a detection's groups in it, keeping the
// stored groups the detection did not see.
func (m Memory) Remember(r Result) Memory {
	out := Memory{NotRelated: m.NotRelated, Muted: m.Muted, Moves: m.Moves, Confirmed: m.Confirmed, LikelySince: m.LikelySince, Reviewed: m.Reviewed, StartedFrom: m.StartedFrom}
	seen := map[string]bool{}
	for _, g := range r.Groups {
		seen[g.ID] = true
		s := Stored{ID: g.ID, Name: g.Name, Renamed: g.Renamed, Anchor: g.Anchor}
		for _, mm := range g.Members {
			s.Members = append(s.Members, mm.Session)
		}
		out.Groups = append(out.Groups, s)
	}
	for _, s := range m.Groups {
		if !seen[s.ID] {
			out.Groups = append(out.Groups, s)
		}
	}
	return out
}

func Path() string {
	return filepath.Join(statedir.Dir(), storeFile)
}

func Load(path string) (Memory, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Memory{}, nil
	}
	if err != nil {
		return Memory{}, err
	}
	var m Memory
	if err := json.Unmarshal(data, &m); err != nil {
		return Memory{}, err
	}
	return m, nil
}

// Save writes memory whole, through a temporary file, so a crash leaves
// the old memory rather than half of the new.
func Save(path string, m Memory) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
