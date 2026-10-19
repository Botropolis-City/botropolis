package appd

import (
	"context"
	"encoding/json"
	"errors"
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
	"botropolis.city/botropolis/pkg/proto"
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

// slow is a loader whose first Load waits until release is closed, as a
// first scan of a large home on a busy machine does.
type slow struct {
	daemon.Snapshotter
	once    sync.Once
	release chan struct{}
}

func (s *slow) Load(now time.Time) (state.Snapshot, error) {
	s.once.Do(func() { <-s.release })
	return s.Snapshotter.Load(now)
}

// failing is a loader that cannot read the home.
type failing struct{ daemon.Snapshotter }

func (failing) Load(time.Time) (state.Snapshot, error) {
	return state.Snapshot{}, errors.New("the home cannot be read")
}

func TestDaemonStart(t *testing.T) {
	const sid = "08b1d655-276f-495b-a61f-2214af35a97a"
	const cwd = "/w/cinders"
	short := os.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("TMPDIR", t.TempDir())

	newSocket := func(t *testing.T) string {
		dir, err := os.MkdirTemp(short, "bt")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		return filepath.Join(dir, "botropolis.sock")
	}

	newHome := func(t *testing.T) *helpers.Home {
		home := helpers.NewHome(t).Session(os.Getpid(), sid, cwd, "interactive", "idle")
		home.Transcript(sid, cwd,
			helpers.UserPrompt(sid, cwd, "2026-09-16T19:50:00.000Z"),
			helpers.AssistantReply(sid, "msg_01", "2026-09-16T19:50:10.000Z"))
		return home
	}

	newApp := func(cfg *config.Config, wrap func(daemon.Snapshotter) daemon.Snapshotter, d **daemon.Daemon) *fx.App {
		return fx.New(
			DaemonModule(cfg, io.Discard),
			fx.Decorate(func(cfg *config.Config, probes state.Probes) *daemon.Daemon {
				return daemon.NewWith(wrap(harnesses(cfg, probes, "", nil)), time.Now)
			}),
			fx.Populate(d),
			fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
		)
	}

	t.Run("when the first scan takes longer than the start deadline (item 143)", func(t *testing.T) {
		start := func(t *testing.T) (*fx.App, func(), string, error) {
			home := newHome(t)
			sock := newSocket(t)
			cfg := &config.Config{Home: home.Path, HomeSet: true, Socket: sock}
			loader := &slow{release: make(chan struct{})}
			var released sync.Once
			release := func() { released.Do(func() { close(loader.release) }) }
			var d *daemon.Daemon
			app := newApp(cfg, func(s daemon.Snapshotter) daemon.Snapshotter { loader.Snapshotter = s; return loader }, &d)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := app.Start(ctx)
			t.Cleanup(func() {
				release()
				_ = app.Stop(context.Background())
			})
			return app, release, sock, err
		}

		t.Run("it should return from Start within a second while the scan is still running", func(t *testing.T) {
			_, _, _, err := start(t)
			assert.NoError(t, err)
		})

		t.Run("and a hook event arrives during the scan", func(t *testing.T) {
			_, _, sock, err := start(t)
			require.NoError(t, err)
			sent := make(chan error, 1)
			go func() {
				sent <- proto.SendEvent(sock, json.RawMessage(`{"hook_event_name":"PreToolUse","session_id":"`+sid+`","cwd":"`+cwd+`","tool_name":"Bash"}`))
			}()

			t.Run("it should be answered before the scan finishes", func(t *testing.T) {
				select {
				case err := <-sent:
					assert.NoError(t, err)
				case <-time.After(time.Second):
					t.Fatal("the hook event was not answered while the first scan ran")
				}
			})
		})

		t.Run("and the scan then finishes", func(t *testing.T) {
			app, release, sock, err := start(t)
			require.NoError(t, err)
			got := make(chan state.Snapshot, 1)
			go func() {
				client, err := proto.Dial(sock)
				if err != nil {
					close(got)
					return
				}
				defer func() { _ = client.Close() }()
				snapshot, err := client.Snapshot()
				if err != nil {
					close(got)
					return
				}
				got <- snapshot
			}()
			time.Sleep(100 * time.Millisecond)
			release()
			var snapshot state.Snapshot
			select {
			case snapshot = <-got:
			case <-time.After(3 * time.Second):
			}

			t.Run("it should serve a snapshot with the home's session over the socket", func(t *testing.T) {
				assert.Len(t, snapshot.Sessions, 1)
			})

			t.Run("it should still be running", func(t *testing.T) {
				select {
				case sig := <-app.Wait():
					t.Fatalf("the daemon stopped with exit code %d", sig.ExitCode)
				case <-time.After(200 * time.Millisecond):
				}
			})
		})

		t.Run("and the daemon is stopped before the scan finishes", func(t *testing.T) {
			app, _, sock, err := start(t)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
			defer cancel()
			stopped := app.Stop(ctx)

			t.Run("it should stop within the stop deadline", func(t *testing.T) {
				assert.NoError(t, stopped)
			})

			t.Run("it should remove its socket", func(t *testing.T) {
				_, err := os.Stat(sock)
				assert.True(t, os.IsNotExist(err))
			})
		})
	})

	t.Run("when the first scan fails", func(t *testing.T) {
		home := newHome(t)
		cfg := &config.Config{Home: home.Path, HomeSet: true, Socket: newSocket(t)}
		var d *daemon.Daemon
		app := newApp(cfg, func(s daemon.Snapshotter) daemon.Snapshotter { return failing{s} }, &d)
		started := app.Start(context.Background())
		t.Cleanup(func() { _ = app.Stop(context.Background()) })
		code := 0
		if started == nil {
			select {
			case sig := <-app.Wait():
				code = sig.ExitCode
			case <-time.After(3 * time.Second):
			}
		}

		t.Run("it should shut down with a non-zero exit code", func(t *testing.T) {
			assert.NotZero(t, code)
		})
	})

	t.Run("when a transcript changes after the first scan and before its directory is watched (item 124)", func(t *testing.T) {
		home := newHome(t)
		cfg := &config.Config{Home: home.Path, HomeSet: true, Socket: newSocket(t)}
		var d *daemon.Daemon
		app := newApp(cfg, func(s daemon.Snapshotter) daemon.Snapshotter {
			return &changing{
				Snapshotter: s,
				change: func() {
					home.AppendTranscript(sid, cwd, helpers.UserPrompt(sid, cwd, "2026-09-16T19:51:00.000Z"))
				},
			}
		}, &d)
		require.NoError(t, app.Start(context.Background()))
		t.Cleanup(func() { _ = app.Stop(context.Background()) })

		t.Run("it should be in the snapshot once the daemon has started", func(t *testing.T) {
			assert.Eventually(t, func() bool {
				snapshot := d.Snapshot()
				return len(snapshot.Sessions) == 1 && snapshot.Sessions[0].Turn == claude.TurnWorking
			}, 2*time.Second, 20*time.Millisecond)
		})
	})
}
