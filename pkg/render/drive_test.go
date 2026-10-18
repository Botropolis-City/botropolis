package render

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

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
}
