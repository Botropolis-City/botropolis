package claude_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The synthetic home is written by hand to hold every shape the readers
// must handle, so the suite no longer depends on what this machine's
// Claude Code last wrote. Item 92.
func TestSyntheticFixtureShapes(t *testing.T) {
	t.Run("when reading the synthetic fixture's manifest", func(t *testing.T) {
		manifest := helpers.Manifest(t, "synthetic")
		count := func(match func(s helpers.FixtureSession) bool) int {
			n := 0
			for _, s := range manifest.Sessions {
				if match(s) {
					n++
				}
			}
			return n
		}

		shapes := []struct {
			name  string
			match func(s helpers.FixtureSession) bool
		}{
			{"a conversation", func(s helpers.FixtureSession) bool { return !s.BridgeStub && !s.TitleOnly }},
			{"a bridge stub", func(s helpers.FixtureSession) bool { return s.BridgeStub }},
			{"a session with subagents", func(s helpers.FixtureSession) bool { return s.SubagentFiles > 0 }},
			{"a title-only transcript", func(s helpers.FixtureSession) bool { return s.TitleOnly }},
		}
		for _, shape := range shapes {
			t.Run("it should hold "+shape.name, func(t *testing.T) {
				assert.Positive(t, count(shape.match))
			})
		}
	})

	t.Run("when loading the synthetic home into a city", func(t *testing.T) {
		home := helpers.LiveFixtureHome(t, "synthetic")
		snapshot, err := state.Load(home, state.Probes{Alive: state.ProcessAlive, Attached: state.UnixSocketConnected}, time.Now())
		require.NoError(t, err)

		t.Run("it should draw a road between two repositories", func(t *testing.T) {
			assert.NotEmpty(t, snapshot.Roads)
		})

		t.Run("it should skip nothing", func(t *testing.T) {
			assert.Empty(t, snapshot.Skipped)
		})
	})
}
