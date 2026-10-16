package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"strings"
)

// ScanToolOutput reads a transcript's tool output, which the finder's
// index leaves out, and returns the first that match accepts. It holds
// nothing: each result is read, tested and dropped. Item 82.
func ScanToolOutput(path string, match func(string) bool) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, maxTranscriptLine)
	for scanner.Scan() {
		var rec saidLineJSON
		if json.Unmarshal(bytes.TrimLeft(scanner.Bytes(), "\x00"), &rec) != nil || rec.Type != "user" || rec.IsSidechain {
			continue
		}
		var blocks []toolResultJSON
		if json.Unmarshal(rec.Message.Content, &blocks) != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type != "tool_result" {
				continue
			}
			if text := b.text(); match(text) {
				return text, true
			}
		}
	}
	return "", false
}

type toolResultJSON struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content"`
}

// text is a tool result's content, which is a string or a list of
// blocks.
func (b toolResultJSON) text() string {
	var text string
	if json.Unmarshal(b.Content, &text) == nil {
		return text
	}
	var blocks []saidBlockJSON
	if json.Unmarshal(b.Content, &blocks) != nil {
		return ""
	}
	var parts []string
	for _, block := range blocks {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}
