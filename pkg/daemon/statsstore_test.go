package daemon_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

// Item 190: the daemon says what became of its statistics store, to a
// client asking what it is and on every snapshot, so a refusal reaches
// doctor and the map and not only the journal.
func TestServeStatsStore(t *testing.T) {
	refused := state.StatsStore{State: state.StoreRefused, Path: "/s/stats.db", Reason: "damaged: move it aside"}

	t.Run("when its store was refused", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		d.SetStatsStore(refused)
		sock := serve(t, d)

		t.Run("and a client asks what it is", func(t *testing.T) {
			_, store, err := proto.AskStore(sock)
			require.NoError(t, err)

			t.Run("it should send the store's state with the reason", func(t *testing.T) {
				assert.Equal(t, &refused, store)
			})
		})

		t.Run("and a client asks for a snapshot", func(t *testing.T) {
			client, err := proto.Dial(sock)
			require.NoError(t, err)
			t.Cleanup(func() { _ = client.Close() })
			snapshot, err := client.Snapshot()
			require.NoError(t, err)

			t.Run("it should carry the store's state", func(t *testing.T) {
				assert.Equal(t, &refused, snapshot.Store)
			})
		})
	})

	t.Run("when it was never told of a store", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))

		t.Run("it should carry no store state on its snapshot", func(t *testing.T) {
			assert.Nil(t, d.Snapshot().Store)
		})
	})
}
