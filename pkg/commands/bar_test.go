package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBar(t *testing.T) {
	asked := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	snapshot := state.Snapshot{Sessions: []state.Session{
		{ID: "a", State: state.NeedsYou, Title: "Fix the CI queue", ContextPercent: 73, LastActivity: asked.Add(4 * time.Minute)},
		{ID: "b", State: state.NeedsYou, Title: "Issue 613", ContextPercent: 12, LastActivity: asked},
		{ID: "c", State: state.Working, Title: "scaffold", ContextPercent: 48},
		{ID: "d", State: state.Resting, Title: "old", ContextPercent: 99},
	}}

	t.Run("when sessions need you", func(t *testing.T) {
		line := commands.Summarize(snapshot, nil, asked)

		t.Run("it should count live states in order of urgency and name who has waited longest", func(t *testing.T) {
			assert.Equal(t, "● 2 need you · 1 working — Issue 613", line.Text)
		})

		t.Run("it should carry the needs-you class", func(t *testing.T) {
			assert.Equal(t, "needs-you", line.Class)
		})

		t.Run("it should list who needs you in the tooltip, longest-waiting first", func(t *testing.T) {
			assert.Equal(t, "needs you:\nIssue 613\nFix the CI queue", line.Tooltip)
		})

		t.Run("it should report the fullest live context as the percentage", func(t *testing.T) {
			assert.Equal(t, 73, line.Percentage)
		})
	})

	t.Run("when two sessions have waited equally long", func(t *testing.T) {
		line := commands.Summarize(state.Snapshot{Sessions: []state.Session{
			{ID: "z", State: state.NeedsYou, Title: "Apples", LastActivity: asked},
			{ID: "m", State: state.NeedsYou, Title: "Zebras", LastActivity: asked},
		}}, nil, asked)

		t.Run("it should name the one whose ID comes first", func(t *testing.T) {
			assert.Equal(t, "● 2 need you — Zebras", line.Text)
		})
	})

	t.Run("when a context percentage overflows", func(t *testing.T) {
		line := commands.Summarize(state.Snapshot{Sessions: []state.Session{{State: state.Working, ContextPercent: 391}}}, nil, asked)

		t.Run("it should clamp the bar percentage to 100", func(t *testing.T) {
			assert.Equal(t, 100, line.Percentage)
		})
	})

	t.Run("when only work is running", func(t *testing.T) {
		line := commands.Summarize(state.Snapshot{Sessions: []state.Session{{State: state.Working}, {State: state.Unattended}}}, nil, asked)

		t.Run("it should say so with the working class", func(t *testing.T) {
			assert.Equal(t, "○ 1 working · 1 unattended", line.Text)
			assert.Equal(t, "working", line.Class)
		})
	})

	t.Run("when nothing is live", func(t *testing.T) {
		line := commands.Summarize(state.Snapshot{Sessions: []state.Session{{State: state.Resting}}}, nil, asked)

		t.Run("it should say no sessions with the idle class", func(t *testing.T) {
			assert.Equal(t, "no sessions", line.Text)
			assert.Equal(t, "idle", line.Class)
		})
	})

	t.Run("when the only session is empty", func(t *testing.T) {
		line := commands.Summarize(state.Snapshot{Sessions: []state.Session{{State: state.Empty, Title: "nothing yet"}}}, nil, asked)

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
			assert.Equal(t, "● 2 need you · 1 working — Issue 613\n", out.String())
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
			assert.Equal(t, "● 2 need you · 1 working — Issue 613\nno sessions\n", out.String())
		})
	})

	t.Run("when one session needs you", func(t *testing.T) {
		line := commands.Summarize(state.Snapshot{Sessions: []state.Session{{ID: "a", State: state.NeedsYou, Title: "Alone", LastActivity: asked}}}, nil, asked)

		t.Run("it should say it needs you in words", func(t *testing.T) {
			assert.Equal(t, "● 1 needs you — Alone", line.Text)
		})
	})

	t.Run("when the longest-waiting session is marked done and has not moved since", func(t *testing.T) {
		line := commands.Summarize(snapshot, &city.Layout{Done: map[string]time.Time{"b": asked}}, asked.Add(time.Hour))

		t.Run("it should name the next one and count only the sessions still in the walk", func(t *testing.T) {
			assert.Equal(t, "● 1 needs you · 1 working · 1 set aside — Fix the CI queue", line.Text)
		})

		t.Run("it should leave it out of the tooltip", func(t *testing.T) {
			assert.Equal(t, "needs you:\nFix the CI queue", line.Tooltip)
		})
	})

	t.Run("when the longest-waiting session is snoozed", func(t *testing.T) {
		layout := &city.Layout{Snoozed: map[string]time.Time{"b": asked.Add(time.Hour)}}

		t.Run("and the snooze has not run out", func(t *testing.T) {
			line := commands.Summarize(snapshot, layout, asked.Add(time.Minute))

			t.Run("it should name the next one", func(t *testing.T) {
				assert.Equal(t, "● 1 needs you · 1 working · 1 set aside — Fix the CI queue", line.Text)
			})
		})

		t.Run("and the snooze has run out", func(t *testing.T) {
			line := commands.Summarize(snapshot, layout, asked.Add(time.Hour))

			t.Run("it should name it again", func(t *testing.T) {
				assert.Equal(t, "● 2 need you · 1 working — Issue 613", line.Text)
			})
		})
	})

	t.Run("when every needs-you session is set aside", func(t *testing.T) {
		layout := &city.Layout{Done: map[string]time.Time{"a": asked.Add(4 * time.Minute), "b": asked}}

		t.Run("and work is running", func(t *testing.T) {
			line := commands.Summarize(snapshot, layout, asked.Add(time.Hour))

			t.Run("it should carry the working class", func(t *testing.T) {
				assert.Equal(t, "working", line.Class)
			})
		})

		t.Run("and nothing else is live", func(t *testing.T) {
			line := commands.Summarize(state.Snapshot{Sessions: snapshot.Sessions[:2]}, layout, asked.Add(time.Hour))

			t.Run("it should read as that many sessions set aside", func(t *testing.T) {
				assert.Equal(t, commands.BarLine{Text: "2 set aside", Class: "idle", Percentage: 73}, line)
			})
		})
	})

	t.Run("when watching a feed and the layout changes between snapshots", func(t *testing.T) {
		var out bytes.Buffer
		layouts := []*city.Layout{nil, {Done: map[string]time.Time{"b": asked}}}
		loads := 0
		load := func() *city.Layout {
			l := layouts[loads]
			loads++
			return l
		}
		feed := func(_ context.Context, offer func(state.Snapshot)) error {
			offer(snapshot)
			offer(snapshot)
			return nil
		}
		_ = commands.NewBar(&fakeSource{}, feed).WithTriage(load, func() time.Time { return asked.Add(time.Hour) }).Watch(context.Background(), &out, commands.BarText)

		t.Run("it should name the next session on the next line", func(t *testing.T) {
			assert.Equal(t, "● 2 need you · 1 working — Issue 613\n● 1 needs you · 1 working · 1 set aside — Fix the CI queue\n", out.String())
		})
	})

	t.Run("when printed once as text with a layout", func(t *testing.T) {
		var out bytes.Buffer
		load := func() *city.Layout { return &city.Layout{Done: map[string]time.Time{"b": asked}} }
		require.NoError(t, commands.NewBar(&fakeSource{snapshot: snapshot}, nil).WithTriage(load, func() time.Time { return asked }).Once(&out, commands.BarText, true))

		t.Run("it should leave the set-aside session out", func(t *testing.T) {
			assert.Equal(t, "● 1 needs you · 1 working · 1 set aside — Fix the CI queue\n", out.String())
		})
	})
}
