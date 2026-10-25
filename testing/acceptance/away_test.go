package acceptance_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/render"
)

// Item 246: the validation checklist's step 4, the city's half,
// scripted. A city that loses the focus and gets it back opens "While
// you were away" when something needed you meanwhile, and only then.
func TestTheAwayListOnRefocus(t *testing.T) {
	home := stageDemo(t, "hamlet")

	// awayAndBack blurs a settled city, hands it an event, and focuses it
	// again, returning what it drew once it had.
	awayAndBack := func(t *testing.T, kind events.Kind) (render.Shown, string) {
		t.Helper()
		d, err := render.NewDrive(render.DriveOptions{Home: home})
		require.NoError(t, err)
		_, err = d.Until(func(s render.Shown) bool { return s.Settled }, settleTicks)
		require.NoError(t, err)
		id := d.Sessions()[0]
		d.SetFocused(false)
		require.NoError(t, d.Run(2))
		d.AddEvents([]events.Event{{At: time.Now().Add(time.Second), Kind: kind, SessionID: id, Title: d.Title(id)}})
		require.NoError(t, d.Run(2))
		d.SetFocused(true)
		require.NoError(t, d.Run(3))
		return d.Shown(), id
	}

	t.Run("when the city loses focus, and a session starts needing you, and it regains focus", func(t *testing.T) {
		s, id := awayAndBack(t, events.NeedsYou)

		t.Run("it should open While you were away", func(t *testing.T) {
			assert.Equal(t, "While you were away", s.TimelineTitle)
		})

		t.Run("it should list that session", func(t *testing.T) {
			assert.Contains(t, s.Away, id)
		})
	})

	t.Run("when the city loses focus, and nothing needs you, and it regains focus", func(t *testing.T) {
		s, _ := awayAndBack(t, events.Started)

		t.Run("it should not open the away list", func(t *testing.T) {
			assert.False(t, s.Timeline)
		})
	})
}
