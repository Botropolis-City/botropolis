package daemon_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

// slowFirstLoad is a loader whose first load holds until it is let go,
// having read 40 of 120 sessions.
type slowFirstLoad struct {
	loading, release chan struct{}
}

func (s *slowFirstLoad) Load(now time.Time) (state.Snapshot, error) {
	close(s.loading)
	<-s.release
	return state.Snapshot{At: now}, nil
}

func (s *slowFirstLoad) WatchDirs() []string { return nil }

func (s *slowFirstLoad) LoadProgress() state.LoadProgress {
	return state.LoadProgress{Done: 40, Total: 120}
}

// Item 159: every request but a hook event waits for the first load
// (item 143), so a city opened during it had nothing to show.
func TestLoadingWhileHeld(t *testing.T) {
	t.Run("when the daemon holds requests for its first load, 40 sessions of 120 in", func(t *testing.T) {
		loader := &slowFirstLoad{loading: make(chan struct{}), release: make(chan struct{})}
		d := daemon.NewWith(loader, func() time.Time { return now })
		d.HoldUntilLoaded()
		sock := serve(t, d)
		go func() { _ = d.Rescan() }()
		<-loader.loading
		t.Cleanup(func() { close(loader.release) })
		answered := make(chan struct{})
		var got state.LoadProgress
		var err error
		go func() {
			defer close(answered)
			got, err = proto.Loading(sock)
		}()
		select {
		case <-answered:
		case <-time.After(5 * time.Second):
			require.FailNow(t, "the loading request was held with the rest")
		}
		require.NoError(t, err)

		t.Run("it should answer a loading request with how far the load has got", func(t *testing.T) {
			assert.Equal(t, state.LoadProgress{Done: 40, Total: 120}, got)
		})
	})
}
