// Package daemonctl restarts the daemon that answered on a socket the way
// it was started: through its systemd user unit, through its launchd job,
// or, started by hand, by stopping that one process and starting this
// build's daemon in its place. It acts only on the process the socket
// names as its peer, never on a guess. Item 152.
package daemonctl

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"botropolis.city/botropolis/pkg/proto"
)

// Kind is how the daemon was started.
type Kind int

const (
	ByHand Kind = iota
	Systemd
	Launchd
)

const (
	unit        = "botropolisd.service"
	launchdName = "homebrew.mxcl.botropolis"
	launchctl   = "/bin/launchctl"
	// StopWithin is how long a daemon started by hand has to exit.
	StopWithin = 5 * time.Second
	// AnswerWithin is how long a restarted daemon has to answer at
	// this build's release.
	AnswerWithin = 10 * time.Second
	pollEvery    = 100 * time.Millisecond
)

// Method is how to restart one daemon: its kind, its process, and for
// one started by hand, the arguments it was started with.
type Method struct {
	Kind  Kind
	PID   int
	Label string
	Args  []string
}

// Describe is the restart as a person would type it.
func (m Method) Describe() string {
	switch m.Kind {
	case Systemd:
		return "systemctl --user restart botropolisd"
	case Launchd:
		return "launchctl kickstart -k " + m.Label
	}
	return fmt.Sprintf("stop botropolisd (pid %d) and start it again", m.PID)
}

// System is everything a restart touches: the service managers, the
// process table, and the socket. Native is the real one.
type System struct {
	GOOS string
	UID  int
	// Run runs a command and returns what it printed, both streams.
	Run func(ctx context.Context, name string, args ...string) (string, error)
	// PeerPID is the process at the other end of the socket.
	PeerPID func(sock string) (int, error)
	Cgroup  func(pid int) (string, error)
	Args    func(pid int) ([]string, error)
	Signal  func(pid int, sig syscall.Signal) error
	Alive   func(pid int) bool
	// Start starts a daemon that outlives this process.
	Start func(path string, args []string) error
	// Binary is this build's daemon.
	Binary func() (string, error)
	// Answer asks the daemon on the socket what it is.
	Answer func(sock string) (proto.Peer, error)
	Sleep  func(time.Duration)
	Now    func() time.Time
}

var launchdPID = regexp.MustCompile(`(?m)^\s*pid = (\d+)\s*$`)

// Plan works out how the daemon answering on sock was started, and
// refuses when the socket cannot say which process that is.
func Plan(ctx context.Context, sys System, sock string) (Method, error) {
	pid, err := sys.PeerPID(sock)
	if err == nil && pid <= 0 {
		err = errors.New("the socket named no process")
	}
	if err != nil {
		return Method{}, fmt.Errorf("cannot tell which process is the daemon on %s: %w", sock, err)
	}
	if sys.GOOS == "darwin" {
		label := fmt.Sprintf("gui/%d/%s", sys.UID, launchdName)
		if out, err := sys.Run(ctx, launchctl, "print", label); err == nil {
			if m := launchdPID.FindStringSubmatch(out); m != nil && m[1] == strconv.Itoa(pid) {
				return Method{Kind: Launchd, PID: pid, Label: label}, nil
			}
		}
	} else if cgroup, err := sys.Cgroup(pid); err == nil {
		if strings.Contains(cgroup, "/"+unit) {
			return Method{Kind: Systemd, PID: pid}, nil
		}
	} else if _, err := sys.Run(ctx, "systemctl", "--user", "is-active", "--quiet", unit); err == nil {
		return Method{Kind: Systemd, PID: pid}, nil
	}
	method := Method{Kind: ByHand, PID: pid}
	if args, err := sys.Args(pid); err == nil {
		method.Args = args
	} else {
		method.Args = []string{"--socket", sock}
	}
	return method, nil
}

// Hint is what to tell a person about a daemon out of sync with this
// build: how to restart the daemon, or, when the daemon is ahead, that
// this is what is out of date.
func Hint(ctx context.Context, sys System, m *proto.Mismatch) string {
	if m.Newer() {
		return "this botropolis is older than its daemon: run the installed one"
	}
	method, err := Plan(ctx, sys, m.Socket)
	switch {
	case err != nil:
		return "restart botropolisd"
	case method.Kind == ByHand:
		return method.Describe()
	}
	return fmt.Sprintf("restart it with %q", method.Describe())
}

// Restart restarts the daemon answering on sock the way it was started,
// and returns once one answers at want's release and protocol.
func Restart(ctx context.Context, sys System, sock string, want proto.Peer) error {
	method, err := Plan(ctx, sys, sock)
	if err != nil {
		return err
	}
	switch method.Kind {
	case Systemd:
		if err := run(ctx, sys, "systemctl", "--user", "restart", unit); err != nil {
			return err
		}
	case Launchd:
		if err := run(ctx, sys, launchctl, "kickstart", "-k", method.Label); err != nil {
			return err
		}
	default:
		if err := byHand(sys, method); err != nil {
			return err
		}
	}
	return await(sys, sock, want)
}

func run(ctx context.Context, sys System, name string, args ...string) error {
	out, err := sys.Run(ctx, name, args...)
	if err == nil {
		return nil
	}
	said := strings.TrimSpace(out)
	if said == "" {
		said = err.Error()
	}
	if i := strings.IndexByte(said, '\n'); i >= 0 {
		said = said[:i]
	}
	return fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), said)
}

func byHand(sys System, method Method) error {
	binary, err := sys.Binary()
	if err != nil {
		return fmt.Errorf("finding botropolisd: %w", err)
	}
	if err := sys.Signal(method.PID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("stopping the daemon (pid %d): %w", method.PID, err)
	}
	deadline := sys.Now().Add(StopWithin)
	for sys.Alive(method.PID) {
		if !sys.Now().Before(deadline) {
			return fmt.Errorf("the daemon (pid %d) did not stop within %s", method.PID, StopWithin)
		}
		sys.Sleep(pollEvery)
	}
	if err := sys.Start(binary, method.Args); err != nil {
		return fmt.Errorf("starting %s: %w", binary, err)
	}
	return nil
}

func await(sys System, sock string, want proto.Peer) error {
	deadline := sys.Now().Add(AnswerWithin)
	var last error
	for {
		peer, err := sys.Answer(sock)
		switch {
		case err != nil:
			last = err
		case proto.Same(peer, want):
			return nil
		default:
			last = fmt.Errorf("the daemon came back as %s, and this is %s", releaseOf(peer), releaseOf(want))
		}
		if !sys.Now().Before(deadline) {
			if err != nil {
				return fmt.Errorf("the daemon did not answer within %s: %w", AnswerWithin, last)
			}
			return last
		}
		sys.Sleep(pollEvery)
	}
}

func releaseOf(p proto.Peer) string {
	if p.Release == "" {
		return "a release from before versions were exchanged"
	}
	return p.Release
}
