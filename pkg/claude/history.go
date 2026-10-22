package claude

import (
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
	return NewHistoryReader().Read(path)
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
