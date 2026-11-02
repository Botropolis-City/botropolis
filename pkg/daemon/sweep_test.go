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

// Item 333: the quiet sweep packs up, in one move, the sessions the map
// counted, each checked again against the daemon's own snapshot.
func TestPackQuiet(t *testing.T) {
	quietFor := 30 * time.Minute
	sweeping := func(t *testing.T, alive bool) (*daemon.Daemon, *proto.Client) {
		t.Helper()
		ledger := filepath.Join(t.TempDir(), "sessions.json")
		p := state.Probes{Alive: func(int) bool { return alive }, Attached: func(string) bool { return false }}
		d := daemon.New(idleHome(t, "interactive").Path, p, func() time.Time { return now }, daemon.WithLedger(ledger))
		require.NoError(t, d.Rescan())
		client, err := proto.Dial(serve(t, d))
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Close() })
		return d, client
	}

	t.Run("when a client sweeps a session that is still quiet", func(t *testing.T) {
		d, client := sweeping(t, false)
		packed, err := client.PackQuiet([]string{sid}, quietFor)
		require.NoError(t, err)

		t.Run("it should answer with how many it packed", func(t *testing.T) {
			assert.Equal(t, 1, packed)
		})

		t.Run("it should mark the session packed in the next snapshot", func(t *testing.T) {
			assert.True(t, only(t, d).Packed)
		})

		t.Run("it should mark the session swept in the next snapshot", func(t *testing.T) {
			assert.True(t, only(t, d).Swept)
		})
	})

	t.Run("when a client sweeps a session that is no longer quiet", func(t *testing.T) {
		d, client := sweeping(t, true)
		packed, err := client.PackQuiet([]string{sid}, quietFor)
		require.NoError(t, err)

		t.Run("it should answer that it packed none", func(t *testing.T) {
			assert.Equal(t, 0, packed)
		})

		t.Run("it should leave the session where it is", func(t *testing.T) {
			assert.False(t, only(t, d).Packed)
		})
	})

	t.Run("when a client sweeps a session the daemon does not know", func(t *testing.T) {
		_, client := sweeping(t, false)
		packed, err := client.PackQuiet([]string{"not-a-session"}, quietFor)
		require.NoError(t, err)

		t.Run("it should answer that it packed none", func(t *testing.T) {
			assert.Equal(t, 0, packed)
		})
	})

	t.Run("when the daemon keeps no ledger", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		client, err := proto.Dial(serve(t, d))
		require.NoError(t, err)
		defer func() { _ = client.Close() }()
		_, err = client.PackQuiet([]string{sid}, quietFor)

		t.Run("it should refuse, saying why", func(t *testing.T) {
			assert.ErrorContains(t, err, "keeps no ledger")
		})
	})
}
