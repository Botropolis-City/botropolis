package acceptance_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/render"
)

// Item 88: the stream is a column down the right, shown until l hides it.
func TestStreamColumn(t *testing.T) {
	home := stageDemo(t, "hamlet")

	t.Run("when the city opens", func(t *testing.T) {
		d := newDrive(t, home)
		require.NoError(t, d.Run(1))

		t.Run("it should show the stream", func(t *testing.T) {
			assert.True(t, d.Shown().Stream)
		})

		t.Run("and l is pressed", func(t *testing.T) {
			require.NoError(t, d.Press("l"))
			require.NoError(t, d.Run(1))

			t.Run("it should hide the column", func(t *testing.T) {
				assert.False(t, d.Shown().Stream)
			})

			t.Run("and pressed again", func(t *testing.T) {
				require.NoError(t, d.Press("l"))
				require.NoError(t, d.Run(1))

				t.Run("it should show the column", func(t *testing.T) {
					assert.True(t, d.Shown().Stream)
				})
			})
		})
	})
}

// Item 100: in a window too narrow for it, the column hides itself, and
// its key still shows it.
func TestStreamColumnNarrow(t *testing.T) {
	home := stageDemo(t, "hamlet")

	t.Run("when the city opens in a narrow window", func(t *testing.T) {
		d, err := render.NewDrive(render.DriveOptions{Home: home, Width: 700, Height: 600})
		require.NoError(t, err)
		require.NoError(t, d.Run(1))

		t.Run("it should hide the stream", func(t *testing.T) {
			assert.False(t, d.Shown().Stream)
		})

		t.Run("and l is pressed", func(t *testing.T) {
			require.NoError(t, d.Press("l"))
			require.NoError(t, d.Run(1))

			t.Run("it should show the column", func(t *testing.T) {
				assert.True(t, d.Shown().Stream)
			})

			t.Run("and pressed again", func(t *testing.T) {
				require.NoError(t, d.Press("l"))
				require.NoError(t, d.Run(1))

				t.Run("it should hide the column", func(t *testing.T) {
					assert.False(t, d.Shown().Stream)
				})
			})
		})
	})
}

// Item 100: the wheel scrolls the stream while the pointer is over it,
// and zooms the map anywhere else.
func TestStreamScrolls(t *testing.T) {
	home := stageDemo(t, "hamlet")
	at := time.Now()
	var log []events.Event
	for i := range 40 {
		log = append(log, events.Event{At: at.Add(-time.Duration(i) * time.Minute), Kind: events.Started, SessionID: fmt.Sprint(i), Title: fmt.Sprint("entry ", i)})
	}

	t.Run("when the pointer is over the stream and the wheel turns down", func(t *testing.T) {
		d, err := render.NewDrive(render.DriveOptions{Home: home, Width: 1600, Height: 900, Events: log})
		require.NoError(t, err)
		require.NoError(t, d.Run(settleTicks))
		before := d.Shown()
		require.NoError(t, d.WheelAt(before.StreamRect.Center(), -3))
		require.NoError(t, d.Run(1))
		after := d.Shown()

		t.Run("it should scroll the stream", func(t *testing.T) {
			assert.Equal(t, 3, after.StreamScrolled)
		})

		t.Run("it should not zoom the map", func(t *testing.T) {
			assert.Equal(t, before.Zoom, after.Zoom)
		})

		t.Run("and the wheel turns elsewhere", func(t *testing.T) {
			require.NoError(t, d.WheelAt(city.Point{X: 400, Y: 450}, 1))
			require.NoError(t, d.Run(1))

			t.Run("it should zoom the map", func(t *testing.T) {
				assert.NotEqual(t, after.Zoom, d.Shown().Zoom)
			})
		})
	})
}
