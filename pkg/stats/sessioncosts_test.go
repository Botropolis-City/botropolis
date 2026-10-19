package stats_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
)

func TestSessionCosts(t *testing.T) {
	t.Run("when two sessions hold messages, one copied by a fork", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA,
			modelMsg("msg_1", "2026-10-05T14:00:00.000Z", "claude-opus-5-5", usage{Output: million}),
		)
		f.transcript(sessionB,
			reply{session: sessionB, id: "msg_1", at: "2026-10-05T14:00:00.000Z", cwd: lib, model: "claude-opus-5-5", usage: usage{Output: million}}.line(),
			reply{session: sessionB, id: "msg_2", at: "2026-10-05T15:00:00.000Z", cwd: lib, model: "claude-haiku-4-5", usage: usage{Output: million}}.line(),
		)
		f.ingest()
		costs, err := f.store.SessionCosts()
		require.NoError(t, err)

		t.Run("it should cost each session's own messages", func(t *testing.T) {
			assert.InDelta(t, 20, costs[sessionA].Low, 1e-9)
		})

		t.Run("it should count the copied message in the fork too", func(t *testing.T) {
			assert.InDelta(t, 25, costs[sessionB].Low, 1e-9)
		})
	})
}

func TestLedgerSessionCosts(t *testing.T) {
	t0 := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	f := newFixture(t)
	path := filepath.Join(t.TempDir(), "stats.db")
	f.store = open(t, path)
	f.seed(modelMsg("msg_1", "2026-10-19T11:00:00.000Z", "claude-opus-5-5", usage{Output: million}))
	l := newLedger(t, path)
	_, err := l.SessionCosts(t0)
	require.NoError(t, err)
	f.transcript(sessionA,
		modelMsg("msg_1", "2026-10-19T11:00:00.000Z", "claude-opus-5-5", usage{Output: million}),
		modelMsg("msg_2", "2026-10-19T11:30:00.000Z", "claude-opus-5-5", usage{Output: million}),
	)
	f.ingest()

	t.Run("when asked again within the cache's life, it should give the first answer", func(t *testing.T) {
		costs, err := l.SessionCosts(t0.Add(stats.LedgerSessions - time.Millisecond))
		require.NoError(t, err)
		assert.InDelta(t, 20, costs[sessionA].Low, 1e-9)
	})

	t.Run("when asked again after it, it should read the store again", func(t *testing.T) {
		costs, err := l.SessionCosts(t0.Add(stats.LedgerSessions))
		require.NoError(t, err)
		assert.InDelta(t, 40, costs[sessionA].Low, 1e-9)
	})

	t.Run("when there is no ledger, it should refuse", func(t *testing.T) {
		var none *stats.Ledger
		_, err := none.SessionCosts(t0)
		assert.Error(t, err)
	})
}
