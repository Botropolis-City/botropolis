package helpers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
)

type Home struct {
	t    *testing.T
	Path string
}

func NewHome(t *testing.T) *Home {
	t.Helper()
	return &Home{t: t, Path: t.TempDir()}
}

func (h *Home) ClaudeDir() string {
	return filepath.Join(h.Path, ".claude")
}

func (h *Home) Session(pid int, sessionID, cwd, kind, status string) *Home {
	h.t.Helper()
	record := fmt.Sprintf(`{"pid":%d,"sessionId":%q,"cwd":%q,"startedAt":1789617482873,"version":"2.1.273",`+
		`"kind":%q,"entrypoint":"cli","name":"record name","status":%q,"jobId":"0898d7e4"}`,
		pid, sessionID, cwd, kind, status)
	h.write(filepath.Join("sessions", strconv.Itoa(pid)+".json"), record)
	return h
}

func (h *Home) Transcript(sessionID, cwd string, lines ...string) string {
	h.t.Helper()
	return h.write(filepath.Join("projects", ProjectSlug(cwd), sessionID+".jsonl"), strings.Join(lines, "\n")+"\n")
}

func (h *Home) AppendTranscript(sessionID, cwd string, lines ...string) {
	h.t.Helper()
	h.appendTo(filepath.Join(h.ClaudeDir(), "projects", ProjectSlug(cwd), sessionID+".jsonl"), lines)
}

func (h *Home) appendTo(path string, lines []string) {
	h.t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(h.t, err)
	defer func() { _ = f.Close() }()
	_, err = f.WriteString(strings.Join(lines, "\n") + "\n")
	require.NoError(h.t, err)
}

func (h *Home) StatsCache(content string) *Home {
	h.t.Helper()
	h.write("stats-cache.json", content)
	return h
}

func (h *Home) RemoveSession(pid int) {
	h.t.Helper()
	require.NoError(h.t, os.Remove(filepath.Join(h.ClaudeDir(), "sessions", strconv.Itoa(pid)+".json")))
}

func (h *Home) AppendSubagent(sessionID, cwd, agentID string, lines ...string) {
	h.t.Helper()
	h.appendTo(filepath.Join(h.ClaudeDir(), "projects", ProjectSlug(cwd), sessionID, "subagents", "agent-"+agentID+".jsonl"), lines)
}

func (h *Home) Subagent(sessionID, cwd, agentID, meta string, lines ...string) string {
	h.t.Helper()
	dir := filepath.Join("projects", ProjectSlug(cwd), sessionID, "subagents")
	if meta != "" {
		h.write(filepath.Join(dir, "agent-"+agentID+".meta.json"), meta)
	}
	return h.write(filepath.Join(dir, "agent-"+agentID+".jsonl"), strings.Join(lines, "\n")+"\n")
}

func (h *Home) Roster(jobID, sessionID, ptySock string) *Home {
	h.t.Helper()
	h.write(filepath.Join("daemon", "roster.json"),
		fmt.Sprintf(`{"workers":{%q:{"sessionId":%q,"ptySock":%q}}}`, jobID, sessionID, ptySock))
	return h
}

func (h *Home) write(rel, content string) string {
	h.t.Helper()
	path := filepath.Join(h.ClaudeDir(), rel)
	require.NoError(h.t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(h.t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// ProjectSlug is Claude Code's transcript folder for a working
// directory. The rule lives in pkg/claude now so the loader's skip and
// the fixtures that exercise it cannot disagree about it.
func ProjectSlug(cwd string) string {
	return claude.ProjectFolder(cwd)
}

func UserPrompt(sessionID, cwd, ts string) string {
	return fmt.Sprintf(`{"type":"user","isSidechain":false,"timestamp":%q,"sessionId":%q,"cwd":%q,"gitBranch":"main",`+
		`"version":"2.1.273","entrypoint":"cli","message":{"role":"user","content":"hi"}}`, ts, sessionID, cwd)
}

func AssistantReply(sessionID, id, ts string) string {
	return fmt.Sprintf(`{"type":"assistant","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
		`"message":{"id":%q,"model":"claude-opus-5","role":"assistant","stop_reason":"end_turn",`+
		`"content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":2,"output_tokens":5,"cache_read_input_tokens":250000}}}`,
		ts, sessionID, id)
}

func AssistantToolCall(sessionID, id, ts, toolUseID, name string) string {
	return fmt.Sprintf(`{"type":"assistant","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
		`"message":{"id":%q,"model":"claude-opus-5","role":"assistant","stop_reason":"tool_use",`+
		`"content":[{"type":"tool_use","id":%q,"name":%q,"input":{}}],"usage":{"input_tokens":2,"output_tokens":5}}}`,
		ts, sessionID, id, toolUseID, name)
}

func ModelAttachment(sessionID, modelID, ts string) string {
	return fmt.Sprintf(`{"type":"attachment","isSidechain":false,"timestamp":%q,"sessionId":%q,`+
		`"attachment":{"type":"model","identity":{"modelId":%q}}}`, ts, sessionID, modelID)
}

func AITitle(sessionID, title string) string {
	return fmt.Sprintf(`{"type":"ai-title","aiTitle":%q,"sessionId":%q}`, title, sessionID)
}
