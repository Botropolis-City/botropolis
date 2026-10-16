package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Prompt is one thing typed into a session, as Claude Code's prompt
// history keeps it. It is read where Claude keeps it and never copied.
type Prompt struct {
	Text   string
	Pasted []string
	At     time.Time
	Dir    string
}

type historyLineJSON struct {
	Display string `json:"display"`
	Pasted  map[string]struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	} `json:"pastedContents"`
	Timestamp int64  `json:"timestamp"`
	Project   string `json:"project"`
	SessionID string `json:"sessionId"`
}

func PromptHistoryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "history.jsonl")
}

// ReadPromptHistory reads every prompt in the history by session, in the
// order typed. A missing file is an empty history.
func ReadPromptHistory(path string) (map[string][]Prompt, error) {
	history := map[string][]Prompt{}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return history, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, maxTranscriptLine)
	for scanner.Scan() {
		line := bytes.TrimLeft(scanner.Bytes(), "\x00")
		var rec historyLineJSON
		if json.Unmarshal(line, &rec) != nil || rec.SessionID == "" {
			continue
		}
		history[rec.SessionID] = append(history[rec.SessionID], Prompt{
			Text:   rec.Display,
			Pasted: pastedTexts(rec),
			At:     time.UnixMilli(rec.Timestamp),
			Dir:    rec.Project,
		})
	}
	return history, scanner.Err()
}

func pastedTexts(rec historyLineJSON) []string {
	keys := make([]string, 0, len(rec.Pasted))
	for k, p := range rec.Pasted {
		if p.Content != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var texts []string
	for _, k := range keys {
		texts = append(texts, rec.Pasted[k].Content)
	}
	return texts
}
