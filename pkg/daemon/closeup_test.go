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

// Item 86, slice 4: the map records a close-up through the daemon, which
// is the only writer of its state.
func TestServeCloseUp(t *testing.T) {
	t.Run("when a client closes up", func(t *testing.T) {
		record := filepath.Join(t.TempDir(), "closeup.json")
		d := daemon.New(idleHome(t, "interactive").Path, probes, func() time.Time { return now }, daemon.WithCloseUp(record))
		require.NoError(t, d.Rescan())
		client, err := proto.Dial(serve(t, d))
		require.NoError(t, err)
		defer func() { _ = client.Close() }()
		err = client.CloseUp([]string{sid}, []string{sid})
		got, loadErr := state.LoadCloseUp(record)
		require.NoError(t, loadErr)

		t.Run("the answer should carry no error", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("the record should hold the close-up", func(t *testing.T) {
			assert.Equal(t, state.CloseUp{At: now, Open: []string{sid}, Pinned: []string{sid}}, got)
		})
	})
}
