package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 352: a notice is one plate under the strip and the stream's
// newest line; SetStatus is the plain notice until its callers go.
func TestNotify(t *testing.T) {
	newGame := func(t *testing.T) *Game {
		t.Helper()
		scene := city.NewScene(city.NewLayout())
		scene.Resize(800, 600)
		return &Game{scene: scene, scripted: -1, clicked: -1}
	}

	t.Run("when Notify is called with a tone and a key", func(t *testing.T) {
		g := newGame(t)
		g.Notify("packed up Sea conditions · undo", ui.ToneNeedsYou, "u")
		g.takeNotices()
		current, ok := g.notice.Current(timeNow())
		require.True(t, ok)

		t.Run("it should show the text as the current notice", func(t *testing.T) {
			assert.Equal(t, "packed up Sea conditions · undo", current.Text)
		})

		t.Run("it should keep the tone and key on the current notice", func(t *testing.T) {
			assert.Equal(t, [2]string{ui.ToneNeedsYou.String(), "u"}, [2]string{current.Tone.String(), current.Key})
		})

		t.Run("it should add it to the stream's events as the newest, of kind notice", func(t *testing.T) {
			list := g.scene.Events()
			require.NotEmpty(t, list)
			assert.Equal(t, [2]string{string(events.Notice), "packed up Sea conditions · undo"}, [2]string{string(list[0].Kind), list[0].Title})
		})
	})

	t.Run("when a notice line with no session is clicked in the stream", func(t *testing.T) {
		g := newGame(t)
		line := ui.NoticeEvent(ui.Message{Text: "saved city.png"}, time.Now())
		g.streamLaid = ui.Stream{
			Rect: city.RectAt(0, 0, 400, 400),
			Rows: []ui.StreamRow{{Event: line, Rect: city.RectAt(0, 0, 400, 40)}},
		}
		took := g.clickStream(city.Point{X: 200, Y: 20})

		t.Run("it should take the click", func(t *testing.T) {
			assert.True(t, took)
		})

		t.Run("it should say nothing", func(t *testing.T) {
			assert.Empty(t, g.notice.Text(timeNow()))
		})
	})
}
