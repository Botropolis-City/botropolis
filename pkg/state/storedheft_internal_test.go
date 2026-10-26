package state

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/claude"
)

// movingStats is a store whose every pass moves one session's heft.
type movingStats struct {
	mu   sync.Mutex
	id   string
	next claude.Usage
}

func (m *movingStats) Ingest(string, time.Time, func()) error { return nil }

func (m *movingStats) Hefts() (map[string]claude.Usage, error) { return nil, nil }

func (m *movingStats) MovedHefts() (map[string]claude.Usage, []string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.next.Output++
	return map[string]claude.Usage{m.id: m.next}, []string{m.id}, nil
}

func heftsOf(n int) map[string]claude.Usage {
	hefts := make(map[string]claude.Usage, n)
	for i := range n {
		hefts[fmt.Sprintf("00000000-0000-4213-b000-%012d", i)] = claude.Usage{Input: int64(i), Output: 1}
	}
	return hefts
}

// Item 213: a pass that moved one heft cloned every session's, which was
// a tenth of what the daemon allocated among 2,000 sessions with one
// writing. The pass changes the map in place, and a load reads it under
// the lock.
func TestStoredHeftsPass(t *testing.T) {
	const moved = "00000000-0000-4213-b000-000000000007"
	passOf := func(h *storedHefts) func() {
		dir := t.TempDir()
		return func() {
			h.mu.Lock()
			h.running = true
			h.mu.Unlock()
			h.pass(dir, time.Unix(0, 0), true, nil, false)
		}
	}
	newHefts := func(n int) *storedHefts {
		c := &fakeClock{at: time.Unix(0, 0)}
		return &storedHefts{src: &movingStats{id: moved}, yield: func() {}, pace: c.pace(1), hefts: heftsOf(n), read: true, pending: map[string]bool{}, clock: c.now}
	}

	t.Run("when a pass moves one session's heft among 2,000", func(t *testing.T) {
		h := newHefts(2000)
		pass := passOf(h)
		var perPass uint64
		onOwnThread(func() {
			pass()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			for range 10 {
				pass()
			}
			runtime.ReadMemStats(&after)
			perPass = (after.TotalAlloc - before.TotalAlloc) / 10
		})

		t.Run("it should allocate under 4 KiB, not a copy of the map", func(t *testing.T) {
			assert.Less(t, perPass, uint64(4<<10))
		})

		t.Run("it should hold the moved heft", func(t *testing.T) {
			assert.Equal(t, int64(11), h.hefts[moved].Output)
		})

		t.Run("it should keep every other heft", func(t *testing.T) {
			assert.Len(t, h.hefts, 2000)
		})
	})

	t.Run("when a load applies the hefts while a pass moves one", func(t *testing.T) {
		h := newHefts(200)
		h.running = true
		sessions := []Session{{ID: moved, Usage: claude.Usage{Partial: true}}}
		pass := passOf(h)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for range 50 {
				pass()
			}
		}()
		var outputs []int64
		for range 50 {
			h.apply(sessions, t.TempDir(), time.Unix(0, 0), nil, false)
			outputs = append(outputs, sessions[0].Heft)
		}
		<-done

		t.Run("it should see a heft the pass set, never a torn one", func(t *testing.T) {
			for _, heft := range outputs {
				assert.Positive(t, heft)
			}
		})
	})
}
