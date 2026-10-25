package demo

import (
	"bufio"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/claude"
)

// HistoryPrompt is one line of a synthetic prompt history.
type HistoryPrompt struct {
	Session string
	Dir     string
	Text    string
	At      time.Time
}

var commonWords = strings.Fields(`build tests failing parser config output server client cache logger
	handler request response module package release deploy script error branch commit review docs
	refactor layout render styles schema migration`)

// SyntheticHistory is a prompt history made up from a seed, in time
// order: sessions working on a dozen topics across a dozen directories,
// each topic with words of its own and some with a ticket key, among
// words every session uses and words only one does. It stands in for a
// real history, which is never copied.
func SyntheticHistory(seed int64, sessions, prompts int, start time.Time) []HistoryPrompt {
	r := rand.New(rand.NewSource(seed))
	var out []HistoryPrompt
	at := start
	for s := 0; s < sessions; s++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", s)
		dir := fmt.Sprintf("/w/repo%02d", r.Intn(12))
		topic := r.Intn(14)
		if r.Intn(5) > 0 {
			at = at.Add(time.Duration(30+r.Intn(240)) * time.Minute)
		} else {
			at = at.Add(time.Duration(r.Intn(50)) * time.Second)
		}
		when := at
		for p := 0; p < prompts; p++ {
			words := []string{commonWords[r.Intn(len(commonWords))], commonWords[r.Intn(len(commonWords))]}
			if topic < 12 {
				words = append(words, fmt.Sprintf("topic%cword%c", 'a'+topic, 'a'+r.Intn(6)))
				if r.Intn(3) == 0 {
					words = append(words, fmt.Sprintf("topic%cword%c", 'a'+topic, 'a'+r.Intn(6)))
				}
				if topic%4 == 0 && r.Intn(8) == 0 {
					words = append(words, fmt.Sprintf("PROJ-%d", 100+topic))
				}
			}
			if r.Intn(4) == 0 {
				words = append(words, fmt.Sprintf("only%dx%d", s, p))
			}
			out = append(out, HistoryPrompt{Session: id, Dir: dir, Text: "please " + strings.Join(words, " "), At: when})
			when = when.Add(time.Duration(1+r.Intn(20)) * time.Minute)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

// ManySessions is a heavy user's prompt history made up from a seed, in
// time order: n small sessions started across the 30 days before end,
// about one topic for every eight sessions and one directory for every
// twenty, so that what any one session shares grows with its own work
// and not with the history's length (item 198).
func ManySessions(seed int64, n int, end time.Time) []HistoryPrompt {
	return manySessions(seed, n, max(1, n/20), false, end)
}

// SynthHistory is ManySessions with the number of projects given, and
// every one of them used: the first sessions take one project each, so
// the history holds exactly as many as were asked for (item 248).
func SynthHistory(seed int64, sessions, projects int, end time.Time) ([]HistoryPrompt, error) {
	if projects < 1 || sessions < projects {
		return nil, fmt.Errorf("%d sessions cannot fill %d projects: each project needs a session", sessions, projects)
	}
	return manySessions(seed, sessions, projects, true, end), nil
}

func manySessions(seed int64, n, dirs int, everyDir bool, end time.Time) []HistoryPrompt {
	r := rand.New(rand.NewSource(seed))
	topics := max(1, n/8)
	span := 29 * 24 * time.Hour
	start := end.Add(-span)
	var out []HistoryPrompt
	for s := 0; s < n; s++ {
		id := fmt.Sprintf("00000000-0000-4198-8000-%012d", s)
		d := r.Intn(dirs)
		if everyDir && s < dirs {
			d = s
		}
		dir := fmt.Sprintf("/w/many%03d", d)
		topic := r.Intn(topics)
		when := start.Add(time.Duration(s) * span / time.Duration(n+1)).Add(time.Duration(r.Intn(600)) * time.Second)
		prompts := 2 + r.Intn(4)
		for p := 0; p < prompts; p++ {
			words := []string{commonWords[r.Intn(len(commonWords))], commonWords[r.Intn(len(commonWords))],
				fmt.Sprintf("many%dtopic%c", topic, 'a'+r.Intn(4))}
			if r.Intn(10) == 0 {
				words = append(words, fmt.Sprintf("MANY-%d", topic))
			}
			if r.Intn(4) == 0 {
				words = append(words, fmt.Sprintf("only%dx%d", s, p))
			}
			out = append(out, HistoryPrompt{Session: id, Dir: dir, Text: "please " + strings.Join(words, " "), At: when})
			when = when.Add(time.Duration(1+r.Intn(5)) * time.Minute)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

// UserPromptLine is a transcript's opening prompt.
func UserPromptLine(sessionID, cwd, ts string) string {
	return fmt.Sprintf(`{"type":"user","isSidechain":false,"timestamp":%q,"sessionId":%q,"cwd":%q,"gitBranch":"main",`+
		`"version":"2.1.273","entrypoint":"cli","message":{"role":"user","content":"hi"}}`, ts, sessionID, cwd)
}

// AssistantReplyLine is a transcript's reply to it.
func AssistantReplyLine(sessionID, id, ts string) string {
	return fmt.Sprintf(`{"type":"assistant","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
		`"message":{"id":%q,"model":"claude-opus-5","role":"assistant","stop_reason":"end_turn",`+
		`"content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":2,"output_tokens":5,"cache_read_input_tokens":250000}}}`,
		ts, sessionID, id)
}

// PromptHistoryLine is one typed prompt in Claude Code's history.jsonl.
func PromptHistoryLine(sessionID, project, text string, at time.Time) string {
	return fmt.Sprintf(`{"display":%q,"pastedContents":{},"timestamp":%d,"project":%q,"sessionId":%q}`+"\n",
		text, at.UnixMilli(), project, sessionID)
}

// Written is what WriteHome wrote.
type Written struct {
	Sessions, Projects, Prompts int
}

// WriteHome writes a history into home's .claude: a transcript of one
// prompt and its reply for each session, timed in layout, and every
// prompt appended to history.jsonl.
func WriteHome(home string, history []HistoryPrompt, layout string) (Written, error) {
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Written{}, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "history.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Written{}, err
	}
	w := bufio.NewWriter(f)
	started, projects := map[string]bool{}, map[string]bool{}
	for _, p := range history {
		if !started[p.Session] {
			started[p.Session], projects[p.Dir] = true, true
			ts := p.At.Format(layout)
			transcript := filepath.Join(dir, "projects", claude.ProjectFolder(p.Dir), p.Session+".jsonl")
			content := UserPromptLine(p.Session, p.Dir, ts) + "\n" +
				AssistantReplyLine(p.Session, "msg_"+p.Session[len(p.Session)-6:], ts) + "\n"
			if err := os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
				_ = f.Close()
				return Written{}, err
			}
			if err := os.WriteFile(transcript, []byte(content), 0o600); err != nil {
				_ = f.Close()
				return Written{}, err
			}
		}
		if _, err := w.WriteString(PromptHistoryLine(p.Session, p.Dir, p.Text, p.At)); err != nil {
			_ = f.Close()
			return Written{}, err
		}
	}
	if err := errors.Join(w.Flush(), f.Close()); err != nil {
		return Written{}, err
	}
	return Written{Sessions: len(started), Projects: len(projects), Prompts: len(history)}, nil
}

// RealClaudeDirs are the places a real Claude home can be: .claude in
// $HOME and in the account's own home, and $CLAUDE_CONFIG_DIR.
func RealClaudeDirs() []string {
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs, filepath.Join(home, ".claude"))
	}
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		dirs = append(dirs, filepath.Join(u.HomeDir, ".claude"))
	}
	if config := os.Getenv("CLAUDE_CONFIG_DIR"); config != "" {
		dirs = append(dirs, config)
	}
	return dirs
}

// RefuseRealClaude refuses a home whose .claude would be, or be inside,
// any of real, links followed, so nothing synthetic is ever written into
// someone's own history.
func RefuseRealClaude(home string, real []string) error {
	target, err := resolve(filepath.Join(home, ".claude"))
	if err != nil {
		return err
	}
	for _, r := range real {
		dir, err := resolve(r)
		if err != nil {
			return err
		}
		if rel, err := filepath.Rel(dir, target); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("refusing %s: its .claude would be inside the real Claude home %s", home, r)
		}
	}
	return nil
}

func resolve(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var rest []string
	for dir := abs; ; dir = filepath.Dir(dir) {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(append([]string{resolved}, rest...)...), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if dir == filepath.Dir(dir) {
			return abs, nil
		}
		rest = append([]string{filepath.Base(dir)}, rest...)
	}
}
