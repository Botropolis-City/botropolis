package claude

import (
	"encoding/json"
	"strings"
	"time"
)

type Turn string

const (
	TurnUnknown      Turn = ""
	TurnWorking      Turn = "working"
	TurnAwaitingUser Turn = "awaiting-user"
	TurnNeedsInput   Turn = "needs-input"
	// TurnWaiting is a turn handed back by a session that armed its own
	// watch — a scheduled wakeup, a Monitor, a cron loop — so it will
	// carry on by itself and only takes input in the meantime.
	TurnWaiting Turn = "waiting"
)

const askUserTool = "AskUserQuestion"

// watchTools arm a session's own watch: it will wake itself.
var watchTools = map[string]bool{"ScheduleWakeup": true, "Monitor": true, "CronCreate": true}

type Tail struct {
	Turn            Turn
	PendingTools    []string
	PendingToolIDs  []string
	Prompts         int
	LastPromptAt    time.Time
	LastAssistantAt time.Time
	// FailedTools is how many tool results in a row on the main line
	// came back as errors, up to the last; one that succeeds or a
	// prompt starts it again (item 280).
	FailedTools int
	// Interrupted is a main line whose last word is the user's
	// interrupt, which Claude Code writes as a user line (item 280).
	Interrupted bool
}

// interruptPrefix opens the line Claude Code writes when the user
// interrupts a turn, with or without " for tool use".
const interruptPrefix = "[Request interrupted by user"

type contentBlockJSON struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	ToolUseID string `json:"tool_use_id"`
	IsError   bool   `json:"is_error"`
	Text      string `json:"text"`
	Input     struct {
		Command      string `json:"command"`
		To           string `json:"to"`
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
		Stop         bool   `json:"stop"`
	} `json:"input"`
}

type contentJSON []contentBlockJSON

func (c *contentJSON) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		*c = nil
		return nil
	}
	var blocks []contentBlockJSON
	if err := json.Unmarshal(data, &blocks); err != nil {
		return err
	}
	*c = blocks
	return nil
}

type pendingTool struct {
	id, name string
}

type tailScan struct {
	tail      Tail
	lastRole  string
	lastStop  string
	messageID string
	pending   []pendingTool
	// watch is set once the session armed a wakeup since the last real
	// prompt, and cleared by the next one.
	watch bool
}

func (s *tailScan) user(rec transcriptLineJSON, ts time.Time) {
	s.lastRole = "user"
	results, prompt := false, !rec.IsMeta
	s.tail.Interrupted = false
	for _, block := range rec.Message.Content {
		if block.Type == "tool_result" {
			results, prompt = true, false
			s.resolve(block.ToolUseID)
			if block.IsError {
				s.tail.FailedTools++
			} else {
				s.tail.FailedTools = 0
			}
		}
		if block.Type == "text" && strings.HasPrefix(block.Text, interruptPrefix) {
			s.tail.Interrupted = true
		}
	}
	if !results && prompt {
		s.tail.Prompts++
		s.tail.LastPromptAt = ts
		s.watch = false
		s.tail.FailedTools = 0
	}
}

func (s *tailScan) assistant(rec transcriptLineJSON, ts time.Time) {
	s.lastRole = "assistant"
	s.tail.LastAssistantAt = ts
	s.tail.Interrupted = false
	if rec.Message.StopReason != "" {
		s.lastStop = rec.Message.StopReason
	}
	if rec.Message.ID != s.messageID {
		s.messageID = rec.Message.ID
		s.pending = nil
	}
	for _, block := range rec.Message.Content {
		if block.Type == "tool_use" {
			s.pending = append(s.pending, pendingTool{id: block.ID, name: block.Name})
			if watchTools[block.Name] {
				s.watch = !block.Input.Stop
			}
		}
	}
}

func (s *tailScan) resolve(toolUseID string) {
	for i, p := range s.pending {
		if p.id == toolUseID {
			s.pending = append(s.pending[:i], s.pending[i+1:]...)
			return
		}
	}
}

func (s *tailScan) finish() Tail {
	tail := s.tail
	for _, p := range s.pending {
		tail.PendingTools = append(tail.PendingTools, p.name)
		tail.PendingToolIDs = append(tail.PendingToolIDs, p.id)
	}
	switch s.lastRole {
	case "user":
		tail.Turn = TurnWorking
	case "assistant":
		tail.Turn = s.assistantTurn()
	}
	// A session that armed its own watch and has nothing in flight is
	// waiting on it, whoever spoke last.
	if s.watch && len(s.pending) == 0 && tail.Turn != TurnNeedsInput {
		tail.Turn = TurnWaiting
	}
	return tail
}

func (s *tailScan) assistantTurn() Turn {
	for _, p := range s.pending {
		if p.name == askUserTool {
			return TurnNeedsInput
		}
	}
	if len(s.pending) > 0 || s.lastStop == "tool_use" {
		return TurnWorking
	}
	return TurnAwaitingUser
}
