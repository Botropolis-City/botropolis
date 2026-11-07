package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
)

// Item 351: holding the explain key over a city with a card, the menu
// and the stream up puts a cap on every control with a key, as bound,
// and grows nothing.
func TestHoldCaps(t *testing.T) {
	home := stageDemo(t, "hamlet")
	opened := func(t *testing.T, keys map[string]string) (*render.Drive, render.Shown) {
		t.Helper()
		d, err := render.NewDrive(render.DriveOptions{Home: home, Width: 1600, Height: 1000, Keys: keys})
		require.NoError(t, err)
		_, err = d.Until(func(s render.Shown) bool { return s.Settled }, settleTicks)
		require.NoError(t, err)
		ids := d.Sessions()
		require.NotEmpty(t, ids)
		require.NoError(t, d.Click(ids[0]))
		_, err = d.Until(func(s render.Shown) bool { return s.Selected == ids[0] && s.Settled }, settleTicks*3)
		require.NoError(t, err)
		require.NoError(t, d.PointAtPoint(d.Shown().CardRect.Min.Sub(d.Shown().CardRect.Size())))
		require.NoError(t, d.Run(2))
		before := d.Shown()
		require.NoError(t, d.Click("button:Session ⌄"))
		require.NoError(t, d.Run(3))
		require.True(t, d.Shown().Menu, "the menu should be open")
		require.NoError(t, d.Press("hold+e"))
		require.NoError(t, d.Run(3))
		return d, before
	}
	capsOf := func(s render.Shown) map[string][]string {
		out := map[string][]string{}
		for _, c := range s.HoldCaps {
			out[c.Name] = c.Keys
		}
		return out
	}

	t.Run("when a session's card, its menu and the stream are up and e is held", func(t *testing.T) {
		d, before := opened(t, nil)
		shown := d.Shown()

		t.Run("it should show the stream", func(t *testing.T) {
			assert.Contains(t, capsOf(shown), "stream")
		})

		t.Run("it should cap the card's Session button with space and s", func(t *testing.T) {
			assert.Equal(t, []string{"space", "s"}, capsOf(shown)["button:Session ⌄"])
		})

		t.Run("it should leave the card's face as wide as it was", func(t *testing.T) {
			assert.Equal(t, before.CardRect.Width(), shown.CardRect.Width())
		})

		t.Run("it should keep every cap inside the window", func(t *testing.T) {
			for _, c := range shown.HoldCaps {
				for _, r := range c.Caps {
					assert.True(t, r.Min.X >= 0 && r.Min.Y >= 0 && r.Max.X <= 1600 && r.Max.Y <= 1000, "%s at %v", c.Name, r)
				}
			}
		})
	})

	t.Run("when the menu's Session column is rebound to w and e is held", func(t *testing.T) {
		d, _ := opened(t, map[string]string{"menu_session": "w"})

		t.Run("it should cap the card's Session button with space and w", func(t *testing.T) {
			assert.Equal(t, []string{"space", "w"}, capsOf(d.Shown())["button:Session ⌄"])
		})
	})
}
