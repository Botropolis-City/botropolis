package claude_test

import (
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadTranscriptsFromFixture(t *testing.T) {
	t.Run("when walking the sample fixture's projects directory", func(t *testing.T) {
		projects := filepath.Join(helpers.FixtureHome(t, "sample"), ".claude", "projects")
		manifest := helpers.Manifest(t, "sample")

		transcripts, skipped, err := claude.ReadTranscripts(projects)
		require.NoError(t, err)

		t.Run("it should return one transcript per manifest session", func(t *testing.T) {
			assert.Len(t, transcripts, len(manifest.Sessions))
		})

		t.Run("it should flag exactly the manifest's bridge stubs", func(t *testing.T) {
			var want, got []string
			for _, session := range manifest.Sessions {
				if session.BridgeStub {
					want = append(want, session.SessionID)
				}
			}
			for _, transcript := range transcripts {
				if transcript.IsBridgeStub {
					got = append(got, transcript.SessionID)
				}
			}
			assert.ElementsMatch(t, want, got)
		})

		t.Run("it should skip nothing", func(t *testing.T) {
			assert.Empty(t, skipped)
		})
	})
}
