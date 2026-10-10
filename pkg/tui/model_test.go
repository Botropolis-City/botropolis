package tui_test

import (
	"strings"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var now = time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC)

func session(id string, st state.State, last time.Duration) state.Session {
	return state.Session{ID: id, Title: "Job " + id, CWD: "/home/avesta/workspaces/github/mCedar/cinders", State: st,
		ContextPercent: 25, LastActivity: now.Add(-last), StartedAt: now.Add(-time.Hour)}
}

func model(sessions ...state.Session) *tui.Model {
	m := tui.New()
	m.SetSnapshot(state.Snapshot{At: now, Sessions: sessions})
	return m
}

func TestModel(t *testing.T) {
	t.Run("when a snapshot with mixed states arrives", func(t *testing.T) {
		m := model(session("p", state.Parked, time.Hour), session("w", state.Working, 2*time.Minute),
			session("n1", state.NeedsYou, 10*time.Minute), session("n2", state.NeedsYou, time.Minute))

		t.Run("it should hide parked sessions by default", func(t *testing.T) {
			assert.Len(t, m.Rows(), 3)
		})

		t.Run("it should order by urgency then recency", func(t *testing.T) {
			ids := []string{}
			for _, r := range m.Rows() {
				ids = append(ids, r.ID)
			}
			assert.Equal(t, []string{"n2", "n1", "w"}, ids)
		})

		t.Run("it should start with the cursor on the first row", func(t *testing.T) {
			s, ok := m.Selected()
			require.True(t, ok)
			assert.Equal(t, "n2", s.ID)
		})

		t.Run("and parked sessions are toggled on", func(t *testing.T) {
			m.Key("a", now)

			t.Run("it should show them last", func(t *testing.T) {
				require.Len(t, m.Rows(), 4)
				assert.Equal(t, "p", m.Rows()[3].ID)
			})
		})
	})

	t.Run("when the cursor moves", func(t *testing.T) {
		m := model(session("a", state.Working, 0), session("b", state.Working, time.Minute), session("c", state.Working, 2*time.Minute))
		m.Key("j", now)
		m.Key("j", now)
		m.Key("j", now)

		t.Run("it should stop at the last row", func(t *testing.T) {
			assert.Equal(t, 2, m.Cursor())
		})

		t.Run("and moves up past the top", func(t *testing.T) {
			m.Key("g", now)
			m.Key("k", now)

			t.Run("it should stop at the first row", func(t *testing.T) {
				assert.Equal(t, 0, m.Cursor())
			})
		})
	})

	t.Run("when a new snapshot reorders the rows", func(t *testing.T) {
		m := model(session("a", state.Working, 0), session("b", state.Working, time.Minute))
		m.Key("j", now)
		m.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{session("b", state.NeedsYou, 0), session("a", state.Working, 0)}})

		t.Run("it should keep the cursor on the same session", func(t *testing.T) {
			s, _ := m.Selected()
			assert.Equal(t, "b", s.ID)
		})
	})

	t.Run("when enter is pressed on a live session", func(t *testing.T) {
		m := model(session("a", state.NeedsYou, 0))

		t.Run("it should attach", func(t *testing.T) {
			assert.Equal(t, tui.Action{Kind: tui.ActionAttach, SessionID: "a"}, m.Key("enter", now))
		})
	})

	t.Run("when enter is pressed on a parked session", func(t *testing.T) {
		m := model(session("a", state.Parked, 0))
		m.Key("a", now)

		t.Run("it should resume", func(t *testing.T) {
			assert.Equal(t, tui.Action{Kind: tui.ActionResume, SessionID: "a"}, m.Key("enter", now))
		})
	})

	t.Run("when s is pressed", func(t *testing.T) {
		t.Run("on a live session", func(t *testing.T) {
			m := model(session("a", state.Working, 0))
			assert.Equal(t, tui.Action{Kind: tui.ActionStop, SessionID: "a"}, m.Key("s", now))
		})

		t.Run("on a parked session", func(t *testing.T) {
			m := model(session("a", state.Parked, 0))
			m.Key("a", now)
			assert.Equal(t, tui.Action{}, m.Key("s", now))
		})
	})

	t.Run("when x is pressed twice quickly", func(t *testing.T) {
		m := model(session("a", state.Working, 0))
		first := m.Key("x", now)
		second := m.Key("x", now.Add(time.Second))

		t.Run("it should arm then demolish", func(t *testing.T) {
			assert.Equal(t, tui.Action{}, first)
			assert.Equal(t, tui.Action{Kind: tui.ActionDemolish, SessionID: "a"}, second)
		})
	})

	t.Run("when x is pressed twice slowly", func(t *testing.T) {
		m := model(session("a", state.Working, 0))
		m.Key("x", now)

		t.Run("it should only re-arm", func(t *testing.T) {
			assert.Equal(t, tui.Action{}, m.Key("x", now.Add(10*time.Second)))
			assert.Contains(t, m.Status(), "press x again")
		})
	})

	t.Run("when q is pressed", func(t *testing.T) {
		t.Run("it should quit", func(t *testing.T) {
			assert.Equal(t, tui.Action{Kind: tui.ActionQuit}, model().Key("q", now))
		})
	})

	t.Run("when rendered", func(t *testing.T) {
		m := model(session("a", state.NeedsYou, 6*time.Minute), session("b", state.Working, 0))
		lines := m.Lines(0)

		t.Run("it should print a header and one line per row", func(t *testing.T) {
			assert.Len(t, lines, 3)
			assert.True(t, strings.HasPrefix(lines[0], "  STATE"))
		})

		t.Run("it should mark the cursor row", func(t *testing.T) {
			assert.True(t, strings.HasPrefix(lines[1], "> needs-you"))
			assert.True(t, strings.HasPrefix(lines[2], "  working"))
		})

		t.Run("it should use the table's formatters, idle time last", func(t *testing.T) {
			assert.Contains(t, lines[1], "25%")
			assert.True(t, strings.HasSuffix(lines[1], " 6m"), lines[1])
			assert.True(t, strings.HasSuffix(lines[0], "IDLE"), lines[0])
		})

		t.Run("it should say what a working session is doing", func(t *testing.T) {
			busy := session("c", state.Working, 0)
			busy.Tool, busy.Subject = "Edit", busy.CWD+"/pkg/queue/queue.go"
			assert.Contains(t, model(busy).Lines(0)[1], "Edit pkg/queue/queue.go")
		})

		t.Run("it should dash the column for an idle session", func(t *testing.T) {
			assert.Contains(t, lines[1], "needs-you  -  ")
		})

		t.Run("and a width is given", func(t *testing.T) {
			for _, l := range m.Lines(30) {
				assert.LessOrEqual(t, len(l), 30)
			}
		})
	})

	t.Run("when there are no rows", func(t *testing.T) {
		m := model()

		t.Run("it should say so", func(t *testing.T) {
			assert.Contains(t, m.Lines(0)[1], "no sessions")
		})

		t.Run("it should offer no selection", func(t *testing.T) {
			_, ok := m.Selected()
			assert.False(t, ok)
		})
	})

	t.Run("when the footer is read", func(t *testing.T) {
		m := model(session("a", state.Working, 0))

		t.Run("it should count rows and show the keys", func(t *testing.T) {
			assert.Contains(t, m.Footer(), "1 sessions (live)")
			assert.Contains(t, m.Footer(), "q quit")
		})

		t.Run("and a status is set", func(t *testing.T) {
			m.SetStatus("attached a")
			assert.Equal(t, "attached a", m.Footer())
		})
	})
}
