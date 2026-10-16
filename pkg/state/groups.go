package state

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/group"
)

// GroupWindow is how far back grouping reads the prompt history.
const GroupWindow = 30 * 24 * time.Hour

// Group is work that runs across directories, as the map shows it
// (item 87).
type Group struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Short   string   `json:"short"`
	Anchor  string   `json:"anchor,omitempty"`
	Members []string `json:"members"`
	Move    Move     `json:"move"`
	Why     string   `json:"why"`
	Journal Journal  `json:"journal"`
}

type groupCache struct {
	path       string
	storeKey   fileKey
	historyKey fileKey
	signature  string
	detections int
	result     group.Result
	// lastPrompts is each session's newest prompt in the history, for a
	// session whose transcript carries no last-prompt record.
	lastPrompts map[string]string
}

func lastPrompts(history map[string][]claude.Prompt) map[string]string {
	last := make(map[string]string, len(history))
	for id, prompts := range history {
		if len(prompts) > 0 {
			last[id] = prompts[len(prompts)-1].Text
		}
	}
	return last
}

// WithGroups remembers the groups detected, with their corrections, at
// path. Without it groups are detected and nothing is written.
func (l *Loader) WithGroups(path string) *Loader {
	l.groups.path = path
	return l
}

// Detections is how many times the loader has grouped sessions.
func (l *Loader) Detections() int {
	return l.groups.detections
}

func (l *Loader) applyGroups(snapshot *Snapshot, claudeDir string, now time.Time) error {
	historyPath := filepath.Join(claudeDir, "history.jsonl")
	key, err := stat(historyPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	signature := groupSignature(snapshot.Sessions)
	c := &l.groups
	var storeKey fileKey
	if c.path != "" {
		if storeKey, err = stat(c.path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if c.detections == 0 || key != c.historyKey || signature != c.signature || storeKey != c.storeKey {
		history, err := claude.ReadPromptHistory(historyPath)
		if err != nil {
			return err
		}
		if err := c.detect(members(history, snapshot.Sessions, now), history, now); err != nil {
			return err
		}
		c.historyKey, c.signature = key, signature
		if c.path != "" {
			c.storeKey, _ = stat(c.path)
		}
		c.lastPrompts = lastPrompts(history)
	}
	for i := range snapshot.Sessions {
		if snapshot.Sessions[i].LastPrompt == "" {
			snapshot.Sessions[i].LastPrompt = c.lastPrompts[snapshot.Sessions[i].ID]
		}
	}
	snapshot.Groups = groupViews(c.result, snapshot.Sessions)
	return nil
}

func (c *groupCache) detect(members []group.Member, history map[string][]claude.Prompt, now time.Time) error {
	memory := group.Memory{}
	if c.path != "" {
		loaded, err := group.Load(c.path)
		if err != nil {
			return err
		}
		memory = loaded
	}
	c.result = group.Detect(members, memory)
	c.detections++
	if c.path == "" {
		return nil
	}
	remembered := memory.Remember(c.result)
	if confirmLikely(&remembered, c.result, history, now) {
		c.result = group.Detect(members, remembered)
		remembered = remembered.Remember(c.result)
	}
	if !reflect.DeepEqual(remembered, memory) {
		return group.Save(c.path, remembered)
	}
	return nil
}

// confirmLikely notes when each likely member was first seen as one, and
// confirms those prompted in since: acting inside a guessed group is
// what turns its tie solid. It reports whether any was confirmed.
func confirmLikely(memory *group.Memory, result group.Result, history map[string][]claude.Prompt, now time.Time) bool {
	confirmed := false
	for _, g := range result.Groups {
		for _, m := range g.Members {
			if m.Tier != group.Likely {
				continue
			}
			since, seen := memory.LikelySince[m.Session]
			if !seen {
				if memory.LikelySince == nil {
					memory.LikelySince = map[string]time.Time{}
				}
				memory.LikelySince[m.Session] = now.UTC()
				continue
			}
			prompts := history[m.Session]
			if len(prompts) > 0 && prompts[len(prompts)-1].At.After(since) {
				memory.Confirmed = append(memory.Confirmed, m.Session)
				delete(memory.LikelySince, m.Session)
				confirmed = true
			}
		}
	}
	return confirmed
}

// groupSignature is what about the sessions detection reads, other than
// the prompt history: who they are, where, on which branch, in which
// team, and whether they have closed.
func groupSignature(sessions []Session) string {
	parts := make([]string, 0, len(sessions))
	for _, s := range sessions {
		parts = append(parts, strings.Join([]string{s.ID, s.CWD, s.Branch, s.Team, boolString(s.State == Resting)}, "\x00"))
	}
	sort.Strings(parts)
	return strings.Join(parts, "\x01")
}

func boolString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func members(history map[string][]claude.Prompt, sessions []Session, now time.Time) []group.Member {
	byID := map[string]Session{}
	for _, s := range sessions {
		byID[s.ID] = s
	}
	var ms []group.Member
	seen := map[string]bool{}
	for id, prompts := range history {
		if len(prompts) == 0 || now.Sub(prompts[len(prompts)-1].At) > GroupWindow {
			continue
		}
		m := group.Member{ID: id, Dir: prompts[0].Dir, Started: prompts[0].At}
		for _, p := range prompts {
			m.Prompts = append(m.Prompts, p.Text)
			m.Pasted = append(m.Pasted, p.Pasted...)
		}
		if s, ok := byID[id]; ok {
			enrich(&m, s)
		}
		seen[id] = true
		ms = append(ms, m)
	}
	for _, s := range sessions {
		if !seen[s.ID] {
			m := group.Member{ID: s.ID, Dir: s.CWD, Started: s.StartedAt}
			enrich(&m, s)
			ms = append(ms, m)
		}
	}
	return ms
}

func enrich(m *group.Member, s Session) {
	if s.CWD != "" {
		m.Dir = s.CWD
	}
	m.Branch, m.Team = s.Branch, s.Team
	if s.State == Resting {
		m.Ended = s.LastActivity
	}
	for path := range s.Touches {
		m.Files = append(m.Files, path)
	}
	sort.Strings(m.Files)
}

// groupViews is each group as the map shows it, with whose move it is
// among the sessions the snapshot holds, and each of those sessions
// marked with its group and why it joined.
func groupViews(result group.Result, sessions []Session) []Group {
	index := map[string]int{}
	for i, s := range sessions {
		index[s.ID] = i
	}
	titleOf := func(id string) string {
		if i, ok := index[id]; ok {
			return title(sessions[i])
		}
		return ShortID(id)
	}
	views := make([]Group, 0, len(result.Groups))
	for _, g := range result.Groups {
		view := Group{ID: g.ID, Name: g.Name, Short: group.Short(g), Anchor: g.Anchor}
		var present []Session
		for _, m := range g.Members {
			view.Members = append(view.Members, m.Session)
			i, ok := index[m.Session]
			if !ok {
				continue
			}
			joined := m.Reason.Describe(titleOf)
			if m.Tier == group.Likely {
				joined = "likely: " + joined
			}
			sessions[i].Group, sessions[i].GroupJoined, sessions[i].GroupLikely = g.ID, joined, m.Tier == group.Likely
			present = append(present, sessions[i])
		}
		view.Move, view.Why = GroupMove(present)
		view.Journal = GroupJournal(present)
		views = append(views, view)
	}
	sort.SliceStable(views, func(i, j int) bool { return views[i].Journal.Active.After(views[j].Journal.Active) })
	return views
}
