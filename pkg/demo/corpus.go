package demo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"botropolis.city/botropolis/pkg/claude"
)

// findTeammate resolves "<lead ref>@<agent name>" to the transcript in
// the lead's team recorded under that name. A lead names its team only
// in the result of the Agent call that spawned a teammate, and the name
// is not its own id (item 105). A teammate is a session of its own, so
// it may be filed under any project of the corpus.
func (c Corpus) findTeammate(ref, leadRef, agent string) (Recorded, error) {
	lead, err := c.Find(leadRef)
	if err != nil {
		return Recorded{}, err
	}
	led, err := claude.ReadTranscript(lead.Path)
	if err != nil {
		return Recorded{}, err
	}
	team := led.LeadsTeam
	if team == "" {
		return Recorded{}, fmt.Errorf("ref %q: %s leads no team", ref, leadRef)
	}
	paths, err := filepath.Glob(filepath.Join(c.Dir, "*", "*.jsonl"))
	if err != nil {
		return Recorded{}, err
	}
	for _, p := range paths {
		t, err := claude.ReadTranscript(p)
		if err != nil || t.TeamName != team || t.AgentName != agent {
			continue
		}
		return Recorded{Project: filepath.Base(filepath.Dir(p)), ID: strings.TrimSuffix(filepath.Base(p), ".jsonl"), Path: p}, nil
	}
	return Recorded{}, fmt.Errorf("ref %q: no session in team %s is named %q", ref, team, agent)
}

// Root is where the recorder keeps its toy projects, so it is the only
// working directory a corpus transcript may name.
const Root = "/home/demo/src"

// Corpus is a directory of recorded transcripts, one folder per toy
// project: <project>/<session>.jsonl, with a session's subagents under
// <project>/<session>/subagents/ as Claude Code files them.
type Corpus struct {
	Dir string
}

// Recorded is one session in the corpus.
type Recorded struct {
	Project string
	ID      string
	Path    string
}

// Subagents is the session's subagent folder, which may not exist.
func (r Recorded) Subagents() string {
	return filepath.Join(filepath.Dir(r.Path), r.ID, "subagents")
}

// Find resolves a ref, "<project>/<script label>" or "<project>/<session
// id or a prefix of it>", to exactly one recorded session. A prefix
// that matches two is refused rather than guessed at, since a scenario
// that silently swaps sessions films something other than what it says.
func (c Corpus) Find(ref string) (Recorded, error) {
	if lead, agent, ok := strings.Cut(ref, "@"); ok {
		return c.findTeammate(ref, lead, agent)
	}
	project, prefix, ok := strings.Cut(ref, "/")
	if !ok || project == "" || prefix == "" {
		return Recorded{}, fmt.Errorf("ref %q: want <project>/<session>", ref)
	}
	entries, err := os.ReadDir(filepath.Join(c.Dir, project))
	if err != nil {
		return Recorded{}, fmt.Errorf("ref %q: %w", ref, err)
	}
	labelled := SessionID(project, prefix) + ".jsonl"
	var found []Recorded
	for _, e := range entries {
		if e.Name() == labelled {
			return Recorded{Project: project, ID: strings.TrimSuffix(labelled, ".jsonl"), Path: filepath.Join(c.Dir, project, labelled)}, nil
		}
	}
	for _, e := range entries {
		id, isTranscript := strings.CutSuffix(e.Name(), ".jsonl")
		if e.IsDir() || !isTranscript || !strings.HasPrefix(id, prefix) {
			continue
		}
		found = append(found, Recorded{Project: project, ID: id, Path: filepath.Join(c.Dir, project, e.Name())})
	}
	switch len(found) {
	case 0:
		return Recorded{}, fmt.Errorf("ref %q: no such session", ref)
	case 1:
		return found[0], nil
	}
	return Recorded{}, fmt.Errorf("ref %q: matches %d sessions", ref, len(found))
}
