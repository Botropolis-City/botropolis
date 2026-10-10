package claude_test

import (
	"path/filepath"
	"strings"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sid = "08b1d655-276f-495b-a61f-2214af35a97a"

	modeLine       = `{"type":"mode","mode":"normal","sessionId":"` + sid + `"}`
	permissionLine = `{"type":"permission-mode","permissionMode":"auto","sessionId":"` + sid + `"}`
	bridgeLine     = `{"type":"bridge-session","sessionId":"` + sid + `","bridgeSessionId":"<scrubbed>",` +
		`"lastSequenceNum":0,"ownerAccountUuid":"<scrubbed>","ownerOrganizationUuid":"<scrubbed>"}`
	userLine = `{"parentUuid":null,"isSidechain":false,"type":"user",` +
		`"message":{"role":"user","content":"<scrubbed>"},"uuid":"850fabd7-9367-4cbf-b301-4cc67e0e13e3",` +
		`"timestamp":"2026-09-16T19:50:32.231Z","userType":"external","entrypoint":"cli",` +
		`"cwd":"/home/avesta/workspaces/github/mCedar/cinders","sessionId":"` + sid + `",` +
		`"version":"2.1.273","gitBranch":"main"}`
	assistantLine = `{"parentUuid":"850fabd7-9367-4cbf-b301-4cc67e0e13e3","isSidechain":false,"type":"assistant",` +
		`"message":{"model":"claude-opus-5","id":"msg_01","type":"message","role":"assistant",` +
		`"content":[{"type":"text","text":"<scrubbed>"}],"stop_reason":"end_turn",` +
		`"usage":{"input_tokens":3,"output_tokens":7}},"effort":"high",` +
		`"uuid":"66f4b3fa-c590-4e8f-bc50-012682fa1d93","timestamp":"2026-09-16T19:50:40.000Z",` +
		`"cwd":"/home/avesta/workspaces/github/mCedar/cinders","sessionId":"` + sid + `",` +
		`"version":"2.1.273","gitBranch":"main"}`
	aiTitleLine = `{"type":"ai-title","aiTitle":"Fix the CI queue","sessionId":"` + sid + `"}`
	modelLine   = `{"parentUuid":"850fabd7-9367-4cbf-b301-4cc67e0e13e3","isSidechain":false,"type":"attachment",` +
		`"attachment":{"type":"model","identity":{"modelId":"claude-opus-5[1m]","marketingName":"<scrubbed>",` +
		`"knowledgeCutoff":"<scrubbed>"},"text":"<scrubbed>"},"uuid":"4f596ac0-3825-4f8e-9bf3-ce89f367d32c",` +
		`"timestamp":"2026-09-16T19:50:32.300Z","sessionId":"` + sid + `"}`
)

func customTitle(title string) string {
	return `{"type":"custom-title","customTitle":"` + title + `","sessionId":"` + sid + `"}`
}

func withBranch(branch string) string {
	return strings.Replace(userLine, `"gitBranch":"main"`, `"gitBranch":"`+branch+`"`, 1)
}

func withModel(model string) string {
	return strings.Replace(assistantLine, `"model":"claude-opus-5"`, `"model":"`+model+`"`, 1)
}

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	return writeFile(t, t.TempDir(), sid+".jsonl", strings.Join(lines, "\n")+"\n")
}

func readTranscript(t *testing.T, lines ...string) claude.Transcript {
	t.Helper()
	transcript, err := claude.ReadTranscript(writeTranscript(t, lines...))
	require.NoError(t, err)
	return transcript
}

func TestReadTranscript(t *testing.T) {
	t.Run("when the file holds a typical transcript head", func(t *testing.T) {
		path := writeTranscript(t, modeLine, permissionLine, bridgeLine, userLine, modelLine, assistantLine, aiTitleLine)
		transcript, err := claude.ReadTranscript(path)
		require.NoError(t, err)

		t.Run("it should read the session id", func(t *testing.T) {
			assert.Equal(t, sid, transcript.SessionID)
		})

		t.Run("it should read the cwd", func(t *testing.T) {
			assert.Equal(t, "/home/avesta/workspaces/github/mCedar/cinders", transcript.CWD)
		})

		t.Run("it should read the branch", func(t *testing.T) {
			assert.Equal(t, "main", transcript.Branch)
		})

		t.Run("it should read the version", func(t *testing.T) {
			assert.Equal(t, "2.1.273", transcript.Version)
		})

		t.Run("it should read the entrypoint", func(t *testing.T) {
			assert.Equal(t, "cli", transcript.Entrypoint)
		})

		t.Run("it should read the model from the assistant message", func(t *testing.T) {
			assert.Equal(t, "claude-opus-5", transcript.Model)
		})

		t.Run("it should read the full model id from the model attachment", func(t *testing.T) {
			assert.Equal(t, "claude-opus-5[1m]", transcript.ModelID)
		})

		t.Run("it should read the effort from the assistant record", func(t *testing.T) {
			assert.Equal(t, "high", transcript.Effort)
		})

		t.Run("it should use the ai title", func(t *testing.T) {
			assert.Equal(t, "Fix the CI queue", transcript.Title)
		})

		t.Run("it should not flag it as a bridge stub", func(t *testing.T) {
			assert.False(t, transcript.IsBridgeStub)
		})

		t.Run("it should record the path", func(t *testing.T) {
			assert.Equal(t, path, transcript.Path)
		})
	})

	t.Run("when the file holds both an ai title and a custom title", func(t *testing.T) {
		t.Run("and the custom title comes first", func(t *testing.T) {
			transcript := readTranscript(t, userLine, customTitle("Queue work"), aiTitleLine)

			t.Run("it should prefer the custom title", func(t *testing.T) {
				assert.Equal(t, "Queue work", transcript.Title)
			})
		})

		t.Run("and the ai title comes first", func(t *testing.T) {
			transcript := readTranscript(t, userLine, aiTitleLine, customTitle("Queue work"))

			t.Run("it should prefer the custom title", func(t *testing.T) {
				assert.Equal(t, "Queue work", transcript.Title)
			})
		})
	})

	t.Run("when the file holds only a summary record", func(t *testing.T) {
		transcript := readTranscript(t, userLine,
			`{"type":"summary","summary":"Queue summary","leafUuid":"66f4b3fa-c590-4e8f-bc50-012682fa1d93"}`)

		t.Run("it should use the summary as the title", func(t *testing.T) {
			assert.Equal(t, "Queue summary", transcript.Title)
		})
	})

	t.Run("when the file holds two custom titles", func(t *testing.T) {
		transcript := readTranscript(t, userLine, customTitle("First name"), customTitle("Second name"))

		t.Run("it should use the latest one", func(t *testing.T) {
			assert.Equal(t, "Second name", transcript.Title)
		})
	})

	t.Run("when the file has no title record", func(t *testing.T) {
		transcript := readTranscript(t, userLine, assistantLine)

		t.Run("it should leave the title empty", func(t *testing.T) {
			assert.Empty(t, transcript.Title)
		})
	})

	t.Run("when the branch changes mid-session", func(t *testing.T) {
		transcript := readTranscript(t, userLine, withBranch("feature/queue"))

		t.Run("it should report the latest branch", func(t *testing.T) {
			assert.Equal(t, "feature/queue", transcript.Branch)
		})
	})

	t.Run("when a record reports the branch as HEAD", func(t *testing.T) {
		transcript := readTranscript(t, userLine, withBranch("HEAD"))

		t.Run("it should keep the previous branch", func(t *testing.T) {
			assert.Equal(t, "main", transcript.Branch)
		})
	})

	t.Run("when the model changes mid-session", func(t *testing.T) {
		transcript := readTranscript(t, userLine, assistantLine, withModel("claude-sonnet-5"))

		t.Run("it should report the latest model", func(t *testing.T) {
			assert.Equal(t, "claude-sonnet-5", transcript.Model)
		})
	})

	t.Run("when the file is a single bridge-session line", func(t *testing.T) {
		transcript := readTranscript(t, bridgeLine)

		t.Run("it should flag it as a bridge stub", func(t *testing.T) {
			assert.True(t, transcript.IsBridgeStub)
		})

		t.Run("it should still read the session id", func(t *testing.T) {
			assert.Equal(t, sid, transcript.SessionID)
		})
	})

	t.Run("when the file is empty", func(t *testing.T) {
		transcript, err := claude.ReadTranscript(writeFile(t, t.TempDir(), sid+".jsonl", ""))

		t.Run("it should not return an error", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should not flag it as a bridge stub", func(t *testing.T) {
			assert.False(t, transcript.IsBridgeStub)
		})
	})

	t.Run("when one line is not valid JSON", func(t *testing.T) {
		transcript := readTranscript(t, userLine, `{"type":"assistant","message":{"model":"claude-opus-5"`, aiTitleLine)

		t.Run("it should still read the fields from the other lines", func(t *testing.T) {
			assert.Equal(t, "Fix the CI queue", transcript.Title)
		})

		t.Run("it should count one malformed line", func(t *testing.T) {
			assert.Equal(t, 1, transcript.Malformed)
		})
	})

	t.Run("when a record is padded with NUL bytes by a torn write", func(t *testing.T) {
		transcript := readTranscript(t, userLine, "\x00\x00\x00\x00"+assistantLine)

		t.Run("it should still read the record", func(t *testing.T) {
			assert.Equal(t, 1, transcript.Usage.Messages)
		})

		t.Run("it should count nothing malformed", func(t *testing.T) {
			assert.Zero(t, transcript.Malformed)
		})
	})

	t.Run("when the file does not exist", func(t *testing.T) {
		_, err := claude.ReadTranscript(filepath.Join(t.TempDir(), "missing.jsonl"))

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}
