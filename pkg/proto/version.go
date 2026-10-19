package proto

import (
	"fmt"
	"strconv"
	"strings"

	"botropolis.city/botropolis/pkg/version"
)

// Protocol is the wire format's version: raised only when a change would
// make one end misread the other, never for a change that only adds a
// field. Daemons from before the exchange send none, and spoke 1.
const Protocol = 1

// Peer is what each end of the socket says it is (item 149).
type Peer struct {
	Release  string `json:"release,omitempty"`
	Protocol int    `json:"protocol,omitempty"`
}

// Self is this build.
func Self() Peer {
	return Peer{Release: version.Version, Protocol: Protocol}
}

func (p Peer) protocol() int {
	if p.Protocol == 0 {
		return 1
	}
	return p.Protocol
}

// Same is whether two ends are the same release speaking the same
// protocol (item 152).
func Same(a, b Peer) bool {
	return a.Release == b.Release && a.protocol() == b.protocol()
}

// Mismatch is a daemon of another release or protocol than this build,
// which every client refuses but a hook's: the two must be the same
// version (item 152). Socket is where the
// daemon answered, so whatever restarts it knows which one.
type Mismatch struct {
	Daemon Peer
	Ours   Peer
	Socket string
}

func (m *Mismatch) Error() string {
	return fmt.Sprintf("the Botropolis city and its daemon must be running the same version (daemon %s, city %s)", m.DaemonVersion(), m.CityVersion())
}

// DaemonVersion is the daemon's version as a person reads it.
func (m *Mismatch) DaemonVersion() string {
	switch m.Daemon.Release {
	case "":
		return "older than " + m.Ours.Release
	case m.Ours.Release:
		return fmt.Sprintf("%s protocol %d", m.Daemon.Release, m.Daemon.protocol())
	}
	return m.Daemon.Release
}

// CityVersion is this build's version as a person reads it.
func (m *Mismatch) CityVersion() string {
	if m.Daemon.Release == m.Ours.Release {
		return fmt.Sprintf("%s protocol %d", m.Ours.Release, m.Ours.protocol())
	}
	return m.Ours.Release
}

// Newer is whether the daemon is ahead of this build, so that this is
// what needs restarting rather than the daemon.
func (m *Mismatch) Newer() bool {
	if m.Daemon.protocol() != m.Ours.protocol() {
		return m.Daemon.protocol() > m.Ours.protocol()
	}
	return release(m.Daemon, m.Ours) > 0
}

// Compare is what a client makes of the daemon it reached: nothing when
// they are the same release speaking the same protocol, and a Mismatch
// otherwise.
func Compare(daemon, ours Peer) error {
	if Same(daemon, ours) {
		return nil
	}
	return &Mismatch{Daemon: daemon, Ours: ours}
}

// release orders two releases, and is 0 when either is not a release
// number, such as a dev build.
func release(a, b Peer) int {
	x, okA := parts(a.Release)
	y, okB := parts(b.Release)
	if !okA || !okB {
		return 0
	}
	for i := range x {
		if x[i] != y[i] {
			if x[i] < y[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parts(r string) ([3]int, bool) {
	var out [3]int
	core, _, _ := strings.Cut(strings.TrimPrefix(r, "v"), "-")
	fields := strings.Split(core, ".")
	if len(fields) != 3 {
		return out, false
	}
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
