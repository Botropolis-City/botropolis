package state

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// fakeClock is a wall clock that moves only when told to, or when the
// pace sleeps on it.
type fakeClock struct {
	at    time.Time
	slept []time.Duration
}

func (c *fakeClock) now() time.Time { return c.at }

func (c *fakeClock) sleep(d time.Duration) {
	c.slept = append(c.slept, d)
	c.at = c.at.Add(d)
}

func (c *fakeClock) pace(fraction float64) *pace {
	p := &pace{fraction: fraction, now: c.now, sleep: c.sleep}
	p.start()
	return p
}

func (c *fakeClock) totalSlept() time.Duration {
	var total time.Duration
	for _, d := range c.slept {
		total += d
	}
	return total
}

// Item 159: a background pass takes at most a fraction of the wall
// clock, so a first launch never takes the machine.
func TestPace(t *testing.T) {
	const work = 100 * time.Millisecond

	for _, tc := range []struct {
		fraction float64
		sleep    time.Duration
	}{
		{0.5, 100 * time.Millisecond},
		{0.25, 300 * time.Millisecond},
		{1, 0},
	} {
		t.Run(fmt.Sprintf("when the fraction is %v", tc.fraction), func(t *testing.T) {
			t.Run("and a chunk of work took 100 ms", func(t *testing.T) {
				c := &fakeClock{at: time.Unix(0, 0)}
				p := c.pace(tc.fraction)
				c.at = c.at.Add(work)
				p.yield(func() {})

				t.Run(fmt.Sprintf("it should sleep %v", tc.sleep), func(t *testing.T) {
					assert.Equal(t, tc.sleep, c.totalSlept())
				})
			})
		})
	}

	t.Run("when no time has passed since the last yield", func(t *testing.T) {
		c := &fakeClock{at: time.Unix(0, 0)}
		c.pace(0.5).yield(func() {})

		t.Run("it should not sleep", func(t *testing.T) {
			assert.Zero(t, c.totalSlept())
		})
	})

	t.Run("when the pace has slept", func(t *testing.T) {
		c := &fakeClock{at: time.Unix(0, 0)}
		p := c.pace(0.5)
		c.at = c.at.Add(work)
		p.yield(func() {})
		c.at = c.at.Add(work)
		p.yield(func() {})

		t.Run("it should not count the sleep as work for the next chunk", func(t *testing.T) {
			assert.Equal(t, []time.Duration{work, work}, c.slept)
		})
	})

	t.Run("when the pass waited at the gate", func(t *testing.T) {
		c := &fakeClock{at: time.Unix(0, 0)}
		p := c.pace(0.5)
		c.at = c.at.Add(work)
		p.yield(func() { c.at = c.at.Add(time.Second) })
		c.at = c.at.Add(work)
		p.yield(func() {})

		t.Run("it should not count the wait as work", func(t *testing.T) {
			assert.Equal(t, []time.Duration{work, work}, c.slept)
		})
	})
}
