package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBar(t *testing.T) {
	snapshot := state.Snapshot{Sessions: []state.Session{
		{ID: "a", State: state.NeedsYou, Title: "Fix the CI queue", ContextPercent: 73},
		{ID: "b", State: state.NeedsYou, Title: "Issue 613", ContextPercent: 12},
		{ID: "c", State: state.Working, Title: "scaffold", ContextPercent: 48},
		{ID: "d", State: state.Resting, Title: "old", ContextPercent: 99},
	}}

	t.Run("when sessions need you", func(t *testing.T) {
		line := commands.Summarize(snapshot)

		t.Run("it should count live states in order of urgency and name who is first", func(t *testing.T) {
			assert.Equal(t, "● 2 needs-you · 1 working — Fix the CI queue", line.Text)
		})

		t.Run("it should carry the needs-you class", func(t *testing.T) {
			assert.Equal(t, "needs-you", line.Class)
		})

		t.Run("it should list who needs you in the tooltip, sorted", func(t *testing.T) {
			assert.Equal(t, "needs you:\nFix the CI queue\nIssue 613", line.Tooltip)
		})

		t.Run("it should report the fullest live context as the percentage", func(t *testing.T) {
			assert.Equal(t, 73, line.Percentage)
		})
	})

	t.Run("when a context percentage overflows", func(t *testing.T) {
		line := commands.Summarize(state.Snapshot{Sessions: []state.Session{{State: state.Working, ContextPercent: 391}}})

		t.Run("it should clamp the bar percentage to 100", func(t *testing.T) {
			assert.Equal(t, 100, line.Percentage)
		})
	})

	t.Run("when only work is running", func(t *testing.T) {
		line := commands.Summarize(state.Snapshot{Sessions: []state.Session{{State: state.Working}, {State: state.Unattended}}})

		t.Run("it should say so with the working class", func(t *testing.T) {
			assert.Equal(t, "○ 1 working · 1 unattended", line.Text)
			assert.Equal(t, "working", line.Class)
		})
	})

	t.Run("when nothing is live", func(t *testing.T) {
		line := commands.Summarize(state.Snapshot{Sessions: []state.Session{{State: state.Resting}}})

		t.Run("it should say no sessions with the idle class", func(t *testing.T) {
			assert.Equal(t, "no sessions", line.Text)
			assert.Equal(t, "idle", line.Class)
		})
	})

	t.Run("when the only session is empty", func(t *testing.T) {
		line := commands.Summarize(state.Snapshot{Sessions: []state.Session{{State: state.Empty, Title: "nothing yet"}}})

		t.Run("it should say no sessions with the idle class", func(t *testing.T) {
			assert.Equal(t, "no sessions", line.Text)
			assert.Equal(t, "idle", line.Class)
		})
	})

	t.Run("when printed once for waybar", func(t *testing.T) {
		var out bytes.Buffer
		require.NoError(t, commands.NewBar(&fakeSource{snapshot: snapshot}, nil).Once(&out, commands.BarWaybar, true))

		t.Run("it should print one JSON object per line", func(t *testing.T) {
			var line commands.BarLine
			require.NoError(t, json.Unmarshal(out.Bytes(), &line))
			assert.Equal(t, "needs-you", line.Class)
			assert.Equal(t, 1, bytes.Count(out.Bytes(), []byte("\n")))
		})
	})

	t.Run("when printed once as text", func(t *testing.T) {
		var out bytes.Buffer
		require.NoError(t, commands.NewBar(&fakeSource{snapshot: snapshot}, nil).Once(&out, commands.BarText, true))

		t.Run("it should print the text line only", func(t *testing.T) {
			assert.Equal(t, "● 2 needs-you · 1 working — Fix the CI queue\n", out.String())
		})
	})

	t.Run("when watching a feed", func(t *testing.T) {
		var out bytes.Buffer
		feed := func(_ context.Context, offer func(state.Snapshot)) error {
			offer(snapshot)
			offer(state.Snapshot{})
			return nil
		}
		_ = commands.NewBar(&fakeSource{}, feed).Watch(context.Background(), &out, commands.BarText)

		t.Run("it should print a line per snapshot", func(t *testing.T) {
			assert.Equal(t, "● 2 needs-you · 1 working — Fix the CI queue\nno sessions\n", out.String())
		})
	})
}
