package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

const (
	Name            = "codex"
	sessionsDir     = "sessions"
	rolloutPrefix   = "rollout-"
	liveWithin      = 2 * time.Minute
	titleLength     = 60
	maxLine         = 16 << 20
	defaultWindow   = 200_000
	tokenCountEvent = "token_count"
)

type Loader struct {
	home  string
	cache map[string]entry
}

type key struct {
	size    int64
	modTime time.Time
}

type entry struct {
	key     key
	session session
}

type session struct {
	ID            string
	CWD           string
	Branch        string
	Model         string
	Title         string
	FirstAt       time.Time
	LastAt        time.Time
	Usage         claude.Usage
	ContextTokens int64
	ContextWindow int64
	LastRole      string
	TurnDone      bool
}

// NewLoader reads every rollout under home, however old: time ages a
// session, it never takes one off the map (item 86).
func NewLoader(home string) *Loader {
	return &Loader{home: home, cache: map[string]entry{}}
}

func (l *Loader) Name() string {
	return Name
}

func (l *Loader) WatchDirs() []string {
	root := filepath.Join(l.home, sessionsDir)
	dirs := []string{root}
	today := time.Now()
	for _, day := range []time.Time{today, today.AddDate(0, 0, -1)} {
		dirs = append(dirs, filepath.Join(root, day.Format("2006"), day.Format("01"), day.Format("02")))
	}
	return dirs
}

func (l *Loader) Load(now time.Time) (state.Snapshot, error) {
	snapshot := state.Snapshot{At: now, Power: state.Power{Since: now.Add(-state.PowerWindow), ByModel: map[string]claude.Usage{}}}
	paths, err := filepath.Glob(filepath.Join(l.home, sessionsDir, "*", "*", "*", rolloutPrefix+"*.jsonl"))
	if err != nil {
		return snapshot, err
	}
	sort.Strings(paths)
	cache := map[string]entry{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			snapshot.Skipped = append(snapshot.Skipped, claude.SkippedFile{Path: path, Err: err})
			continue
		}
		k := key{size: info.Size(), modTime: info.ModTime()}
		e, ok := l.cache[path]
		if !ok || e.key != k {
			s, err := readRollout(path)
			if err != nil {
				snapshot.Skipped = append(snapshot.Skipped, claude.SkippedFile{Path: path, Err: err})
				continue
			}
			e = entry{key: k, session: s}
		}
		cache[path] = e
		if e.session.ID == "" {
			continue
		}
		snapshot.Sessions = append(snapshot.Sessions, toSession(e.session, info.ModTime(), now))
		if !e.session.LastAt.Before(snapshot.Power.Since) && e.session.Model != "" {
			snapshot.Power.ByModel[e.session.Model] = snapshot.Power.ByModel[e.session.Model].Add(e.session.Usage)
			snapshot.Power.Fresh += e.session.Usage.Input + e.session.Usage.Output
			snapshot.Power.Cached += e.session.Usage.CacheRead
		}
	}
	l.cache = cache
	return snapshot, nil
}

func toSession(s session, modTime, now time.Time) state.Session {
	out := state.Session{
		ID: s.ID, Harness: Name, Title: s.Title, CWD: s.CWD, Branch: s.Branch, Model: s.Model,
		Usage: s.Usage, ContextTokens: s.ContextTokens, ContextWindow: s.ContextWindow,
		StartedAt: s.FirstAt, LastActivity: s.LastAt, State: state.Resting,
	}
	if out.Title == "" {
		out.Title = s.ID
	}
	if out.ContextWindow == 0 {
		out.ContextWindow = defaultWindow
	}
	if out.ContextTokens > 0 {
		out.ContextPercent = 100 * float64(out.ContextTokens) / float64(out.ContextWindow)
	}
	if now.Sub(modTime) <= liveWithin {
		out.Alive = true
		switch {
		case s.TurnDone || s.LastRole == "assistant":
			out.State, out.Turn = state.NeedsYou, claude.TurnAwaitingUser
		default:
			out.State, out.Turn = state.Working, claude.TurnWorking
		}
	}
	return out
}

type line struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type meta struct {
	ID         string `json:"id"`
	CWD        string `json:"cwd"`
	CLIVersion string `json:"cli_version"`
	Git        struct {
		Branch string `json:"branch"`
	} `json:"git"`
}

type item struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type event struct {
	Type string `json:"type"`
	Info struct {
		Total struct {
			Input     int64 `json:"input_tokens"`
			Cached    int64 `json:"cached_input_tokens"`
			Output    int64 `json:"output_tokens"`
			Reasoning int64 `json:"reasoning_output_tokens"`
		} `json:"total_token_usage"`
		Last struct {
			Input  int64 `json:"input_tokens"`
			Cached int64 `json:"cached_input_tokens"`
		} `json:"last_token_usage"`
		ContextWindow int64 `json:"model_context_window"`
	} `json:"info"`
}

type turnContext struct {
	Model string `json:"model"`
}

// Window is how much of a rollout's head, and of its tail, is read when
// the rollout is larger than both together; Claude's catalogue reads a
// transcript the same way (item 86). The head always runs to the end of
// its first line, which carries the session's id, however long it is.
const Window = 64 << 10

func readRollout(path string) (session, error) {
	f, err := os.Open(path)
	if err != nil {
		return session{}, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return session{}, err
	}
	var s session
	if info.Size() <= 2*Window {
		return s, s.scan(f)
	}
	r := bufio.NewReader(f)
	first, err := r.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return s, err
	}
	s.apply(bytes.TrimRight(first, "\n"))
	if rest := Window - int64(len(first)); rest > 0 {
		head := make([]byte, rest)
		n, err := io.ReadFull(r, head)
		if err != nil && err != io.ErrUnexpectedEOF {
			return s, err
		}
		head = head[:n]
		if cut := bytes.LastIndexByte(head, '\n'); cut >= 0 {
			if err := s.scan(bytes.NewReader(head[:cut+1])); err != nil {
				return s, err
			}
		}
	}
	tail := make([]byte, Window)
	n, err := f.ReadAt(tail, info.Size()-Window)
	if err != nil && err != io.EOF {
		return s, err
	}
	tail = tail[:n]
	if cut := bytes.IndexByte(tail, '\n'); cut >= 0 {
		return s, s.scan(bytes.NewReader(tail[cut+1:]))
	}
	return s, nil
}

func (s *session) scan(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(nil, maxLine)
	for scanner.Scan() {
		s.apply(scanner.Bytes())
	}
	return scanner.Err()
}

func (s *session) apply(raw []byte) {
	var l line
	if err := json.Unmarshal(raw, &l); err != nil {
		return
	}
	if ts, err := time.Parse(time.RFC3339Nano, l.Timestamp); err == nil {
		if s.FirstAt.IsZero() {
			s.FirstAt = ts
		}
		s.LastAt = ts
	}
	switch l.Type {
	case "session_meta":
		var m meta
		if json.Unmarshal(l.Payload, &m) == nil {
			s.ID, s.CWD, s.Branch = m.ID, m.CWD, m.Git.Branch
		}
	case "turn_context":
		var tc turnContext
		if json.Unmarshal(l.Payload, &tc) == nil && tc.Model != "" {
			s.Model = tc.Model
		}
	case "response_item":
		var it item
		if json.Unmarshal(l.Payload, &it) != nil {
			return
		}
		switch it.Type {
		case "message":
			s.LastRole = it.Role
			s.TurnDone = false
			if it.Role == "user" && s.Title == "" {
				s.Title = title(it)
			}
		case "function_call":
			s.LastRole = "tool"
			s.TurnDone = false
		}
	case "event_msg":
		var ev event
		if json.Unmarshal(l.Payload, &ev) != nil {
			return
		}
		switch ev.Type {
		case tokenCountEvent:
			s.Usage = claude.Usage{
				Input:     ev.Info.Total.Input - ev.Info.Total.Cached,
				CacheRead: ev.Info.Total.Cached,
				Output:    ev.Info.Total.Output,
				Thinking:  ev.Info.Total.Reasoning,
				Messages:  s.Usage.Messages + 1,
			}
			s.ContextTokens = ev.Info.Last.Input
			if ev.Info.ContextWindow > 0 {
				s.ContextWindow = ev.Info.ContextWindow
			}
		case "task_complete", "turn_complete":
			s.TurnDone = true
		}
	}
}

func title(it item) string {
	for _, c := range it.Content {
		text := strings.TrimSpace(c.Text)
		if text == "" || strings.HasPrefix(text, "<") {
			continue
		}
		if line := strings.SplitN(text, "\n", 2)[0]; len(line) > titleLength {
			return line[:titleLength] + "…"
		} else {
			return line
		}
	}
	return ""
}
