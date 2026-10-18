package city_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestSkipLevel(t *testing.T) {
	cases := []struct {
		percent float64
		want    int
	}{{49, 0}, {50, 1}, {64, 1}, {65, 2}, {80, 3}, {94, 3}, {95, 4}, {100, 4}}
	for _, c := range cases {
		t.Run(fmt.Sprintf("when a live session's context is at %g%%", c.percent), func(t *testing.T) {
			s := state.Session{State: state.Working, ContextPercent: c.percent}

			t.Run(fmt.Sprintf("it should fill the skip to level %d", c.want), func(t *testing.T) {
				assert.Equal(t, c.want, city.SkipLevel(s))
			})
		})
	}

	t.Run("when the session is stopped", func(t *testing.T) {
		t.Run("and its context was at 90%", func(t *testing.T) {
			t.Run("it should have no skip", func(t *testing.T) {
				assert.Zero(t, city.SkipLevel(state.Session{State: state.Resting, ContextPercent: 90}))
			})
		})
	})
}

// Item 109: a compaction is a skip collected and a course poured, as one
// moment of about 4.8 s.
func TestCompactionJob(t *testing.T) {
	start := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	clocked := func(t *testing.T, compactions int) (*city.Scene, *time.Time) {
		t.Helper()
		clock := start
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return clock }, time.UTC)
		s.Resize(800, 600)
		sess := session("a", cinders, state.Working)
		sess.ContextPercent, sess.Compactions = 90, compactions
		s.SetSnapshot(snapshot(sess))
		return s, &clock
	}
	compact := func(s *city.Scene, by int) {
		sess := session("a", cinders, state.Working)
		sess.ContextPercent, sess.Compactions = 12, 1+by
		s.SetSnapshot(snapshot(sess))
	}

	t.Run("when a live session's compactions rise by one", func(t *testing.T) {
		s, clock := clocked(t, 1)
		compact(s, 1)
		_, started := s.CompactionJob("a")

		t.Run("it should start one job", func(t *testing.T) {
			assert.True(t, started)
		})

		t.Run("it should remember the skip as it was before", func(t *testing.T) {
			job, _ := s.CompactionJob("a")
			assert.Equal(t, 3, job.SkipLevel)
		})

		t.Run("and 1 s in", func(t *testing.T) {
			*clock = start.Add(1000 * time.Millisecond)
			job, ok := s.CompactionJob("a")
			require.True(t, ok)

			t.Run("it should have the skip truck at the skip", func(t *testing.T) {
				assert.Equal(t, [2]any{true, 1.0}, [2]any{job.SkipTruck.Present, job.SkipTruck.Along})
			})

			t.Run("it should not have the mixer yet", func(t *testing.T) {
				assert.False(t, job.Mixer.Present)
			})
		})

		t.Run("and 3.2 s in", func(t *testing.T) {
			*clock = start.Add(3200 * time.Millisecond)
			job, ok := s.CompactionJob("a")
			require.True(t, ok)

			t.Run("it should have the skip truck leaving with the skip", func(t *testing.T) {
				assert.True(t, job.SkipTruck.Present && job.SkipTruck.Loaded && job.SkipTruck.Along > 1)
			})

			t.Run("it should have the mixer pouring", func(t *testing.T) {
				assert.Equal(t, [2]any{true, 1.0}, [2]any{job.Mixer.Present, job.Mixer.Along})
			})

			t.Run("it should have the new course part poured", func(t *testing.T) {
				assert.True(t, job.Pour > 0 && job.Pour < 1, "%v", job.Pour)
			})
		})

		t.Run("and 4.8 s in", func(t *testing.T) {
			*clock = start.Add(4800 * time.Millisecond)

			t.Run("it should have finished the job and taken both trucks away", func(t *testing.T) {
				_, ok := s.CompactionJob("a")
				assert.False(t, ok)
			})
		})
	})

	t.Run("when the city opens on a session that has already compacted", func(t *testing.T) {
		s, _ := clocked(t, 4)

		t.Run("it should start no job", func(t *testing.T) {
			_, ok := s.CompactionJob("a")
			assert.False(t, ok)
		})
	})

	t.Run("when compactions rise by two in one snapshot", func(t *testing.T) {
		s, clock := clocked(t, 1)
		compact(s, 2)
		*clock = start.Add(4800 * time.Millisecond)
		compact(s, 2)

		t.Run("it should start one job", func(t *testing.T) {
			_, ok := s.CompactionJob("a")
			assert.False(t, ok)
		})
	})

	t.Run("when compactions rise and the city is still", func(t *testing.T) {
		s, _ := clocked(t, 1)
		s.SetReducedMotion(true)
		compact(s, 1)

		t.Run("it should start no job", func(t *testing.T) {
			_, ok := s.CompactionJob("a")
			assert.False(t, ok)
		})
	})
}
