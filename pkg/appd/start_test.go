package appd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// changing is a loader whose first WatchDirs call changes the home: that
// call is made after the daemon's first scan and before its watcher has
// been told about any directory, so the change lands in the window
// between them.
type changing struct {
	daemon.Snapshotter
	once   sync.Once
	change func()
}

func (c *changing) WatchDirs() []string {
	c.once.Do(c.change)
	return c.Snapshotter.WatchDirs()
}

func TestDaemonStart(t *testing.T) {
	const sid = "08b1d655-276f-495b-a61f-2214af35a97a"
	const cwd = "/w/cinders"
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	sockDir, err := os.MkdirTemp("", "bt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	t.Setenv("TMPDIR", t.TempDir())

	t.Run("when a transcript changes after the first scan and before its directory is watched (item 124)", func(t *testing.T) {
		home := helpers.NewHome(t).Session(os.Getpid(), sid, cwd, "interactive", "idle")
		home.Transcript(sid, cwd,
			helpers.UserPrompt(sid, cwd, "2026-09-16T19:50:00.000Z"),
			helpers.AssistantReply(sid, "msg_01", "2026-09-16T19:50:10.000Z"))
		cfg := &config.Config{Home: home.Path, HomeSet: true, Socket: filepath.Join(sockDir, "botropolis.sock")}
		var d *daemon.Daemon
		app := fx.New(
			DaemonModule(cfg, io.Discard),
			fx.Decorate(func(cfg *config.Config, probes state.Probes) *daemon.Daemon {
				return daemon.NewWith(&changing{
					Snapshotter: harnesses(cfg, probes, "", nil),
					change: func() {
						home.AppendTranscript(sid, cwd, helpers.UserPrompt(sid, cwd, "2026-09-16T19:51:00.000Z"))
					},
				}, time.Now)
			}),
			fx.Populate(&d),
			fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
		)
		require.NoError(t, app.Start(context.Background()))
		t.Cleanup(func() { _ = app.Stop(context.Background()) })
		snapshot := d.Snapshot()

		t.Run("it should be in the snapshot once the daemon has started", func(t *testing.T) {
			require.Len(t, snapshot.Sessions, 1)
			assert.Equal(t, claude.TurnWorking, snapshot.Sessions[0].Turn)
		})
	})
}
