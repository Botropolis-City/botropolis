package claude_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadTranscriptFromFixture(t *testing.T) {
	t.Run("when reading every transcript in the sample fixture", func(t *testing.T) {
		home := helpers.FixtureHome(t, "sample")

		for _, session := range helpers.Manifest(t, "sample").Sessions {
			t.Run("and the transcript is "+session.SessionID, func(t *testing.T) {
				path := filepath.Join(home, ".claude", "projects", session.Project, session.SessionID+".jsonl")
				transcript, err := claude.ReadTranscript(path)
				require.NoError(t, err)

				t.Run("it should agree with the manifest about being a bridge stub", func(t *testing.T) {
					assert.Equal(t, session.BridgeStub, transcript.IsBridgeStub)
				})

				t.Run("it should read a session id equal to the file stem", func(t *testing.T) {
					assert.Equal(t, session.SessionID, transcript.SessionID)
				})

				t.Run("it should parse every line", func(t *testing.T) {
					assert.Zero(t, transcript.Malformed)
				})

				if !session.BridgeStub {
					t.Run("it should read a cwd", func(t *testing.T) {
						assert.NotEmpty(t, transcript.CWD)
					})

					t.Run("it should count one message per distinct api message id", func(t *testing.T) {
						assert.Equal(t, countDistinctMessageIDs(t, path), transcript.Usage.Messages)
					})

					// A session with no assistant records at all is a real shape, not a
					// broken fixture: prompts typed into a session that never ran, or one
					// started with --bg and never picked up. The sample carries one. The
					// invariant that holds for every transcript is the one above — the
					// count matches the file — and it reads zero here, correctly. The model
					// comes from the same records, so it is empty for the same reason.
					// Asserting either for every transcript asserted the fixture's
					// composition rather than the reader's behaviour, and it went unnoticed
					// until the fixture was regenerated on a day with two such sessions in
					// it. The sample has two: one with three prompts and no reply, one that
					// only ever queued.
					if countDistinctMessageIDs(t, path) > 0 {
						t.Run("it should read a model", func(t *testing.T) {
							assert.NotEmpty(t, transcript.Model)
						})

						t.Run("it should count at least one api message", func(t *testing.T) {
							assert.Positive(t, transcript.Usage.Messages)
						})

						t.Run("it should report a context size", func(t *testing.T) {
							assert.Positive(t, transcript.ContextTokens)
						})
					}

					t.Run("it should report a last timestamp no earlier than the first", func(t *testing.T) {
						assert.False(t, transcript.LastAt.Before(transcript.FirstAt))
					})

					t.Run("it should know whose turn it is", func(t *testing.T) {
						assert.NotEqual(t, claude.TurnUnknown, transcript.Tail.Turn)
					})

					t.Run("it should count at least one prompt", func(t *testing.T) {
						assert.Positive(t, transcript.Tail.Prompts)
					})

					t.Run("it should have heard from the assistant after the last prompt or be working", func(t *testing.T) {
						if transcript.Tail.Turn != claude.TurnWorking {
							assert.False(t, transcript.Tail.LastAssistantAt.Before(transcript.Tail.LastPromptAt))
						}
					})
				}
			})
		}
	})
}

func countDistinctMessageIDs(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, 64<<20)
	for scanner.Scan() {
		var rec struct {
			Type    string `json:"type"`
			Message struct {
				ID    string          `json:"id"`
				Usage json.RawMessage `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(scanner.Bytes(), &rec) != nil || rec.Type != "assistant" || len(rec.Message.Usage) == 0 {
			continue
		}
		seen[rec.Message.ID] = true
	}
	require.NoError(t, scanner.Err())
	return len(seen)
}
