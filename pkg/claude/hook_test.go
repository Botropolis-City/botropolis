package claude_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHookEvent(t *testing.T) {
	t.Run("when the payload is a PreToolUse event", func(t *testing.T) {
		event, err := claude.ParseHookEvent([]byte(`{"session_id":"` + sid + `","transcript_path":"/home/avesta/.claude/projects/x/` + sid + `.jsonl",` +
			`"cwd":"/home/avesta/workspaces/github/mCedar/cinders","permission_mode":"auto","hook_event_name":"PreToolUse",` +
			`"tool_name":"Bash","tool_input":{"command":"make test"},"tool_use_id":"toolu_01"}`))
		require.NoError(t, err)

		t.Run("it should read the event name", func(t *testing.T) {
			assert.Equal(t, claude.HookPreToolUse, event.Name)
		})

		t.Run("it should read the session id", func(t *testing.T) {
			assert.Equal(t, sid, event.SessionID)
		})

		t.Run("it should read the tool name", func(t *testing.T) {
			assert.Equal(t, "Bash", event.ToolName)
		})

		t.Run("it should read the tool use id", func(t *testing.T) {
			assert.Equal(t, "toolu_01", event.ToolUseID)
		})

		t.Run("it should name the command as the subject", func(t *testing.T) {
			assert.Equal(t, "make test", event.Subject())
		})

		t.Run("it should read the transcript path", func(t *testing.T) {
			assert.Equal(t, "/home/avesta/.claude/projects/x/"+sid+".jsonl", event.TranscriptPath)
		})
	})

	t.Run("when the tool input names a file", func(t *testing.T) {
		event, err := claude.ParseHookEvent([]byte(`{"session_id":"` + sid + `","hook_event_name":"PreToolUse","tool_name":"Edit",` +
			`"tool_input":{"file_path":"/home/avesta/workspaces/github/auroq/botropolis/pkg/city/city.go","old_string":"a","new_string":"b"}}`))
		require.NoError(t, err)

		t.Run("it should make the file the subject", func(t *testing.T) {
			assert.Equal(t, "/home/avesta/workspaces/github/auroq/botropolis/pkg/city/city.go", event.Subject())
		})
	})

	t.Run("when the payload is a Notification event", func(t *testing.T) {
		event, err := claude.ParseHookEvent([]byte(`{"session_id":"` + sid + `","hook_event_name":"Notification",` +
			`"message":"Claude needs your permission to use Bash","notification_type":"permission_prompt"}`))
		require.NoError(t, err)

		t.Run("it should read the notification type", func(t *testing.T) {
			assert.Equal(t, "permission_prompt", event.NotificationType)
		})

		t.Run("it should read the message", func(t *testing.T) {
			assert.Equal(t, "Claude needs your permission to use Bash", event.Message)
		})
	})

	t.Run("when the payload has no event name", func(t *testing.T) {
		_, err := claude.ParseHookEvent([]byte(`{"session_id":"` + sid + `"}`))

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})
	})

	t.Run("when the payload has no session id", func(t *testing.T) {
		_, err := claude.ParseHookEvent([]byte(`{"hook_event_name":"Stop"}`))

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})
	})

	t.Run("when the payload is not JSON", func(t *testing.T) {
		_, err := claude.ParseHookEvent([]byte(`nope`))

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}
