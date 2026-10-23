package daemon_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 205: a background pass that moved a heft rescanned at once, so
// with a session writing, each pass started the next and the rescans
// outran the watcher's pace. It now asks the watcher instead.
func TestRequestRescan(t *testing.T) {
	const (
		paced = time.Second
		slack = 200 * time.Millisecond
	)
	t.Run("when the watcher has just rescanned and a background pass asks for another", func(t *testing.T) {
		home := idleHome(t, "interactive")
		c := &loadClock{Loader: state.NewLoader(home.Path, probes)}
		d := daemon.NewWith(c, func() time.Time { return now })
		require.NoError(t, d.Rescan())
		watch(t, d)
		time.Sleep(paced + slack)
		from := time.Now()
		home.AppendTranscript(sid, cwd, helpers.AssistantReply(sid, "msg_request_1", stamp(30*time.Second)))
		require.Eventually(t, func() bool { return len(c.since(from)) == 1 }, settle, tick)
		first := c.since(from)[0]
		for i := 0; i < 5; i++ {
			d.RequestRescan()
		}
		require.Eventually(t, func() bool { return len(c.since(from)) >= 2 }, settle, tick)
		second := c.since(from)[1]

		t.Run("it should wait for the pace before rescanning again", func(t *testing.T) {
			assert.GreaterOrEqual(t, second.Sub(first), paced-slack)
		})

		t.Run("it should rescan once for every ask in between", func(t *testing.T) {
			time.Sleep(paced + slack)
			assert.Len(t, c.since(from), 2)
		})
	})

	t.Run("when the watcher is not up yet", func(t *testing.T) {
		home := idleHome(t, "interactive")
		c := &loadClock{Loader: state.NewLoader(home.Path, probes)}
		d := daemon.NewWith(c, func() time.Time { return now })
		from := time.Now()
		d.RequestRescan()

		t.Run("it should rescan at once", func(t *testing.T) {
			assert.Len(t, c.since(from), 1)
		})
	})
}
