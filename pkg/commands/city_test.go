package commands_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	daemonT0  = time.Date(2026, time.September, 16, 19, 50, 0, 0, time.UTC)
	daemonNow = daemonT0.Add(time.Hour)
)

// sequenceSource answers each poll with the next snapshot, then repeats
// the last.
type sequenceSource struct {
	mu        sync.Mutex
	snapshots []state.Snapshot
	i         int
}

func (s *sequenceSource) Snapshot(bool) (state.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := s.snapshots[min(s.i, len(s.snapshots)-1)]
	s.i++
	return snapshot, nil
}

// servedDaemon runs a daemon over a home with one idle interactive
// session, on a socket in a short temp dir.
func servedDaemon(t *testing.T) (*daemon.Daemon, string) {
	t.Helper()
	d, sock, _ := servedDaemonHome(t)
	return d, sock
}

// servedDaemonHome is servedDaemon with the home it reads.
func servedDaemonHome(t *testing.T) (*daemon.Daemon, string, string) {
	t.Helper()
	home := helpers.NewHome(t).Session(4242, sid, cwd, "interactive", "idle")
	home.Transcript(sid, cwd,
		helpers.UserPrompt(sid, cwd, daemonT0.Format(time.RFC3339Nano)),
		helpers.AssistantReply(sid, "msg_01", daemonT0.Add(10*time.Second).Format(time.RFC3339Nano)),
		helpers.AITitle(sid, "Fix the CI queue"))
	probes := state.Probes{Alive: func(int) bool { return true }, Attached: func(string) bool { return false }}
	d := daemon.New(home.Path, probes, func() time.Time { return daemonNow })
	require.NoError(t, d.Rescan())
	dir, err := os.MkdirTemp("", "bt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "botropolis.sock")
	listener, err := net.Listen("unix", sock)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = d.Serve(ctx, listener)
	}()
	t.Cleanup(func() {
		cancel()
		_ = listener.Close()
		<-done
	})
	return d, sock, home.Path
}

func TestFeed(t *testing.T) {
	t.Run("when no daemon is listening", func(t *testing.T) {
		source := &fakeSource{snapshot: state.Snapshot{Sessions: []state.Session{{ID: sid}}}}
		feed := commands.Feed{Socket: t.TempDir() + "/none.sock", Source: source, Poll: time.Millisecond, Retry: 20 * time.Millisecond}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		got := make(chan state.Snapshot, 8)
		go feed.Run(ctx, func(s state.Snapshot) {
			select {
			case got <- s:
			default:
			}
		})

		t.Run("it should poll the source directly", func(t *testing.T) {
			select {
			case snapshot := <-got:
				assert.Len(t, snapshot.Sessions, 1)
			case <-time.After(2 * time.Second):
				t.Fatal("no snapshot delivered")
			}
		})

		t.Run("it should ask for a direct scan", func(t *testing.T) {
			assert.True(t, source.direct)
		})
	})

	t.Run("when a daemon streams updates", func(t *testing.T) {
		d, sock := servedDaemon(t)
		d.Apply(claude.HookEvent{Name: claude.HookPreToolUse, SessionID: sid, ToolName: "Grep"}, daemonNow)
		d.Apply(claude.HookEvent{Name: claude.HookStop, SessionID: sid}, daemonNow.Add(time.Minute))
		var mu sync.Mutex
		var calls []string
		feed := commands.Feed{Socket: sock, Source: &fakeSource{}, Poll: time.Millisecond, Retry: 20 * time.Millisecond,
			Since: func() time.Time { return daemonT0 },
			Events: func(fresh []events.Event) {
				mu.Lock()
				defer mu.Unlock()
				calls = append(calls, fmt.Sprintf("events:%d", len(fresh)))
			}}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go feed.Run(ctx, func(state.Snapshot) {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, "snapshot")
		})
		require.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(calls) >= 2 }, 2*time.Second, 5*time.Millisecond)

		t.Run("it should hand the backlog since the moment on before the first snapshot", func(t *testing.T) {
			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, []string{"events:1", "snapshot"}, calls[:2])
		})

		t.Run("and the daemon logs more", func(t *testing.T) {
			d.Apply(claude.HookEvent{Name: claude.HookPreToolUse, SessionID: sid, ToolName: "Bash"}, daemonNow.Add(2*time.Minute))
			d.Apply(claude.HookEvent{Name: claude.HookStop, SessionID: sid}, daemonNow.Add(3*time.Minute))

			t.Run("it should stream the fresh events", func(t *testing.T) {
				assert.Eventually(t, func() bool {
					mu.Lock()
					defer mu.Unlock()
					return slices.Contains(calls[2:], "events:1")
				}, 2*time.Second, 5*time.Millisecond)
			})
		})
	})

	t.Run("when polling directly and the source changes", func(t *testing.T) {
		source := &sequenceSource{snapshots: []state.Snapshot{
			{Sessions: []state.Session{{ID: sid, State: state.Working, Title: "Fix the CI queue"}}},
			{Sessions: []state.Session{{ID: sid, State: state.NeedsYou, Title: "Fix the CI queue"}}},
		}}
		fresh := make(chan []events.Event, 8)
		feed := commands.Feed{Socket: t.TempDir() + "/none.sock", Source: source, Poll: time.Millisecond, Retry: 20 * time.Millisecond,
			Events: func(batch []events.Event) {
				if len(batch) > 0 {
					select {
					case fresh <- batch:
					default:
					}
				}
			}}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go feed.Run(ctx, func(state.Snapshot) {})

		t.Run("it should diff the snapshots itself and hand the fresh events on", func(t *testing.T) {
			select {
			case batch := <-fresh:
				require.Len(t, batch, 1)
				assert.Equal(t, events.NeedsYou, batch[0].Kind)
			case <-time.After(2 * time.Second):
				t.Fatal("no events delivered")
			}
		})
	})

	t.Run("when the source fails", func(t *testing.T) {
		source := &fakeSource{err: errors.New("boom")}
		failures := make(chan error, 4)
		feed := commands.Feed{Socket: t.TempDir() + "/none.sock", Source: source, Poll: time.Millisecond, Retry: 20 * time.Millisecond,
			OnError: func(err error) {
				select {
				case failures <- err:
				default:
				}
			}}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go feed.Run(ctx, func(state.Snapshot) { t.Error("a failed scan should not be offered") })

		t.Run("it should report the failure", func(t *testing.T) {
			select {
			case err := <-failures:
				assert.ErrorContains(t, err, "boom")
			case <-time.After(2 * time.Second):
				t.Fatal("no failure reported")
			}
		})
	})

	t.Run("when the context is cancelled", func(t *testing.T) {
		feed := commands.Feed{Socket: t.TempDir() + "/none.sock", Source: &fakeSource{}, Poll: time.Millisecond, Retry: time.Millisecond}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { defer close(done); feed.Run(ctx, func(state.Snapshot) {}) }()
		cancel()

		t.Run("it should stop", func(t *testing.T) {
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("feed did not stop")
			}
		})
	})
}

func TestActor(t *testing.T) {
	t.Run("when a running session is clicked", func(t *testing.T) {
		ctl := &fakeController{id: "0898d7e4"}
		actor := commands.Actor{Sessions: commands.NewSessions(t.TempDir(), ctl)}
		require.NoError(t, actor.Do(city.Action{Kind: city.ActionAttach, SessionID: "0898d7e4"}))

		t.Run("it should attach it", func(t *testing.T) {
			assert.Equal(t, []string{"attach 0898d7e4"}, ctl.calls)
		})
	})

	t.Run("when a resting session is clicked", func(t *testing.T) {
		home := helpers.NewHome(t)
		home.Transcript(sid, cwd, helpers.UserPrompt(sid, cwd, "2026-09-16T19:50:00.000Z"))
		ctl := &fakeController{id: "08b1d655"}
		actor := commands.Actor{Sessions: commands.NewSessions(home.Path, ctl)}
		require.NoError(t, actor.Do(city.Action{Kind: city.ActionResume, SessionID: sid}))

		t.Run("it should resume it in its own directory and attach", func(t *testing.T) {
			assert.Equal(t, []string{"resume " + cwd + " " + sid, "attach 08b1d655"}, ctl.calls)
		})
	})

	t.Run("when a card's actions are used", func(t *testing.T) {
		cases := []struct {
			action city.Action
			call   string
		}{
			{city.Action{Kind: city.ActionStop, SessionID: "0898d7e4"}, "stop 0898d7e4"},
			{city.Action{Kind: city.ActionNew, Dir: cwd}, "new " + cwd + " "},
			{city.Action{Kind: city.ActionReveal, Dir: cwd}, "reveal " + cwd},
			{city.Action{Kind: city.ActionCopyPath, Dir: cwd}, "copy " + cwd},
			{city.Action{Kind: city.ActionFork, SessionID: sid, Dir: cwd}, "fork " + cwd + " " + sid},
			{city.Action{Kind: city.ActionOpenPR, Text: "https://example.com/pr/1"}, "reveal https://example.com/pr/1"},
			{city.Action{Kind: city.ActionCopy, Text: "cd " + cwd + " && claude --resume " + sid}, "copy cd " + cwd + " && claude --resume " + sid},
		}
		for _, tc := range cases {
			t.Run("it should "+tc.call, func(t *testing.T) {
				ctl := &fakeController{id: "0898d7e4"}
				actor := commands.Actor{Sessions: commands.NewSessions(t.TempDir(), ctl)}
				require.NoError(t, actor.Do(tc.action))
				assert.Equal(t, tc.call, ctl.calls[0])
			})
		}
	})

	t.Run("when nothing was clicked", func(t *testing.T) {
		ctl := &fakeController{}
		actor := commands.Actor{Sessions: commands.NewSessions(t.TempDir(), ctl)}
		require.NoError(t, actor.Do(city.Action{}))

		t.Run("it should do nothing", func(t *testing.T) {
			assert.Empty(t, ctl.calls)
		})
	})
}
