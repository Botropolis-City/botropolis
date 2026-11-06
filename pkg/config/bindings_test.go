package config_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
)

// anyKey stands in for the city's key names: letters, digits, the
// brackets and slash, and f3.
func anyKey(name string) bool {
	name = strings.ToLower(name)
	if len(name) == 1 && name[0] >= 'a' && name[0] <= 'z' {
		return true
	}
	switch name {
	case "space", "tab", "slash", "bracketleft", "bracketright", "f3", "digit0", "digit1", "digit2", "digit3", "digit4", "digit5", "digit6":
		return true
	}
	return false
}

// Item 350, slice 5a: every binding is a row of one table, read from the
// config file's keys map and the old *_key settings, and a key two
// bindings would share where both answer is refused.
func TestBindings(t *testing.T) {
	keys := func(t *testing.T, cfg config.Config, goos string) (map[string]config.Chord, error) {
		t.Helper()
		return config.LoadBindings(cfg, goos, anyKey)
	}
	byName := map[string]config.Binding{}
	for _, b := range config.Bindings {
		byName[b.Name] = b
	}

	t.Run("when nothing is set", func(t *testing.T) {
		got, err := keys(t, config.Config{}, "linux")
		require.NoError(t, err)

		for _, b := range config.Bindings {
			t.Run("it should bind "+b.Name+" to its default", func(t *testing.T) {
				want, err := config.ParseChord(b.Name, b.Default, "linux", anyKey)
				require.NoError(t, err)
				assert.Equal(t, want, got[b.Name])
			})
		}
	})

	for _, c := range []struct {
		name  string
		set   func(*config.Config, string)
		value string
		want  config.Chord
	}{
		{"journal", func(c *config.Config, v string) { c.JournalKey = v }, "y", config.Chord{Key: "y"}},
		{"triage", func(c *config.Config, v string) { c.TriageKey = v }, "y", config.Chord{Key: "y"}},
		{"stream", func(c *config.Config, v string) { c.StreamKey = v }, "y", config.Chord{Key: "y"}},
		{"city_plan", func(c *config.Config, v string) { c.CityPlanKey = v }, "y", config.Chord{Key: "y"}},
		{"labels", func(c *config.Config, v string) { c.LabelsKey = v }, "y", config.Chord{Key: "y"}},
		{"explain", func(c *config.Config, v string) { c.ExplainKey = v }, "y", config.Chord{Key: "y"}},
		{"new_session", func(c *config.Config, v string) { c.NewSessionKey = v }, "shift+y", config.Chord{Shift: true, Key: "y"}},
		{"find", func(c *config.Config, v string) { c.FindKey = v }, "alt+f3", config.Chord{Alt: true, Key: "f3"}},
		{"find_copy", func(c *config.Config, v string) { c.FindCopyKey = v }, "ctrl+f3", config.Chord{Ctrl: true, Key: "f3"}},
		{"find_show", func(c *config.Config, v string) { c.FindShowKey = v }, "ctrl+f3", config.Chord{Ctrl: true, Key: "f3"}},
	} {
		t.Run("when the old setting for "+c.name+" is set", func(t *testing.T) {
			var cfg config.Config
			c.set(&cfg, c.value)
			got, err := keys(t, cfg, "linux")
			require.NoError(t, err)

			t.Run("it should take it, as it always has", func(t *testing.T) {
				assert.Equal(t, c.want, got[c.name])
			})
		})
	}

	t.Run("when keys sets a binding", func(t *testing.T) {
		got, err := keys(t, config.Config{Keys: map[string]string{"timeline": "y"}}, "linux")
		require.NoError(t, err)

		t.Run("it should take it", func(t *testing.T) {
			assert.Equal(t, config.Chord{Key: "y"}, got["timeline"])
		})

		t.Run("it should leave the others at their defaults", func(t *testing.T) {
			assert.Equal(t, config.Chord{Key: "x"}, got["breakdown"])
		})
	})

	t.Run("when an old setting and keys set one binding", func(t *testing.T) {
		t.Run("and to different chords", func(t *testing.T) {
			_, err := keys(t, config.Config{JournalKey: "y", Keys: map[string]string{"journal": "k"}}, "linux")

			t.Run("it should refuse, naming both", func(t *testing.T) {
				require.Error(t, err)
				assert.True(t, strings.Contains(err.Error(), "journal_key") && strings.Contains(err.Error(), "keys.journal"), err.Error())
			})
		})

		t.Run("and to the same chord", func(t *testing.T) {
			got, err := keys(t, config.Config{JournalKey: "y", Keys: map[string]string{"journal": "y"}}, "linux")

			t.Run("it should take it", func(t *testing.T) {
				require.NoError(t, err)
				assert.Equal(t, config.Chord{Key: "y"}, got["journal"])
			})
		})
	})

	t.Run("when two bindings in one scope are set to one chord", func(t *testing.T) {
		_, err := keys(t, config.Config{Keys: map[string]string{"timeline": "y", "breakdown": "y"}}, "linux")

		t.Run("it should refuse, naming both", func(t *testing.T) {
			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), "timeline") && strings.Contains(err.Error(), "breakdown"), err.Error())
		})
	})

	t.Run("when a binding takes a chord another scope uses", func(t *testing.T) {
		_, err := keys(t, config.Config{Keys: map[string]string{"menu_session": "w"}}, "linux")

		t.Run("it should load, since the two never answer together", func(t *testing.T) {
			assert.NoError(t, err)
		})
	})

	t.Run("when a binding takes the chord of one that answers everywhere", func(t *testing.T) {
		_, err := keys(t, config.Config{Keys: map[string]string{"timeline": "ctrl+k"}}, "linux")

		t.Run("it should refuse", func(t *testing.T) {
			assert.Error(t, err)
		})
	})

	t.Run("when a binding is set to a chord macOS keeps, on darwin", func(t *testing.T) {
		_, err := keys(t, config.Config{Keys: map[string]string{"timeline": "cmd+q"}}, "darwin")

		t.Run("it should refuse it", func(t *testing.T) {
			assert.Error(t, err)
		})
	})

	for _, c := range []struct{ name, chord string }{
		{"timeline", "d"},
		{"timeline", "tab"},
		{"menu_stop", "j"},
		{"menu_session", "k"},
		{"menu_focus", "tab"},
	} {
		t.Run("when "+c.name+" is set to "+c.chord+", a key that stays fixed where it answers", func(t *testing.T) {
			_, err := keys(t, config.Config{Keys: map[string]string{c.name: c.chord}}, "linux")

			t.Run("it should refuse, naming what the key does", func(t *testing.T) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "fixed")
			})
		})
	}

	t.Run("when keys names no binding", func(t *testing.T) {
		_, err := keys(t, config.Config{Keys: map[string]string{"teleport": "y"}}, "linux")

		t.Run("it should refuse, naming it", func(t *testing.T) {
			require.Error(t, err)
			assert.Contains(t, err.Error(), "teleport")
		})
	})

	t.Run("when the table is read as a whole", func(t *testing.T) {
		t.Run("it should name each binding once", func(t *testing.T) {
			assert.Len(t, byName, len(config.Bindings))
		})

		t.Run("it should give every binding a scope", func(t *testing.T) {
			for _, b := range config.Bindings {
				assert.NotEmpty(t, b.Scope, b.Name)
			}
		})
	})
}

// Item 350, slice 5a: a chord as the reference and a cap show it.
func TestChordLabel(t *testing.T) {
	for _, c := range []struct {
		chord config.Chord
		want  string
	}{
		{config.Chord{Key: "t"}, "t"},
		{config.Chord{Shift: true, Key: "n"}, "shift+n"},
		{config.Chord{Key: "slash"}, "/"},
		{config.Chord{Shift: true, Key: "slash"}, "?"},
		{config.Chord{Key: "digit3"}, "3"},
		{config.Chord{Key: "bracketleft"}, "["},
		{config.Chord{Ctrl: true, Key: "k"}, "ctrl+k"},
	} {
		t.Run("when the chord is "+c.chord.String(), func(t *testing.T) {
			t.Run("it should read "+c.want, func(t *testing.T) {
				assert.Equal(t, c.want, c.chord.Label())
			})
		})
	}
}
