// Package find finds a session by what was said in it. The index is
// held in memory and rebuilt from Claude Code's own files: nothing
// derived from them is written anywhere, and a transcript Claude Code
// deletes takes its passages with it. Matching is plain lowercase
// substring search, which costs milliseconds over the few megabytes of
// prompts, replies, recaps, commands and paths a machine holds; tool
// output, ten times that, stays out and is read from the transcripts
// only when asked for. Item 82.
package find

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Query is a search: every word must appear in one passage.
type Query struct {
	Words string `json:"words"`
	// Limit is how many sessions to return at most; zero is all.
	Limit int `json:"limit,omitempty"`
	// ToolOutput also reads tool output from the transcripts, which the
	// index does not hold.
	ToolOutput bool `json:"toolOutput,omitempty"`
	// Live is the state of each session that is running, by id; it
	// ranks live and waiting sessions first.
	Live map[string]state.State `json:"-"`
}

// Hit is one session found, by its best passage.
type Hit struct {
	SessionID   string          `json:"sessionId"`
	Dir         string          `json:"dir"`
	Branch      string          `json:"branch,omitempty"`
	Kind        claude.SaidKind `json:"kind"`
	Passage     string          `json:"passage"`
	FirstPrompt string          `json:"firstPrompt,omitempty"`
	// At is when the session was last active.
	At time.Time `json:"at"`
	// Gone is set when the session's transcript has been deleted and it
	// was found by the prompts history.jsonl kept.
	// Recap is the session's newest recap, and LastPrompt the last thing
	// typed into it, which says where it was left.
	Recap      string      `json:"recap,omitempty"`
	LastPrompt string      `json:"lastPrompt,omitempty"`
	Gone       bool        `json:"gone,omitempty"`
	State      state.State `json:"state,omitempty"`
}

// ToolOutput is the kind a hit found only in tool output carries.
const ToolOutput claude.SaidKind = -1

// weight ranks the kinds: your prompts, then recaps, then replies, then
// commands and paths, then titles, then tool output.
func weight(k claude.SaidKind) int {
	switch k {
	case claude.SaidPrompt:
		return 0
	case claude.SaidRecap:
		return 1
	case claude.SaidReply:
		return 2
	case claude.SaidCommand, claude.SaidPath:
		return 3
	case claude.SaidTitle:
		return 4
	}
	return 5
}

type stamp struct {
	size int64
	mod  time.Time
}

type transcript struct {
	stamp stamp
	said  claude.Said
}

// Index holds what was said in every session under a Claude home.
type Index struct {
	claudeDir string

	mu          sync.Mutex
	transcripts map[string]transcript
	history     map[string][]claude.Prompt
	historyAt   stamp
}

// New is an empty index of the Claude home under home; Refresh fills it.
func New(home string) *Index {
	return &Index{claudeDir: filepath.Join(home, ".claude"), transcripts: map[string]transcript{}}
}

// Refresh reads what changed since the last refresh: transcripts that
// are new or have grown are read again, deleted ones are dropped, and
// history.jsonl is read again when it changed.
func (ix *Index) Refresh() error {
	paths, err := filepath.Glob(filepath.Join(ix.claudeDir, "projects", "*", "*.jsonl"))
	if err != nil {
		return err
	}
	ix.mu.Lock()
	known := ix.transcripts
	ix.mu.Unlock()
	fresh := make(map[string]transcript, len(paths))
	for _, path := range paths {
		st, ok := stampOf(path)
		if !ok {
			continue
		}
		if t, ok := known[path]; ok && t.stamp == st {
			fresh[path] = t
			continue
		}
		said, err := claude.ReadSaid(path)
		if err != nil && said.SessionID == "" {
			continue
		}
		if said.SessionID == "" {
			said.SessionID = strings.TrimSuffix(filepath.Base(path), ".jsonl")
		}
		fresh[path] = transcript{stamp: st, said: said}
	}
	historyPath := filepath.Join(ix.claudeDir, "history.jsonl")
	hst, hasHistory := stampOf(historyPath)
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.transcripts = fresh
	switch {
	case !hasHistory:
		ix.history, ix.historyAt = nil, stamp{}
	case hst != ix.historyAt:
		history, err := claude.ReadPromptHistory(historyPath)
		if err != nil {
			return err
		}
		ix.history, ix.historyAt = history, hst
	}
	return nil
}

func stampOf(path string) (stamp, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return stamp{}, false
	}
	return stamp{size: info.Size(), mod: info.ModTime()}, true
}

// Find returns the sessions with a passage holding every word of the
// query, ranked: live and waiting sessions first, then by the weight of
// the best passage's kind, then the most recently active.
func (ix *Index) Find(q Query) []Hit {
	words := strings.Fields(strings.ToLower(q.Words))
	if len(words) == 0 {
		return nil
	}
	matches := func(text string) bool {
		lower := strings.ToLower(text)
		for _, w := range words {
			if !strings.Contains(lower, w) {
				return false
			}
		}
		return true
	}
	ix.mu.Lock()
	transcripts := make(map[string]transcript, len(ix.transcripts))
	for path, t := range ix.transcripts {
		transcripts[path] = t
	}
	history := ix.history
	ix.mu.Unlock()

	var hits []Hit
	seen := map[string]bool{}
	for path, t := range transcripts {
		s := t.said
		seen[s.SessionID] = true
		hit := Hit{SessionID: s.SessionID, Dir: s.CWD, Branch: s.Branch, At: s.LastAt, State: q.Live[s.SessionID], Kind: ToolOutput}
		found := false
		for _, p := range s.Passages {
			if hit.FirstPrompt == "" && p.Kind == claude.SaidPrompt {
				hit.FirstPrompt = p.Text
			}
			switch p.Kind {
			case claude.SaidPrompt:
				hit.LastPrompt = p.Text
			case claude.SaidRecap:
				hit.Recap = p.Text
			}
			if !matches(p.Text) {
				continue
			}
			if !found || weight(p.Kind) < weight(hit.Kind) {
				hit.Kind, hit.Passage, found = p.Kind, p.Text, true
			}
		}
		if hit.FirstPrompt == "" && len(history[s.SessionID]) > 0 {
			hit.FirstPrompt = history[s.SessionID][0].Text
		}
		if !found && q.ToolOutput {
			if text, ok := claude.ScanToolOutput(path, matches); ok {
				hit.Passage, found = text, true
			}
		}
		if found {
			hits = append(hits, hit)
		}
	}
	for id, prompts := range history {
		if seen[id] || len(prompts) == 0 {
			continue
		}
		last := prompts[len(prompts)-1]
		hit := Hit{SessionID: id, Dir: last.Dir, At: last.At, Gone: true, State: q.Live[id], FirstPrompt: prompts[0].Text, LastPrompt: last.Text}
		for _, p := range prompts {
			if matches(p.Text) {
				hit.Kind, hit.Passage = claude.SaidPrompt, p.Text
				hits = append(hits, hit)
				break
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if la, lb := Live(a.State), Live(b.State); la != lb {
			return la
		}
		if wa, wb := weight(a.Kind), weight(b.Kind); wa != wb {
			return wa < wb
		}
		if !a.At.Equal(b.At) {
			return a.At.After(b.At)
		}
		return a.SessionID < b.SessionID
	})
	if q.Limit > 0 && len(hits) > q.Limit {
		hits = hits[:q.Limit]
	}
	return hits
}

// Live reports a session that is running: anything with a state but
// parked.
func Live(st state.State) bool {
	return st != "" && st != state.Resting && st != state.Empty
}
