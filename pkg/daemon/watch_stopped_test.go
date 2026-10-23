package daemon_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 205: a rescan looks only at the stopped transcripts the watcher
// says changed, so the watcher holds every project directory, not only
// those a live session is in.
func TestWatchStoppedTranscripts(t *testing.T) {
	const (
		stoppedSid = "eeeeeeee-0000-4205-8000-000000000001"
		elsewhere  = "/home/user/src/elsewhere"
	)
	t.Run("when a stopped session's transcript grows in a project no live session is in", func(t *testing.T) {
		home := idleHome(t, "interactive")
		home.Transcript(stoppedSid, elsewhere,
			helpers.UserPrompt(stoppedSid, elsewhere, stamp(0)), helpers.AssistantReply(stoppedSid, "msg_205_a", stamp(0)))
		d := newDaemon(t, home)
		watch(t, d)
		home.AppendTranscript(stoppedSid, elsewhere, helpers.AssistantReply(stoppedSid, "msg_205_b", stamp(time.Hour)))

		t.Run("it should rescan and show the change", func(t *testing.T) {
			assert.Eventually(t, func() bool {
				s, _ := session(d, stoppedSid)
				return s.LastActivity.Equal(t0.Add(time.Hour))
			}, settle, tick)
		})
	})

	t.Run("when the watch on a stopped session's project cannot be added", func(t *testing.T) {
		home := idleHome(t, "interactive")
		path := home.Transcript(stoppedSid, elsewhere,
			helpers.UserPrompt(stoppedSid, elsewhere, stamp(0)), helpers.AssistantReply(stoppedSid, "msg_205_a", stamp(0)))
		d := newDaemon(t, home)
		daemon.SetAddWatch(d, func(w *fsnotify.Watcher, dir string) error {
			if dir == filepath.Dir(path) {
				return errors.New("no space left on device")
			}
			return w.Add(dir)
		})
		watch(t, d)
		require.NoError(t, d.Rescan())
		home.AppendTranscript(stoppedSid, elsewhere, helpers.AssistantReply(stoppedSid, "msg_205_b", stamp(time.Hour)))
		require.NoError(t, d.Rescan())

		t.Run("it should look at that project's transcripts on the next rescan", func(t *testing.T) {
			s, _ := session(d, stoppedSid)
			assert.Equal(t, t0.Add(time.Hour), s.LastActivity)
		})
	})
}
