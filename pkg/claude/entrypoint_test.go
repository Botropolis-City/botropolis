package claude_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
)

func TestScripted(t *testing.T) {
	for _, c := range []struct {
		entrypoint string
		want       bool
	}{
		{"sdk-cli", true},
		{"sdk-ts", true},
		{"sdk-py", true},
		{"cli", false},
		{"", false},
		{"claude-vscode", false},
		{"something-new", false},
	} {
		t.Run(fmt.Sprintf("when the entrypoint is %q", c.entrypoint), func(t *testing.T) {
			t.Run(fmt.Sprintf("it should say scripted is %v", c.want), func(t *testing.T) {
				assert.Equal(t, c.want, claude.Scripted(c.entrypoint))
			})
		})
	}
}

func TestReadSaidEntrypoint(t *testing.T) {
	t.Run("when a transcript names its entrypoint", func(t *testing.T) {
		line := fmt.Sprintf(`{"type":"user","isSidechain":false,"timestamp":"2026-10-16T09:00:00.000Z","sessionId":%q,"cwd":"/src/kiln",`+
			`"entrypoint":"sdk-cli","message":{"role":"user","content":"hi"}}`, sid)
		said, err := claude.ReadSaid(writeTranscript(t, line))
		require.NoError(t, err)

		t.Run("it should keep it with the session", func(t *testing.T) {
			assert.Equal(t, "sdk-cli", said.Entrypoint)
		})
	})
}
