package state

import (
	"cmp"
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

// BackgroundAgentQuiet is how long an agent launched in the background
// may go without writing a line before it no longer counts as in
// flight: one that died mid-turn would otherwise fly for ever. The
// loader has no other clock on work, since a session's state comes from
// its process and its turn (item 89).
const BackgroundAgentQuiet = 10 * time.Minute

type Session struct {
	ID      string      `json:"id"`
	Harness string      `json:"harness,omitempty"`
	Title   string      `json:"title"`
	CWD     string      `json:"cwd"`
	Branch  string      `json:"branch"`
	Model   string      `json:"model"`
	Kind    claude.Kind `json:"kind"`
	// Entrypoint is how the session was started: cli at a prompt, sdk-*
	// from a program (item 273).
	Entrypoint string `json:"entrypoint,omitempty"`
	// Job is the background job the session runs as, by which New
	// session records the group it was started in (item 87).
	Job string `json:"job,omitempty"`
	// Name is the session's own name, the one other sessions message it
	// by; the title can change from it (item 89).
	Name     string      `json:"name,omitempty"`
	PID      int         `json:"pid"`
	Alive    bool        `json:"alive"`
	Attached bool        `json:"attached"`
	State    State       `json:"state"`
	Turn     claude.Turn `json:"turn"`
	// RecordStatus is the CLI's own word for the session, which the
	// daemon's hook overlay yields to while it says shell (item 250).
	RecordStatus claude.Status `json:"-"`
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
	SubagentDirs  []string `json:"subagentDirs,omitempty"`
	ContextTokens int64    `json:"contextTokens"`
	ContextWindow int64    `json:"contextWindow"`
	// ContextKnown is whether the window was read or proved rather than
	// assumed: an unknown model is measured against nothing (item 147).
	ContextKnown       bool         `json:"contextKnown,omitempty"`
	ContextPercent     float64      `json:"contextPercent"`
	Usage              claude.Usage `json:"usage"`
	TokensPerHour      float64      `json:"tokensPerHour"`
	FreshTokensPerHour float64      `json:"freshTokensPerHour"`
	CacheReadPerHour   float64      `json:"cacheReadPerHour"`
	// OutputPerHour is output tokens an hour, kept apart from fresh
	// tokens because output is priced apart (item 116).
	OutputPerHour float64 `json:"outputPerHour"`
	CostUSD       float64 `json:"costUSD"`
	CostKnown     bool    `json:"costKnown"`
	// Heft is the session's weight over its whole life, subagents
	// included; HeftPartial marks one read only through the catalogue's
	// window, so a lower bound. Storeys is its building's height on the
	// user's own scale. Item 109.
	Heft        int64 `json:"heft,omitempty"`
	HeftPartial bool  `json:"heftPartial,omitempty"`
	Storeys     int   `json:"storeys,omitempty"`
	// Spire is how many segments rise past the top line (item 109).
	Spire             int    `json:"spire,omitempty"`
	Subagents         int    `json:"subagents"`
	SubagentsInFlight int    `json:"subagentsInFlight"`
	Team              string `json:"team,omitempty"`
	// Group is the id of the group the session belongs to, and
	// GroupJoined why it joined (item 87).
	Group       string `json:"group,omitempty"`
	GroupJoined string `json:"groupJoined,omitempty"`
	// GroupLikely is whether it joined on a guess, drawn as a dashed tie.
	GroupLikely bool   `json:"groupLikely,omitempty"`
	Agent       string `json:"agent,omitempty"`
	// Leads is the team the session leads (item 253).
	Leads    string         `json:"leads,omitempty"`
	Messages map[string]int `json:"messages,omitempty"`
	// ToolCalls counts the main thread's tool calls, for the pulses up
	// its building (item 89).
	ToolCalls int `json:"toolCalls,omitempty"`
	// PeerMessages are the messages other sessions sent this one. Roads
	// read them in the loader, so they are not sent on (item 89).
	PeerMessages []claude.PeerMessage `json:"-"`
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
	// Fresh is prompt tokens not read from the cache, input and cache
	// writes; Cached is cache reads; Output is output. Each is priced
	// apart, so none is added to another (item 116).
	Fresh  int64 `json:"fresh"`
	Cached int64 `json:"cached"`
	Output int64 `json:"output"`
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
// ownProjects are the project folders Botropolis's own runs of Claude
// Code file their transcripts under, worked out once a build rather
// than once a session (item 196).
type ownProjects [2]string

func newOwnProjects() ownProjects {
	return ownProjects{claude.UsageProbeProject(), claude.ProjectFolder(claude.SummaryDir())}
}

func (o ownProjects) run(cwd string) bool {
	return cwd != "" && o.project(claude.ProjectFolder(cwd))
}

func (o ownProjects) project(project string) bool {
	return project == o[0] || project == o[1]
}

func Build(src Sources, probes Probes, now time.Time) []Session {
	return mergeSessions(buildLive(src, probes, now), buildParked(src.Stopped, src.StoppedSubagents))
}

// buildLive is the sessions Claude Code has records for, sorted.
func buildLive(src Sources, probes Probes, now time.Time) []Session {
	byID := make(map[string]claude.Transcript, len(src.Transcripts))
	for _, t := range src.Transcripts {
		byID[t.SessionID] = t
	}
	sessions := make([]Session, 0, len(src.Records))
	own := newOwnProjects()
	for _, r := range src.Records {
		if own.run(r.CWD) {
			continue
		}
		transcript, hasTranscript := byID[r.SessionID]
		isAlive := probes.Alive(r.PID)
		isAttached := false
		if worker, ok := src.Roster[r.JobID]; ok && r.JobID != "" && worker.PtySock != "" {
			isAttached = probes.Attached(worker.PtySock)
		}
		sessions = append(sessions, buildSession(r, transcript, hasTranscript, src.Subagents[r.SessionID], isAlive, isAttached, now))
	}
	sortSessions(sessions)
	return sessions
}

// buildParked is the stopped sessions, sorted. It reads nothing but the
// transcripts, not even the time, so a loader keeps it while none of
// them changes (item 205).
func buildParked(stopped []claude.Transcript, subagents map[string][]claude.Subagent) []Session {
	sessions := make([]Session, 0, len(stopped))
	own := newOwnProjects()
	for _, t := range stopped {
		if own.run(t.CWD) || own.project(t.Project) {
			continue
		}
		s := parkedSession(t)
		for _, sub := range subagents[t.SessionID] {
			visit(&s, sub.Transcript)
		}
		sessions = append(sessions, s)
	}
	sortSessions(sessions)
	return sessions
}

// mergeSessions is the live and the stopped sessions in one order, the
// live first where two tie, as one stable sort of both, live first,
// would give: by directory, then by when each started.
func mergeSessions(live, parked []Session) []Session {
	sessions := make([]Session, 0, len(live)+len(parked))
	i, j := 0, 0
	for i < len(live) && j < len(parked) {
		if sessionBefore(&parked[j], &live[i]) {
			sessions = append(sessions, parked[j])
			j++
		} else {
			sessions = append(sessions, live[i])
			i++
		}
	}
	sessions = append(append(sessions, live[i:]...), parked[j:]...)
	dropSoloTeams(sessions)
	return sessions
}

func sortSessions(sessions []Session) {
	sort.SliceStable(sessions, func(i, j int) bool { return sessionBefore(&sessions[i], &sessions[j]) })
}

func sessionBefore(a, b *Session) bool {
	if a.CWD != b.CWD {
		return a.CWD < b.CWD
	}
	return a.StartedAt.Before(b.StartedAt)
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

// Scripted is whether a program started the session rather than
// someone at a prompt (item 273).
func (s Session) Scripted() bool {
	return claude.Scripted(s.Entrypoint)
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
	subagents []claude.Subagent, isAlive, isAttached bool, now time.Time) Session {
	s := Session{
		ID:           r.SessionID,
		Title:        r.Name,
		Name:         r.Name,
		CWD:          r.CWD,
		Kind:         r.Kind,
		Entrypoint:   r.Entrypoint,
		Job:          r.JobID,
		PID:          r.PID,
		Alive:        isAlive,
		Attached:     isAttached,
		RecordStatus: r.Status,
		StartedAt:    r.StartedAt,
		LastActivity: r.StartedAt,
	}
	if hasTranscript {
		if t.Title != "" {
			s.Title = t.Title
		}
		s.Branch = t.Branch
		if t.Entrypoint != "" {
			s.Entrypoint = t.Entrypoint
		}
		s.Model = t.Model
		if t.ModelID != "" {
			s.Model = t.ModelID
		}
		s.Turn = t.Tail.Turn
		s.ContextTokens = t.ContextTokens
		window, known := contextWindow(s.Model, t.Cost.Models, max(t.MaxContext, t.ContextTokens))
		s.ContextWindow, s.ContextKnown = window, known
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
		if pending[sub.ToolUseID] || backgroundWorking(t, sub, isAlive, now) {
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
		s.FreshTokensPerHour = float64(s.Usage.Input+s.Usage.CacheCreate) / hours
		s.CacheReadPerHour = float64(s.Usage.CacheRead) / hours
		s.OutputPerHour = float64(s.Usage.Output) / hours
		s.TokensPerHour = s.FreshTokensPerHour + s.CacheReadPerHour + s.OutputPerHour
	}
	conversation := hasTranscript && (t.Tail.Prompts > 0 || !t.Tail.LastAssistantAt.IsZero())
	s.State = derive(r, s.Turn, conversation, isAlive, isAttached)
	return s
}

// backgroundWorking reports whether an agent launched in the background
// is still at work: the session is alive, the agent's own turn is not
// finished, and it has written within BackgroundAgentQuiet.
func backgroundWorking(parent claude.Transcript, sub claude.Subagent, isAlive bool, now time.Time) bool {
	return isAlive && parent.Background[sub.ToolUseID] &&
		sub.Tail.Turn == claude.TurnWorking && now.Sub(sub.LastAt) <= BackgroundAgentQuiet
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
		Entrypoint:   t.Entrypoint,
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
	s.ContextWindow, s.ContextKnown = window, known
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
	s.Team = cmp.Or(t.TeamName, t.LeadsTeam)
	s.Leads = t.LeadsTeam
	s.Agent = t.AgentName
	s.Hourly = recentHours(t.Hourly, HourlyKeep)
	s.Messages = t.Messages
	s.PeerMessages = t.PeerMessages
	s.ToolCalls = t.ToolCalls
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
		for _, m := range s.PeerMessages {
			if from, ok := peerSender(sessions, m); ok {
				add(from, s.CWD, "", 1, 0, 0)
			}
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
			}
			if !ok {
				member, ok = memberBySession(sessions, s.Team, name)
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

// peerSender is the session that sent a peer message: the one whose
// process the recipient verified, or failing that the only one with the
// name it gave. A message from a sender that cannot be told is dropped.
func peerSender(sessions []Session, m claude.PeerMessage) (Session, bool) {
	if m.FromPID != 0 {
		for _, s := range sessions {
			if s.PID == m.FromPID {
				return s, true
			}
		}
	}
	var found Session
	n := 0
	for _, s := range sessions {
		if m.FromName != "" && s.Name == m.FromName {
			found = s
			n++
		}
	}
	return found, n == 1
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

// memberBySession finds a teammate among the sessions when the team's
// config is gone, as Claude Code deletes it when the lead ends: the lead
// is the session whose Agent results name the team, and a teammate the
// session in the team with that agent name (item 253).
func memberBySession(sessions []Session, teamName, name string) (claude.TeamMember, bool) {
	for _, s := range sessions {
		if name == teamLead && s.Leads == teamName || name != teamLead && s.Team == teamName && s.Agent == name {
			return claude.TeamMember{Name: name, CWD: s.CWD}, true
		}
	}
	if name == teamLead {
		return leadByTeamName(sessions, teamName)
	}
	return claude.TeamMember{}, false
}

// leadByTeamName is the fallback for transcripts from older Claude Code
// versions, which named a team after its lead's id.
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
// hourly buckets. It carries no cost: cost comes from the statistics
// store, each message priced under its own model (item 116). ByModel
// stays the lifetime split.
func PowerSince(transcripts []claude.Transcript, since time.Time) Power {
	return powerSince(since, transcripts)
}

// powerSince is PowerSince over several lists, so a load need not copy
// every stopped transcript into one (item 205).
func powerSince(since time.Time, lists ...[]claude.Transcript) Power {
	power := Power{Since: since, ByModel: map[string]claude.Usage{}}
	sinceHour := since.Unix() / 3600
	for _, transcripts := range lists {
		power.add(transcripts, since, sinceHour)
	}
	return power
}

func (power *Power) add(transcripts []claude.Transcript, since time.Time, sinceHour int64) {
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
		for model, cost := range t.Cost.Models {
			power.ByModel[model] = power.ByModel[model].Add(cost.Usage)
		}
		power.Fresh += window.Input + window.CacheCreate
		power.Cached += window.CacheRead
		power.Output += window.Output
	}
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

// ModelWindow is the window the model table gives a model by its name
// alone, and whether the table knows it (item 147).
func ModelWindow(model string) (int64, bool) {
	return contextWindow(model, nil, 0)
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
