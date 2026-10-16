package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"time"
)

// SaidKind is what a passage of a session is: your prompt, Claude's
// recap or reply, a command it ran or a path it touched, or a title.
type SaidKind int

const (
	SaidPrompt SaidKind = iota
	SaidRecap
	SaidReply
	SaidCommand
	SaidPath
	SaidTitle
)

// Passage is one thing said in a session.
type Passage struct {
	Kind SaidKind
	Text string
	At   time.Time
}

// Said is what was said in one session's transcript, for finding it by
// its words. Tool output is left out, and so are subagents and the
// CLI's own wrapped messages. Item 82.
type Said struct {
	SessionID string
	CWD       string
	Branch    string
	FirstAt   time.Time
	LastAt    time.Time
	Passages  []Passage
}

type saidLineJSON struct {
	Type        string          `json:"type"`
	Subtype     string          `json:"subtype"`
	SessionID   string          `json:"sessionId"`
	CWD         string          `json:"cwd"`
	GitBranch   string          `json:"gitBranch"`
	Timestamp   string          `json:"timestamp"`
	IsSidechain bool            `json:"isSidechain"`
	IsMeta      bool            `json:"isMeta"`
	IsCompact   bool            `json:"isCompactSummary"`
	AITitle     string          `json:"aiTitle"`
	CustomTitle string          `json:"customTitle"`
	Summary     string          `json:"summary"`
	Content     json.RawMessage `json:"content"`
	Message     struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type saidBlockJSON struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Name  string `json:"name"`
	Input struct {
		Command      string `json:"command"`
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
		Path         string `json:"path"`
	} `json:"input"`
}

// ReadSaid reads what was said in a transcript, skipping any line that
// does not parse.
func ReadSaid(path string) (Said, error) {
	f, err := openTranscript(path)
	if err != nil {
		return Said{}, err
	}
	defer func() { _ = f.Close() }()
	var said Said
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, maxTranscriptLine)
	for scanner.Scan() {
		var rec saidLineJSON
		if err := json.Unmarshal(bytes.TrimLeft(scanner.Bytes(), "\x00"), &rec); err != nil {
			continue
		}
		said.apply(rec)
	}
	return said, scanner.Err()
}

func (s *Said) apply(rec saidLineJSON) {
	if rec.IsSidechain {
		return
	}
	if s.SessionID == "" {
		s.SessionID = rec.SessionID
	}
	if s.CWD == "" {
		s.CWD = rec.CWD
	}
	if rec.GitBranch != "" && rec.GitBranch != "HEAD" {
		s.Branch = rec.GitBranch
	}
	at, hasTime := parseTimestamp(rec.Timestamp)
	if hasTime {
		if s.FirstAt.IsZero() {
			s.FirstAt = at
		}
		s.LastAt = at
	}
	add := func(kind SaidKind, text string) {
		if text = strings.TrimSpace(text); text != "" {
			s.Passages = append(s.Passages, Passage{Kind: kind, Text: text, At: at})
		}
	}
	switch rec.Type {
	case "user":
		if !rec.IsMeta && !rec.IsCompact {
			add(SaidPrompt, typedPrompt(rec.Message.Content))
		}
	case "assistant":
		var blocks []saidBlockJSON
		if json.Unmarshal(rec.Message.Content, &blocks) != nil {
			return
		}
		for _, b := range blocks {
			switch b.Type {
			case "text":
				add(SaidReply, b.Text)
			case "tool_use":
				add(SaidCommand, b.Input.Command)
				for _, p := range []string{b.Input.FilePath, b.Input.NotebookPath, b.Input.Path} {
					add(SaidPath, p)
				}
			}
		}
	case "system":
		var text string
		if rec.Subtype == "away_summary" && json.Unmarshal(rec.Content, &text) == nil {
			add(SaidRecap, text)
		}
	case "custom-title":
		add(SaidTitle, rec.CustomTitle)
	case "ai-title":
		add(SaidTitle, rec.AITitle)
	case "summary":
		add(SaidTitle, rec.Summary)
	}
}

// typedPrompt is the text of a user record that someone typed: not a
// tool result, and not one of the CLI's own messages, which it wraps in
// a tag.
func typedPrompt(content json.RawMessage) string {
	var text string
	if json.Unmarshal(content, &text) != nil {
		var blocks []saidBlockJSON
		if json.Unmarshal(content, &blocks) != nil {
			return ""
		}
		var parts []string
		for _, b := range blocks {
			if b.Type == "tool_result" {
				return ""
			}
			if b.Type == "text" {
				parts = append(parts, b.Text)
			}
		}
		text = strings.Join(parts, "\n")
	}
	if strings.HasPrefix(strings.TrimSpace(text), "<") {
		return ""
	}
	return text
}
