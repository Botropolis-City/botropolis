package claude

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/measure"
)

// From is who a message in a conversation is from: you, Claude, or a
// tool Claude called.
type From int

const (
	FromYou From = iota
	FromClaude
	FromTool
)

// Message is one turn of a conversation as the reading pane shows it. A
// tool call is one line: the command, the path, or else the tool's name.
type Message struct {
	From From
	Text string
	At   time.Time
}

// Question is one question a session is waiting on you to answer, word
// for word, with the options it offered.
type Question struct {
	Text    string
	Options []string
}

// PendingCall is a tool call with no result yet, word for word: the
// command, else the path. It is what a session waiting on your approval
// is asking to run, though the transcript alone cannot tell that wait
// from a call still running.
type PendingCall struct {
	Tool  string
	Input string
}

// Conversation is a page of a session's conversation, oldest first.
// AtStart says the page reaches the conversation's first turn.
// Questions are what the session is asking you, and Pending the calls
// still out at the end of it.
type Conversation struct {
	Messages  []Message
	AtStart   bool
	Questions []Question
	Pending   []PendingCall
}

// conversationChunk is how much of a transcript is read at a time,
// walking back from its end.
const conversationChunk = 64 << 10

type conversationLineJSON struct {
	Type        string `json:"type"`
	Timestamp   string `json:"timestamp"`
	IsSidechain bool   `json:"isSidechain"`
	IsMeta      bool   `json:"isMeta"`
	IsCompact   bool   `json:"isCompactSummary"`
	Message     struct {
		ID      string          `json:"id"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type conversationBlockJSON struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	ToolUseID string `json:"tool_use_id"`
	Input     struct {
		Command      string `json:"command"`
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
		Path         string `json:"path"`
		Questions    []struct {
			Question string `json:"question"`
			Options  []struct {
				Label string `json:"label"`
			} `json:"options"`
		} `json:"questions"`
	} `json:"input"`
}

// ReadConversation reads a page of a session's conversation: count
// turns, after leaving out the newest skip. Item 83.
func ReadConversation(path string, skip, count int) (Conversation, error) {
	f, err := openTranscript(path)
	if err != nil {
		return Conversation{}, err
	}
	measure.Count(measure.ConversationsRead)
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return Conversation{}, err
	}
	return ReadConversationAt(f, info.Size(), skip, count)
}

// ReadConversationAt reads a page of a conversation from a transcript
// of size bytes, walking back from its end a chunk at a time and
// stopping once it holds the page, so it never reads much more of the
// file than it shows. It goes one turn past the page before stopping,
// because the oldest turn read can be the end of a reply written as
// several records.
func ReadConversationAt(r io.ReaderAt, size int64, skip, count int) (Conversation, error) {
	var records []conversationLineJSON
	var partial []byte
	pos := size
	for {
		start := max(0, pos-conversationChunk)
		chunk := make([]byte, pos-start, int(pos-start)+len(partial))
		if _, err := r.ReadAt(chunk, start); err != nil && err != io.EOF {
			return Conversation{}, err
		}
		chunk = append(chunk, partial...)
		pos = start
		lines := bytes.Split(chunk, []byte("\n"))
		partial = nil
		if pos > 0 {
			partial, lines = lines[0], lines[1:]
		}
		var read []conversationLineJSON
		for _, line := range lines {
			var rec conversationLineJSON
			if json.Unmarshal(bytes.TrimLeft(line, "\x00"), &rec) == nil {
				read = append(read, rec)
			}
		}
		records = append(read, records...)
		conv := conversationOf(records)
		if pos == 0 || len(conv.Messages) > skip+count {
			conv.AtStart = pos == 0 && len(conv.Messages) <= skip+count
			end := max(0, len(conv.Messages)-skip)
			conv.Messages = conv.Messages[max(0, end-count):end]
			return conv, nil
		}
	}
}

// conversationOf is the turns the records hold, and the questions still
// waiting on an answer at the end of them.
func conversationOf(records []conversationLineJSON) Conversation {
	var conv Conversation
	asked := map[string][]Question{}
	pending := map[string]PendingCall{}
	var order []string
	lastID := ""
	for _, rec := range records {
		if rec.IsSidechain {
			continue
		}
		at, _ := parseTimestamp(rec.Timestamp)
		var blocks []conversationBlockJSON
		switch rec.Type {
		case "user":
			for _, b := range contentBlocks(rec.Message.Content) {
				if b.Type == "tool_result" {
					delete(asked, b.ToolUseID)
					delete(pending, b.ToolUseID)
				}
			}
			if rec.IsMeta || rec.IsCompact {
				continue
			}
			if text := strings.TrimSpace(typedPrompt(rec.Message.Content)); text != "" {
				conv.Messages = append(conv.Messages, Message{From: FromYou, Text: text, At: at})
			}
			lastID = ""
		case "assistant":
			blocks = contentBlocks(rec.Message.Content)
			for _, b := range blocks {
				switch b.Type {
				case "text":
					text := strings.TrimSpace(b.Text)
					if text == "" {
						continue
					}
					if n := len(conv.Messages); n > 0 && lastID == rec.Message.ID && conv.Messages[n-1].From == FromClaude {
						conv.Messages[n-1].Text += "\n\n" + text
						continue
					}
					conv.Messages = append(conv.Messages, Message{From: FromClaude, Text: text, At: at})
				case "tool_use":
					conv.Messages = append(conv.Messages, Message{From: FromTool, Text: toolLine(b), At: at})
					if b.Name == askUserTool {
						var qs []Question
						for _, q := range b.Input.Questions {
							question := Question{Text: q.Question}
							for _, o := range q.Options {
								question.Options = append(question.Options, o.Label)
							}
							qs = append(qs, question)
						}
						asked[b.ID] = qs
					} else {
						pending[b.ID] = PendingCall{Tool: b.Name, Input: verbatim(b)}
					}
					order = append(order, b.ID)
				}
			}
			lastID = rec.Message.ID
		}
	}
	for _, id := range order {
		conv.Questions = append(conv.Questions, asked[id]...)
		delete(asked, id)
		if call, ok := pending[id]; ok {
			conv.Pending = append(conv.Pending, call)
			delete(pending, id)
		}
	}
	return conv
}

func contentBlocks(content json.RawMessage) []conversationBlockJSON {
	var blocks []conversationBlockJSON
	if json.Unmarshal(content, &blocks) != nil {
		return nil
	}
	return blocks
}

// verbatim is a tool call's command, else its path, as written.
func verbatim(b conversationBlockJSON) string {
	for _, s := range []string{b.Input.Command, b.Input.FilePath, b.Input.NotebookPath, b.Input.Path} {
		if s != "" {
			return s
		}
	}
	return ""
}

// toolLine is a tool call in one line: the command it ran, else the
// path it touched, else the tool's name.
func toolLine(b conversationBlockJSON) string {
	if line := strings.Join(strings.Fields(verbatim(b)), " "); line != "" {
		return line
	}
	return b.Name
}
