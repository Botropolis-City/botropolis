package daemon_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 86: the map packs a session up, and unpacks it, through the
// daemon, which is the only thing that writes the ledger.
func TestServePack(t *testing.T) {
	packing := func(t *testing.T) (*daemon.Daemon, *proto.Client) {
		t.Helper()
		ledger := filepath.Join(t.TempDir(), "sessions.json")
		d := daemon.New(idleHome(t, "interactive").Path, probes, func() time.Time { return now }, daemon.WithLedger(ledger))
		require.NoError(t, d.Rescan())
		client, err := proto.Dial(serve(t, d))
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Close() })
		return d, client
	}

	t.Run("when a client packs a session up", func(t *testing.T) {
		d, client := packing(t)
		err := client.Pack(sid, true)

		t.Run("it should answer without an error", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should mark the session packed in the next snapshot", func(t *testing.T) {
			assert.True(t, only(t, d).Packed)
		})

		t.Run("and the daemon rescans again", func(t *testing.T) {
			require.NoError(t, d.Rescan())

			t.Run("it should keep it packed (item 278)", func(t *testing.T) {
				assert.True(t, only(t, d).Packed)
			})
		})

		t.Run("and then unpacks it", func(t *testing.T) {
			require.NoError(t, client.Pack(sid, false))

			t.Run("it should not mark it packed", func(t *testing.T) {
				assert.False(t, only(t, d).Packed)
			})
		})
	})

	t.Run("when a client packs a session that then works (item 278)", func(t *testing.T) {
		home := idleHome(t, "interactive")
		ledger := filepath.Join(t.TempDir(), "sessions.json")
		d := daemon.New(home.Path, probes, func() time.Time { return now }, daemon.WithLedger(ledger))
		require.NoError(t, d.Rescan())
		require.NoError(t, d.Pack(sid, true))
		home.AppendTranscript(sid, cwd, helpers.AssistantReply(sid, "msg_02", stamp(2*time.Hour)))
		require.NoError(t, d.Rescan())

		t.Run("it should bring it back", func(t *testing.T) {
			assert.False(t, only(t, d).Packed)
		})
	})

	t.Run("when the daemon keeps no ledger", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		client, err := proto.Dial(serve(t, d))
		require.NoError(t, err)
		defer func() { _ = client.Close() }()

		t.Run("it should refuse, saying why", func(t *testing.T) {
			assert.ErrorContains(t, client.Pack(sid, true), "keeps no ledger")
		})
	})
}
