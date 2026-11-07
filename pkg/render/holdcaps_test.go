package render

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 351: holding the explain key puts a cap on every visible control
// with a key, read from the live bindings, and letting go takes them
// away.
func TestHoldCaps(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000351"
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local)
	drive := func(t *testing.T, keys map[string]string) *Drive {
		t.Helper()
		h := helpers.NewHome(t)
		cwd := "/home/user/work/harbour"
		ts := now.Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
		h.Transcript(id, cwd, helpers.UserPrompt(id, cwd, ts), helpers.AssistantReply(id, "msg_1", ts), helpers.AITitle(id, "Boarded up a week"))
		d, err := NewDrive(DriveOptions{Home: h.Path, Now: now})
		require.NoError(t, err)
		if keys != nil {
			chords, err := config.FindChords(config.Config{Keys: keys}, runtime.GOOS, KnownKey)
			require.NoError(t, err)
			d.game.chords = &chords
		}
		require.NoError(t, d.Run(10))
		return d
	}
	selected := func(t *testing.T, keys map[string]string) *Drive {
		t.Helper()
		d := drive(t, keys)
		require.NoError(t, d.Click(id))
		require.NoError(t, d.Run(5))
		require.Equal(t, id, d.Shown().Selected)
		require.NoError(t, d.PointAtPoint(city.Point{X: 1, Y: 1}))
		return d
	}
	holding := func(t *testing.T, d *Drive) Shown {
		t.Helper()
		require.NoError(t, d.Press("hold+e"))
		require.NoError(t, d.Run(2))
		return d.Shown()
	}
	capsOf := func(s Shown) map[string][]string {
		out := map[string][]string{}
		for _, c := range s.HoldCaps {
			out[c.Name] = c.Keys
		}
		return out
	}
	// bound is each control's binding in config.Bindings, the table the
	// caps must agree with; "" is a fixed key.
	bound := map[string][]string{
		"numbers": {"details"},
		"lenses":  {"lenses"},
		"stream":  {"stream"},
		"sidebar": {"sidebar"},
	}

	t.Run("when a session's card is pinned and e is held", func(t *testing.T) {
		d := selected(t, nil)
		before := d.Shown().CardRect
		shown := holding(t, d)
		caps := capsOf(shown)

		for control, want := range map[string][]string{
			"button:resume":    {"enter"},
			"button:Session ⌄": {"space", "s"},
			"button:Project ⌄": {"space", "p"},
			"button:Group ⌄":   {"space", "g"},
			"numbers":          {"i"},
			"lenses":           {"v"},
		} {
			t.Run("it should cap "+control+" with "+keysName(want), func(t *testing.T) {
				assert.Equal(t, want, caps[control])
			})
		}

		t.Run("it should give every cap its control's live binding", func(t *testing.T) {
			chords := d.game.findChords()
			for name, keys := range caps {
				names, ok := bound[name]
				if !ok {
					continue
				}
				var want []string
				for _, n := range names {
					want = append(want, chords.Bound[n].Label())
				}
				assert.Equal(t, want, keys, name)
			}
		})

		t.Run("it should cap nothing it has no key for", func(t *testing.T) {
			for _, c := range shown.HoldCaps {
				assert.NotEmpty(t, c.Keys, c.Name)
			}
		})

		t.Run("it should leave the card as it was", func(t *testing.T) {
			assert.Equal(t, before, shown.CardRect)
		})

		t.Run("it should still caption the map's objects", func(t *testing.T) {
			assert.True(t, shown.Explaining)
		})

		t.Run("and e is released", func(t *testing.T) {
			require.NoError(t, d.Press("release+e"))
			require.NoError(t, d.Run(2))

			t.Run("it should draw no caps", func(t *testing.T) {
				assert.Empty(t, d.Shown().HoldCaps)
			})
		})
	})

	t.Run("when the sidebar is shown and e is held", func(t *testing.T) {
		d := drive(t, nil)
		require.NoError(t, d.Press("b"))
		require.NoError(t, d.Run(3))
		caps := capsOf(holding(t, d))

		t.Run("it should cap the sidebar with b", func(t *testing.T) {
			assert.Equal(t, []string{"b"}, caps["sidebar"])
		})
	})

	t.Run("when the menu is open and e is held", func(t *testing.T) {
		d := selected(t, nil)
		require.NoError(t, d.Press("space"))
		require.NoError(t, d.Run(2))
		require.True(t, d.Shown().Menu)
		shown := holding(t, d)
		caps := capsOf(shown)

		t.Run("it should cap the card behind it", func(t *testing.T) {
			assert.Equal(t, []string{"space", "s"}, caps["button:Session ⌄"])
		})

		t.Run("it should give the menu's rows no second cap", func(t *testing.T) {
			for _, c := range shown.HoldCaps {
				assert.False(t, shown.MenuRect.Contains(c.Rect.Center()), "%s at %v", c.Name, c.Rect)
			}
		})
	})

	t.Run("when the clinic is open and e is held", func(t *testing.T) {
		d := drive(t, nil)
		d.game.scene.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{{ID: id, CWD: "/home/user/work/harbour", State: state.NeedsYou, LastActivity: now}}})
		require.NoError(t, d.Run(2))
		require.NoError(t, d.Press("a"))
		require.NoError(t, d.Run(3))
		caps := capsOf(holding(t, d))

		t.Run("it should cap its first row with 1", func(t *testing.T) {
			assert.Equal(t, []string{"1"}, caps["row:1"])
		})
	})

	t.Run("when the finder has the keyboard and e is pressed", func(t *testing.T) {
		d := drive(t, nil)
		require.NoError(t, d.Press("slash"))
		require.NoError(t, d.Run(2))

		t.Run("it should hold nothing", func(t *testing.T) {
			assert.Empty(t, holding(t, d).HoldCaps)
		})
	})

	t.Run("when menu_session is rebound and e is held", func(t *testing.T) {
		d := selected(t, map[string]string{"menu_session": "w"})
		caps := capsOf(holding(t, d))

		t.Run("it should cap the Session button with the rebound letter", func(t *testing.T) {
			assert.Equal(t, []string{"space", "w"}, caps["button:Session ⌄"])
		})
	})

	t.Run("when a card is pinned for the first time on this machine", func(t *testing.T) {
		d := selected(t, nil)
		require.NoError(t, d.Run(2))

		t.Run("it should say which key to hold for every key", func(t *testing.T) {
			assert.Equal(t, "hold e to see every key", d.Shown().KeysSighting)
		})

		t.Run("it should sit above the card, clear of where the menu opens", func(t *testing.T) {
			shown := d.Shown()
			assert.LessOrEqual(t, shown.KeysSightingRect.Max.Y, shown.CardRect.Min.Y, "caption %v, card %v", shown.KeysSightingRect, shown.CardRect)
		})

		t.Run("and once it has gone a card is pinned again", func(t *testing.T) {
			_, err := d.Until(func(s Shown) bool { return s.KeysSighting == "" }, 1200)
			require.NoError(t, err)
			require.NoError(t, d.Press("escape"))
			require.NoError(t, d.Run(2))
			require.NoError(t, d.Click(id))
			require.NoError(t, d.Run(2))

			t.Run("it should not say it again", func(t *testing.T) {
				assert.Empty(t, d.Shown().KeysSighting)
			})
		})
	})
}
