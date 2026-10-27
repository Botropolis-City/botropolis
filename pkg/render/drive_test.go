package render

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/testing/helpers"
)

// lateActor hands each action on to the Drive only once released, as a
// goroutine beside the loop does when the machine is busy (item 298).
type lateActor struct {
	d       *Drive
	release chan struct{}
}

func (a lateActor) Do(action city.Action) error {
	<-a.release
	return a.d.Do(action)
}

func TestDrive(t *testing.T) {
	heap := func() uint64 {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}

	t.Run("when a drive ticks many frames", func(t *testing.T) {
		now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.Local)
		d, err := NewDrive(DriveOptions{Home: helpers.ParkedWeek(t, 200, now, "Battery check compares as text"), Now: now})
		require.NoError(t, err)
		require.NoError(t, d.Run(10))
		after10 := heap()
		require.NoError(t, d.Run(50))
		after60 := heap()

		t.Run("it should hold no more heap after 60 ticks than after 10, within a few MB (item 104)", func(t *testing.T) {
			const few = 8 << 20
			assert.Less(t, int64(after60)-int64(after10), int64(few))
		})
	})

	t.Run("when the actor beside the loop is a tick late handing the resume on (item 298)", func(t *testing.T) {
		const title = "Battery check compares as text"
		now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.Local)
		d, err := NewDrive(DriveOptions{Home: helpers.ParkedWeek(t, 2, now, title), Now: now})
		require.NoError(t, err)
		require.NoError(t, d.Run(10))
		var id string
		for _, s := range d.Sessions() {
			if d.Title(s) == title {
				id = s
			}
		}
		require.NotEmpty(t, id)
		require.NoError(t, d.Click(id))
		require.NoError(t, d.Run(5))
		release := make(chan struct{})
		d.game.actor = lateActor{d: d, release: release}
		time.AfterFunc(100*time.Millisecond, func() { close(release) })
		require.NoError(t, d.Press("Enter"))
		ticks, err := d.UntilActed(func(a city.Action) bool { return a.Kind == city.ActionResume }, 100000)
		require.NoError(t, err)

		t.Run("it should count the tick the city handed it over on", func(t *testing.T) {
			assert.Equal(t, 1, ticks)
		})
	})

	t.Run("when the actor beside the loop never answers", func(t *testing.T) {
		now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.Local)
		d, err := NewDrive(DriveOptions{Home: helpers.ParkedWeek(t, 2, now, "Battery check compares as text"), Now: now})
		require.NoError(t, err)
		require.NoError(t, d.Run(10))
		require.NoError(t, d.Click(d.Sessions()[0]))
		require.NoError(t, d.Run(5))
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		d.game.actor = lateActor{d: d, release: release}
		limit := driveSettleLimit
		driveSettleLimit = 50 * time.Millisecond
		t.Cleanup(func() { driveSettleLimit = limit })
		require.NoError(t, d.Press("Enter"))
		_, err = d.UntilActed(func(a city.Action) bool { return a.Kind == city.ActionResume }, 10)

		t.Run("it should fail the tick rather than hang", func(t *testing.T) {
			assert.ErrorContains(t, err, "an action beside the loop did not finish")
		})
	})
}
