package claude_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/claude"
)

// Item 89: a message from another session is recorded in the
// recipient's transcript, once, with who sent it.
func TestPeerMessages(t *testing.T) {
	peer := func(id string) string {
		return `{"type":"user","timestamp":"2026-10-17T12:00:00.000Z","turnOrigin":"peer","promptSource":"system","origin":{"kind":"peer","from":"uds:/run/x/42.sock","verifiedPeerPid":42,"name":"roadmap manager","msg_id":"` + id + `"},"message":{"role":"user","content":"<cross-session-message from=\"uds:/run/x/42.sock\">hi</cross-session-message>"}}`
	}

	t.Run("when a user line carries a peer origin", func(t *testing.T) {
		tr := readTranscript(t, peer("m1"))

		t.Run("it should record its id, the sender's pid and name, and when", func(t *testing.T) {
			assert.Equal(t, []claude.PeerMessage{{
				ID: "m1", FromPID: 42, FromName: "roadmap manager", At: time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC),
			}}, tr.PeerMessages)
		})
	})

	t.Run("when the same message id appears twice", func(t *testing.T) {
		tr := readTranscript(t, peer("m1"), peer("m1"))

		t.Run("it should record it once", func(t *testing.T) {
			assert.Len(t, tr.PeerMessages, 1)
		})
	})

	t.Run("when a user line is an ordinary prompt", func(t *testing.T) {
		tr := readTranscript(t, `{"type":"user","timestamp":"2026-10-17T12:00:00.000Z","message":{"role":"user","content":"hello"}}`)

		t.Run("it should record nothing", func(t *testing.T) {
			assert.Empty(t, tr.PeerMessages)
		})
	})

	t.Run("when a session sends with SendMessage", func(t *testing.T) {
		tr := readTranscript(t, `{"type":"assistant","timestamp":"2026-10-17T12:00:00.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"SendMessage","input":{"to":"roadmap manager","message":"hi"}}]}}`)

		t.Run("it should record nothing on the sender's side", func(t *testing.T) {
			assert.Empty(t, tr.PeerMessages)
		})
	})
}
