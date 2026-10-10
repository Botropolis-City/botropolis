package render

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 73. Generating a summary is a `claude -p` run of a few seconds,
// so it happens off the frame, one at a time per session, and its
// result is handed back to the frame loop rather than written into the
// scene from another goroutine.

func TestGenerateSummary(t *testing.T) {
	at := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	restore := timeNow
	timeNow = func() time.Time { return at }
	t.Cleanup(func() { timeNow = restore })

	game := func(answer func(ctx context.Context, id string) (string, error)) *Game {
		return &Game{scene: city.NewScene(city.NewLayout()), summarise: answer}
	}
	settled := func(t *testing.T, g *Game) []generatedResult {
		t.Helper()
		var got []generatedResult
		require.Eventually(t, func() bool {
			got = g.takeGenerated()
			return len(got) > 0
		}, time.Second, time.Millisecond)
		return got
	}

	t.Run("when Generate is pressed twice while the first run is out", func(t *testing.T) {
		release := make(chan struct{})
		g := game(func(context.Context, string) (string, error) { <-release; return "s", nil })
		first := g.generate("a")
		second := g.generate("a")
		close(release)
		settled(t, g)

		t.Run("it should start the first", func(t *testing.T) {
			assert.True(t, first)
		})

		t.Run("it should refuse the second", func(t *testing.T) {
			assert.False(t, second)
		})
	})

	t.Run("when the run answers", func(t *testing.T) {
		g := game(func(_ context.Context, id string) (string, error) { return "summary of " + id, nil })
		g.generate("a")
		got := settled(t, g)

		t.Run("it should hand back the text, stamped, for that session", func(t *testing.T) {
			assert.Equal(t, []generatedResult{{id: "a", generated: city.Generated{Text: "summary of a", At: at}}}, got)
		})

		t.Run("it should let the next one start", func(t *testing.T) {
			assert.True(t, g.generate("a"))
			settled(t, g)
		})
	})

	t.Run("when the run fails", func(t *testing.T) {
		g := game(func(context.Context, string) (string, error) { return "", errors.New("claude: not logged in") })
		g.generate("a")
		got := settled(t, g)

		t.Run("it should hand back why", func(t *testing.T) {
			assert.Equal(t, []generatedResult{{id: "a", generated: city.Generated{Err: "claude: not logged in"}}}, got)
		})
	})
}
