package render

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/state"
)

// Item 114: the feed saves the ledger into the state directory, so Run
// must not return, and its caller remove a private directory, while a
// save is still under way.
func TestStartFeed(t *testing.T) {
	t.Run("when the feed is stopped while it is still writing", func(t *testing.T) {
		var finished atomic.Bool
		feed := func(ctx context.Context, _ func(state.Snapshot), _ func([]events.Event)) {
			<-ctx.Done()
			time.Sleep(50 * time.Millisecond)
			finished.Store(true)
		}
		stop := startFeed(context.Background(), feed, func(state.Snapshot) {}, func([]events.Event) {})
		stop()

		t.Run("it should wait for the feed to return", func(t *testing.T) {
			assert.True(t, finished.Load())
		})
	})

	t.Run("when the run's context is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		returned := make(chan struct{})
		feed := func(ctx context.Context, _ func(state.Snapshot), _ func([]events.Event)) {
			<-ctx.Done()
			close(returned)
		}
		stop := startFeed(ctx, feed, func(state.Snapshot) {}, func([]events.Event) {})
		t.Cleanup(stop)
		cancel()

		t.Run("it should stop the feed", func(t *testing.T) {
			select {
			case <-returned:
			case <-time.After(time.Second):
				t.Fatal("the feed was still running a second after its context was cancelled")
			}
		})
	})

	t.Run("when there is no feed", func(t *testing.T) {
		stop := startFeed(context.Background(), nil, func(state.Snapshot) {}, func([]events.Event) {})

		t.Run("it should stop at once", func(t *testing.T) {
			assert.NotPanics(t, stop)
		})
	})
}
