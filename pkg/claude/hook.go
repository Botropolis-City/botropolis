package claude

import (
	"encoding/json"
	"errors"
)

type HookName string

const (
	HookSessionStart     HookName = "SessionStart"
	HookSessionEnd       HookName = "SessionEnd"
	HookUserPromptSubmit HookName = "UserPromptSubmit"
	HookPreToolUse       HookName = "PreToolUse"
	HookPostToolUse      HookName = "PostToolUse"
	HookNotification     HookName = "Notification"
	HookStop             HookName = "Stop"
	HookSubagentStart    HookName = "SubagentStart"
	HookSubagentStop     HookName = "SubagentStop"
)

// NotificationIdlePrompt is the notification Claude Code sends when a
// turn has ended and the prompt has sat idle (item 250).
const NotificationIdlePrompt = "idle_prompt"

type HookEvent struct {
	Name             HookName `json:"hook_event_name"`
	SessionID        string   `json:"session_id"`
	TranscriptPath   string   `json:"transcript_path"`
	CWD              string   `json:"cwd"`
	ToolName         string   `json:"tool_name"`
	ToolUseID        string   `json:"tool_use_id"`
	NotificationType string   `json:"notification_type"`
	Message          string   `json:"message"`
	AgentID          string   `json:"agent_id"`
	AgentType        string   `json:"agent_type"`
	Reason           string   `json:"reason"`
	Source           string   `json:"source"`
	ToolInput        struct {
		FilePath     string `json:"file_path"`
		Path         string `json:"path"`
		NotebookPath string `json:"notebook_path"`
		Command      string `json:"command"`
		Pattern      string `json:"pattern"`
	} `json:"tool_input"`
}

// Subject is the thing the tool is working on: a file path when there is
// one, otherwise the command or pattern, or nothing.
func (e HookEvent) Subject() string {
	switch {
	case e.ToolInput.FilePath != "":
		return e.ToolInput.FilePath
	case e.ToolInput.NotebookPath != "":
		return e.ToolInput.NotebookPath
	case e.ToolInput.Path != "":
		return e.ToolInput.Path
	case e.ToolInput.Command != "":
		return e.ToolInput.Command
	case e.ToolInput.Pattern != "":
		return e.ToolInput.Pattern
	}
	return ""
}

var (
	errHookNoName    = errors.New("hook event has no hook_event_name")
	errHookNoSession = errors.New("hook event has no session_id")
)

func ParseHookEvent(raw []byte) (HookEvent, error) {
	var event HookEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return HookEvent{}, err
	}
	if event.Name == "" {
		return HookEvent{}, errHookNoName
	}
	if event.SessionID == "" {
		return HookEvent{}, errHookNoSession
	}
	return event, nil
}
