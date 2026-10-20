package tui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/tui"
	"github.com/charmbracelet/x/ansi"
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
		m := model(session("p", state.Resting, time.Hour), session("w", state.Working, 2*time.Minute),
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
		m := model(session("a", state.Resting, 0))
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
			m := model(session("a", state.Resting, 0))
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

		ctx := column(lines[0], "CTX")
		row := func(edit func(*state.Session), width int) string {
			s := session("i", state.Working, 0)
			edit(&s)
			return model(s).Lines(width)[1]
		}
		dashed := func(s *state.Session) { s.Title = strings.Repeat("a", 38) + "— and well past forty columns" }
		accented := func(s *state.Session) {
			s.CWD = "/home/x/" + strings.Repeat("é", 20)
			s.Tool, s.Subject = "Edit", s.CWD+"/a"+strings.Repeat("é", 30)
		}

		t.Run("and a title has an em dash at byte 39", func(t *testing.T) {
			line := row(dashed, 0)

			t.Run("it should stay valid UTF-8", func(t *testing.T) {
				assert.True(t, utf8.ValidString(line), line)
			})

			t.Run("it should shorten the title to 40 columns ending in …", func(t *testing.T) {
				assert.Contains(t, line, strings.Repeat("a", 38)+"—… ")
			})
		})

		t.Run("and the doing and project columns are cut inside an accented character", func(t *testing.T) {
			t.Run("it should stay valid UTF-8", func(t *testing.T) {
				line := row(accented, 0)
				assert.True(t, utf8.ValidString(line), line)
			})
		})

		t.Run("and a line with non-ASCII text is wider than the given width", func(t *testing.T) {
			cut := row(accented, 60)

			t.Run("it should stay valid UTF-8", func(t *testing.T) {
				assert.True(t, utf8.ValidString(cut), cut)
			})

			t.Run("it should fill the width exactly", func(t *testing.T) {
				assert.Equal(t, 60, ansi.StringWidth(cut), cut)
			})
		})

		for name, title := range map[string]string{
			"and a title is in a wide script":                        "漢字の題",
			"and a wide character straddles the title's 40th column": strings.Repeat("漢", 30),
		} {
			t.Run(name, func(t *testing.T) {
				wide := session("w", state.Working, 0)
				wide.Title = title
				line := model(wide).Lines(0)[1]

				t.Run("it should start the CTX column where the header does", func(t *testing.T) {
					assert.Equal(t, ctx, column(line, "25%"), line)
				})
			})
		}
	})

	t.Run("when there are more rows than the height allows", func(t *testing.T) {
		const height = 10
		fresh := func() *tui.Model {
			m := model(many(28)...)
			m.SetHeight(height)
			return m
		}

		t.Run("and the cursor is on the first row", func(t *testing.T) {
			lines := fresh().Lines(0)

			t.Run("it should draw the header first", func(t *testing.T) {
				assert.True(t, strings.HasPrefix(lines[0], "  STATE"), lines[0])
			})

			t.Run("it should draw exactly height-1 lines, leaving one for the footer", func(t *testing.T) {
				assert.Len(t, lines, height-1)
			})

			t.Run("it should draw the cursor's row", func(t *testing.T) {
				assert.True(t, drawsCursor(lines, "s00"), strings.Join(lines, "\n"))
			})

			t.Run("it should say how many rows lie below", func(t *testing.T) {
				assert.Equal(t, "  ↑ 0 above · ↓ 21 below", lines[len(lines)-1])
			})
		})

		t.Run("and the cursor walks down past the last drawn row", func(t *testing.T) {
			m := fresh()
			press(m, "j", 12)
			lines := m.Lines(0)

			t.Run("it should still draw the header first", func(t *testing.T) {
				assert.True(t, strings.HasPrefix(lines[0], "  STATE"), lines[0])
			})

			t.Run("it should draw the cursor's row", func(t *testing.T) {
				assert.True(t, drawsCursor(lines, "s12"), strings.Join(lines, "\n"))
			})

			t.Run("it should draw exactly height-1 lines", func(t *testing.T) {
				assert.Len(t, lines, height-1)
			})

			t.Run("it should say how many rows lie above and below", func(t *testing.T) {
				assert.Equal(t, "  ↑ 6 above · ↓ 15 below", lines[len(lines)-1])
			})

			t.Run("and walks back up one row", func(t *testing.T) {
				press(m, "k", 1)

				t.Run("it should not scroll", func(t *testing.T) {
					assert.Contains(t, m.Lines(0)[1], "Job s06 ")
				})
			})
		})

		t.Run("and the cursor jumps to the end", func(t *testing.T) {
			m := fresh()
			press(m, "G", 1)
			lines := m.Lines(0)

			t.Run("it should draw the last row", func(t *testing.T) {
				assert.True(t, drawsCursor(lines, "s27"), strings.Join(lines, "\n"))
			})

			t.Run("it should say nothing lies below", func(t *testing.T) {
				assert.Equal(t, "  ↑ 21 above · ↓ 0 below", lines[len(lines)-1])
			})
		})

		t.Run("and the height shrinks while the cursor is low in the window", func(t *testing.T) {
			m := fresh()
			press(m, "j", 6)
			m.Lines(0)
			m.SetHeight(5)

			t.Run("it should still draw the cursor's row", func(t *testing.T) {
				assert.True(t, drawsCursor(m.Lines(0), "s06"), strings.Join(m.Lines(0), "\n"))
			})
		})

		t.Run("and rows disappear while the window is scrolled to the end", func(t *testing.T) {
			m := fresh()
			press(m, "G", 1)
			m.Lines(0)
			m.SetSnapshot(state.Snapshot{At: now, Sessions: many(12)})
			lines := m.Lines(0)

			t.Run("it should fill the window with rows instead of blank lines", func(t *testing.T) {
				assert.Equal(t, "  ↑ 5 above · ↓ 0 below", lines[len(lines)-1])
			})

			t.Run("it should still draw the cursor's row", func(t *testing.T) {
				assert.True(t, drawsCursor(lines, "s11"), strings.Join(lines, "\n"))
			})
		})

		t.Run("and the height grows to fit every row", func(t *testing.T) {
			m := fresh()
			m.SetHeight(30)

			t.Run("it should draw every row with no above/below line", func(t *testing.T) {
				assert.Len(t, m.Lines(0), 29)
			})
		})

		for _, tiny := range []struct {
			name   string
			height int
			want   []string
		}{
			{"and the height has room for the header and one row", 3, []string{"  STATE  ", "> working"}},
			{"and the height has room for one line", 2, []string{"> working"}},
		} {
			t.Run(tiny.name, func(t *testing.T) {
				m := fresh()
				press(m, "j", 4)
				m.SetHeight(tiny.height)
				lines := m.Lines(0)
				got := []string{}
				for _, l := range lines {
					got = append(got, l[:min(len(l), 9)])
				}

				t.Run("it should drop the above/below line first, then the header", func(t *testing.T) {
					assert.Equal(t, tiny.want, got)
				})

				t.Run("it should still draw the cursor's row", func(t *testing.T) {
					assert.True(t, drawsCursor(lines, "s04"), strings.Join(lines, "\n"))
				})
			})
		}
	})

	t.Run("when the rows fit the height", func(t *testing.T) {
		m := model(many(3)...)
		m.SetHeight(10)

		t.Run("it should draw no above/below line", func(t *testing.T) {
			assert.Len(t, m.Lines(0), 4)
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

func many(n int) []state.Session {
	sessions := []state.Session{}
	for i := range n {
		sessions = append(sessions, session(fmt.Sprintf("s%02d", i), state.Working, time.Duration(i)*time.Minute))
	}
	return sessions
}

func press(m *tui.Model, key string, times int) {
	for range times {
		m.Key(key, now)
	}
}

func drawsCursor(lines []string, id string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, "> ") && strings.Contains(l, "Job "+id+" ") {
			return true
		}
	}
	return false
}

func column(line, sub string) int {
	i := strings.Index(line, sub)
	if i < 0 {
		return -1
	}
	return ansi.StringWidth(line[:i])
}
