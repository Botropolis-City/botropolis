package render

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 351: a hovered button's line, and the menu's focused row, end
// with the keys that do the same thing, read from the live bindings.
func TestHintKeys(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000351"
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local)
	selected := func(t *testing.T, keys map[string]string) *Drive {
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
		require.NoError(t, d.Click(id))
		require.NoError(t, d.Run(5))
		require.Equal(t, id, d.Shown().Selected)
		return d
	}
	numbersRow := func(d *Drive) city.Point { return d.game.pinned.NumbersRect.Center() }

	for _, binding := range []struct {
		name string
		keys map[string]string
		want map[string][]string
	}{
		{"by default", nil, map[string][]string{
			"button:resume": {"enter"}, "button:Session ⌄": {"space", "s"}, "button:Project ⌄": {"space", "p"}, "button:Group ⌄": {"space", "g"}, "numbers": {"i"},
		}},
		{"with the menu's Session column and the details rebound", map[string]string{"menu_session": "w", "details": "y"}, map[string][]string{
			"button:Session ⌄": {"space", "w"}, "numbers": {"y"},
		}},
	} {
		t.Run("when the selection's card is pinned "+binding.name, func(t *testing.T) {
			for target, want := range binding.want {
				t.Run("and the pointer is on "+target, func(t *testing.T) {
					d := selected(t, binding.keys)
					if target == "numbers" {
						require.NoError(t, d.PointAtPoint(numbersRow(d)))
					} else {
						require.NoError(t, d.PointAt(target))
					}
					require.NoError(t, d.Run(2))

					t.Run("it should end the line with "+keysName(want), func(t *testing.T) {
						assert.Equal(t, want, d.Shown().CardHintKeys)
					})
				})
			}
		})
	}

	for _, binding := range []struct {
		name string
		keys map[string]string
		want []string
	}{
		{"by default", nil, []string{"space", "s", "a"}},
		{"with the row rebound", map[string]string{"menu_attach": "z"}, []string{"space", "s", "z"}},
	} {
		t.Run("when space then the Session column's letter opens the selection's menu "+binding.name, func(t *testing.T) {
			d := selected(t, binding.keys)
			require.NoError(t, d.PointAtPoint(city.Point{X: 1, Y: 1}))
			require.NoError(t, d.Press("space"))
			require.NoError(t, d.Run(2))
			column := "s"
			require.NoError(t, d.Press(column))
			require.NoError(t, d.Run(2))
			require.True(t, d.Shown().Menu)

			t.Run("it should end the focused row's line with the path from the map, "+keysName(binding.want), func(t *testing.T) {
				assert.Equal(t, binding.want, d.Shown().MenuDescriptionKeys)
			})
		})
	}

	t.Run("when a district's menu is open on its Project column", func(t *testing.T) {
		d := selected(t, nil)
		require.NoError(t, d.PointAtPoint(city.Point{X: 1, Y: 1}))
		g := d.game
		g.openMenuWith([]city.ActionKind{city.ActionNew, city.ActionReveal}, city.Point{X: 100, Y: 100}, menuFirstRows, func(city.ActionKind) {})
		require.NoError(t, d.Run(2))

		t.Run("it should end the focused row's line with the key that answers in that menu", func(t *testing.T) {
			assert.Equal(t, []string{"n"}, d.Shown().MenuDescriptionKeys)
		})
	})
}

// keysName spells keys for a test's name.
func keysName(keys []string) string {
	name := ""
	for i, k := range keys {
		if i > 0 {
			name += " "
		}
		name += k
	}
	return name
}
