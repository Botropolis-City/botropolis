package state_test

import (
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// priorityStats is a statistics store that notes the CPU priority of
// the thread it is asked to ingest on.
type priorityStats struct {
	priority chan int
}

func (p *priorityStats) Ingest(string, time.Time, func()) error {
	raw, _ := syscall.Getpriority(syscall.PRIO_PROCESS, syscall.Gettid())
	p.priority <- 20 - raw
	return nil
}

func (p *priorityStats) Hefts() (map[string]claude.Usage, error) {
	return map[string]claude.Usage{}, nil
}

// Items 138 and 159.
func TestStoredHeftsPriority(t *testing.T) {
	t.Run("when the background pass ingests", func(t *testing.T) {
		src := &priorityStats{priority: make(chan int, 1)}
		_, err := state.NewLoader(helpers.NewHome(t).Path, alive).WithStats(src, nil).Load(now)
		require.NoError(t, err)
		var got int
		select {
		case got = <-src.priority:
		case <-time.After(30 * time.Second):
			require.FailNow(t, "the pass never ingested")
		}

		t.Run("it should ingest at nice 10, unless the process is lower already", func(t *testing.T) {
			raw, err := syscall.Getpriority(syscall.PRIO_PROCESS, syscall.Getpid())
			require.NoError(t, err)
			assert.Equal(t, max(20-raw, 10), got)
		})
	})
}
