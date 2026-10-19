package daemon_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

// Item 144: demolishing a session tells the daemon, which is the only
// thing that writes the ledger, to forget it.
func TestServeForget(t *testing.T) {
	gone := "dddddddd-0000-0000-0000-000000000004"
	shown := func(d *daemon.Daemon, id string) bool {
		for _, s := range d.Snapshot().Sessions {
			if s.ID == id {
				return true
			}
		}
		return false
	}

	t.Run("when a client asks the daemon to forget a demolished session", func(t *testing.T) {
		ledger := filepath.Join(t.TempDir(), "sessions.json")
		require.NoError(t, state.Ledger{gone: {CWD: cwd, StartedAt: t0, LastActivity: t0}}.Save(ledger))
		d := daemon.New(idleHome(t, "interactive").Path, probes, func() time.Time { return now }, daemon.WithLedger(ledger))
		require.NoError(t, d.Rescan())
		require.True(t, shown(d, gone))
		client, err := proto.Dial(serve(t, d))
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Close() })
		err = client.Forget(gone)

		t.Run("it should answer without an error", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should drop it from the next snapshot", func(t *testing.T) {
			assert.False(t, shown(d, gone))
		})

		t.Run("it should keep the session that is still there", func(t *testing.T) {
			assert.True(t, shown(d, sid))
		})
	})

	t.Run("when a client asks the daemon to forget a session whose transcript remains", func(t *testing.T) {
		ledger := filepath.Join(t.TempDir(), "sessions.json")
		home := idleHome(t, "interactive")
		home.RemoveSession(pid)
		d := daemon.New(home.Path, probes, func() time.Time { return now }, daemon.WithLedger(ledger))
		require.NoError(t, d.Rescan())
		require.True(t, shown(d, sid))
		client, err := proto.Dial(serve(t, d))
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Close() })
		require.NoError(t, client.Forget(sid))

		t.Run("it should drop it from the next snapshot", func(t *testing.T) {
			assert.False(t, shown(d, sid))
		})

		t.Run("and the daemon restarts on the same ledger", func(t *testing.T) {
			restarted := daemon.New(home.Path, probes, func() time.Time { return now }, daemon.WithLedger(ledger))
			require.NoError(t, restarted.Rescan())

			t.Run("it should not be in its first snapshot", func(t *testing.T) {
				assert.False(t, shown(restarted, sid))
			})
		})
	})

	t.Run("when a session was demolished while no daemon ran", func(t *testing.T) {
		ledger := filepath.Join(t.TempDir(), "sessions.json")
		require.NoError(t, state.Ledger{gone: {CWD: cwd, StartedAt: t0, LastActivity: t0}}.Save(ledger))
		require.NoError(t, state.NoteForgotten(ledger, gone))

		t.Run("and the daemon then starts", func(t *testing.T) {
			d := daemon.New(idleHome(t, "interactive").Path, probes, func() time.Time { return now }, daemon.WithLedger(ledger))
			require.NoError(t, d.Rescan())

			t.Run("it should not be in the first snapshot", func(t *testing.T) {
				assert.False(t, shown(d, gone))
			})
		})
	})
}
