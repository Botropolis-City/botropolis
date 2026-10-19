package proto

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

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

// Compare is what a client makes of the daemon it reached: nothing when
// they match; a notice, once, when only the release differs, since the
// daemon an upgrade left running still speaks the same protocol; and a
// refusal when the protocol differs.
func Compare(daemon, ours Peer) (string, error) {
	if daemon.protocol() != ours.protocol() {
		switch {
		case daemon.Release == "":
			return "", fmt.Errorf("restart the daemon: it is older than %s", ours.Release)
		case daemon.protocol() > ours.protocol():
			return "", fmt.Errorf("this is %s and the daemon is %s: restart this", ours.Release, daemon.Release)
		default:
			return "", fmt.Errorf("restart the daemon: it is %s, this is %s", daemon.Release, ours.Release)
		}
	}
	switch {
	case daemon.Release == ours.Release:
		return "", nil
	case daemon.Release == "":
		return fmt.Sprintf("daemon is older than %s: restart it to update", ours.Release), nil
	case release(daemon, ours) > 0:
		return fmt.Sprintf("daemon is %s, this is %s: restart this to update", daemon.Release, ours.Release), nil
	default:
		return fmt.Sprintf("daemon is %s, this is %s: restart it to update", daemon.Release, ours.Release), nil
	}
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

var (
	noticeMu sync.Mutex
	listener func(string)
	heard    = map[string]bool{}
)

// OnNotice sets what hears a version notice, once per notice for this
// process, and returns what puts back the one before.
func OnNotice(f func(string)) func() {
	noticeMu.Lock()
	defer noticeMu.Unlock()
	was, wasHeard := listener, heard
	listener, heard = f, map[string]bool{}
	return func() {
		noticeMu.Lock()
		defer noticeMu.Unlock()
		listener, heard = was, wasHeard
	}
}

// Report passes a notice on, unless it has been heard already.
func Report(notice string) {
	noticeMu.Lock()
	f := listener
	fresh := notice != "" && !heard[notice]
	if fresh {
		heard[notice] = true
	}
	noticeMu.Unlock()
	if fresh && f != nil {
		f(notice)
	}
}
