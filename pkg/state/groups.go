package state

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
	// MemberDirs is each member's directory, in Members' order, and
	// Reason why the group formed, from its strongest tie.
	MemberDirs []string `json:"memberDirs"`
	Reason     string   `json:"reason,omitempty"`
	Move       Move     `json:"move"`
	Why        string   `json:"why"`
	Journal    Journal  `json:"journal"`
}

type groupCache struct {
	path       string
	storeKey   fileKey
	historyKey fileKey
	signature  [sha256.Size]byte
	detections int
	// detector keeps each pair's score from run to run, so a prompt
	// scores again only the pairs it can have changed (item 158).
	detector *group.Detector
	result   group.Result
	// lastPrompts is each session's newest prompt in the history, for a
	// session whose transcript carries no last-prompt record.
	lastPrompts map[string]string
	// reviewed is whether the first run's proposal has been confirmed.
	reviewed bool
	texts    map[string]memberTexts
	parts    signatureParts
	// logf reports, and capped is how many sessions it last reported
	// left out of a full group (item 271).
	logf   func(format string, args ...any)
	capped int
	common []string
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

// WithLog sends what grouping has to report, such as sessions left out
// of a full group, to logf rather than the standard logger.
func (l *Loader) WithLog(logf func(format string, args ...any)) *Loader {
	l.groups.logf = logf
	return l
}

// Detections is how many times the loader has grouped sessions.
func (l *Loader) Detections() int {
	return l.groups.detections
}

// PairsScored is how many pairs of sessions the loader has compared to
// group them.
func (l *Loader) PairsScored() int {
	if l.groups.detector == nil {
		return 0
	}
	return l.groups.detector.PairsScored()
}

func (l *Loader) applyGroups(snapshot *Snapshot, claudeDir string, now time.Time) error {
	historyPath := filepath.Join(claudeDir, "history.jsonl")
	key, err := stat(historyPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	resolveMessaged(snapshot.Sessions)
	c := &l.groups
	signature := c.parts.signature(snapshot.Sessions)
	var storeKey fileKey
	if c.path != "" {
		if storeKey, err = stat(c.path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if c.detections == 0 || key != c.historyKey || signature != c.signature || storeKey != c.storeKey {
		history, err := l.promptHistory(historyPath)
		if err != nil {
			return err
		}
		if err := c.detect(c.members(history, snapshot.Sessions, now), history, now); err != nil {
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
	snapshot.GroupsReviewed = c.reviewed
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
	if c.detector == nil {
		c.detector = group.NewDetector()
	}
	c.result = c.detector.Detect(members, memory)
	c.detections++
	c.reportCapped()
	if c.path == "" {
		return nil
	}
	remembered := memory.Remember(c.result)
	if confirmLikely(&remembered, c.result, history, now) {
		c.result = c.detector.Detect(members, remembered)
		remembered = remembered.Remember(c.result)
	}
	c.reviewed = remembered.Reviewed != nil
	if !reflect.DeepEqual(remembered, memory) {
		return group.Save(c.path, remembered)
	}
	return nil
}

// reportCapped logs, rather than draws, the sessions detection left out
// of a full group and the keys too many sessions share to group any,
// each when it changes (item 271).
func (c *groupCache) reportCapped() {
	logf := c.logf
	if logf == nil {
		logf = log.Printf
	}
	if n := len(c.result.Capped); n != c.capped {
		c.capped = n
		if n > 0 {
			logf("botropolisd: groups are full at %d sessions; %d more left out of them (item 271)", group.MaxMembers, n)
		}
	}
	if common := c.result.Common; !slices.Equal(common, c.common) {
		c.common = common
		if len(common) > 0 {
			logf("botropolisd: shared by more than %d sessions, so grouping none of them: %s (item 271)", group.MaxMembers, firstFew(common, 5))
		}
	}
}

// firstFew is up to n of names, and how many more there are.
func firstFew(names []string, n int) string {
	if len(names) <= n {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:n], ", "), len(names)-n)
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
func groupSignature(sessions []Session) [sha256.Size]byte {
	parts := make([]string, 0, len(sessions))
	for _, s := range sessions {
		parts = append(parts, signatureLine(s))
	}
	sort.Strings(parts)
	// A digest rather than the parts joined: the joined string was the
	// largest thing a settled load allocated (item 196).
	h := sha256.New()
	for _, p := range parts {
		_, _ = io.WriteString(h, p)
		_, _ = io.WriteString(h, "\x01")
	}
	return [sha256.Size]byte(h.Sum(nil))
}

// signatureLine is one session's part of the signature.
func signatureLine(s Session) string {
	visits := make([]string, 0, len(s.Visits))
	for dir := range s.Visits {
		visits = append(visits, dir)
	}
	sort.Strings(visits)
	return strings.Join([]string{s.ID, s.CWD, s.Branch, s.Team, s.Job, boolString(s.State == Resting),
		strings.Join(visits, "\x02"), strings.Join(s.Messaged, "\x02")}, "\x00")
}

func boolString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// memberTexts are a session's prompts and pastes as detection reads
// them, kept while the session's history has not changed so that a load
// does not copy every session's prompts again (item 198).
type memberTexts struct {
	from            *claude.Prompt
	n               int
	prompts, pasted []string
}

func (c *groupCache) members(history map[string][]claude.Prompt, sessions []Session, now time.Time) []group.Member {
	byID := make(map[string]int, len(sessions))
	for i := range sessions {
		byID[sessions[i].ID] = i
	}
	texts := make(map[string]memberTexts, len(history))
	ms := make([]group.Member, 0, len(history)+len(sessions))
	seen := make(map[string]bool, len(history))
	for id, prompts := range history {
		if len(prompts) == 0 || now.Sub(prompts[len(prompts)-1].At) > GroupWindow {
			continue
		}
		t, ok := c.texts[id]
		if !ok || t.n != len(prompts) || t.from != &prompts[0] {
			t = memberTexts{from: &prompts[0], n: len(prompts), prompts: make([]string, 0, len(prompts))}
			for _, p := range prompts {
				t.prompts = append(t.prompts, p.Text)
				t.pasted = append(t.pasted, p.Pasted...)
			}
		}
		texts[id] = t
		m := group.Member{ID: id, Dir: prompts[0].Dir, Started: prompts[0].At, Prompts: t.prompts, Pasted: t.pasted}
		if i, ok := byID[id]; ok {
			enrich(&m, &sessions[i])
		}
		seen[id] = true
		ms = append(ms, m)
	}
	c.texts = texts
	for i := range sessions {
		if !seen[sessions[i].ID] {
			m := group.Member{ID: sessions[i].ID, Dir: sessions[i].CWD, Started: sessions[i].StartedAt}
			enrich(&m, &sessions[i])
			ms = append(ms, m)
		}
	}
	return ms
}

func members(history map[string][]claude.Prompt, sessions []Session, now time.Time) []group.Member {
	return (&groupCache{}).members(history, sessions, now)
}

func enrich(m *group.Member, s *Session) {
	if s.CWD != "" {
		m.Dir = s.CWD
	}
	m.Branch, m.Team, m.Job = s.Branch, s.Team, s.Job
	if s.State == Resting {
		m.Ended = s.LastActivity
	}
	for path := range s.Touches {
		m.Files = append(m.Files, path)
	}
	sort.Strings(m.Files)
	for dir := range s.Visits {
		span := s.VisitSpans[dir]
		m.Visited = append(m.Visited, group.Visit{Dir: dir, From: span.From, To: span.To})
	}
	sort.Slice(m.Visited, func(i, j int) bool { return m.Visited[i].Dir < m.Visited[j].Dir })
	m.Messaged = s.Messaged
}

// resolveMessaged names the sessions each session messaged, from the
// names it sent them to: a session's id, title or agent name. A name
// two live sessions answer to is ambiguous and names neither (item 90).
func resolveMessaged(sessions []Session) {
	for i := range sessions {
		sessions[i].Messaged = nil
		for to := range sessions[i].Messages {
			if id, ok := answering(sessions, to); ok && id != sessions[i].ID {
				sessions[i].Messaged = append(sessions[i].Messaged, id)
			}
		}
		sort.Strings(sessions[i].Messaged)
	}
}

func answering(sessions []Session, name string) (string, bool) {
	var live, any []string
	for _, s := range sessions {
		if s.ID != name && !strings.EqualFold(s.Title, name) && !strings.EqualFold(s.Agent, name) {
			continue
		}
		any = append(any, s.ID)
		if s.Alive {
			live = append(live, s.ID)
		}
	}
	switch {
	case len(live) == 1:
		return live[0], true
	case len(live) == 0 && len(any) == 1:
		return any[0], true
	}
	return "", false
}

// groupViews is each group as the map shows it, with whose move it is
// among the sessions the snapshot holds, and each of those sessions
// marked with its group and why it joined.
// TitleOf names a session as the city does: by its title, or by its
// short id when it has none or is not among sessions.
func TitleOf(sessions []Session) func(id string) string {
	index := make(map[string]int, len(sessions))
	for i, s := range sessions {
		index[s.ID] = i
	}
	return func(id string) string {
		if i, ok := index[id]; ok {
			return title(sessions[i])
		}
		return ShortID(id)
	}
}

func groupViews(result group.Result, sessions []Session) []Group {
	index := map[string]int{}
	for i, s := range sessions {
		index[s.ID] = i
	}
	titleOf := TitleOf(sessions)
	views := make([]Group, 0, len(result.Groups))
	// present is one buffer for every group, since a copy of each member
	// per group was most of what this allocated (item 205).
	var present []Session
	for _, g := range result.Groups {
		view := Group{ID: g.ID, Name: g.Name, Short: group.Short(g), Anchor: g.Anchor}
		present = present[:0]
		for _, m := range g.Members {
			view.Members = append(view.Members, m.Session)
			view.MemberDirs = append(view.MemberDirs, m.Dir)
			if view.Reason == "" && m.Reason.Signal != "" {
				view.Reason = m.Reason.Describe(titleOf)
			}
			i, ok := index[m.Session]
			if !ok {
				continue
			}
			joined := m.Reason.Describe(titleOf)
			if m.Tier == group.Likely {
				joined = "likely: " + joined
			}
			sessions[i].Group, sessions[i].GroupJoined, sessions[i].GroupLikely = g.ID, joined, m.Tier == group.Likely
			sessions[i].GroupWords = nil
			if m.Reason.Signal == group.SignalWording {
				sessions[i].GroupWords = strings.Fields(m.Reason.Detail)
			}
			present = append(present, sessions[i])
		}
		view.Move, view.Why = GroupMove(present)
		view.Journal = GroupJournal(present)
		views = append(views, view)
	}
	sort.SliceStable(views, func(i, j int) bool { return views[i].Journal.Active.After(views[j].Journal.Active) })
	return views
}
