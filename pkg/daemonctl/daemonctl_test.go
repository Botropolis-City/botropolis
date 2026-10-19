package daemonctl_test

import (
	"context"
	"errors"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/daemonctl"
	"botropolis.city/botropolis/pkg/proto"
)

const sock = "/run/user/1000/botropolis/botropolis.sock"

var ours = proto.Peer{Release: "0.2.3", Protocol: 1}

// fakeSystem is a service manager and process table that records what is
// asked of it and touches nothing real.
type fakeSystem struct {
	calls     []string
	pid       int
	pidErr    error
	cgroup    string
	cgroupErr error
	args      []string
	argsErr   error
	run       map[string]error
	output    map[string]string
	stubborn  bool
	signalled bool
	started   bool
	startErr  error
	answer    func(started bool) (proto.Peer, error)
	now       time.Time
}

func newFake() *fakeSystem {
	return &fakeSystem{
		pid:    4242,
		cgroup: "0::/user.slice/user-1000.slice/user@1000.service/app.slice/i3.scope\n",
		args:   []string{"--home", "/home/someone"},
		run:    map[string]error{},
		output: map[string]string{},
		answer: func(bool) (proto.Peer, error) { return ours, nil },
		now:    time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC),
	}
}

func (f *fakeSystem) record(call string) { f.calls = append(f.calls, call) }

func (f *fakeSystem) system(goos string) daemonctl.System {
	return daemonctl.System{
		GOOS: goos,
		UID:  501,
		Run: func(_ context.Context, name string, args ...string) (string, error) {
			call := strings.Join(append([]string{name}, args...), " ")
			f.record(call)
			return f.output[call], f.run[call]
		},
		PeerPID: func(string) (int, error) { return f.pid, f.pidErr },
		Cgroup:  func(int) (string, error) { return f.cgroup, f.cgroupErr },
		Args:    func(int) ([]string, error) { return f.args, f.argsErr },
		Signal: func(pid int, sig syscall.Signal) error {
			f.record("signal " + sig.String())
			f.signalled = true
			return nil
		},
		Alive: func(int) bool { return !f.signalled || f.stubborn },
		Start: func(path string, args []string) error {
			f.record("start " + strings.Join(append([]string{path}, args...), " "))
			f.started = f.startErr == nil
			return f.startErr
		},
		Binary: func() (string, error) { return "/usr/bin/botropolisd", nil },
		Answer: func(string) (proto.Peer, error) { return f.answer(f.started || f.ran("restart") || f.ran("kickstart")) },
		Sleep:  func(d time.Duration) { f.now = f.now.Add(d) },
		Now:    func() time.Time { return f.now },
	}
}

func (f *fakeSystem) ran(word string) bool {
	for _, c := range f.calls {
		if strings.Contains(c, " "+word+" ") {
			return true
		}
	}
	return false
}

const launchdPrint = "/bin/launchctl print gui/501/homebrew.mxcl.botropolis"

// Item 152: how the daemon that answered on the socket was started.
func TestPlan(t *testing.T) {
	cases := []struct {
		scenario string
		goos     string
		setup    func(f *fakeSystem)
		kind     daemonctl.Kind
		describe string
	}{
		{"when the peer's cgroup names the user unit", "linux", func(f *fakeSystem) {
			f.cgroup = "0::/user.slice/user-1000.slice/user@1000.service/app.slice/botropolisd.service\n"
		}, daemonctl.Systemd, "systemctl --user restart botropolisd"},
		{"when the peer's cgroup cannot be read and the unit is active", "linux", func(f *fakeSystem) {
			f.cgroupErr = errors.New("no such file")
		}, daemonctl.Systemd, "systemctl --user restart botropolisd"},
		{"when the peer's cgroup cannot be read and the unit is not active", "linux", func(f *fakeSystem) {
			f.cgroupErr = errors.New("no such file")
			f.run["systemctl --user is-active --quiet botropolisd.service"] = errors.New("exit status 3")
		}, daemonctl.ByHand, "stop botropolisd (pid 4242) and start it again"},
		{"when the peer runs in no unit", "linux", func(*fakeSystem) {}, daemonctl.ByHand, "stop botropolisd (pid 4242) and start it again"},
		{"when on macOS the launchd job runs the peer", "darwin", func(f *fakeSystem) {
			f.output[launchdPrint] = "gui/501/homebrew.mxcl.botropolis = {\n\tstate = running\n\tpid = 4242\n}\n"
		}, daemonctl.Launchd, "launchctl kickstart -k gui/501/homebrew.mxcl.botropolis"},
		{"when on macOS the launchd job runs another process", "darwin", func(f *fakeSystem) {
			f.output[launchdPrint] = "gui/501/homebrew.mxcl.botropolis = {\n\tpid = 99\n}\n"
		}, daemonctl.ByHand, "stop botropolisd (pid 4242) and start it again"},
		{"when on macOS there is no launchd job", "darwin", func(f *fakeSystem) {
			f.run[launchdPrint] = errors.New("exit status 113")
		}, daemonctl.ByHand, "stop botropolisd (pid 4242) and start it again"},
	}
	for _, c := range cases {
		t.Run(c.scenario, func(t *testing.T) {
			f := newFake()
			c.setup(f)
			method, err := daemonctl.Plan(context.Background(), f.system(c.goos), sock)

			t.Run("it should find a method", func(t *testing.T) {
				assert.NoError(t, err)
			})

			t.Run("it should be the right one", func(t *testing.T) {
				assert.Equal(t, c.kind, method.Kind)
			})

			t.Run("it should describe it to a person", func(t *testing.T) {
				assert.Equal(t, c.describe, method.Describe())
			})
		})
	}

	for _, c := range []struct {
		scenario string
		setup    func(f *fakeSystem)
	}{
		{"when the peer on the socket cannot be identified", func(f *fakeSystem) { f.pid, f.pidErr = 0, errors.New("not a unix socket") }},
		{"when the peer reports no process", func(f *fakeSystem) { f.pid = 0 }},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			f := newFake()
			c.setup(f)
			_, err := daemonctl.Plan(context.Background(), f.system("linux"), sock)

			t.Run("it should refuse, naming the socket", func(t *testing.T) {
				assert.ErrorContains(t, err, "cannot tell which process is the daemon on "+sock)
			})
		})
	}
}

// Item 152: restarting the daemon the way it was started, then waiting
// for it to answer at this build's release.
func TestRestart(t *testing.T) {
	restart := func(f *fakeSystem, goos string) error {
		return daemonctl.Restart(context.Background(), f.system(goos), sock, ours)
	}
	systemd := func(f *fakeSystem) {
		f.cgroup = "0::/user.slice/user@1000.service/app.slice/botropolisd.service\n"
	}

	t.Run("when the user unit runs the daemon", func(t *testing.T) {
		f := newFake()
		systemd(f)
		err := restart(f, "linux")

		t.Run("it should succeed", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should restart the unit and signal nothing", func(t *testing.T) {
			assert.Equal(t, []string{"systemctl --user restart botropolisd.service"}, f.calls)
		})
	})

	t.Run("when systemctl fails", func(t *testing.T) {
		f := newFake()
		systemd(f)
		f.run["systemctl --user restart botropolisd.service"] = errors.New("exit status 1")
		f.output["systemctl --user restart botropolisd.service"] = "Failed to restart botropolisd.service: Unit botropolisd.service not found.\n"
		err := restart(f, "linux")

		t.Run("it should say what systemctl said", func(t *testing.T) {
			assert.EqualError(t, err, "systemctl --user restart botropolisd.service: Failed to restart botropolisd.service: Unit botropolisd.service not found.")
		})
	})

	t.Run("when the launchd job runs the daemon", func(t *testing.T) {
		f := newFake()
		f.output[launchdPrint] = "\tpid = 4242\n"
		err := restart(f, "darwin")

		t.Run("it should succeed", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should kickstart the job and signal nothing", func(t *testing.T) {
			assert.Equal(t, []string{launchdPrint, "/bin/launchctl kickstart -k gui/501/homebrew.mxcl.botropolis"}, f.calls)
		})
	})

	t.Run("when the daemon was started by hand", func(t *testing.T) {
		f := newFake()
		err := restart(f, "linux")

		t.Run("it should succeed", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should stop the peer, then start this build's daemon with the peer's arguments", func(t *testing.T) {
			assert.Equal(t, []string{"signal terminated", "start /usr/bin/botropolisd --home /home/someone"}, f.calls)
		})

		t.Run("and its arguments cannot be read", func(t *testing.T) {
			f := newFake()
			f.argsErr = errors.New("no /proc")
			_ = restart(f, "linux")

			t.Run("it should start it on the socket the map dialed", func(t *testing.T) {
				assert.Equal(t, "start /usr/bin/botropolisd --socket "+sock, f.calls[len(f.calls)-1])
			})
		})

		t.Run("and it outlives the wait", func(t *testing.T) {
			f := newFake()
			f.stubborn = true
			err := restart(f, "linux")

			t.Run("it should say the daemon did not stop", func(t *testing.T) {
				assert.EqualError(t, err, "the daemon (pid 4242) did not stop within 5s")
			})

			t.Run("it should start nothing", func(t *testing.T) {
				assert.Equal(t, []string{"signal terminated"}, f.calls)
			})
		})

		t.Run("and this build's daemon cannot be started", func(t *testing.T) {
			f := newFake()
			f.startErr = errors.New("permission denied")
			err := restart(f, "linux")

			t.Run("it should say so", func(t *testing.T) {
				assert.EqualError(t, err, "starting /usr/bin/botropolisd: permission denied")
			})
		})
	})

	t.Run("when the peer cannot be identified", func(t *testing.T) {
		f := newFake()
		f.pid, f.pidErr = 0, errors.New("not a unix socket")
		err := restart(f, "linux")

		t.Run("it should refuse", func(t *testing.T) {
			assert.ErrorContains(t, err, "cannot tell which process is the daemon")
		})

		t.Run("it should touch nothing", func(t *testing.T) {
			assert.Empty(t, f.calls)
		})
	})

	t.Run("when the daemon comes back another release", func(t *testing.T) {
		f := newFake()
		systemd(f)
		f.answer = func(bool) (proto.Peer, error) { return proto.Peer{Release: "0.2.2", Protocol: 1}, nil }
		err := restart(f, "linux")

		t.Run("it should fail, naming both", func(t *testing.T) {
			assert.EqualError(t, err, "the daemon came back as 0.2.2, and this is 0.2.3")
		})
	})

	t.Run("when the daemon never answers", func(t *testing.T) {
		f := newFake()
		systemd(f)
		f.answer = func(bool) (proto.Peer, error) { return proto.Peer{}, errors.New("connection refused") }
		err := restart(f, "linux")

		t.Run("it should fail after the limit", func(t *testing.T) {
			assert.EqualError(t, err, "the daemon did not answer within 10s: connection refused")
		})
	})

	t.Run("when the old daemon answers once before the new one", func(t *testing.T) {
		f := newFake()
		systemd(f)
		asked := 0
		f.answer = func(bool) (proto.Peer, error) {
			asked++
			if asked == 1 {
				return proto.Peer{Release: "0.2.2", Protocol: 1}, nil
			}
			return ours, nil
		}
		err := restart(f, "linux")

		t.Run("it should wait for the new one", func(t *testing.T) {
			assert.NoError(t, err)
		})
	})
}

// Item 152: what a command tells a person to do about a daemon out of
// sync with it.
func TestHint(t *testing.T) {
	older := &proto.Mismatch{Daemon: proto.Peer{Release: "0.2.2", Protocol: 1}, Ours: ours, Socket: sock}
	newer := &proto.Mismatch{Daemon: proto.Peer{Release: "0.2.4", Protocol: 1}, Ours: ours, Socket: sock}
	for _, c := range []struct {
		scenario string
		mismatch *proto.Mismatch
		setup    func(f *fakeSystem)
		hint     string
	}{
		{"when the user unit runs an older daemon", older, func(f *fakeSystem) {
			f.cgroup = "0::/user.slice/user@1000.service/app.slice/botropolisd.service\n"
		}, `restart the daemon with "systemctl --user restart botropolisd"`},
		{"when an older daemon was started by hand", older, func(*fakeSystem) {}, "stop botropolisd (pid 4242) and start it again"},
		{"when the older daemon cannot be identified", older, func(f *fakeSystem) { f.pidErr = errors.New("no") }, "restart botropolisd"},
		{"when the daemon is newer than this", newer, func(*fakeSystem) {}, "restart the city to run the installed one"},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			f := newFake()
			c.setup(f)

			t.Run("it should say what to do", func(t *testing.T) {
				assert.Equal(t, c.hint, daemonctl.Hint(context.Background(), f.system("linux"), c.mismatch))
			})
		})
	}
}
