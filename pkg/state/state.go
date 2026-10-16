package state

import (
	"sort"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/claude"
)

type State string

const (
	NeedsYou State = "needs-you"
	Working  State = "working"
	// Waiting is a session that handed the turn back but armed its own
	// watch — a scheduled wakeup, a Monitor, a cron loop — so it will
	// carry on by itself; you can talk to it, it does not need you.
	Waiting    State = "waiting"
	Unattended State = "unattended"
	Resting    State = "resting"
	// Empty is a live session nothing has been typed into yet: no
	// transcript, or one with no prompt and no reply. It is drawn as a
	// plot without a building, never counted, and pruned once it has sat
	// for an hour.
	Empty State = "empty"
)

const (
	contextWindowDefault int64 = 200_000
	contextWindowLarge   int64 = 1_000_000
	largeContextSuffix         = "[1m]"
	minRateSpan                = time.Minute
)

type Session struct {
	ID      string      `json:"id"`
	Harness string      `json:"harness,omitempty"`
	Title   string      `json:"title"`
	CWD     string      `json:"cwd"`
	Branch  string      `json:"branch"`
	Model   string      `json:"model"`
	Kind    claude.Kind `json:"kind"`
	// Job is the background job the session runs as, by which New
	// session records the group it was started in (item 87).
	Job      string      `json:"job,omitempty"`
	PID      int         `json:"pid"`
	Alive    bool        `json:"alive"`
	Attached bool        `json:"attached"`
	State    State       `json:"state"`
	Turn     claude.Turn `json:"turn"`
	// Hooked is whether the daemon has ever seen a hook event from the
	// session; without one it is reading files, and the map draws no
	// wire to it.
	Hooked        bool     `json:"hooked,omitempty"`
	Tool          string   `json:"tool,omitempty"`
	Note          string   `json:"note,omitempty"`
	Subject       string   `json:"subject,omitempty"`
	SubagentNames []string `json:"subagentNames,omitempty"`
	// SubagentDirs is where each subagent in flight works, in the same
	// order: the file it touched most outside its own directory, or that
	// directory. Item 89.
	SubagentDirs       []string     `json:"subagentDirs,omitempty"`
	ContextTokens      int64        `json:"contextTokens"`
	ContextWindow      int64        `json:"contextWindow"`
	ContextPercent     float64      `json:"contextPercent"`
	Usage              claude.Usage `json:"usage"`
	TokensPerHour      float64      `json:"tokensPerHour"`
	FreshTokensPerHour float64      `json:"freshTokensPerHour"`
	CacheReadPerHour   float64      `json:"cacheReadPerHour"`
	CostUSD            float64      `json:"costUSD"`
	CostKnown          bool         `json:"costKnown"`
	Subagents          int          `json:"subagents"`
	SubagentsInFlight  int          `json:"subagentsInFlight"`
	Team               string       `json:"team,omitempty"`
	// Group is the id of the group the session belongs to, and
	// GroupJoined why it joined (item 87).
	Group       string `json:"group,omitempty"`
	GroupJoined string `json:"groupJoined,omitempty"`
	// GroupLikely is whether it joined on a guess, drawn as a dashed tie.
	GroupLikely bool           `json:"groupLikely,omitempty"`
	Agent       string         `json:"agent,omitempty"`
	Messages    map[string]int `json:"messages,omitempty"`
	// Messaged are the sessions those messages reached, by id (item 90).
	Messaged []string `json:"messaged,omitempty"`
	// GroupWords are the words that joined it to its group, when its
	// wording is why it joined, which its card offers to mute (item 90).
	GroupWords []string       `json:"groupWords,omitempty"`
	Touches    map[string]int `json:"touches,omitempty"`
	// Visits is each directory outside the session's own where one of
	// its subagents worked, by how many subagents worked there. Item 85.
	Visits map[string]int `json:"visits,omitempty"`
	// VisitSpans is when its subagents worked in each of those
	// directories, which grouping reads to tell a session they were
	// coordinating from one that came later (item 90).
	VisitSpans map[string]Span `json:"visitSpans,omitempty"`
	MCPCalls   map[string]int  `json:"mcpCalls,omitempty"`
	Skills     map[string]int  `json:"skills,omitempty"`
	PRs        []claude.PR     `json:"prs,omitempty"`
	// OrderLines are the lines its group's order is read from, held in
	// memory and never written (item 90).
	OrderLines []claude.OrderLine `json:"orderLines,omitempty"`
	Recap      claude.Recap       `json:"recap"`
	LastPrompt string             `json:"lastPrompt,omitempty"`
	// ConversationGone is a session whose transcript Claude Code has
	// deleted; it keeps its lot from the prompt history (item 86).
	ConversationGone bool `json:"conversationGone,omitempty"`
	// Packed is a session you packed up, or whose pull request merged
	// once it had stopped: it is off the map, in the depot (item 86).
	Packed   bool      `json:"packed,omitempty"`
	PackedAt time.Time `json:"packedAt,omitzero"`
	// Hourly is the session's usage by unix hour over its last week,
	// for sparklines and the plant's breakdown.
	Hourly           map[int64]claude.Usage `json:"hourly,omitempty"`
	APIErrors        int                    `json:"apiErrors"`
	LastErrorAt      time.Time              `json:"lastErrorAt"`
	Compactions      int                    `json:"compactions"`
	LastCompactionAt time.Time              `json:"lastCompactionAt"`
	StartedAt        time.Time              `json:"startedAt"`
	LastActivity     time.Time              `json:"lastActivity"`
}

type Server struct {
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"`
	Configured bool   `json:"configured"`
	Calls      int    `json:"calls"`
	Sessions   int    `json:"sessions"`
}

type Skill struct {
	Name     string `json:"name"`
	Calls    int    `json:"calls"`
	Sessions int    `json:"sessions"`
}

type Power struct {
	Since   time.Time               `json:"since"`
	ByModel map[string]claude.Usage `json:"byModel"`
	CostUSD float64                 `json:"costUSD"`
	// CostKnown is whether any transcript in the window carried a
	// cost-state record; without one CostUSD is unknown, not zero.
	CostKnown bool  `json:"costKnown"`
	Fresh     int64 `json:"fresh"`
	Cached    int64 `json:"cached"`
}

const PowerWindow = 24 * time.Hour

// HitRatio is the share of prompt tokens served from the cache, or 0
// when nothing has been read.
func (p Power) HitRatio() float64 {
	total := p.Fresh + p.Cached
	if total == 0 {
		return 0
	}
	return float64(p.Cached) / float64(total)
}

type Sources struct {
	Records     []claude.SessionRecord
	Transcripts []claude.Transcript
	Subagents   map[string][]claude.Subagent
	Roster      map[string]claude.Worker
	Stopped     []claude.Transcript
	// StoppedSubagents are stopped sessions' subagents, by session, for
	// where they worked (item 90).
	StoppedSubagents map[string][]claude.Subagent
	MCP              claude.MCPConfig
	Teams            []claude.Team
}

// Doing is the tool the session is in and what it is working on, with
// the subject shown relative to the project when it lies inside it.
func (s Session) Doing() string {
	if s.Tool == "" {
		return ""
	}
	if s.Subject == "" {
		return s.Tool
	}
	subject := s.Subject
	if s.CWD != "" && strings.HasPrefix(subject, s.CWD+"/") {
		subject = strings.TrimPrefix(subject, s.CWD+"/")
	}
	return s.Tool + " " + subject
}

// Road is a connection between two projects: one per pair, whichever
// way the sessions crossed. Messages and Files count the crossings from
// From to To, and the Back counts the crossings the other way. Item 85.
type Road struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Messages     int    `json:"messages"`
	Files        int    `json:"files"`
	BackMessages int    `json:"backMessages,omitempty"`
	BackFiles    int    `json:"backFiles,omitempty"`
	// Visits is how many subagents worked across the pair.
	Visits int `json:"visits,omitempty"`
	// Sessions is every session that crossed, either way, and Active
	// the ones among them working right now.
	Sessions int `json:"sessions"`
	Active   int `json:"active,omitempty"`
	// Paths are the most-touched files on the road, at most MaxRoadPaths.
	Paths []string `json:"paths,omitempty"`
}

const MaxRoadPaths = 5

type Probes struct {
	Alive    func(pid int) bool
	Attached func(sock string) bool
}

// isOwnRun reports whether a working directory is one botropolis runs
// `claude -p` in itself, and so not a session at all.
//
// Bug 54, and the reason it sits here. `claude -p "/usage"` is a real
// Claude Code session while it runs: it holds a PID and writes a
// transcript, so it reaches the city by BOTH paths — live through
// Records and afterwards through Stopped. The original fence covered
// only the parked one, in the loader, so a running probe became a live
// session: it sailed a tug up the river and re-planned the map around
// itself every time the city asked itself what it had spent.
//
// One fact with two readers and only one told, which is the shape this
// project keeps paying for. Build is where every session arrives by
// whichever path, so the fence is here and nowhere else. A guard that
// has to be repeated in each consumer is a guard that will be missed
// again, because it already was.
//
// The summariser is the second of botropolis's own runs, and it is
// fenced the same way for the same reason.
func isOwnRun(cwd string) bool {
	return cwd != "" && isOwnProject(claude.ProjectFolder(cwd))
}

func isOwnProject(project string) bool {
	return project == claude.UsageProbeProject() || project == claude.ProjectFolder(claude.SummaryDir())
}

func Build(src Sources, probes Probes, now time.Time) []Session {
	byID := map[string]claude.Transcript{}
	for _, t := range src.Transcripts {
		byID[t.SessionID] = t
	}
	sessions := make([]Session, 0, len(src.Records))
	for _, r := range src.Records {
		if isOwnRun(r.CWD) {
			continue
		}
		transcript, hasTranscript := byID[r.SessionID]
		isAlive := probes.Alive(r.PID)
		isAttached := false
		if worker, ok := src.Roster[r.JobID]; ok && r.JobID != "" && worker.PtySock != "" {
			isAttached = probes.Attached(worker.PtySock)
		}
		sessions = append(sessions, buildSession(r, transcript, hasTranscript, src.Subagents[r.SessionID], isAlive, isAttached))
	}
	for _, t := range src.Stopped {
		if isOwnRun(t.CWD) || isOwnProject(t.Project) {
			continue
		}
		s := parkedSession(t)
		for _, sub := range src.StoppedSubagents[t.SessionID] {
			visit(&s, sub.Transcript)
		}
		sessions = append(sessions, s)
	}
	dropSoloTeams(sessions)
	sort.SliceStable(sessions, func(i, j int) bool {
		if sessions[i].CWD != sessions[j].CWD {
			return sessions[i].CWD < sessions[j].CWD
		}
		return sessions[i].StartedAt.Before(sessions[j].StartedAt)
	})
	return sessions
}

// dropSoloTeams clears the team of a session that is alone in a team
// named after a session: a one-member team named after itself is not a
// team, and gets no line on the card and no camp on the map.
func dropSoloTeams(sessions []Session) {
	members := map[string]int{}
	for _, s := range sessions {
		if s.Team != "" {
			members[s.Team]++
		}
	}
	for i := range sessions {
		s := &sessions[i]
		if strings.HasPrefix(s.Team, teamNamePrefix) && members[s.Team] == 1 {
			s.Team, s.Agent = "", ""
		}
	}
}

// ShortID is the eight-character form of a session id, the name a
// session goes by when it has no title.
func ShortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func buildSession(r claude.SessionRecord, t claude.Transcript, hasTranscript bool,
	subagents []claude.Subagent, isAlive, isAttached bool) Session {
	s := Session{
		ID:           r.SessionID,
		Title:        r.Name,
		CWD:          r.CWD,
		Kind:         r.Kind,
		Job:          r.JobID,
		PID:          r.PID,
		Alive:        isAlive,
		Attached:     isAttached,
		StartedAt:    r.StartedAt,
		LastActivity: r.StartedAt,
	}
	if hasTranscript {
		if t.Title != "" {
			s.Title = t.Title
		}
		s.Branch = t.Branch
		s.Model = t.Model
		if t.ModelID != "" {
			s.Model = t.ModelID
		}
		s.Turn = t.Tail.Turn
		s.ContextTokens = t.ContextTokens
		window, known := contextWindow(s.Model, t.Cost.Models, max(t.MaxContext, t.ContextTokens))
		s.ContextWindow = window
		if known {
			s.ContextPercent = 100 * float64(t.ContextTokens) / float64(window)
		}
		s.Usage = t.Usage
		s.CostUSD = t.Cost.TotalUSD
		s.CostKnown = t.Cost.Known
		if !t.LastAt.IsZero() {
			s.LastActivity = t.LastAt
		}
		attribute(&s, t)
	}
	if s.Title == "" {
		s.Title = ShortID(s.ID)
	}
	pending := map[string]bool{}
	for _, id := range t.Tail.PendingToolIDs {
		pending[id] = true
	}
	for _, sub := range subagents {
		visit(&s, sub.Transcript)
		s.Subagents++
		s.Usage = s.Usage.Add(sub.Usage)
		if pending[sub.ToolUseID] {
			s.SubagentsInFlight++
			name := sub.AgentType
			if name == "" {
				name = sub.AgentID
			}
			s.SubagentNames = append(s.SubagentNames, name)
			s.SubagentDirs = append(s.SubagentDirs, worksIn(sub.Transcript))
		}
	}
	if hasTranscript {
		hours := rateSpan(t.FirstAt, t.LastAt).Hours()
		s.FreshTokensPerHour = float64(s.Usage.Input+s.Usage.Output+s.Usage.CacheCreate) / hours
		s.CacheReadPerHour = float64(s.Usage.CacheRead) / hours
		s.TokensPerHour = s.FreshTokensPerHour + s.CacheReadPerHour
	}
	conversation := hasTranscript && (t.Tail.Prompts > 0 || !t.Tail.LastAssistantAt.IsZero())
	s.State = derive(r, s.Turn, conversation, isAlive, isAttached)
	return s
}

// worksIn is where a transcript did its work: the file it touched most
// outside its own directory, or that directory. Ties go to the path that
// sorts first, so the answer does not move between reads.
func worksIn(t claude.Transcript) string {
	best, most := t.CWD, 0
	for path, n := range t.Touches {
		if n > most || (n == most && path < best) {
			best, most = path, n
		}
	}
	return best
}

func parkedSession(t claude.Transcript) Session {
	s := Session{
		ID:           t.SessionID,
		Title:        t.Title,
		CWD:          t.CWD,
		Branch:       t.Branch,
		Model:        t.Model,
		State:        Resting,
		Turn:         t.Tail.Turn,
		StartedAt:    t.FirstAt,
		LastActivity: t.LastAt,
		Usage:        t.Usage,
		CostUSD:      t.Cost.TotalUSD,
		CostKnown:    t.Cost.Known,
	}
	if t.ModelID != "" {
		s.Model = t.ModelID
	}
	if s.Title == "" {
		s.Title = ShortID(t.SessionID)
	}
	attribute(&s, t)
	s.ContextTokens = t.ContextTokens
	window, known := contextWindow(s.Model, t.Cost.Models, max(t.MaxContext, t.ContextTokens))
	s.ContextWindow = window
	if known {
		s.ContextPercent = 100 * float64(t.ContextTokens) / float64(window)
	}
	return s
}

// visit folds a subagent's crossings into its session's: the directory
// it worked in when that is not the session's own, and every file it
// touched outside the session's project. The session's maps may be the
// transcript's own, so they are copied rather than written. Item 85.
// Span is a stretch of time, From to To.
type Span struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

func visit(s *Session, sub claude.Transcript) {
	inside := func(path string) bool { return path == s.CWD || strings.HasPrefix(path, s.CWD+"/") }
	if sub.CWD != "" && !inside(sub.CWD) {
		visits := map[string]int{sub.CWD: 1}
		for k, v := range s.Visits {
			visits[k] += v
		}
		s.Visits = visits
		spans := map[string]Span{}
		for k, v := range s.VisitSpans {
			spans[k] = v
		}
		span, seen := spans[sub.CWD]
		if !seen || sub.FirstAt.Before(span.From) {
			span.From = sub.FirstAt
		}
		if sub.LastAt.After(span.To) {
			span.To = sub.LastAt
		}
		spans[sub.CWD] = span
		s.VisitSpans = spans
	}
	for path, n := range sub.Touches {
		if inside(path) {
			continue
		}
		touches := map[string]int{path: n}
		for k, v := range s.Touches {
			touches[k] += v
		}
		s.Touches = touches
	}
}

// HourlyKeep is how many hourly buckets a session carries: a week.
const HourlyKeep = 7 * 24

func attribute(s *Session, t claude.Transcript) {
	s.Team = t.TeamName
	s.Agent = t.AgentName
	s.Hourly = recentHours(t.Hourly, HourlyKeep)
	s.Messages = t.Messages
	s.Touches = t.Touches
	s.MCPCalls = t.MCPCalls
	s.Skills = t.SkillCalls
	s.PRs = t.PRs
	s.OrderLines = t.OrderLines
	s.Recap = t.Recap
	s.LastPrompt = t.LastPrompt
	s.APIErrors = t.APIErrors
	s.LastErrorAt = t.LastErrorAt
	s.Compactions = t.Compactions
	s.LastCompactionAt = t.LastCompactionAt
}

func Roads(sessions []Session, teams []claude.Team) []Road {
	byName := map[string]claude.Team{}
	for _, team := range teams {
		byName[team.Name] = team
	}
	// Crossings are gathered one way at a time, and then each pair is
	// folded into one road: the direction that sorts first keeps From
	// and To, and the other way becomes its Back counts.
	type key struct{ from, to string }
	type crossing struct {
		messages, files, visits int
		travellers              map[string]State
		touched                 map[string]int
	}
	crossings := map[key]*crossing{}
	add := func(s Session, to, path string, messages, files, visits int) {
		if to == "" || to == s.CWD {
			return
		}
		k := key{s.CWD, to}
		c, ok := crossings[k]
		if !ok {
			c = &crossing{travellers: map[string]State{}, touched: map[string]int{}}
			crossings[k] = c
		}
		c.messages += messages
		c.files += files
		c.visits += visits
		if path != "" {
			c.touched[path] += files
		}
		c.travellers[s.ID] = s.State
	}
	roots := projectRoots(sessions)
	for _, s := range sessions {
		if s.CWD == "" {
			continue
		}
		for path, n := range s.Touches {
			add(s, projectOf(roots, path), path, 0, n, 0)
		}
		for dir, n := range s.Visits {
			add(s, projectOf(roots, dir+"/"), "", 0, 0, n)
		}
		if s.Team == "" {
			continue
		}
		team, known := byName[s.Team]
		for name, n := range s.Messages {
			var member claude.TeamMember
			var ok bool
			if known {
				member, ok = team.Member(name)
			} else if name == teamLead {
				member, ok = leadByTeamName(sessions, s.Team)
			}
			if !ok {
				continue
			}
			add(s, member.CWD, "", n, 0, 0)
		}
	}
	keys := make([]key, 0, len(crossings))
	for k := range crossings {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].from != keys[j].from {
			return keys[i].from < keys[j].from
		}
		return keys[i].to < keys[j].to
	})
	type pair struct {
		road       *Road
		travellers map[string]State
		touched    map[string]int
	}
	pairs := map[key]*pair{}
	var order []key
	for _, k := range keys {
		c := crossings[k]
		p, back := pairs[key{k.to, k.from}]
		if back {
			p.road.BackMessages += c.messages
			p.road.BackFiles += c.files
		} else {
			p = &pair{road: &Road{From: k.from, To: k.to, Messages: c.messages, Files: c.files},
				travellers: map[string]State{}, touched: map[string]int{}}
			pairs[k] = p
			order = append(order, k)
		}
		p.road.Visits += c.visits
		for id, st := range c.travellers {
			p.travellers[id] = st
		}
		for path, n := range c.touched {
			p.touched[path] += n
		}
	}
	out := make([]Road, 0, len(order))
	for _, k := range order {
		p := pairs[k]
		p.road.Sessions = len(p.travellers)
		for _, st := range p.travellers {
			if st == Working {
				p.road.Active++
			}
		}
		p.road.Paths = topPaths(p.touched, MaxRoadPaths)
		out = append(out, *p.road)
	}
	return out
}

const (
	teamLead       = "team-lead"
	teamNamePrefix = "session-"
)

const worktreesDir = "/.claude/worktrees/"

// projectRoots is every session directory that is a project of its own:
// a directory that merely contains other sessions' projects (a home or a
// workspaces folder) is a hub, not a road destination.  A project's own
// worktrees do not make it a hub.
func topPaths(counts map[string]int, n int) []string {
	if len(counts) == 0 {
		return nil
	}
	paths := make([]string, 0, len(counts))
	for path := range counts {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		if counts[paths[i]] != counts[paths[j]] {
			return counts[paths[i]] > counts[paths[j]]
		}
		return paths[i] < paths[j]
	})
	if len(paths) > n {
		paths = paths[:n]
	}
	return paths
}

func projectRoots(sessions []Session) []string {
	seen := map[string]bool{}
	var all []string
	for _, s := range sessions {
		if s.CWD != "" && !seen[s.CWD] {
			seen[s.CWD] = true
			all = append(all, s.CWD)
		}
	}
	var roots []string
	for _, root := range all {
		hub := false
		for _, other := range all {
			if strings.HasPrefix(other, root+"/") && !strings.HasPrefix(other, root+worktreesDir) {
				hub = true
				break
			}
		}
		if !hub {
			roots = append(roots, root)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return len(roots[i]) > len(roots[j]) })
	return roots
}

// projectOf is the longest known project directory containing path.
func projectOf(roots []string, path string) string {
	for _, root := range roots {
		if strings.HasPrefix(path, root+"/") {
			return root
		}
	}
	return ""
}

func leadByTeamName(sessions []Session, teamName string) (claude.TeamMember, bool) {
	if !strings.HasPrefix(teamName, teamNamePrefix) {
		return claude.TeamMember{}, false
	}
	short := strings.TrimPrefix(teamName, teamNamePrefix)
	for _, s := range sessions {
		if strings.HasPrefix(s.ID, short) {
			return claude.TeamMember{Name: teamLead, CWD: s.CWD}, true
		}
	}
	return claude.TeamMember{}, false
}

func Servers(sessions []Session, mcp claude.MCPConfig) []Server {
	byName := map[string]*Server{}
	for _, cfg := range mcp.Global {
		byName[cfg.Name] = &Server{Name: cfg.Name, Type: cfg.Type, Configured: true}
	}
	for _, servers := range mcp.Projects {
		for _, cfg := range servers {
			if _, ok := byName[cfg.Name]; !ok {
				byName[cfg.Name] = &Server{Name: cfg.Name, Type: cfg.Type, Configured: true}
			}
		}
	}
	for _, s := range sessions {
		for name, calls := range s.MCPCalls {
			server, ok := byName[name]
			if !ok {
				server = &Server{Name: name}
				byName[name] = server
			}
			server.Calls += calls
			server.Sessions++
		}
	}
	out := make([]Server, 0, len(byName))
	for _, server := range byName {
		out = append(out, *server)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func Skills(sessions []Session) []Skill {
	byName := map[string]*Skill{}
	for _, s := range sessions {
		for name, calls := range s.Skills {
			skill, ok := byName[name]
			if !ok {
				skill = &Skill{Name: name}
				byName[name] = skill
			}
			skill.Calls += calls
			skill.Sessions++
		}
	}
	out := make([]Skill, 0, len(byName))
	for _, skill := range byName {
		out = append(out, *skill)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Calls != out[j].Calls {
			return out[i].Calls > out[j].Calls
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// recentHours keeps the newest n buckets.
func recentHours(hourly map[int64]claude.Usage, n int) map[int64]claude.Usage {
	if len(hourly) == 0 {
		return nil
	}
	hours := make([]int64, 0, len(hourly))
	for h := range hourly {
		hours = append(hours, h)
	}
	sort.Slice(hours, func(i, j int) bool { return hours[i] > hours[j] })
	if len(hours) > n {
		hours = hours[:n]
	}
	out := make(map[int64]claude.Usage, len(hours))
	for _, h := range hours {
		out[h] = hourly[h]
	}
	return out
}

// PowerSince sums the tokens spent since a moment from each transcript's
// hourly buckets. Cost is only known for a whole session (one cost-state
// record, no timestamp), so the window's cost is the session's cost
// pro-rated by the window's share of its tokens: an estimate, and shown
// as one. ByModel stays the lifetime split, for the plant's card.
func PowerSince(transcripts []claude.Transcript, since time.Time) Power {
	power := Power{Since: since, ByModel: map[string]claude.Usage{}}
	sinceHour := since.Unix() / 3600
	for _, t := range transcripts {
		if t.LastAt.Before(since) {
			continue
		}
		var window claude.Usage
		for hour, u := range t.Hourly {
			if hour >= sinceHour {
				window = window.Add(u)
			}
		}
		var lifetime claude.Usage
		var lifetimeUSD float64
		for model, cost := range t.Cost.Models {
			power.ByModel[model] = power.ByModel[model].Add(cost.Usage)
			lifetime = lifetime.Add(cost.Usage)
			lifetimeUSD += cost.USD
		}
		if all := lifetime.Context() + lifetime.Output; all > 0 {
			power.CostUSD += lifetimeUSD * float64(window.Context()+window.Output) / float64(all)
		}
		power.CostKnown = power.CostKnown || t.Cost.Known
		power.Fresh += window.Input + window.Output + window.CacheCreate
		power.Cached += window.CacheRead
	}
	return power
}

// derive picks a live session's state. The record's own status is the
// CLI's word and wins over the transcript tail: an idle session needs you
// (or is waiting on its own watch), and any other word — busy, shell, or
// one this build has never seen — is a session doing something, unless
// it is blocked in a question only you can answer. A session with no
// conversation yet is empty unless the CLI says it is busy. The tail
// alone decides for a record that carries no status.
func derive(r claude.SessionRecord, turn claude.Turn, hasConversation, isAlive, isAttached bool) State {
	if !isAlive {
		return Resting
	}
	switch r.Status {
	case claude.StatusIdle:
		if !hasConversation {
			return Empty
		}
		if turn == claude.TurnWaiting {
			return Waiting
		}
		return NeedsYou
	case "":
	default:
		if turn == claude.TurnNeedsInput {
			return NeedsYou
		}
		return busy(r, isAttached)
	}
	if !hasConversation {
		return Empty
	}
	switch turn {
	case claude.TurnNeedsInput, claude.TurnAwaitingUser:
		return NeedsYou
	case claude.TurnWaiting:
		return Waiting
	}
	return busy(r, isAttached)
}

func busy(r claude.SessionRecord, isAttached bool) State {
	if r.Kind == claude.KindBackground && !isAttached {
		return Unattended
	}
	return Working
}

func ContextWindow(model string, costModels map[string]claude.ModelCost) int64 {
	window, _ := contextWindow(model, costModels, 0)
	return window
}

// contextWindow decides the window a session's context is measured against.
// A [1m] suffix or a [1m] cost-state entry names it outright; a context that
// was ever larger than the default window proves the large one (context
// cannot exceed the window); otherwise the default is only trusted for model
// families known to ship with it, and anything else stays unknown rather
// than guessed.
func contextWindow(model string, costModels map[string]claude.ModelCost, maxObserved int64) (int64, bool) {
	if strings.HasSuffix(model, largeContextSuffix) {
		return contextWindowLarge, true
	}
	if _, ok := costModels[model+largeContextSuffix]; ok {
		return contextWindowLarge, true
	}
	if maxObserved > contextWindowDefault {
		return contextWindowLarge, true
	}
	for _, family := range defaultWindowFamilies {
		if strings.Contains(model, family) {
			return contextWindowDefault, true
		}
	}
	return contextWindowDefault, false
}

var defaultWindowFamilies = []string{"opus", "sonnet", "haiku"}

func rateSpan(first, last time.Time) time.Duration {
	span := last.Sub(first)
	if span < minRateSpan {
		span = minRateSpan
	}
	return span
}
