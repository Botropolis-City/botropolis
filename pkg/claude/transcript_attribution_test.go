package claude_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func attributed(id, server, skill string) string {
	line := assistantWithUsage(usageSpec{id: id, ts: "2026-09-16T19:51:00.000Z", in: 1, out: 1})
	extra := ""
	if server != "" {
		extra += `"attributionMcpServer":"` + server + `",`
	}
	if skill != "" {
		extra += `"attributionSkill":"` + skill + `",`
	}
	return `{` + extra + line[1:]
}

const (
	prLine    = `{"type":"pr-link","sessionId":"` + sid + `","prNumber":1181,"prUrl":"https://github.com/mCedar/mullet/pull/1181","prRepository":"mCedar/mullet","timestamp":"2026-09-17T04:26:44.601Z"}`
	prMerged  = `{"type":"user","sessionId":"` + sid + `","timestamp":"2026-09-17T05:00:00.000Z","pr":{"number":1181,"url":"https://github.com/mCedar/mullet/pull/1181","action":"merged"},"message":{"role":"user","content":"<scrubbed>"}}`
	prClosed  = `{"type":"user","sessionId":"` + sid + `","timestamp":"2026-09-17T05:00:00.000Z","pr":{"number":57,"action":"closed"},"message":{"role":"user","content":"<scrubbed>"}}`
	prCreated = `{"type":"user","sessionId":"` + sid + `","timestamp":"2026-09-17T05:00:00.000Z","pr":{"number":61,"url":"https://github.com/mCedar/Cinders/pull/61","action":"created"},"message":{"role":"user","content":"<scrubbed>"}}`
	errLine   = `{"parentUuid":null,"isSidechain":false,"type":"system","subtype":"api_error","level":"error","content":"<scrubbed>","timestamp":"2026-09-17T04:27:00.000Z","sessionId":"` + sid + `"}`
	errMsg    = `{"type":"assistant","isSidechain":false,"isApiErrorMessage":true,"timestamp":"2026-09-17T04:27:01.000Z","sessionId":"` + sid + `","message":{"id":"msg_err","role":"assistant","content":[{"type":"text","text":"<scrubbed>"}]}}`
	compacted = `{"parentUuid":null,"isSidechain":false,"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto","preTokens":190000},"timestamp":"2026-09-17T04:30:00.000Z","sessionId":"` + sid + `"}`
)

func TestTranscriptAttribution(t *testing.T) {
	t.Run("when assistant records attribute MCP servers and skills", func(t *testing.T) {
		transcript := readTranscript(t, userLine,
			attributed("msg_01", "atlassian", ""),
			attributed("msg_02", "atlassian", "amberPylon:umberEstuary"),
			attributed("msg_03", "claude-in-chrome", "my-writing-style"),
			attributed("msg_03", "claude-in-chrome", "my-writing-style"))

		t.Run("it should count calls per MCP server", func(t *testing.T) {
			assert.Equal(t, map[string]int{"atlassian": 2, "claude-in-chrome": 1}, transcript.MCPCalls)
		})

		t.Run("it should count calls per skill", func(t *testing.T) {
			assert.Equal(t, map[string]int{"amberPylon:umberEstuary": 1, "my-writing-style": 1}, transcript.SkillCalls)
		})

		t.Run("it should count a split message once", func(t *testing.T) {
			assert.Equal(t, 1, transcript.MCPCalls["claude-in-chrome"])
		})
	})

	t.Run("when the transcript links a pull request twice", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prLine, prLine)

		t.Run("it should list it once", func(t *testing.T) {
			assert.Equal(t, []claude.PR{{Number: 1181, URL: "https://github.com/mCedar/mullet/pull/1181", Repository: "mCedar/mullet", State: claude.PROpen}}, transcript.PRs)
		})
	})

	t.Run("when the transcript records API errors", func(t *testing.T) {
		transcript := readTranscript(t, userLine, errLine, errMsg)

		t.Run("it should count them", func(t *testing.T) {
			assert.Equal(t, 2, transcript.APIErrors)
		})

		t.Run("it should remember the last one", func(t *testing.T) {
			assert.Equal(t, "2026-09-17T04:27:01Z", transcript.LastErrorAt.Format("2006-01-02T15:04:05Z"))
		})
	})

	t.Run("when the transcript was compacted", func(t *testing.T) {
		transcript := readTranscript(t, userLine, compacted)

		t.Run("it should count the compaction", func(t *testing.T) {
			assert.Equal(t, 1, transcript.Compactions)
		})

		t.Run("it should remember when", func(t *testing.T) {
			assert.Equal(t, "2026-09-17T04:30:00Z", transcript.LastCompactionAt.Format("2006-01-02T15:04:05Z"))
		})
	})

	t.Run("when nothing is attributed", func(t *testing.T) {
		transcript := readTranscript(t, userLine, assistantLine)

		t.Run("it should leave the maps empty", func(t *testing.T) {
			assert.Empty(t, transcript.MCPCalls)
			assert.Empty(t, transcript.SkillCalls)
		})
	})
}

func TestTranscriptPRState(t *testing.T) {
	t.Run("when a linked PR is later merged", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prLine, prMerged)

		t.Run("it should mark it merged", func(t *testing.T) {
			require.Len(t, transcript.PRs, 1)
			assert.True(t, transcript.PRs[0].Merged())
		})
	})

	t.Run("when a PR is only linked", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prLine)

		t.Run("it should be open", func(t *testing.T) {
			require.Len(t, transcript.PRs, 1)
			assert.Equal(t, claude.PROpen, transcript.PRs[0].State)
		})
	})

	t.Run("when a PR is created by action without a link record", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prCreated)

		t.Run("it should be listed open with its repository", func(t *testing.T) {
			require.Len(t, transcript.PRs, 1)
			assert.Equal(t, claude.PR{Number: 61, URL: "https://github.com/mCedar/Cinders/pull/61", Repository: "mCedar/Cinders", State: claude.PROpen}, transcript.PRs[0])
		})
	})

	t.Run("when an unknown PR is closed by number alone", func(t *testing.T) {
		transcript := readTranscript(t, userLine, prClosed)

		t.Run("it should be listed closed", func(t *testing.T) {
			require.Len(t, transcript.PRs, 1)
			assert.Equal(t, claude.PRClosed, transcript.PRs[0].State)
		})
	})
}
