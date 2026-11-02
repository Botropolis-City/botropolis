package main

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 330: the quiet check ran before make test-performance queued for the
// landing lock, so a landing could run between the check and the stress
// city's reading. settle runs inside the lock.

func TestParseLoadavg(t *testing.T) {
	t.Run("when /proc/loadavg reads as the kernel writes it", func(t *testing.T) {
		l, err := parseLoadavg("1.49 3.86 8.16 2/1234 5678\n")
		require.NoError(t, err)

		t.Run("it should read the three averages", func(t *testing.T) {
			assert.Equal(t, load{1.49, 3.86, 8.16}, l)
		})
	})

	for _, bad := range []string{"", "1.49 3.86", "a b c 1/2 3"} {
		t.Run("when it reads "+`"`+bad+`"`, func(t *testing.T) {
			_, err := parseLoadavg(bad)

			t.Run("it should refuse it", func(t *testing.T) {
				assert.Error(t, err)
			})
		})
	}
}

func TestSettle(t *testing.T) {
	bars := quietBars{One: 3, Five: 4}
	const poll, limit = 10 * time.Second, time.Minute

	// run settles over loads read in turn, the last repeated, on a clock
	// that each sleep advances.
	run := func(ci bool, loads ...load) (string, int, error) {
		var clock time.Duration
		looks := 0
		read := func() (load, error) {
			l := loads[min(looks, len(loads)-1)]
			looks++
			return l, nil
		}
		said, err := settle(settleOptions{
			Read: read, Sleep: func(d time.Duration) { clock += d }, Since: func() time.Duration { return clock },
			Bars: bars, Poll: poll, Limit: limit, CI: ci,
		})
		return said, looks, err
	}

	for _, c := range []struct {
		scenario string
		ci       bool
		loads    []load
		said     string
		looks    int
	}{
		{
			scenario: "when the load is under both bars at the first look",
			loads:    []load{{1, 2, 3}},
			said:     "quiet after 0s: load 1.00 2.00 3.00, the bars 3 over 1 minute and 4 over 5",
			looks:    1,
		},
		{
			scenario: "when the load falls under both bars at the third look",
			loads:    []load{{5, 6, 7}, {3.5, 4.5, 7}, {2, 3, 7}},
			said:     "quiet after 20s: load 2.00 3.00 7.00, the bars 3 over 1 minute and 4 over 5",
			looks:    3,
		},
		{
			scenario: "when only the 1-minute load is under its bar",
			loads:    []load{{1, 5, 7}, {1, 3.9, 7}},
			said:     "quiet after 10s: load 1.00 3.90 7.00, the bars 3 over 1 minute and 4 over 5",
			looks:    2,
		},
		{
			scenario: "when the load never falls under within the limit",
			loads:    []load{{9, 9, 9}},
			said:     "not quiet after 1m0s: load 9.00 9.00 9.00, the bars 3 over 1 minute and 4 over 5; measuring anyway, so read this run's figures as a busy machine's",
			looks:    7,
		},
		{
			scenario: "when CI is set",
			ci:       true,
			loads:    []load{{9, 9, 9}},
			said:     "not waiting for a quiet machine: CI is set, and a shared runner reports rather than gates (item 305); load 9.00 9.00 9.00",
			looks:    1,
		},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			said, looks, err := run(c.ci, c.loads...)
			require.NoError(t, err)

			t.Run("it should say so", func(t *testing.T) {
				assert.Equal(t, c.said, said)
			})

			t.Run("it should look at the load as often as it waited", func(t *testing.T) {
				assert.Equal(t, c.looks, looks)
			})
		})
	}

	t.Run("when the load cannot be read", func(t *testing.T) {
		_, err := settle(settleOptions{
			Read:  func() (load, error) { return load{}, errors.New("no /proc") },
			Sleep: func(time.Duration) {}, Since: func() time.Duration { return 0 },
			Bars: bars, Poll: poll, Limit: limit,
		})

		t.Run("it should fail rather than call the machine quiet", func(t *testing.T) {
			assert.ErrorContains(t, err, "no /proc")
		})
	})
}
