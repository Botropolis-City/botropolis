package stats_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
)

func TestSessionCost(t *testing.T) {
	t.Run("when a session's reads straddle a price change", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			reply{session: sessionA, id: "msg_1", at: "2026-10-06T23:00:00.000Z", cwd: lib, model: "claude-sonnet-5-5", usage: usage{CacheRead: million}}.line(),
			reply{session: sessionA, id: "msg_2", at: "2026-10-07T01:00:00.000Z", cwd: lib, model: "claude-sonnet-5-5", usage: usage{CacheRead: million}}.line(),
		)
		total, _, err := f.store.SessionCost(sessionA, stats.Prices)
		require.NoError(t, err)

		t.Run("it should price each by the row in force when it was sent", func(t *testing.T) {
			assert.InDelta(t, 0.30, total.Low, 1e-9)
		})
	})

	t.Run("when a session's cache writes have an unknown lifetime", func(t *testing.T) {
		f := newFixture(t)
		f.seed(reply{session: sessionA, id: "msg_1", at: replyAt, cwd: lib, model: "claude-opus-5-5", usage: usage{CacheCreate: million}}.line())
		total, _, err := f.store.SessionCost(sessionA, stats.Prices)
		require.NoError(t, err)

		t.Run("it should give the range from 5 m to 1 h", func(t *testing.T) {
			assert.Equal(t, [2]float64{5, 8}, [2]float64{total.Low, total.High})
		})
	})
}
