package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

type statsChoice struct {
	prompt stats.Prompt
	keep   bool
}

type statsHarness struct {
	g       *Game
	chosen  []statsChoice
	applied []ui.Setting
}

// Item 110: the statistics prompts.
func TestStatsPrompt(t *testing.T) {
	now := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	since := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)

	newHarness := func(on bool, owed func(on bool) stats.Prompt) *statsHarness {
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now})
		h := &statsHarness{}
		value := "off"
		if on {
			value = "on"
		}
		h.g = &Game{scene: s, actor: &recordingActor{}, scripted: -1, clicked: -1,
			settings: ui.NewSettings([]ui.Setting{{Key: config.KeyStats, Label: "statistics", Options: []string{"on", "off"}, Value: value}}),
			stats: statsHooks{
				on:    on,
				owed:  owed,
				since: func() (time.Time, bool) { return since, true },
				choose: func(p stats.Prompt, keep bool) error {
					h.chosen = append(h.chosen, statsChoice{p, keep})
					return nil
				},
			},
			apply: func(s ui.Setting) error {
				h.applied = append(h.applied, s)
				return nil
			},
		}
		return h
	}
	owing := func(p stats.Prompt) func(bool) stats.Prompt {
		return func(bool) stats.Prompt { return p }
	}
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		require.NoError(t, g.handleKeys())
		g.scripted = -1
	}
	row := func(g *Game) string { return g.settings.Items[0].Value }

	for _, c := range []struct {
		name   string
		prompt statsPrompt
		want   statsWording
	}{
		{"the off prompt from the settings row", statsPrompt{kind: stats.OffPrompt}, statsWording{
			Title: "Turn statistics off",
			Lines: []string{
				"While statistics are off, Botropolis records nothing.",
				"Messages in any transcript Claude Code deletes in that time are lost to statistics for good, so turning statistics back on later leaves a gap that may not be accurate.",
			},
			Keys: "k  keep what is stored    d  delete it    esc  leave statistics on",
		}},
		{"the off prompt at open", statsPrompt{kind: stats.OffPrompt, atOpen: true}, statsWording{
			Title: "Statistics were turned off",
			Lines: []string{
				"While statistics are off, Botropolis records nothing.",
				"Messages in any transcript Claude Code deletes in that time are lost to statistics for good, so turning statistics back on later leaves a gap that may not be accurate.",
			},
			Keys: "k  keep what is stored    d  delete it    esc  ask me next time",
		}},
		{"the on prompt from the settings row", statsPrompt{kind: stats.OnPrompt, since: since}, statsWording{
			Title: "Turn statistics back on",
			Lines: []string{
				"Statistics were off from Oct 18 until now, and what was stored before was kept.",
				"Messages from transcripts Claude Code deleted in that time are missing, so anything that spans the gap may not be accurate.",
			},
			Keys: "k  keep it and mark the gap    f  delete it and start fresh    esc  leave statistics off",
		}},
		{"the on prompt at open", statsPrompt{kind: stats.OnPrompt, atOpen: true, since: since}, statsWording{
			Title: "Statistics are back on",
			Lines: []string{
				"Statistics were off from Oct 18 until now, and what was stored before was kept.",
				"Messages from transcripts Claude Code deleted in that time are missing, so anything that spans the gap may not be accurate.",
			},
			Keys: "k  keep it and mark the gap    f  delete it and start fresh    esc  ask me next time",
		}},
	} {
		t.Run("when it is "+c.name, func(t *testing.T) {
			t.Run("it should say exactly its wording", func(t *testing.T) {
				assert.Equal(t, c.want, c.prompt.wording())
			})
		})
	}

	t.Run("when the city opens owing the off prompt", func(t *testing.T) {
		opened := func(t *testing.T) *statsHarness {
			t.Helper()
			h := newHarness(false, owing(stats.OffPrompt))
			h.g.maybeStatsPrompt()
			return h
		}

		t.Run("it should show it", func(t *testing.T) {
			h := opened(t)
			p := h.g.statsPrompt()
			require.NotNil(t, p)
			assert.Equal(t, [2]any{stats.OffPrompt, true}, [2]any{p.kind, p.atOpen})
		})

		for _, c := range []struct {
			key  ebiten.Key
			name string
			want []statsChoice
		}{
			{ebiten.KeyK, "k", []statsChoice{{stats.OffPrompt, true}}},
			{ebiten.KeyD, "d", []statsChoice{{stats.OffPrompt, false}}},
			{ebiten.KeyEscape, "esc", nil},
		} {
			t.Run("and "+c.name+" is pressed", func(t *testing.T) {
				h := opened(t)
				press(t, h.g, c.key)

				t.Run("it should choose "+describe(c.want), func(t *testing.T) {
					assert.Equal(t, c.want, h.chosen)
				})

				t.Run("it should close", func(t *testing.T) {
					assert.Nil(t, h.g.statsPrompt())
				})
			})
		}

		t.Run("and esc is pressed, and the city goes on", func(t *testing.T) {
			h := opened(t)
			press(t, h.g, ebiten.KeyEscape)
			h.g.maybeStatsPrompt()

			t.Run("it should not ask again until the next launch", func(t *testing.T) {
				assert.Nil(t, h.g.statsPrompt())
			})
		})
	})

	t.Run("when the city opens owing the on prompt", func(t *testing.T) {
		for _, c := range []struct {
			key  ebiten.Key
			name string
			want []statsChoice
		}{
			{ebiten.KeyK, "k", []statsChoice{{stats.OnPrompt, true}}},
			{ebiten.KeyF, "f", []statsChoice{{stats.OnPrompt, false}}},
			{ebiten.KeyEscape, "esc", nil},
		} {
			t.Run("and "+c.name+" is pressed", func(t *testing.T) {
				h := newHarness(true, owing(stats.OnPrompt))
				h.g.maybeStatsPrompt()
				press(t, h.g, c.key)

				t.Run("it should choose "+describe(c.want), func(t *testing.T) {
					assert.Equal(t, c.want, h.chosen)
				})
			})
		}
	})

	t.Run("when the city opens owing nothing", func(t *testing.T) {
		h := newHarness(true, owing(stats.NoPrompt))
		h.g.maybeStatsPrompt()

		t.Run("it should show no prompt", func(t *testing.T) {
			assert.Nil(t, h.g.statsPrompt())
		})
	})

	for _, run := range []struct {
		name string
		set  func(*Game)
	}{
		{"a screenshot", func(g *Game) { g.screenshot = "shot.png" }},
		{"a recording", func(g *Game) { g.record = "frames" }},
		{"a driven run", func(g *Game) { g.driven = true }},
	} {
		t.Run("when the run is "+run.name+", owing the off prompt", func(t *testing.T) {
			h := newHarness(false, owing(stats.OffPrompt))
			run.set(h.g)
			h.g.maybeStatsPrompt()

			t.Run("it should show no prompt", func(t *testing.T) {
				assert.Nil(t, h.g.statsPrompt())
			})
		})
	}

	for _, c := range []struct {
		cue  string
		want stats.Prompt
	}{{"off", stats.OffPrompt}, {"on", stats.OnPrompt}} {
		t.Run("when a recording's script shows the "+c.cue+" prompt", func(t *testing.T) {
			h := newHarness(false, owing(stats.NoPrompt))
			h.g.record = "frames"
			h.g.playStatsPrompt(c.cue)

			t.Run("it should show it as it would be at open", func(t *testing.T) {
				p := h.g.statsPrompt()
				require.NotNil(t, p)
				assert.Equal(t, [2]any{c.want, true}, [2]any{p.kind, p.atOpen})
			})
		})
	}

	t.Run("when the statistics row is turned off in the settings", func(t *testing.T) {
		turnedOff := func() *statsHarness {
			h := newHarness(true, owing(stats.NoPrompt))
			h.g.applySetting(h.g.settings.Adjust(1))
			return h
		}

		t.Run("it should show the off prompt", func(t *testing.T) {
			p := turnedOff().g.statsPrompt()
			require.NotNil(t, p)
			assert.Equal(t, [2]any{stats.OffPrompt, false}, [2]any{p.kind, p.atOpen})
		})

		t.Run("it should not save the row yet", func(t *testing.T) {
			assert.Empty(t, turnedOff().applied)
		})

		t.Run("and esc is pressed", func(t *testing.T) {
			h := turnedOff()
			press(t, h.g, ebiten.KeyEscape)

			t.Run("it should leave the row on", func(t *testing.T) {
				assert.Equal(t, "on", row(h.g))
			})
		})

		t.Run("and k is pressed", func(t *testing.T) {
			h := turnedOff()
			press(t, h.g, ebiten.KeyK)

			t.Run("it should choose off, keeping the data", func(t *testing.T) {
				assert.Equal(t, []statsChoice{{stats.OffPrompt, true}}, h.chosen)
			})

			t.Run("it should show the row off", func(t *testing.T) {
				assert.Equal(t, "off", row(h.g))
			})
		})
	})

	t.Run("when the row is turned on, and data was kept while off", func(t *testing.T) {
		h := newHarness(false, func(on bool) stats.Prompt {
			if on {
				return stats.OnPrompt
			}
			return stats.NoPrompt
		})
		h.g.applySetting(h.g.settings.Adjust(1))

		t.Run("it should show the on prompt", func(t *testing.T) {
			p := h.g.statsPrompt()
			require.NotNil(t, p)
			assert.Equal(t, [2]any{stats.OnPrompt, false}, [2]any{p.kind, p.atOpen})
		})
	})

	t.Run("when the row is turned on, and nothing was kept", func(t *testing.T) {
		h := newHarness(false, owing(stats.NoPrompt))
		h.g.applySetting(h.g.settings.Adjust(1))

		t.Run("it should save it", func(t *testing.T) {
			assert.Len(t, h.applied, 1)
		})

		t.Run("it should show no prompt", func(t *testing.T) {
			assert.Nil(t, h.g.statsPrompt())
		})
	})
}

func describe(choices []statsChoice) string {
	if len(choices) == 0 {
		return "nothing"
	}
	c := choices[0]
	switch {
	case c.prompt == stats.OffPrompt && c.keep:
		return "off, keeping the data"
	case c.prompt == stats.OffPrompt:
		return "off, deleting it"
	case c.keep:
		return "on, keeping the old data"
	}
	return "on, starting fresh"
}
